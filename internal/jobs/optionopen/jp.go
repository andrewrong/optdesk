package optionopen

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
	// The zone database is embedded so that a zone name loads on every machine.
	_ "time/tzdata"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

// The Japan side of the option open pull (docs/design/option-open-pull.md, Decision 21): the same jobs over the
// calendar XTKS and the zone Asia/Tokyo. The session date of a run is the date in Asia/Tokyo: the pull compares
// the day of its key with the clock in the zone of its manifest, and the plan and resolve days are the days of
// the XTKS calendar.

//go:embed job_plan_jp.toml
var planManifestJP []byte

//go:embed job_pull_jp.toml
var pullManifestJP []byte

//go:embed job_resolve_jp.toml
var resolveManifestJP []byte

// jpRules are the JP terms of the plan manifest that PlanParams does not carry: the names without options and
// the chain of a name.
type jpRules struct {
	NoOption []string `toml:"-"`
	Params   struct {
		NoOption []string `toml:"no_option"`
	} `toml:"params"`
	Chain struct {
		Exchange    string   `toml:"exchange"`
		Multiplier  string   `toml:"multiplier"`
		IndexNames  []string `toml:"index_names"`
		IndexClass  string   `toml:"index_class"`
		MaxPerIndex int      `toml:"max_per_index"`
		MaxPerStock int      `toml:"max_per_stock"`
	} `toml:"chain"`
}

func parseJPRules(data []byte) (jpRules, error) {
	var r jpRules
	if err := toml.Unmarshal(data, &r); err != nil {
		return jpRules{}, fmt.Errorf("optionopen: jp manifest: %w", err)
	}
	c := r.Chain
	if c.Exchange == "" || c.Multiplier == "" || c.MaxPerIndex < 1 || c.MaxPerStock < 1 || (len(c.IndexNames) > 0 && c.IndexClass == "") {
		return jpRules{}, errors.New("optionopen: jp manifest: chain needs exchange, multiplier, max_per_index and max_per_stock")
	}
	r.NoOption = r.Params.NoOption
	return r, nil
}

func (r jpRules) noOption(symbol string) bool { return slices.Contains(r.NoOption, symbol) }

func (r jpRules) isIndex(symbol string) bool { return slices.Contains(r.Chain.IndexNames, symbol) }

// tradingClass is the class of the chain of a name: the stock code, 225 for the Nikkei 225 index.
func (r jpRules) tradingClass(symbol string) string {
	if r.isIndex(symbol) {
		return r.Chain.IndexClass
	}
	return symbol
}

// planFunc wraps base: a name of no_option keeps its place in the plan as the underlying only (no candidate and
// no held contract), and the stocks and the index names are planned apart, at most 60 and 80 contracts per name.
func (r jpRules) planFunc(base PlanFunc) PlanFunc {
	return func(day time.Time, p PlanParams, names []NameInput) Plan {
		var stocks, index []NameInput
		for _, n := range names {
			if r.noOption(n.Symbol) {
				n.Candidates, n.Held = nil, nil
			}
			if r.isIndex(n.Symbol) {
				index = append(index, n)
			} else {
				stocks = append(stocks, n)
			}
		}
		p.CeilPerStock = r.Chain.MaxPerStock
		out := base(day, p, stocks)
		if len(index) > 0 {
			p.CeilPerStock = r.Chain.MaxPerIndex
			more := base(day, p, index)
			out.Names = append(out.Names, more.Names...)
			out.Items = append(out.Items, more.Items...)
		}
		// Ranks run over the merged plan, the deferred items last.
		slices.SortStableFunc(out.Items, func(a, b Item) int {
			switch {
			case a.Deferred == b.Deferred:
				return 0
			case a.Deferred:
				return 1
			}
			return -1
		})
		for i := range out.Items {
			out.Items[i].Rank = i + 1
		}
		return out
	}
}

// watch is the JP stocks of the watch list that have options, as the entries the shared stock list reads.
func (r jpRules) watch(in []jobs.WatchEntry) []jobs.WatchEntry {
	var out []jobs.WatchEntry
	for _, e := range in {
		sym := e.Contract.Symbol
		kind := e.Contract.SecType
		if e.Market != "JP" || r.noOption(sym) || (kind != "STK" && (kind != "IND" || !r.isIndex(sym))) {
			continue
		}
		// The shared list selects market US and stocks: the JP rule is applied here, and an index entry
		// passes as a stock (jpBroker and jpPull send it as the index it is).
		e.Market, e.Contract.SecType = "US", "STK"
		out = append(out, e)
	}
	return out
}

// jpAccounts shows only the JPY positions of the names that have options.
type jpAccounts struct {
	store.AccountReader
	r jpRules
}

func (a jpAccounts) Positions(ctx context.Context, alias, day string) ([]store.Position, error) {
	rows, err := a.AccountReader.Positions(ctx, alias, day)
	if err != nil {
		return nil, err
	}
	out := rows[:0:0]
	for _, p := range rows {
		if p.Currency == "JPY" && !a.r.noOption(p.Symbol) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r jpRules) stored(s Stored) Stored {
	s.Watch = r.watch(s.Watch)
	if s.Accounts != nil {
		s.Accounts = jpAccounts{s.Accounts, r}
	}
	return s
}

// jpBroker keeps of the chains of a name only the one of OSE.JPN with multiplier 100 whose trading class is the
// code of the stock (225 for the index).
type jpBroker struct {
	broker.Broker
	r jpRules
}

func (b jpBroker) ListChains(ctx context.Context, u broker.Contract) ([]broker.Chain, error) {
	if b.r.isIndex(u.Symbol) {
		u.SecType, u.Exchange = "IND", b.r.Chain.Exchange
	}
	all, err := b.Broker.ListChains(ctx, u)
	if err != nil {
		return nil, err
	}
	var out []broker.Chain
	for _, c := range all {
		if c.Exchange == b.r.Chain.Exchange && c.Multiplier == b.r.Chain.Multiplier && c.TradingClass == b.r.tradingClass(u.Symbol) {
			out = append(out, c)
		}
	}
	return out, nil
}

// PlanDatasetIDJP is the dataset of the one key of the JP plan job, subject planSubjectJP.
const PlanDatasetIDJP = "plan.option_open_item.jp"

const planSubjectJP = "jp"

// ResolveDatasetIDJP is the dataset of the keys of the JP resolve: its own, so that its window is the XTKS one.
const ResolveDatasetIDJP = "master.option_contract.jp"

// NewPlanJP builds option_open_plan_jp from the embedded job_plan_jp.toml: the plan of the JP names of the watch
// list from stored con_ids only, DTE 14 to 90.
func NewPlanJP(d PlanDeps) (contracts.Job, error) { return newPlanJP(planManifestJP, d) }

func newPlanJP(data []byte, d PlanDeps) (contracts.Job, error) {
	if d.Instruments == nil || d.Bars == nil || d.Options == nil || d.Plans == nil || d.Clock == nil {
		return nil, errors.New("optionopen: instruments, bars, options, plan writer and clock are required")
	}
	m, err := parseManifest(data)
	if err != nil {
		return nil, err
	}
	r, err := parseJPRules(data)
	if err != nil {
		return nil, err
	}
	if d.Func == nil {
		d.Func = BuildPlan
	}
	d.Func = r.planFunc(d.Func)
	d.Stored = r.stored(d.Stored)
	return &planJob{m: m, d: d, dataset: PlanDatasetIDJP, subject: planSubjectJP}, nil
}

// NewResolveJP builds option_open_resolve_jp from the embedded job_resolve_jp.toml: NewResolve over the JP names,
// asking for the chains of OSE.JPN only.
func NewResolveJP(d ResolveDeps, o ResolveOptions) (contracts.Job, error) {
	r, err := parseJPRules(planManifestJP)
	if err != nil {
		return nil, err
	}
	d.Stored = r.stored(d.Stored)
	if d.Broker != nil {
		d.Broker = jpBroker{d.Broker, r}
	}
	return newListed(resolveManifestJP, ResolveDatasetIDJP, d, o)
}

var _ NewPullFunc = NewPullJP

// PullDatasetIDJP is the dataset id of the keys of the JP pull. It is a key dataset id only, not a stored table: the
// rows stay in obs.option_open. A key of its own keeps the JP batches from being dropped as duplicates of the US keys.
const PullDatasetIDJP = "obs.option_open.jp"

// NewPullJP builds option_open_jp from the embedded job_pull_jp.toml. It is NewPull over the JP manifest: the key
// day is the date in Asia/Tokyo.
func NewPullJP(d PullDeps) (contracts.Job, error) {
	j, err := newPullFrom(pullManifestJP, d)
	if err != nil {
		return nil, err
	}
	m, err := parseManifest(planManifestJP)
	if err != nil {
		return nil, err
	}
	r, err := parseJPRules(planManifestJP)
	if err != nil {
		return nil, err
	}
	pl := j.(*pull)
	pl.datasetID = PullDatasetIDJP
	return &jpPull{pull: pl, params: m.params, rules: r}, nil
}

// jpPull is the pull with the JP plan parameters: at the open the core chooses the strikes again with DTE 14 to
// 90, not the US 20 to 45, and an index name is sent as the index of OSE.JPN.
type jpPull struct {
	*pull
	params PlanParams
	rules  jpRules
}

// After is the ids of the jobs of the same day that the JP pull reads.
func (p *jpPull) After() []string { return []string{"option_open_plan_jp"} }

// Run is pull.Run with runKey of jpPull.
func (p *jpPull) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	return p.fault(day, keys, p.run(ctx, day, keys))
}

func (p *jpPull) run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	bs, terr := p.runPlan(ctx, day)
	if terr != nil {
		return contracts.JobResult{Err: terr}
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
		if mins := p.d.Clock.Now().In(p.loc); mins.Hour()*60+mins.Minute() >= p.stop && p.d.Log != nil {
			p.d.Log.Warn(fmt.Sprintf("key %s started after stop_after %s", k.SubjectKey, p.m.StopAfter))
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

// runKey is pull.runKey with the JP parameters of the core.
func (p *jpPull) runKey(ctx context.Context, b batch) ([]contracts.OptionOpen, contracts.TypedError) {
	ts, err := p.targets(ctx, b.items)
	if err != nil {
		return nil, asTyped(err, contracts.Retryable)
	}
	for i := range ts {
		if p.rules.isIndex(ts[i].Symbol) {
			ts[i].Under.SecType, ts[i].Under.Exchange = "IND", p.rules.Chain.Exchange
		}
	}
	c := &Core{
		B: p.d.Broker, Clk: p.d.Clock, Params: p.params, Slot: p.m.Slot, Chunk: p.m.Chunk, Retry: p.retry,
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

// newPullFrom is the body of NewPull for a manifest given.
func newPullFrom(data []byte, d PullDeps) (contracts.Job, error) {
	if d.Broker == nil || d.Plans == nil || d.Contracts == nil || d.Writer == nil || d.Instruments == nil || d.Clock == nil {
		return nil, errors.New("optionopen: pull: broker, plan store, contract store, writer, instrument reader and clock are required")
	}
	var m pullTable
	if err := toml.Unmarshal(data, &m); err != nil {
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

// jpLunchBreak is the lunch_break value of job_pull_jp.toml, read once.
var jpLunchBreak = sync.OnceValue(func() [2]string {
	var m struct {
		LunchBreak []string `toml:"lunch_break"`
	}
	if err := toml.Unmarshal(pullManifestJP, &m); err != nil || len(m.LunchBreak) != 2 {
		panic("optionopen: job_pull_jp.toml: lunch_break must be two HHMM values")
	}
	for _, s := range m.LunchBreak {
		if _, err := hhmm(s); err != nil {
			panic("optionopen: job_pull_jp.toml: lunch_break must be two HHMM values")
		}
	}
	return [2]string{m.LunchBreak[0], m.LunchBreak[1]}
})

// JPLunchBreak returns the start and the end of the TSE lunch break in Asia/Tokyo as HHMM, the value
// lunch_break of job_pull_jp.toml: "1130" and "1230". The session check reads it.
func JPLunchBreak() [2]string { return jpLunchBreak() }
