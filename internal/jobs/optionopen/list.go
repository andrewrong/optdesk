package optionopen

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

//go:embed job_list_preopen.toml
var listManifest []byte

// ListDatasetID is the dataset of the one key of option_list_preopen. No other job plans it, so no other job's
// window rule applies to it.
const ListDatasetID = "master.option_list"

// ListSubject is the subject of that key.
const ListSubject = "preopen"

// targetDTE is the days to expiry an expiry is ranked nearest to (option-trader plan_v4.toml).
const targetDTE = 30

// listPause is the wait after a timeout: the details lane of pyside holds starts for 30 s after one.
const listPause = 30 * time.Second

// ListDeps are what option_list_preopen uses. Lister is the broker client: it also answers ListChains (the
// constructor checks it). Log receives the summary line and list_unserved (nil: discarded). Pause waits d after a
// timeout (nil: a real timer).
type ListDeps struct {
	Stored
	Lister    broker.ContractLister
	ListStore openquote.OptionListStore
	Clock     clock.Clock
	Log       func(string)
	Pause     func(ctx context.Context, d time.Duration) error
}

// chainLister is the other call the job makes of the broker client.
type chainLister interface {
	ListChains(ctx context.Context, underlying broker.Contract) ([]broker.Chain, error)
}

type listTable struct {
	Version       int           `toml:"version"`
	ID            string        `toml:"id"`
	Zone          string        `toml:"zone"`
	BudgetMinutes int           `toml:"list_budget_minutes"`
	DTEMin        int           `toml:"dte_min"`
	DTEMax        int           `toml:"dte_max"`
	MinRatio      float64       `toml:"list_min_ratio"`
	MaxNew        int           `toml:"list_max_new"`
	StopTime      string        `toml:"stop_time"`
	Window        windowTable   `toml:"window"`
	Schedule      scheduleTable `toml:"schedule"`

	loc      *time.Location
	offset   time.Duration
	runBound time.Duration
	stopH    int
	stopM    int
}

// parseList reads the manifest of option_list_preopen; it shares nothing with parseManifest but the table types.
func parseList(data []byte) (listTable, error) {
	var m listTable
	if err := toml.Unmarshal(data, &m); err != nil {
		return listTable{}, fmt.Errorf("optionopen: list manifest: %w", err)
	}
	if m.ID == "" || m.BudgetMinutes <= 0 || m.MaxNew <= 0 {
		return listTable{}, errors.New("optionopen: list manifest needs an id, list_budget_minutes and list_max_new")
	}
	var err error
	if m.loc, err = time.LoadLocation(m.Zone); err != nil {
		return listTable{}, errors.New("optionopen: list manifest: zone is unknown")
	}
	t, err := time.Parse("15:04", m.StopTime)
	if err != nil {
		return listTable{}, errors.New("optionopen: list manifest: stop_time is not HH:MM")
	}
	m.stopH, m.stopM = t.Hour(), t.Minute()
	if m.DTEMin < 0 || m.DTEMax < m.DTEMin || m.MinRatio < 0 || m.MinRatio > 1 {
		return listTable{}, errors.New("optionopen: list manifest: dte_min, dte_max or list_min_ratio is out of range")
	}
	if m.Window.Rule != "after_close" || m.Window.LatenessHours < 0 || m.Window.Calendar == "" {
		return listTable{}, errors.New("optionopen: list manifest: window must be after_close with a calendar")
	}
	if (m.Schedule.Anchor != "session_open" && m.Schedule.Anchor != "session_close") || m.Schedule.Calendar == "" {
		return listTable{}, errors.New("optionopen: list manifest: schedule needs a calendar and an anchor")
	}
	if m.offset, err = time.ParseDuration(m.Schedule.Offset); err != nil {
		return listTable{}, errors.New("optionopen: list manifest: schedule.offset is not a duration")
	}
	if m.runBound, err = time.ParseDuration(m.Schedule.RunBound); err != nil || m.runBound <= 0 {
		return listTable{}, errors.New("optionopen: list manifest: schedule.run_bound is not a duration")
	}
	return m, nil
}

type listJob struct {
	m      listTable
	d      ListDeps
	chains chainLister
}

// NewListPreopen builds option_list_preopen from the embedded job_list_preopen.toml. It lists the real contracts
// of every watched US stock, one call per underlying, class and expiry, and stores them in master.option_list.
func NewListPreopen(d ListDeps) (contracts.Job, error) { return newListPreopen(listManifest, d) }

func newListPreopen(data []byte, d ListDeps) (contracts.Job, error) {
	if d.Lister == nil || d.ListStore == nil || d.Instruments == nil || d.Options == nil || d.Clock == nil {
		return nil, errors.New("optionopen: lister, list store, instruments, options and clock are required")
	}
	cl, ok := d.Lister.(chainLister)
	if !ok {
		return nil, errors.New("optionopen: the lister must also list chains")
	}
	m, err := parseList(data)
	if err != nil {
		return nil, err
	}
	return &listJob{m: m, d: d, chains: cl}, nil
}

func (j *listJob) ID() string { return j.m.ID }

// Schedule is the slot as plain values.
func (j *listJob) Schedule() (string, string, string, time.Duration) {
	return j.m.Schedule.Calendar, j.m.Schedule.Anchor, j.m.Zone, j.m.offset
}

// Window is the window rule of the manifest: after_close with no lateness.
func (j *listJob) Window() (string, time.Duration, time.Duration) {
	return j.m.Window.Calendar, 0, time.Duration(j.m.Window.LatenessHours) * time.Hour
}

// RunBound is the longest time one run may take.
func (j *listJob) RunBound() time.Duration { return j.m.runBound }

// RestartSensitive is false: a restart resumes from the stored rows.
func (j *listJob) RestartSensitive() bool { return j.m.Schedule.RestartSensitive }

// Plan returns the one key of the job whatever the names.
func (j *listJob) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	if _, err := dayTime(day); err != nil {
		return nil, "", err
	}
	key := contracts.ExpectedKey{DatasetID: ListDatasetID, SubjectKey: ListSubject}
	return []contracts.ExpectedKey{key}, hashOf(fmt.Sprintf("%s\n%s", day, j.m.ID)), nil
}

// ChooseExpiries keeps the expiries whose calendar days from day lie in [dteMin, dteMax], inclusive, once each,
// nearest to 30 days first and the earlier expiry on a tie. An expiry that is not a day is dropped.
func ChooseExpiries(day contracts.Day, expiries []contracts.Day, dteMin, dteMax int) []contracts.Day {
	d0, err := dayTime(day)
	if err != nil {
		return nil
	}
	type pick struct {
		e   contracts.Day
		dte int
	}
	seen := map[contracts.Day]bool{}
	var kept []pick
	for _, e := range expiries {
		t, err := dayTime(e)
		if err != nil || seen[e] {
			continue
		}
		seen[e] = true
		if dte := int(math.Round(t.Sub(d0).Hours() / 24)); dte >= dteMin && dte <= dteMax {
			kept = append(kept, pick{e, dte})
		}
	}
	sort.Slice(kept, func(a, b int) bool {
		da, db := absInt(kept[a].dte-targetDTE), absInt(kept[b].dte-targetDTE)
		if da != db {
			return da < db
		}
		return kept[a].e < kept[b].e
	})
	out := make([]contracts.Day, len(kept))
	for i, k := range kept {
		out[i] = k.e
	}
	return out
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// classify says what a list is: listed (both rights, at least list_min_ratio of the stored contracts of the group),
// short, or error (empty, or one right only). Contracts of another class or multiplier are dropped, and a strike
// spelled two ways is one contract.
func (r *listRun) classify(n *listName, g listGroup, got []broker.Contract) (string, []broker.Contract) {
	mult := n.mult
	if mult == "" {
		mult = modalMultiplier(got, g.class)
	}
	seen := map[string]bool{}
	var out []broker.Contract
	rights := map[string]bool{}
	for _, c := range got {
		k := c.Right + "|" + strikeKey(c.Strike)
		if c.TradingClass != g.class || c.ConID <= 0 || c.Strike == "" || (c.Right != "C" && c.Right != "P") ||
			c.Multiplier != mult || seen[k] {
			continue
		}
		seen[k], rights[c.Right] = true, true
		out = append(out, c)
	}
	if !rights["C"] || !rights["P"] {
		return "error", nil
	}
	if stored := n.refCount[g.key()]; stored > 0 && float64(len(out)) < r.j.m.MinRatio*float64(stored) {
		return "short", nil
	}
	return "listed", out
}

func modalMultiplier(got []broker.Contract, class string) string {
	count := map[string]int{}
	best := ""
	for _, c := range got {
		if c.TradingClass == class {
			if count[c.Multiplier]++; count[c.Multiplier] > count[best] || (count[c.Multiplier] == count[best] && c.Multiplier < best) {
				best = c.Multiplier
			}
		}
	}
	return best
}

// write stores the contracts of a list the store does not hold, matched by strikeKey, up to list_max_new for the
// run; the strike text is the broker's. capped is true when the cap stopped it.
func (r *listRun) write(ctx context.Context, n *listName, g listGroup, list []broker.Contract) (capped bool, terr contracts.TypedError) {
	d := r.j.d
	yyyymmdd := strings.ReplaceAll(string(g.expiry), "-", "")
	for _, c := range list {
		key := refKey(g.expiry, c.Strike, c.Right, g.class)
		if n.refSet[key] {
			continue
		}
		if r.written >= r.j.m.MaxNew {
			return true, nil
		}
		cur := c.Currency
		if cur == "" {
			cur = n.st.Currency
		}
		req := broker.Contract{Symbol: n.st.Symbol, Expiry: yyyymmdd, Right: c.Right, Strike: c.Strike}
		spec := store.InstrumentSpec{Type: "option", Name: strings.Join([]string{req.Symbol, req.Expiry, req.Right, req.Strike}, " "),
			Currency: cur, CalendarID: "XNYS", Status: "active", PriceIncrement: "0.01", LotSize: c.Multiplier}
		var extra []store.Identifier
		if osi, ok := osiOf(req); ok {
			extra = append(extra, store.Identifier{Scheme: "osi", Value: osi})
		}
		iid, err := d.Instruments.ResolveInstrument(ctx, store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(c.ConID, 10)}, spec, extra)
		if err != nil {
			return false, asTyped(err, contracts.Permanent)
		}
		exch := c.Exchange
		if exch == "" {
			exch = n.st.Exchange
		}
		err = d.Options.PutOption(ctx, openquote.OptionRef{UnderlyingID: n.id, Currency: cur, Exchange: exch, TradingClass: g.class,
			Multiplier: c.Multiplier, Expiry: g.expiry, Strike: c.Strike, Right: c.Right, InstrumentID: iid})
		if err != nil {
			return false, asTyped(err, contracts.Retryable)
		}
		n.refSet[key] = true
		n.refCount[g.key()]++
		r.written++
	}
	return false, nil
}

func (r *listRun) mainReason() string {
	if r.stop != "" {
		return r.stop
	}
	for _, n := range r.names {
		if n.reason != reasonNone {
			return n.reason
		}
	}
	return "no_names"
}

// summary logs the one line of the run: the reasons by count and the names that did not end complete.
func (r *listRun) summary(runErr contracts.TypedError) {
	counts := map[string]int{}
	var open []string
	for _, n := range r.names {
		counts[n.reason]++
		if n.reason != reasonNone {
			open = append(open, n.st.Symbol+"="+n.reason)
		}
	}
	reasons := make([]string, 0, len(counts))
	for k, v := range counts {
		reasons = append(reasons, fmt.Sprintf("%s:%d", k, v))
	}
	sort.Strings(reasons)
	stop := r.stop
	if stop == "" {
		stop = "none"
	}
	r.log(fmt.Sprintf("%s day=%s names=%d groups_listed=%d new_contracts=%d stop=%s reasons=%s open=%s run_err=%s",
		r.j.m.ID, r.day, len(r.names), r.groups, r.written, stop, strings.Join(reasons, ","), strings.Join(open, ","), runErrText(runErr)))
}
