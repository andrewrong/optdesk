package optionopen

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	// The zone database is embedded so that a zone name loads on every machine.
	_ "time/tzdata"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

//go:embed job_pull.toml
var pullManifest []byte

// pullDataset is the dataset of the keys of the pull.
const pullDataset = "obs.option_open"

// PlanReader is the read side of openquote.OptionOpenPlanStore: the pull never writes a plan.
type PlanReader interface {
	Plan(ctx context.Context, day contracts.Day, slot string, underlyingIDs ...int64) ([]contracts.OptionOpenItem, error)
}

// ContractLister is the read side of openquote.OptionContractStore.
type ContractLister interface {
	ListOptions(ctx context.Context, underlyingID int64) ([]openquote.OptionRef, error)
}

// PullDeps are the ports of the pull job. Log may be nil.
type PullDeps struct {
	Broker      broker.Broker
	Plans       PlanReader
	Contracts   ContractLister
	Writer      openquote.OptionOpenWriter
	Instruments store.InstrumentReader
	Clock       clock.Clock
	Log         *slog.Logger
}

// NewPullFunc is the constructor of the option_open job (NewPull).
type NewPullFunc func(PullDeps) (contracts.Job, error)

var _ NewPullFunc = NewPull

type pullTable struct {
	Version          int      `toml:"version"`
	ID               string   `toml:"id"`
	Zone             string   `toml:"zone"`
	Currency         string   `toml:"currency"`
	Exchange         string   `toml:"exchange"`
	Slot             string   `toml:"slot"`
	Chunk            int      `toml:"chunk"`
	MaxPerKey        int      `toml:"max_per_key"`
	BudgetMinutes    int      `toml:"budget_minutes"`
	StopAfter        string   `toml:"stop_after"`
	HeartbeatSeconds int      `toml:"heartbeat_seconds"`
	MinOKShare       float64  `toml:"min_ok_share"`
	RetryClasses     []string `toml:"retry_classes"`
	Schedule         struct {
		Calendar         string `toml:"calendar"`
		Anchor           string `toml:"anchor"`
		Offset           string `toml:"offset"`
		RunBound         string `toml:"run_bound"`
		RestartSensitive bool   `toml:"restart_sensitive"`
	} `toml:"schedule"`
}

type pull struct {
	m        pullTable
	d        PullDeps
	loc      *time.Location
	stop     int // stop_after in minutes after midnight, local
	offset   time.Duration
	runBound time.Duration
	retry    map[Class]bool
}

// NewPull builds the option_open job from the embedded job_pull.toml.
func NewPull(d PullDeps) (contracts.Job, error) {
	if d.Broker == nil || d.Plans == nil || d.Contracts == nil || d.Writer == nil || d.Instruments == nil || d.Clock == nil {
		return nil, errors.New("optionopen: pull: broker, plan store, contract store, writer, instrument reader and clock are required")
	}
	var m pullTable
	if err := toml.Unmarshal(pullManifest, &m); err != nil {
		return nil, fmt.Errorf("optionopen: pull manifest: %w", err)
	}
	loc, err := time.LoadLocation(m.Zone)
	if err != nil {
		return nil, errors.New("optionopen: pull manifest: zone is unknown")
	}
	p := &pull{m: m, d: d, loc: loc, retry: map[Class]bool{}}
	if m.ID == "" || len(m.Slot) != 4 || m.Chunk < 1 || m.MaxPerKey < 1 || m.BudgetMinutes < 1 || m.MinOKShare < 0 || m.MinOKShare > 1 {
		return nil, errors.New("optionopen: pull manifest: id, slot, chunk, max_per_key, budget_minutes and min_ok_share are out of range")
	}
	if p.stop, err = hhmm(m.StopAfter); err != nil {
		return nil, errors.New("optionopen: pull manifest: stop_after must be HHMM")
	}
	if p.offset, err = time.ParseDuration(m.Schedule.Offset); err != nil {
		return nil, errors.New("optionopen: pull manifest: schedule.offset is not a duration")
	}
	if p.runBound, err = time.ParseDuration(m.Schedule.RunBound); err != nil {
		return nil, errors.New("optionopen: pull manifest: schedule.run_bound is not a duration")
	}
	for _, c := range m.RetryClasses {
		p.retry[Class(c)] = true
	}
	return p, nil
}

func hhmm(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if len(s) != 4 || err != nil || n/100 > 23 || n%100 > 59 {
		return 0, errors.New("bad time")
	}
	return n/100*60 + n%100, nil
}

// ID is the job id of the manifest.
func (p *pull) ID() string { return p.m.ID }

// Schedule is the slot of the manifest as plain values; the scheduler turns it into start instants.
func (p *pull) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	return p.m.Schedule.Calendar, p.m.Schedule.Anchor, "", p.offset
}

// RunBound is the longest time one run may take.
func (p *pull) RunBound() time.Duration { return p.runBound }

// RestartSensitive is true when a restart between the slot start and the window close must be avoided.
func (p *pull) RestartSensitive() bool { return p.m.Schedule.RestartSensitive }

// errNoPlan marks the empty stored plan inside the NoValue error of batches, so Plan can tell it from a store error.
var errNoPlan = errors.New("no plan")

// batch is one key: stocks whose items together are at most max_per_key (a single stock over it is a batch alone).
type batch struct {
	subject string
	items   []contracts.OptionOpenItem
}

// batches reads the plan of the day and cuts it into keys, in the order of the underlying ids.
func (p *pull) batches(ctx context.Context, day contracts.Day) ([]batch, error) {
	items, err := p.d.Plans.Plan(ctx, day, p.m.Slot)
	if err != nil {
		return nil, newErr(contracts.Retryable, "plan: %v", err)
	}
	if len(items) == 0 {
		return nil, newErr(contracts.NoValue, "%w", errNoPlan)
	}
	var out []batch
	for i := 0; i < len(items); {
		j := i
		for j < len(items) && items[j].UnderlyingID == items[i].UnderlyingID {
			j++
		}
		if n := len(out); n > 0 && len(out[n-1].items)+(j-i) <= p.m.MaxPerKey {
			out[n-1].items = append(out[n-1].items, items[i:j]...)
		} else {
			out = append(out, batch{subject: fmt.Sprintf("batch-%03d", len(out)+1), items: items[i:j:j]})
		}
		i = j
	}
	return out, nil
}

// Plan returns one key per batch of the stored plan and a hash of the batches and their plan ids. Before the plan of
// the day is stored it returns no keys and no error, so the pull waits for a later pass; Run of the same day with no
// plan is NoValue.
func (p *pull) Plan(ctx context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	bs, err := p.batches(ctx, day)
	if errors.Is(err, errNoPlan) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	h := sha256.New()
	keys := make([]contracts.ExpectedKey, len(bs))
	for i, b := range bs {
		keys[i] = contracts.ExpectedKey{DatasetID: pullDataset, SubjectKey: b.subject}
		fmt.Fprintf(h, "%s|%s|%d\n", b.subject, b.items[0].PlanID, len(b.items))
	}
	return keys, fmt.Sprintf("%x", h.Sum(nil)), nil
}

type typedError struct {
	kind contracts.ErrorKind
	err  error
}

func (e *typedError) Error() string             { return e.err.Error() }
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func newErr(kind contracts.ErrorKind, format string, a ...any) contracts.TypedError {
	return &typedError{kind: kind, err: fmt.Errorf("option_open: "+format, a...)}
}

// Run quotes the keys of day. A key is written at the end, in one call; no new key starts after stop_after. At the
// end the postcondition of Decision 22 holds: at least min_ok_share of the planned items that are not deferred
// are ok, else the run is a failed run.
func (p *pull) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	now := p.d.Clock.Now()
	local := now.In(p.loc)
	if string(day) != local.Format(time.DateOnly) {
		return contracts.JobResult{Err: newErr(contracts.NoValue, "wrong day")}
	}
	bs, err := p.batches(ctx, day)
	if err != nil {
		return contracts.JobResult{Err: asTyped(err, contracts.Retryable)}
	}
	byKey := map[string]batch{}
	for _, b := range bs {
		byKey[b.subject] = b
	}
	var res contracts.JobResult
	planned, ok := 0, 0
	for _, k := range keys {
		b, found := byKey[k.SubjectKey]
		if !found {
			return withErr(res, newErr(contracts.Permanent, "unknown key %q", k.SubjectKey))
		}
		for _, it := range b.items {
			if !it.Deferred {
				planned++
			}
		}
		if mins := p.d.Clock.Now().In(p.loc); mins.Hour()*60+mins.Minute() >= p.stop {
			return withErr(res, newErr(contracts.Permanent, "after %s: key %s not started, %d of %d planned items ok", p.m.StopAfter, k.SubjectKey, ok, planned))
		}
		rows, terr := p.runKey(ctx, b)
		if terr != nil {
			return withErr(res, terr)
		}
		res.Returned += len(rows)
		for _, r := range rows {
			if r.Status == stOK {
				ok++
			}
		}
		res.Usable = ok
	}
	if need := p.m.MinOKShare * float64(planned); float64(ok) < need {
		return withErr(res, newErr(contracts.Permanent, "%d of %d planned items ok, under the share %.2f", ok, planned, p.m.MinOKShare))
	}
	return res
}

func withErr(r contracts.JobResult, e contracts.TypedError) contracts.JobResult {
	r.Err = e
	return r
}

func asTyped(err error, kind contracts.ErrorKind) contracts.TypedError {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te
	}
	return &typedError{kind: kind, err: err}
}

// runKey quotes one batch and writes its rows in one call.
func (p *pull) runKey(ctx context.Context, b batch) ([]contracts.OptionOpen, contracts.TypedError) {
	ts, err := p.targets(ctx, b.items)
	if err != nil {
		return nil, asTyped(err, contracts.Retryable)
	}
	c := &Core{
		B: p.d.Broker, Clk: p.d.Clock, Params: DefaultPlanParams(), Slot: p.m.Slot, Chunk: p.m.Chunk, Retry: p.retry,
		Budget: time.Duration(p.m.BudgetMinutes) * time.Minute, Beat: time.Duration(p.m.HeartbeatSeconds) * time.Second,
		Log: p.d.Log,
	}
	r := c.Pull(ctx, ts)
	c.logger().Info(r.Summary.String(), "key", b.subject)
	if err := p.d.Writer.InsertOptionOpen(ctx, r.Rows); err != nil {
		return nil, asTyped(err, contracts.Retryable)
	}
	return r.Rows, nil
}

// targets builds the contracts of the items from the stored option contracts and instruments. An item whose
// contract cannot be built is a target with a Problem, which ends `error`.
func (p *pull) targets(ctx context.Context, items []contracts.OptionOpenItem) ([]Target, error) {
	ids := map[int64]bool{}
	for _, it := range items {
		ids[int64(it.UnderlyingID)], ids[int64(it.InstrumentID)] = true, true
	}
	idList := make([]int64, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	insts, err := p.d.Instruments.Instruments(ctx, idList)
	if err != nil {
		return nil, err
	}
	conID := map[int64]int64{}
	spec := map[int64]store.InstrumentSpec{}
	for _, in := range insts {
		spec[in.ID] = in.Spec
		for _, idf := range in.Identifiers {
			if n, err := strconv.ParseInt(idf.Value, 10, 64); err == nil && idf.Scheme == "ibkr_conid" {
				conID[in.ID] = n
			}
		}
	}
	refs := map[int64]map[int64]openquote.OptionRef{}
	var out []Target
	for _, it := range items {
		u, o := int64(it.UnderlyingID), int64(it.InstrumentID)
		if refs[u] == nil {
			list, err := p.d.Contracts.ListOptions(ctx, u)
			if err != nil {
				return nil, err
			}
			refs[u] = map[int64]openquote.OptionRef{}
			for _, r := range list {
				refs[u][r.InstrumentID] = r
			}
		}
		t := Target{Item: it, Currency: p.m.Currency}
		ref, hasRef := refs[u][o]
		us, hasU := spec[u]
		switch {
		case !hasU || conID[u] == 0 || conID[o] == 0:
			t.Problem = "NO_INSTRUMENT"
		case !hasRef:
			t.Problem = "NO_CONTRACT"
		default:
			exp, perr := time.Parse(time.DateOnly, string(ref.Expiry))
			strike, serr := strconv.ParseFloat(ref.Strike, 64)
			if perr != nil || serr != nil {
				t.Problem = "BAD_CONTRACT"
				break
			}
			if us.Currency != "" {
				t.Currency = us.Currency
			}
			t.Symbol, t.Expiry, t.Strike, t.Right = us.Name, exp, strike, Right(ref.Right)
			t.Under = broker.Contract{SecType: "STK", Symbol: us.Name, Exchange: p.m.Exchange, Currency: t.Currency, ConID: conID[u]}
			t.Option = broker.Contract{SecType: "OPT", Symbol: us.Name, Exchange: ref.Exchange, Currency: ref.Currency,
				Expiry: strings.ReplaceAll(string(ref.Expiry), "-", ""), Strike: ref.Strike, Right: ref.Right,
				Multiplier: ref.Multiplier, TradingClass: ref.TradingClass, ConID: conID[o]}
		}
		if t.Symbol == "" {
			t.Symbol = us.Name
		}
		out = append(out, t)
	}
	return out, nil
}
