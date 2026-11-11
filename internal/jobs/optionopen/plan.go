package optionopen

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

//go:embed job_plan.toml
var planManifest []byte

// PlanDatasetID is the dataset of the keys of the plan job: one key per stock.
const PlanDatasetID = "plan.option_open_item"

// NotListedReader reads the marks that are not yet due.
type NotListedReader interface {
	NotListed(ctx context.Context, now time.Time) ([]contracts.OptionNotListed, error)
}

// PlanWriter writes the plan items.
type PlanWriter interface {
	PutPlan(ctx context.Context, items []contracts.OptionOpenItem) error
}

// PlanDeps are what the plan job reads and writes. Func nil means BuildPlan. Marks nil means no marks.
type PlanDeps struct {
	Stored
	Marks NotListedReader
	Plans PlanWriter
	Clock clock.Clock
	Func  PlanFunc
}

var _ openquote.OptionOpenPlanStore // the full store satisfies PlanWriter

type planJob struct {
	m manifest
	d PlanDeps
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
	return &planJob{m: m, d: d}, nil
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

// Plan returns one key per stock and the hash of day, the manifest and the symbols.
func (j *planJob) Plan(ctx context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	t, err := dayTime(day)
	if err != nil {
		return nil, "", err
	}
	stocks, _, err := j.d.stocks(ctx, t)
	if err != nil {
		return nil, "", err
	}
	keys := make([]contracts.ExpectedKey, 0, len(stocks))
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n%+v", day, j.m.ID, j.m.params)
	for _, s := range stocks {
		keys = append(keys, contracts.ExpectedKey{DatasetID: PlanDatasetID, SubjectKey: s.Symbol})
		b.WriteString("\n" + s.Symbol)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return keys, hex.EncodeToString(sum[:]), nil
}

type typedError struct {
	kind contracts.ErrorKind
	err  error
}

func (e *typedError) Error() string             { return e.err.Error() }
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func newErr(kind contracts.ErrorKind, format string, a ...any) contracts.TypedError {
	return &typedError{kind: kind, err: fmt.Errorf(format, a...)}
}

func asTyped(err error, kind contracts.ErrorKind) contracts.TypedError {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te
	}
	return &typedError{kind: kind, err: err}
}

// skipped is a stock the run could not plan: why, and whether the run should be retried.
type skipped struct {
	err contracts.TypedError
}

// Run plans the stocks of keys. The cross-stock order and ceilings need every stock, so each call reads the
// stored inputs of all of them (reads only) inside the budget and writes the items of its own keys.
func (j *planJob) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	t, err := dayTime(day)
	if err != nil {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "bad day")}
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
	budget := time.Duration(j.m.BudgetMinutes) * time.Minute
	var names []NameInput
	why := map[string]skipped{}
	for _, s := range stocks {
		if j.d.Clock.Now().Sub(start) >= budget {
			why[s.Symbol] = skipped{newErr(contracts.Retryable, "plan budget of %d minutes ended before %s", j.m.BudgetMinutes, s.Symbol)}
			continue
		}
		n, serr := j.nameInput(ctx, t, s, pos, marks)
		if serr != nil {
			why[s.Symbol] = skipped{serr}
			continue
		}
		names = append(names, n)
	}
	plan := j.d.Func(t, j.m.params, names)
	var res contracts.JobResult
	var noValue contracts.TypedError
	for _, k := range keys {
		if s, bad := why[k.SubjectKey]; bad {
			if s.err.Kind() == contracts.Retryable {
				res.Err = s.err
			} else if noValue == nil {
				noValue = s.err
			}
			continue
		}
		items, ok := j.itemsOf(day, k.SubjectKey, start, plan)
		if !ok {
			if noValue == nil {
				noValue = newErr(contracts.NoValue, "%s is not on the stock list", k.SubjectKey)
			}
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
	if res.Err == nil && noValue != nil {
		res.Err = noValue
	}
	return res
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
func (j *planJob) nameInput(ctx context.Context, day time.Time, s stock, pos []store.Position, marks map[string]bool) (NameInput, contracts.TypedError) {
	id, err := j.d.underlyingID(ctx, s)
	if err != nil {
		return NameInput{}, asTyped(err, contracts.Retryable)
	}
	spot, ok, err := j.d.lastClose(ctx, id, day.AddDate(0, 0, -1))
	if err != nil {
		return NameInput{}, asTyped(err, contracts.Retryable)
	}
	if !ok {
		return NameInput{}, newErr(contracts.NoValue, "no stored close of %s", s.Symbol)
	}
	refs, err := j.d.Options.ListOptions(ctx, id)
	if err != nil {
		return NameInput{}, asTyped(err, contracts.Retryable)
	}
	if len(refs) == 0 {
		return NameInput{}, newErr(contracts.NoValue, "no resolved chain of %s", s.Symbol)
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
		return NameInput{}, asTyped(err, contracts.Retryable)
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
