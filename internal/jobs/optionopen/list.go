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
	"github.com/andrewrong/optdesk/internal/store/runview"
)

//go:embed job_list_preopen.toml
var listManifest []byte

// ListDatasetID is the dataset of the one key of option_list_preopen (the US toml's `dataset`). No other job plans
// it, so no other job's window rule applies to it.
const ListDatasetID = "master.option_list"

// ListSubject is the subject of that key (the US toml's `subject`).
const ListSubject = "preopen"

// The modes of the toml key `mode`; absent means preopen.
const (
	modePreopen = "preopen"
	modeZone    = "zone"
)

// maxSpotAge is the oldest EventTs a pre-open spot of the zone mode may have.
const maxSpotAge = 96 * time.Hour

// targetDTE is the days to expiry an expiry is ranked nearest to (option-trader plan_v4.toml).
const targetDTE = 30

// listPause is the wait after a timeout: the details lane of pyside holds starts for 30 s after one.
const listPause = 30 * time.Second

// Snapshotter is the read-only part of broker.Broker the zone mode uses: Snapshot has the signature of
// broker.Broker.Snapshot.
type Snapshotter interface {
	Snapshot(ctx context.Context, kind broker.Kind, cs []broker.Contract) ([]broker.Quote, error)
}

// ListDeps are what option_list_preopen uses. Lister is the broker client: it also answers ListChains (the
// constructor checks it). Bars (in Stored) gives the prior close. History asks the gateway for the last daily bar
// of a name with no stored close (nil: that name has no spot and the run is Retryable). Spot is required by the
// zone mode only. ReasonWriter receives held_stale, outside_range and no_spot (nil: no reasons). Log receives the
// summary line and list_unserved (nil: discarded). Pause waits d after a timeout (nil: a real timer).
type ListDeps struct {
	Stored
	Lister       broker.ContractLister
	ListStore    openquote.OptionListStore
	Clock        clock.Clock
	History      broker.HistoryClient
	Spot         Snapshotter
	ReasonWriter runview.ReasonWriter
	Log          func(string)
	Pause        func(ctx context.Context, d time.Duration) error
}

// listRules is what a market adds to the job: which names it lists and the class, exchange and multiplier of
// their contracts. Exchange and Multiplier empty mean the stock's exchange and the chain's multiplier.
type listRules struct {
	Names      func(ctx context.Context, s Stored, day time.Time) ([]stock, []store.Position, error)
	Class      func(st stock) string
	Exchange   string
	Multiplier string
}

// usRules lists the US stocks of the watch list and the held ones, class = symbol.
func usRules() listRules {
	return listRules{
		Names: func(ctx context.Context, s Stored, day time.Time) ([]stock, []store.Position, error) {
			return s.stocks(ctx, day)
		},
		Class: func(st stock) string { return st.Symbol },
	}
}

// windowTableDTE is one [[window_dte]] table.
type windowTableDTE struct {
	MinDTE int `toml:"min_dte"`
	MaxDTE int `toml:"max_dte"`
}

// chainLister is the other call the job makes of the broker client.
type chainLister interface {
	ListChains(ctx context.Context, underlying broker.Contract) ([]broker.Chain, error)
}

type listTable struct {
	Version       int              `toml:"version"`
	ID            string           `toml:"id"`
	Zone          string           `toml:"zone"`
	BudgetMinutes int              `toml:"list_budget_minutes"`
	TTLDays       int              `toml:"list_ttl_days"`
	BandK         float64          `toml:"band_k"`
	BandFloorPct  float64          `toml:"band_floor_pct"`
	MinSide       int              `toml:"min_strikes_side"`
	ZoneNear      float64          `toml:"zone_near"`
	ZoneFar       float64          `toml:"zone_far"`
	Dataset       string           `toml:"dataset"`
	Subject       string           `toml:"subject"`
	Mode          string           `toml:"mode"`
	Windows       []windowTableDTE `toml:"window_dte"`
	MinRatio      float64          `toml:"list_min_ratio"`
	MaxNew        int              `toml:"list_max_new"`
	StopTime      string           `toml:"stop_time"`
	Window        windowTable      `toml:"window"`
	Schedule      scheduleTable    `toml:"schedule"`

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
	if m.MinRatio < 0 || m.MinRatio > 1 {
		return listTable{}, errors.New("optionopen: list manifest: list_min_ratio is out of range")
	}
	if err := m.checkBand(); err != nil {
		return listTable{}, err
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

// checkBand validates the window list, the freshness and band numbers and the mode, and fills the defaults.
func (m *listTable) checkBand() error {
	if len(m.Windows) == 0 {
		return errors.New("optionopen: list manifest needs a [[window_dte]] table")
	}
	for _, w := range m.Windows {
		if w.MinDTE < 0 || w.MaxDTE < w.MinDTE {
			return errors.New("optionopen: list manifest: window_dte min_dte or max_dte is out of range")
		}
	}
	if m.TTLDays == 0 {
		m.TTLDays = 1
	}
	if m.TTLDays < 0 || m.BandK <= 0 || m.BandFloorPct < 0 || m.MinSide < 0 || m.ZoneNear < 0 || m.ZoneFar <= m.ZoneNear {
		return errors.New("optionopen: list manifest: list_ttl_days, band_k, band_floor_pct, min_strikes_side, zone_near or zone_far is out of range")
	}
	if m.Dataset == "" {
		m.Dataset = ListDatasetID
	}
	if m.Subject == "" {
		m.Subject = ListSubject
	}
	switch m.Mode {
	case "":
		m.Mode = modePreopen
	case modePreopen, modeZone:
	default:
		return errors.New("optionopen: list manifest: mode is neither preopen nor zone")
	}
	return nil
}

type listJob struct {
	m      listTable
	d      ListDeps
	chains chainLister
	rules  listRules
}

// NewListPreopen builds option_list_preopen from the embedded job_list_preopen.toml. It lists the real contracts
// of every watched US stock, one call per underlying, class and expiry, and stores them in master.option_list.
func NewListPreopen(d ListDeps) (contracts.Job, error) {
	return newListPreopen(listManifest, usRules(), d)
}

func newListPreopen(manifest []byte, rules listRules, d ListDeps) (contracts.Job, error) {
	if d.Lister == nil || d.ListStore == nil || d.Instruments == nil || d.Options == nil || d.Clock == nil || d.Bars == nil {
		return nil, errors.New("optionopen: lister, list store, instruments, options, bars and clock are required")
	}
	if rules.Names == nil || rules.Class == nil {
		return nil, errors.New("optionopen: the list rules need a names function and a class function")
	}
	cl, ok := d.Lister.(chainLister)
	if !ok {
		return nil, errors.New("optionopen: the lister must also list chains")
	}
	m, err := parseList(manifest)
	if err != nil {
		return nil, err
	}
	if m.Mode == modeZone && d.Spot == nil {
		return nil, errors.New("optionopen: the zone mode needs a spot reader")
	}
	return &listJob{m: m, d: d, chains: cl, rules: rules}, nil
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

// Plan returns the one key of the job (dataset and subject of the manifest) whatever the names.
func (j *listJob) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	if _, err := dayTime(day); err != nil {
		return nil, "", err
	}
	key := contracts.ExpectedKey{DatasetID: j.m.Dataset, SubjectKey: j.m.Subject}
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
	if len(got) == 0 {
		// An empty answer for an expiry the chain lists is a listed group of no contract, unless contracts are stored.
		if n.refCount[g.key()] > 0 {
			return "short", nil
		}
		return "listed", nil
	}
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
			exch = r.exchange(n)
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
	r.log(fmt.Sprintf("%s day=%s names=%d groups_listed=%d new_contracts=%d stop=%s reasons=%s open=%s past_stop_time=%t run_err=%s",
		r.j.m.ID, r.day, len(r.names), r.groups, r.written, stop, strings.Join(reasons, ","), strings.Join(open, ","), !r.start.Before(r.stopAt), runErrText(runErr)))
}

// BandParams are the manifest values of ListBand.
type BandParams struct {
	K        float64 // band_k
	FloorPct float64 // band_floor_pct
	MinSide  int     // min_strikes_side
}

// ListBand is the strike range of one expiry that is listed: from spot*exp(-K*s) to spot*exp(+K*s), s =
// sigma*sqrt(dte/365), at least FloorPct of spot on each side, and wide enough to hold MinSide of the listed
// strikes on each side of the spot (the nearest ones when fewer lie inside). Candidates are the listed strikes
// in [lo, hi].
func ListBand(spot, sigma float64, dte int, strikes []float64, p BandParams) (lo, hi float64) {
	s := sigma * math.Sqrt(float64(max(dte, 1))/365)
	lo, hi = spot*math.Exp(-p.K*s), spot*math.Exp(p.K*s)
	lo, hi = math.Min(lo, spot*(1-p.FloorPct)), math.Max(hi, spot*(1+p.FloorPct))
	var below, above []float64
	for _, k := range strikes {
		switch {
		case k < spot:
			below = append(below, k)
		case k > spot:
			above = append(above, k)
		}
	}
	sort.Float64s(below)
	sort.Float64s(above)
	if p.MinSide > 0 {
		if n := len(below); n > 0 {
			lo = math.Min(lo, below[max(n-p.MinSide, 0)])
		}
		if len(above) > 0 {
			hi = math.Max(hi, above[min(p.MinSide, len(above))-1])
		}
	}
	return lo, hi
}

// zoneInside says whether the target zone of an expiry from spot lies inside [lo, hi]: below the spot from
// spot*exp(-far*s) to spot*exp(-near*s), above it from spot*exp(+near*s) to spot*exp(+far*s).
func zoneInside(spot, sigma float64, dte int, near, far, lo, hi float64) bool {
	s := sigma * math.Sqrt(float64(max(dte, 1))/365)
	edges := []float64{spot * math.Exp(-far*s), spot * math.Exp(-near*s), spot * math.Exp(near*s), spot * math.Exp(far*s)}
	for _, e := range edges {
		if e < lo || e > hi {
			return false
		}
	}
	return true
}

func fmtStrike(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// daysTo is the calendar days from day t to the expiry.
func daysTo(t time.Time, expiry contracts.Day) int {
	e, err := dayTime(expiry)
	if err != nil {
		return 0
	}
	return int(math.Round(e.Sub(t).Hours() / 24))
}

// spotFrom is the price of a pre-open quote and whether it is usable: an EventTs within maxSpotAge of now (the
// receive time is never used) and a last, else a bid and ask midpoint, above 0.
func spotFrom(q broker.Quote, now time.Time) (float64, bool) {
	if q.EventTs == nil || q.ErrorCode != 0 || now.Sub(*q.EventTs) > maxSpotAge {
		return 0, false
	}
	switch {
	case q.Last != nil && *q.Last > 0:
		return *q.Last, true
	case q.Bid != nil && q.Ask != nil && *q.Bid > 0 && *q.Ask > 0:
		return (*q.Bid + *q.Ask) / 2, true
	}
	return 0, false
}

// ownIDs are the instrument ids of the names of this job's own run.
func (j *listJob) ownIDs(ctx context.Context, t time.Time) (map[int64]bool, error) {
	stocks, _, err := j.rules.Names(ctx, j.d.Stored, t)
	if err != nil {
		return nil, err
	}
	ids := map[int64]bool{}
	for _, st := range stocks {
		id, err := j.d.underlyingID(ctx, st)
		if err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, nil
}

func (j *listJob) listedOf(ctx context.Context, day contracts.Day, ids map[int64]bool) (bool, error) {
	rows, err := j.d.ListStore.ListRuns(ctx, day)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if ids[row.UnderlyingID] && row.Listed > 0 {
			return true, nil
		}
	}
	return false, nil
}

// Postcondition is true when some run row of the day, of a name of this job's own run, has listed above 0. The
// zone mode writes no run row: its key is present whenever it ran.
func (j *listJob) Postcondition(ctx context.Context, day, _, _ string) (bool, error) {
	if j.m.Mode == modeZone {
		return true, nil
	}
	t, err := dayTime(contracts.Day(day))
	if err != nil {
		return false, err
	}
	ids, err := j.ownIDs(ctx, t)
	if err != nil {
		return false, err
	}
	return j.listedOf(ctx, contracts.Day(day), ids)
}

func (r *listRun) ownListed(ctx context.Context) (bool, error) {
	ids := map[int64]bool{}
	for _, n := range r.names {
		ids[n.id] = true
	}
	return r.j.listedOf(ctx, r.day, ids)
}

// heldStale lists the groups of the held and priority names that are neither fresh nor answered in this run, as
// "SYMBOL expiry"; a name whose chains were not answered stands for its unknown groups as "SYMBOL".
func (r *listRun) heldStale() []string {
	var out []string
	for _, n := range r.names {
		if n.rank > 1 {
			continue
		}
		if n.expected == nil {
			if !n.chainsOK {
				out = append(out, n.st.Symbol)
			}
			continue
		}
		for _, e := range n.expected {
			if !n.lists[e] && !n.answered[e] {
				expiry, _, _ := strings.Cut(e, "|")
				out = append(out, n.st.Symbol+" "+expiry)
			}
		}
	}
	sort.Strings(out)
	return out
}

func (r *listRun) heldStaleReasons(stale []string) []runview.Reason {
	if len(stale) == 0 {
		return nil
	}
	return []runview.Reason{{Scope: r.j.m.ID, Code: codeHeldStale, Detail: strings.Join(stale, ",")}}
}

// writeReasons replaces the reasons of the key with rs (none clears an earlier set).
func (r *listRun) writeReasons(ctx context.Context, rs []runview.Reason) error {
	w := r.j.d.ReasonWriter
	if w == nil {
		return nil
	}
	at := r.j.d.Clock.Now().UTC()
	out := make([]runview.Reason, len(rs))
	for i, x := range rs {
		x.Day, x.Dataset, x.Subject, x.At = r.day, r.j.m.Dataset, r.j.m.Subject, at
		out[i] = x
	}
	return w.ReplaceReasons(context.WithoutCancel(ctx), r.day, r.j.m.Dataset, r.j.m.Subject, out)
}

func (r *listRun) exchange(n *listName) string {
	if e := r.j.rules.Exchange; e != "" {
		return e
	}
	return n.st.Exchange
}

// zoneNames loads the names of the zone run that have a stored group with a range inside the windows.
func (r *listRun) zoneNames(ctx context.Context, stocks []stock, pos []store.Position) contracts.TypedError {
	d := r.j.d
	for _, st := range stocks {
		id, err := d.underlyingID(ctx, st)
		if err != nil {
			return asTyped(err, contracts.Retryable)
		}
		rows, err := d.ListStore.Lists(ctx, id)
		if err != nil {
			return asTyped(err, contracts.Retryable)
		}
		n := &listName{st: st, id: id, rank: 2, mult: r.j.rules.Multiplier, answered: map[string]bool{}, recs: map[string]openquote.ListRecord{}}
		if hasPosition(pos, st.Symbol) {
			n.rank = 0
		} else if r.priority(st.Symbol) {
			n.rank = 1
		}
		for _, row := range rows {
			if row.BandLo != "" && row.BandHi != "" && row.Sigma > 0 && r.j.inWindow(daysTo(r.t, row.Expiry)) {
				n.recs[string(row.Expiry)+"|"+row.Class] = row
			}
		}
		if len(n.recs) > 0 {
			r.names = append(r.names, n)
		}
	}
	return nil
}

// outsideReasons is one outside_range reason for each name with a group the run did not relist.
func (r *listRun) outsideReasons(calls []listGroup) []runview.Reason {
	left := map[*listName][]string{}
	var order []*listName
	for _, g := range calls {
		if !r.relist[fmt.Sprintf("%d|%s", g.n.id, g.key())] {
			if left[g.n] == nil {
				order = append(order, g.n)
			}
			left[g.n] = append(left[g.n], string(g.expiry))
		}
	}
	var out []runview.Reason
	for _, n := range order {
		sort.Strings(left[n])
		out = append(out, runview.Reason{Scope: n.st.Symbol, Code: codeOutsideRange, Detail: strings.Join(left[n], ",")})
	}
	return out
}

// runZone is the mode zone: it reads a pre-open spot of each name that has a group with a stored range, tests the
// target zone of each expiry against that range and lists only the expiries whose zone is not inside it. It writes
// option_list groups and reason rows only, never option_list_run.
func (r *listRun) runZone(ctx context.Context) contracts.JobResult {
	res := func(terr contracts.TypedError) contracts.JobResult {
		return contracts.JobResult{Returned: r.groups, Usable: r.groups, Err: terr}
	}
	d := r.j.d
	stocks, pos, err := r.j.rules.Names(ctx, d.Stored, r.t)
	if err != nil {
		return res(asTyped(err, contracts.Retryable))
	}
	if terr := r.zoneNames(ctx, stocks, pos); terr != nil {
		return res(terr)
	}
	var reasons []runview.Reason
	quotes := r.snapshot(ctx)
	var calls []listGroup
	m := r.j.m
	for i, n := range r.names {
		spot, ok := 0.0, false
		if quotes != nil {
			spot, ok = spotFrom(quotes[i], d.Clock.Now())
		}
		if !ok {
			n.spotSt = spotNone
			reasons = append(reasons, runview.Reason{Scope: n.st.Symbol, Code: codeNoSpot, Detail: "no usable pre-open quote with an EventTs"})
			continue
		}
		n.spot, n.spotSt = spot, spotKnown
		for _, rec := range n.recs {
			if !zoneInside(spot, rec.Sigma, daysTo(r.t, rec.Expiry), m.ZoneNear, m.ZoneFar, strikeValue(rec.BandLo), strikeValue(rec.BandHi)) {
				calls = append(calls, listGroup{n: n, expiry: rec.Expiry, class: rec.Class, sigma: rec.Sigma, source: rec.SigmaSource})
			}
		}
	}
	sortGroups(calls)
	for _, g := range calls {
		if r.stop = r.halt(); r.stop != "" {
			break
		}
		if r.written >= m.MaxNew {
			r.stop = reasonMaxNew
			break
		}
		if terr := r.ask(ctx, g); terr != nil {
			return res(terr)
		}
		if r.stop != "" {
			break
		}
	}
	reasons = append(reasons, r.outsideReasons(calls)...)
	if r.abort != nil {
		return res(r.abort)
	}
	if err := r.writeReasons(ctx, reasons); err != nil {
		return res(asTyped(err, contracts.Retryable))
	}
	r.log(fmt.Sprintf("%s day=%s zone names=%d groups_listed=%d stop=%s reasons=%d", m.ID, r.day, len(r.names), r.groups, r.stop, len(reasons)))
	return res(nil)
}

// snapshot reads the underlyings of r.names in one call; nil when the call failed or did not answer each.
func (r *listRun) snapshot(ctx context.Context) []broker.Quote {
	if len(r.names) == 0 {
		return nil
	}
	cs := make([]broker.Contract, len(r.names))
	for i, n := range r.names {
		cs[i] = r.underlying(n)
	}
	qs, err := r.j.d.Spot.Snapshot(ctx, broker.KindSnapshot, cs)
	if err != nil || len(qs) != len(cs) {
		return nil
	}
	return qs
}

// inWindow says whether dte lies in one of the [[window_dte]] pairs.
func (j *listJob) inWindow(dte int) bool {
	for _, w := range j.m.Windows {
		if dte >= w.MinDTE && dte <= w.MaxDTE {
			return true
		}
	}
	return false
}
