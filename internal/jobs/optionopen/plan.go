package optionopen

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

//go:embed job_plan.toml
var planManifest []byte

// PlanDatasetID is the dataset of the key of the US plan job: one fixed key, subject planSubjectUS, whose run
// plans every stock.
const PlanDatasetID = "plan.option_open_item"

const planSubjectUS = "us"

// The reason codes of a plan run that could not plan a name (docs/design/option-open-gap-jp.md, Decision 10).
// Only fetch_failed is Retryable.
const (
	reasonNoConID     = "no_con_id"
	reasonNoClose     = "no_close"
	reasonNoChain     = "no_chain"
	reasonNoList      = "no_list"
	reasonNoStockList = "no_stock_list"
	reasonFetchFailed = "fetch_failed"
)

// maxFetchFailedRuns is the number of consecutive runs of one day, in one process, that end fetch_failed and are
// retried; the next one ends Permanent for the day with the reason rows still fetch_failed, so a name whose
// gateway answer never comes does not make the key retry until the window close.
const maxFetchFailedRuns = 4

// NotListedReader reads the marks that are not yet due.
type NotListedReader interface {
	NotListed(ctx context.Context, now time.Time) ([]contracts.OptionNotListed, error)
}

// PlanWriter writes the plan items.
type PlanWriter interface {
	PutPlan(ctx context.Context, items []contracts.OptionOpenItem) error
}

// ListRows reads the rows of the list job: the groups of a name and the reason of a run of a session.
type ListRows interface {
	Lists(ctx context.Context, underlyingID int64) ([]openquote.ListRecord, error)
	ListRuns(ctx context.Context, session contracts.Day) ([]openquote.ListRunRecord, error)
}

// PlanDeps are what the plan job reads and writes. Func nil means BuildPlan. Marks nil means no marks.
// Reasons nil writes no reason rows. History nil asks the gateway for no missing close. ListRows nil means a
// name with no stored chain is never taken for a name with no options. Log receives the fault lines (nil:
// discarded).
type PlanDeps struct {
	Stored
	Marks    NotListedReader
	Plans    PlanWriter
	Reasons  runview.ReasonWriter
	History  broker.HistoryClient
	ListRows ListRows
	Clock    clock.Clock
	Func     PlanFunc
	Log      func(line string)
}

var _ openquote.OptionOpenPlanStore // the full store satisfies PlanWriter

type planJob struct {
	m       manifest
	d       PlanDeps
	dataset string // the dataset of the one key
	subject string // the subject of the one key

	mu       sync.Mutex
	fetchDay contracts.Day // the day the counter below belongs to
	fetchRun int           // consecutive retried fetch_failed runs of that day
}

// NewPlan builds option_open_plan (US) from the embedded job_plan.toml. The job reads stored data only: it
// lists the contracts of each stock from the stored con_ids and never asks the broker.
func NewPlan(d PlanDeps) (contracts.Job, error) {
	if d.Instruments == nil || d.Bars == nil || d.Options == nil || d.Plans == nil || d.Clock == nil {
		return nil, errors.New("optionopen: instruments, bars, options, plan writer and clock are required")
	}
	m, err := parseManifest(planManifest)
	if err != nil {
		return nil, err
	}
	if d.Func == nil {
		d.Func = BuildPlan
	}
	return &planJob{m: m, d: d, dataset: PlanDatasetID, subject: planSubjectUS}, nil
}

func (j *planJob) ID() string { return j.m.ID }

// Schedule is the slot as plain values.
func (j *planJob) Schedule() (string, string, string, time.Duration) { return j.m.schedule() }

// Window is the window rule of the manifest.
func (j *planJob) Window() (string, time.Duration, time.Duration) { return j.m.window() }

// RunBound is the longest time one run may take.
func (j *planJob) RunBound() time.Duration { return j.m.runBound }

// RestartSensitive is false: a restart only repeats a plan that is written idempotently.
func (j *planJob) RestartSensitive() bool { return j.m.Schedule.RestartSensitive }

// Plan returns the one fixed key of the job and the hash of day, the manifest and the symbols. A failed read of the
// stock list is an error and an empty watch list gives no key: both end the plan without a key.
func (j *planJob) Plan(ctx context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	t, err := dayTime(day)
	if err != nil {
		return nil, "", err
	}
	stocks, _, err := j.d.stocks(ctx, t)
	if err != nil {
		return nil, "", err
	}
	if len(stocks) == 0 {
		return nil, "", nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n%+v", day, j.m.ID, j.m.params)
	for _, s := range stocks {
		b.WriteString("\n" + s.Symbol)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return []contracts.ExpectedKey{{DatasetID: j.dataset, SubjectKey: j.subject}}, hex.EncodeToString(sum[:]), nil
}

// nameFail is a stock the run could not plan. A code is a reason row; an empty code is a name the gateway proved
// has nothing (NoValue), which is no gap and no row.
type nameFail struct {
	symbol string
	code   string
	err    contracts.TypedError
}

// Run plans every stock whatever the keys say, and ends as Decision 10 says: no error when an item was stored,
// Retryable when every failure is fetch_failed (up to maxFetchFailedRuns runs of the day), else Permanent. Each
// Retryable end writes one fault line.
func (j *planJob) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	res := j.run(ctx, day, keys)
	if res.Err != nil && res.Err.Kind() == contracts.Retryable {
		if j.d.Log != nil {
			j.d.Log(fmt.Sprintf("fault stage=%s day=%s key=%s reason=%s", j.m.ID, day, j.subject, res.Err.Error()))
		}
	}
	return res
}

func (j *planJob) run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	t, err := dayTime(day)
	if err != nil {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "bad day")}
	}
	for _, k := range keys {
		if k.DatasetID != j.dataset || k.SubjectKey != j.subject {
			return contracts.JobResult{Err: newErr(contracts.Permanent, "unknown key %s/%s", k.DatasetID, k.SubjectKey)}
		}
	}
	start := j.d.Clock.Now()
	stocks, pos, err := j.d.stocks(ctx, t)
	if err != nil {
		return contracts.JobResult{Err: asTyped(err, contracts.Retryable)}
	}
	marks, err := j.marked(ctx, start)
	if err != nil {
		return contracts.JobResult{Err: asTyped(err, contracts.Retryable)}
	}
	names, fails, failed := j.nameInputs(ctx, t, start, stocks, pos, marks)
	plan := j.d.Func(t, j.m.params, names)
	var res contracts.JobResult
	for _, s := range stocks {
		if failed[s.Symbol] {
			continue
		}
		items, ok := j.itemsOf(day, s.Symbol, start, plan)
		if !ok {
			fails = append(fails, nameFail{s.Symbol, reasonNoStockList, newErr(contracts.Permanent, "%s is not on the stock list", s.Symbol)})
			continue
		}
		if err := j.d.Plans.PutPlan(ctx, items); err != nil {
			res.Err = asTyped(err, contracts.Retryable)
			return res
		}
		res.Returned += len(items)
		for _, it := range items {
			if !it.Deferred {
				res.Usable++
			}
		}
	}
	if err := j.writeReasons(ctx, day, fails); err != nil {
		res.Err = asTyped(err, contracts.Retryable)
		return res
	}
	if res.Returned > 0 {
		j.endFetchRuns(day, false)
		return res
	}
	res.Err = j.endOf(day, fails)
	return res
}

// nameInputs reads the input of every stock inside the budget: the names that have one, the failures, and the
// symbols that failed.
func (j *planJob) nameInputs(ctx context.Context, t, start time.Time, stocks []stock, pos []store.Position,
	marks map[string]bool) ([]NameInput, []nameFail, map[string]bool) {
	budget := time.Duration(j.m.BudgetMinutes) * time.Minute
	var names []NameInput
	var fails []nameFail
	failed := map[string]bool{}
	for _, s := range stocks {
		if j.d.Clock.Now().Sub(start) >= budget {
			err := newErr(contracts.Retryable, "plan budget of %d minutes ended before %s", j.m.BudgetMinutes, s.Symbol)
			fails = append(fails, nameFail{s.Symbol, reasonFetchFailed, err})
			failed[s.Symbol] = true
			continue
		}
		n, nf := j.nameInput(ctx, t, s, pos, marks)
		if nf != nil {
			fails = append(fails, *nf)
			failed[s.Symbol] = true
			continue
		}
		names = append(names, n)
	}
	return names, fails, failed
}

// writeReasons replaces the reason rows of the day with one row per failed name; no failure clears them.
func (j *planJob) writeReasons(ctx context.Context, day contracts.Day, fails []nameFail) error {
	if j.d.Reasons == nil {
		return nil
	}
	now := j.d.Clock.Now().UTC()
	rows := []runview.Reason{}
	for _, f := range fails {
		if f.code == "" {
			continue
		}
		rows = append(rows, runview.Reason{Day: day, Dataset: j.dataset, Subject: j.subject, Scope: f.symbol, Code: f.code,
			Detail: f.err.Error(), At: now})
	}
	return j.d.Reasons.ReplaceReasons(ctx, day, j.dataset, j.subject, rows)
}

// endOf is the end of a run that stored no item.
func (j *planJob) endOf(day contracts.Day, fails []nameFail) contracts.TypedError {
	var first *nameFail
	allFetch := true
	for i := range fails {
		if fails[i].code == "" {
			continue
		}
		if first == nil || (fails[i].code != reasonFetchFailed && first.code == reasonFetchFailed) {
			first = &fails[i]
		}
		if fails[i].code != reasonFetchFailed {
			allFetch = false
		}
	}
	if first == nil {
		j.endFetchRuns(day, false)
		return newErr(contracts.NoValue, "no name of the plan has options")
	}
	if !allFetch {
		j.endFetchRuns(day, false)
		return newErr(contracts.Permanent, "%s: %s", first.code, first.err.Error())
	}
	if j.endFetchRuns(day, true) {
		return newErr(contracts.Permanent, "%s: %s (%d runs of the day)", first.code, first.err.Error(), maxFetchFailedRuns)
	}
	return newErr(contracts.Retryable, "%s: %s", first.code, first.err.Error())
}

// endFetchRuns counts a run of day that ended fetch_failed (or resets the count of any other end) and says whether
// maxFetchFailedRuns retried runs of the day went before it.
func (j *planJob) endFetchRuns(day contracts.Day, fetchFailed bool) (exhausted bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.fetchDay != day || !fetchFailed {
		j.fetchDay, j.fetchRun = day, 0
	}
	if !fetchFailed {
		return false
	}
	if j.fetchRun >= maxFetchFailedRuns {
		return true
	}
	j.fetchRun++
	return false
}

func (j *planJob) marked(ctx context.Context, now time.Time) (map[string]bool, error) {
	out := map[string]bool{}
	if j.d.Marks == nil {
		return out, nil
	}
	ms, err := j.d.Marks.NotListed(ctx, now)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		out[fmt.Sprintf("%d|%s", m.UnderlyingID, markKey(m))] = true
	}
	return out, nil
}

// nameInput reads the stored close, the stored contracts, the sigma and the held contracts of one stock.
func (j *planJob) nameInput(ctx context.Context, day time.Time, s stock, pos []store.Position, marks map[string]bool) (NameInput, *nameFail) {
	fetch := func(err error) (NameInput, *nameFail) {
		return NameInput{}, &nameFail{s.Symbol, reasonFetchFailed, asTyped(err, contracts.Retryable)}
	}
	fail := func(code string, kind contracts.ErrorKind, format string, a ...any) (NameInput, *nameFail) {
		return NameInput{}, &nameFail{s.Symbol, code, newErr(kind, format, a...)}
	}
	if s.ConID == 0 {
		return fail(reasonNoConID, contracts.Permanent, "no con_id of %s", s.Symbol)
	}
	id, err := j.d.underlyingID(ctx, s)
	if err != nil {
		return fetch(err)
	}
	spot, ok, err := j.d.lastClose(ctx, id, day.AddDate(0, 0, -1))
	if err != nil {
		return fetch(err)
	}
	if !ok {
		if spot, ok, err = fetchClose(ctx, j.d.History, s); err != nil {
			return fetch(err)
		}
		if !ok {
			return fail(reasonNoClose, contracts.Permanent, "no stored close of %s", s.Symbol)
		}
	}
	refs, err := j.d.Options.ListOptions(ctx, id)
	if err != nil {
		return fetch(err)
	}
	if len(refs) == 0 {
		none, err := j.listedNothing(ctx, id, contracts.Day(day.Format(time.DateOnly)))
		if err != nil {
			return fetch(err)
		}
		if none {
			return NameInput{}, &nameFail{symbol: s.Symbol, err: newErr(contracts.NoValue, "%s has no options in the window", s.Symbol)}
		}
		return fail(reasonNoChain, contracts.Permanent, "no resolved chain of %s", s.Symbol)
	}
	n := NameInput{UnderlyingID: id, Symbol: s.Symbol, Spot: spot, HasPosition: hasPosition(pos, s.Symbol)}
	heldRef, _ := heldRefs(pos, s.Symbol, refs)
	for _, r := range heldRef {
		if c, ok := contractOf(r); ok {
			n.Held = append(n.Held, c)
		}
	}
	for _, r := range refs {
		if marks[fmt.Sprintf("%d|%s", id, refKey(r.Expiry, r.Strike, r.Right, r.TradingClass))] {
			continue
		}
		if c, ok := contractOf(r); ok {
			n.Candidates = append(n.Candidates, c)
		}
	}
	if n.PriorCloseSigma, n.LastKnownSigma, err = j.d.sigmas(ctx, id, refs, spot, day); err != nil {
		return fetch(err)
	}
	return n, nil
}

func contractOf(r openquote.OptionRef) (Contract, bool) {
	e, err := dayTime(r.Expiry)
	if err != nil || (r.Right != string(Put) && r.Right != string(Call)) {
		return Contract{}, false
	}
	return Contract{InstrumentID: r.InstrumentID, Expiry: e, Strike: strikeValue(r.Strike), Right: Right(r.Right)}, true
}

// itemsOf turns the items of one stock of the plan into rows; ok is false when the stock is not in the plan.
func (j *planJob) itemsOf(day contracts.Day, symbol string, now time.Time, plan Plan) ([]contracts.OptionOpenItem, bool) {
	var ns *NameSigma
	for i := range plan.Names {
		if plan.Names[i].Symbol == symbol {
			ns = &plan.Names[i]
		}
	}
	if ns == nil {
		return nil, false
	}
	d, _ := dayTime(day)
	planID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(day)+"|"+j.m.Slot+"|"+symbol)).String()
	var out []contracts.OptionOpenItem
	for _, it := range plan.Items {
		if it.Symbol != symbol {
			continue
		}
		item := contracts.OptionOpenItem{
			Deferred: it.Deferred, InstrumentID: int(it.Contract.InstrumentID),
			PlanID: planID, PlannedAt: now.UTC(), PlannedSpot: ns.Spot, Rank: it.Rank,
			SessionDay: sessionDate(d), Sigma: ns.Sigma,
			Slot: j.m.Slot, UnderlyingID: int(ns.UnderlyingID),
		}
		setEnum(&item.Class, it.Class)
		setEnum(&item.SigmaSource, ns.SigmaSource)
		out = append(out, item)
	}
	return out, true
}

// setEnum sets a generated string enum field from its wire value; the generic parameter is inferred from
// the field, so this package names no generated type.
func setEnum[T, S ~string](dst *T, v S) { *dst = T(v) }

// listedNothing says whether the list job proved that the name has no option in the window: a group of the session
// that is listed with Count 0 and none with a contract, or the list reason no_expiry_in_window.
func (j *planJob) listedNothing(ctx context.Context, id int64, session contracts.Day) (bool, error) {
	if j.d.ListRows == nil {
		return false, nil
	}
	groups, err := j.d.ListRows.Lists(ctx, id)
	if err != nil {
		return false, err
	}
	zero := false
	for _, g := range groups {
		if g.ListedSession != session || g.Outcome != "listed" {
			continue
		}
		if g.Count > 0 {
			return false, nil
		}
		zero = true
	}
	if zero {
		return true, nil
	}
	runs, err := j.d.ListRows.ListRuns(ctx, session)
	if err != nil {
		return false, err
	}
	for _, r := range runs {
		if r.UnderlyingID == id && r.Reason == "no_expiry_in_window" {
			return true, nil
		}
	}
	return false, nil
}

// fetchClose asks the gateway once for the last daily bar of the stock; ok is false when it holds no close. A nil
// client has nothing to ask: no close.
func fetchClose(ctx context.Context, h broker.HistoryClient, s stock) (float64, bool, error) {
	if h == nil {
		return 0, false, nil
	}
	bars, err := h.Bars(ctx, broker.BarQuery{Contract: broker.Contract{SecType: "STK", Symbol: s.Symbol, Exchange: s.Exchange,
		Currency: s.Currency, ConID: s.ConID}, Kind: broker.BarTrades, DurationDays: 5})
	if err != nil {
		return 0, false, err
	}
	for i := len(bars) - 1; i >= 0; i-- {
		if v, err := strconv.ParseFloat(bars[i].Close, 64); err == nil && v > 0 {
			return v, true, nil
		}
	}
	return 0, false, nil
}
