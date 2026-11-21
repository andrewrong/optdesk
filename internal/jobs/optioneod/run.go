package optioneod

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/runview"
	"github.com/andrewrong/optdesk/internal/symbol"
)

var (
	_ NewOptionEodFunc      = New
	_ NewOptionEodWatchFunc = NewWithWatch
)

type job struct {
	m    manifest
	b    broker.Broker
	res  store.InstrumentResolver
	sym  symbol.Translator   // nil: no translation, the symbol is its own option root
	acct store.AccountReader // nil: no held short calls are captured
	w    store.OptionEodWriter
	// closes gives the stored index closes of the index job (spot fallback and sigma); nil: none.
	closes IndexCloseReader
	clk    clock.Clock
	loc    *time.Location

	reasons runview.ReasonWriter // nil: no reasons are written; set by WithReasons

	logger *slog.Logger           // nil: slog.Default; set by tests
	stored ContractReader         // nil: every pair is resolved at run time; set by WithContracts
	insts  store.InstrumentReader // the instruments of the stored contracts
	// calendar gives the session close for a close-anchored slot; nil: such a run is never early. Set by WithCalendar.
	calendar clock.Calendar

	cal         string
	open, close time.Duration
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

// asTyped returns err unchanged when it is a TypedError, else wraps it with kind.
func asTyped(err error, kind contracts.ErrorKind) contracts.TypedError {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te
	}
	return &typedError{kind: kind, err: err}
}

// New builds the option_eod job from the embedded job.toml.
func New(b broker.Broker, res store.InstrumentResolver, w store.OptionEodWriter, clk clock.Clock) (contracts.Job, error) {
	return NewWithSymbols(b, res, w, clk, nil)
}

var _ NewOptionEodSymbolsFunc = NewWithSymbols

// NewWithSymbols is New with a translator that gives the option root of a symbol; nil means no translation.
func NewWithSymbols(b broker.Broker, res store.InstrumentResolver, w store.OptionEodWriter, clk clock.Clock, t symbol.Translator) (contracts.Job, error) {
	if b == nil || res == nil || w == nil || clk == nil {
		return nil, errors.New("optioneod: broker, resolver, writer and clock are required")
	}
	j, err := build(manifestBytes, b, res, w, clk)
	if err != nil {
		return nil, err
	}
	j.sym = t
	// Without a watch list the job keeps the nearest max_expiries and the strike band, as before.
	j.m.MinDTE, j.m.MaxDTE, j.m.MaxStrikes = 0, 0, 0
	j.m.StrikeBandBelow, j.m.StrikeBandAbove = 0, 0
	return j, nil
}

func build(data []byte, b broker.Broker, res store.InstrumentResolver, w store.OptionEodWriter, clk clock.Clock) (*job, error) {
	m, err := parseManifest(data)
	if err != nil {
		return nil, err
	}
	cal, open, closeOff, err := windowOf(m)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(m.Zone)
	if err != nil {
		return nil, errors.New("optioneod: manifest: zone is unknown")
	}
	return &job{m: m, b: b, res: res, w: w, clk: clk, loc: loc, cal: cal, open: open, close: closeOff}, nil
}

// root is the option root of symbol: the translator's OSIRoot, or symbol itself without a translator.
func (j *job) root(symbol string) string {
	if j.sym == nil {
		return symbol
	}
	return j.sym.OSIRoot(symbol)
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Run collects the keys of day: it stops with Retryable once budget_minutes have passed, keeping the rows written.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	now := j.clk.Now()
	// The broker answers for the present only: a day other than the clock's local date cannot be collected.
	if string(day) != now.In(j.loc).Format(time.DateOnly) {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "option_eod: day %s is not the current day; a past option chain cannot be collected", day)}
	}
	if j.m.Slot == "" {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "option_eod: manifest has no slot")}
	}
	dayTime, err := time.Parse(time.DateOnly, string(day))
	if err != nil {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "bad day")}
	}
	early := j.beforeSlot(ctx, day, now)
	slot := j.m.Slot
	if early {
		slot = "early" + now.UTC().Format(time.RFC3339Nano) // its own capture, not the scheduled run's
	}
	r := &runState{j: j, day: day, dayTime: dayTime, start: now, slot: slot}
	var res contracts.JobResult
	for _, k := range keys {
		if err := r.runKey(ctx, k, &res); err != nil {
			res.Err = err
			return res
		}
	}
	if early && res.Err == nil {
		res.Err = newErr(contracts.Retryable, "option_eod: %s: the run started before the slot %s; its rows are stored under their own capture, "+
			"the key stays open for the scheduled run", reasonBeforeSlot, j.m.Slot)
	}
	return res
}

type runState struct {
	j       *job
	day     contracts.Day
	dayTime time.Time
	start   time.Time
	slot    string // the slot argument of the capture id
}

// checkBudget is read before every broker call.
func (r *runState) checkBudget() contracts.TypedError {
	if r.j.clk.Now().Sub(r.start) >= time.Duration(r.j.m.BudgetMinutes)*time.Minute {
		return newErr(contracts.Retryable, "budget of %d minutes spent", r.j.m.BudgetMinutes)
	}
	return nil
}

func (j *job) underlyingOf(symbol string) (underlying, bool) {
	for _, u := range j.m.Underlying {
		if u.Symbol == symbol {
			return u, true
		}
	}
	return underlying{}, false
}

func (u underlying) contract() broker.Contract {
	return broker.Contract{SecType: u.SecType, Symbol: u.Symbol, Exchange: u.Exchange, Currency: u.Currency, ConID: u.ConID}
}

func (u underlying) spec() store.InstrumentSpec {
	typ := "stock"
	if u.SecType == "IND" {
		typ = "index"
	}
	return store.InstrumentSpec{Type: typ, Name: u.Symbol, Currency: u.Currency, MIC: u.MIC, CalendarID: u.CalendarID,
		Status: "active", PriceIncrement: u.PriceIncrement, LotSize: u.LotSize}
}

func (r *runState) runKey(ctx context.Context, k contracts.ExpectedKey, res *contracts.JobResult) contracts.TypedError {
	u, ok := r.j.underlyingOf(k.SubjectKey)
	if !ok {
		return newErr(contracts.Permanent, "unknown underlying %q", k.SubjectKey)
	}
	if r.j.m.isStock(u) {
		if u.ConID == 0 {
			return r.j.noValue("no con_id in watch entry", u)
		}
		if u.Currency == "" {
			return r.j.noValue("no currency in watch entry", u)
		}
	}
	price, own, terr := r.price(ctx, u)
	if terr != nil {
		return terr
	}
	pairs, terr := r.pairs(ctx, u, price)
	if terr != nil {
		return terr
	}
	var undID int64
	if r.j.stored != nil {
		var terr contracts.TypedError
		if undID, terr = r.underlyingID(ctx, u); terr != nil {
			return terr
		}
	}
	listed, terr := r.listed(ctx, u, undID, pairs)
	if terr != nil {
		return terr
	}
	if _, ok := r.j.m.indexOf(u.Symbol); ok {
		listed = completePairs(listed)
	}
	if r.j.stored == nil {
		var terr contracts.TypedError
		if undID, terr = r.underlyingID(ctx, u); terr != nil {
			return terr
		}
	}
	capture := captureID(r.day, k, r.slot)
	var tl tally
	var chunkErr contracts.TypedError
	for i := 0; i < len(listed); i += r.j.m.Chunk {
		end := min(i+r.j.m.Chunk, len(listed))
		if chunkErr = r.chunk(ctx, u, undID, price, own, capture, listed[i:end], res, &tl); chunkErr != nil {
			break
		}
	}
	r.writeReasons(ctx, k, &tl)
	return chunkErr
}

// writeReasons replaces the reason rows of the key with its tally; a key that stored nothing, or a job without a
// writer, writes nothing. A writer error is logged: it never fails the run.
func (r *runState) writeReasons(ctx context.Context, k contracts.ExpectedKey, tl *tally) {
	if r.j.reasons == nil || tl.total == 0 {
		return
	}
	rows := tl.reasons(r.day, k.DatasetID, k.SubjectKey, r.j.clk.Now().UTC())
	if err := r.j.reasons.ReplaceReasons(ctx, r.day, k.DatasetID, k.SubjectKey, rows); err != nil {
		slog.Warn("optioneod: reasons not written", "dataset", k.DatasetID, "subject", k.SubjectKey, "err", err)
	}
}

// price is Last of the underlying's snapshot, else Close, else UndPrice, else the stored spot of an index row, else the
// mid of a positive Bid and Ask; none is Retryable and its text names the quote status and the absent fields. own is
// true when the price is the stock's own (Last or Close of the snapshot): only such a price overrides the option quote's
// UndPrice on a row.
func (r *runState) price(ctx context.Context, u underlying) (price float64, own bool, terr contracts.TypedError) {
	if terr := r.checkBudget(); terr != nil {
		return 0, false, terr
	}
	qs, err := r.j.b.Snapshot(ctx, broker.KindSnapshot, []broker.Contract{u.contract()})
	if err != nil {
		return 0, false, asTyped(err, contracts.Permanent)
	}
	var q *broker.Quote
	if len(qs) > 0 {
		q = &qs[0]
		for i, p := range []*float64{q.Last, q.Close, q.UndPrice} {
			if positive(p) {
				return *p, i < 2, nil
			}
		}
	}
	if row, ok := r.j.m.indexOf(u.Symbol); ok {
		if v, ok := r.storedSpot(ctx, row); ok {
			return v, false, nil
		}
	}
	if q != nil && positive(q.Bid) && positive(q.Ask) {
		return (*q.Bid + *q.Ask) / 2, false, nil
	}
	if q == nil {
		return 0, false, newErr(contracts.Retryable, "no price of %s: no quote returned", u.Symbol)
	}
	var absent []string
	for _, f := range []struct {
		name string
		v    *float64
	}{{"Last", q.Last}, {"Close", q.Close}, {"UndPrice", q.UndPrice}, {"Bid", q.Bid}, {"Ask", q.Ask}} {
		if !positive(f.v) {
			absent = append(absent, f.name)
		}
	}
	return 0, false, newErr(contracts.Retryable, "no price of %s: quote status %d, absent %s", u.Symbol, q.Status,
		strings.Join(absent, ","))
}

func positive(p *float64) bool { return p != nil && *p > 0 }

// pairs lists the chains and selects from the chain chainOf picks; no such chain is a Permanent failure.
func (r *runState) pairs(ctx context.Context, u underlying, price float64) ([]broker.Contract, contracts.TypedError) {
	if terr := r.checkBudget(); terr != nil {
		return nil, terr
	}
	chains, err := r.j.b.ListChains(ctx, u.contract())
	if err != nil {
		return nil, asTyped(err, contracts.Permanent)
	}
	stock := r.j.m.isStock(u)
	if len(chains) == 0 {
		if stock {
			return nil, r.j.noValue("no listed options", u)
		}
		return nil, newErr(contracts.Permanent, "empty selection for %s", u.Symbol)
	}
	var chain broker.Chain
	if stock {
		var terr contracts.TypedError
		if chain, terr = r.j.stockChain(chains, u); terr != nil {
			return nil, terr
		}
	} else {
		cm := r.j.m
		if row, isIdx := cm.indexOf(u.Symbol); isIdx {
			c, ok := indexChain(chains, u, row.TradingClass)
			if !ok {
				return nil, newErr(contracts.Permanent, "no chain of trading class %s for %s: the gateway lists classes %s",
					row.TradingClass, u.Symbol, listedClasses(chains))
			}
			chain = c
		} else {
			c, class, ok := cm.chainOfRoot(chains, u, r.j.root(u.Symbol))
			if !ok {
				return nil, newErr(contracts.Permanent, "no chain of trading class %s on %s for %s", class, u.Exchange, u.Symbol)
			}
			chain = c
		}
	}
	sel := r.j.m
	if stock {
		sel = sel.forStock()
	}
	var out []broker.Contract
	if row, ok := r.j.m.indexOf(u.Symbol); ok {
		out = selectIndex(chain, u, row, r.j.m, string(r.day), price, r.sigma(ctx, row))
	} else {
		out = selectContracts(chain, u, string(r.day), price, sel)
	}
	if r.j.acct != nil {
		held, err := r.j.heldOptions(ctx, u.Symbol, r.dayTime)
		if err != nil {
			return nil, asTyped(err, contracts.Retryable)
		}
		out = addHeld(out, chain, u, held)
	}
	if len(out) == 0 {
		if stock {
			return nil, r.j.noValue("no contract in window", u)
		}
		return nil, newErr(contracts.Permanent, "empty selection for %s", u.Symbol)
	}
	return out, nil
}

// noValue is the final NoValue of a stock key; the text names the job and the symbol only.
func (j *job) noValue(text string, u underlying) contracts.TypedError {
	return newErr(contracts.NoValue, "%s %s: %s", j.m.ID, u.Symbol, text)
}

// stockChain picks the chain of a stock: the first on the [stock] chain_exchange whose trading class is the root of
// the code and whose multiplier is the [stock] multiplier. No chain of the class is "no listed options"; chains of the
// class with another multiplier only are "no chain of multiplier 100". Both are final.
func (j *job) stockChain(chains []broker.Chain, u underlying) (broker.Chain, contracts.TypedError) {
	class := j.root(u.Symbol)
	other := false
	for _, c := range chains {
		if c.Exchange != j.m.Stock.ChainExchange || c.TradingClass != class {
			continue
		}
		if c.Multiplier == j.m.Stock.Multiplier {
			return c, nil
		}
		other = true
	}
	if other {
		return broker.Chain{}, j.noValue("no chain of multiplier "+j.m.Stock.Multiplier, u)
	}
	return broker.Chain{}, j.noValue("no listed options", u)
}

func (r *runState) underlyingID(ctx context.Context, u underlying) (int64, contracts.TypedError) {
	id, err := r.j.res.ResolveInstrument(ctx, store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(u.ConID, 10)}, u.spec(), nil)
	if err != nil {
		return 0, asTyped(err, contracts.Permanent)
	}
	return id, nil
}

// chunk resolves the instruments of cs, asks their quotes and writes the rows; price is the row's underlying_price
// (see withUnderlyingPrice for own).
func (r *runState) chunk(ctx context.Context, u underlying, undID int64, price float64, own bool, capture jobs.CaptureID, cs []broker.Contract,
	res *contracts.JobResult, tl *tally) contracts.TypedError {
	ids := make([]int64, len(cs))
	for i, c := range cs {
		osi, err := osiSymbolWith(c, osiRootOf(c.TradingClass, r.j.root(c.Symbol)))
		if err != nil {
			return asTyped(err, contracts.Permanent)
		}
		mic := u.MIC
		if r.j.m.isStock(u) {
			mic = r.j.m.Stock.OptionMIC
		}
		spec := store.InstrumentSpec{Type: "option", Name: optionName(c), Currency: u.Currency, MIC: mic, CalendarID: u.CalendarID,
			Status: "active", PriceIncrement: r.j.m.OptionPriceIncrement, LotSize: c.Multiplier}
		key := store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(c.ConID, 10)}
		ids[i], err = r.j.res.ResolveInstrument(ctx, key, spec, []store.Identifier{{Scheme: "osi", Value: osi}})
		if err != nil {
			return asTyped(err, contracts.Permanent)
		}
	}
	if terr := r.checkBudget(); terr != nil {
		return terr
	}
	qs, err := r.j.b.Snapshot(ctx, broker.KindOptionEod, cs)
	if err != nil {
		return asTyped(err, contracts.Permanent)
	}
	if len(qs) != len(cs) {
		return newErr(contracts.Permanent, "broker answered %d quotes for %d contracts", len(qs), len(cs))
	}
	rows := make([]contracts.OptionEod, len(qs))
	usable := 0
	for i, q := range qs {
		rows[i] = withUnderlyingPrice(mapRow(q, ids[i], undID, r.dayTime, capture, u.Currency), price, own)
		if rows[i].Status == statusOK {
			usable++
		}
	}
	if err := r.j.w.InsertOptionEod(ctx, rows); err != nil {
		res.Usable = 0
		return asTyped(err, contracts.Retryable)
	}
	for i, q := range qs {
		tl.add(q, rows[i].Status)
	}
	res.Returned += len(rows)
	res.Usable += usable
	return nil
}

// chainOf is chainOfRoot with the underlying's symbol as its root.
func (m manifest) chainOf(chains []broker.Chain, u underlying) (chain broker.Chain, class string, ok bool) {
	return m.chainOfRoot(chains, u, u.Symbol)
}

// chainOfRoot picks the chain to select from, whatever order the gateway lists the chains in: the first chain on the
// underlying's exchange whose trading class is the manifest's trading_class, or root when none is set. There is no
// fallback to another chain: it reports ok false, and the class it looked for.
func (m manifest) chainOfRoot(chains []broker.Chain, u underlying, root string) (chain broker.Chain, class string, ok bool) {
	class = m.TradingClass
	if class == "" {
		class = root
	}
	for _, c := range chains {
		if c.Exchange == u.Exchange && c.TradingClass == class {
			return c, class, true
		}
	}
	return broker.Chain{}, class, false
}

// indexChain picks the chain of an [[index]] row: the first of the class on the underlying's exchange, else the first
// of the class on any exchange. The gateway lists some index options (RUT) under the exchange of the options, not of
// the index. It is one more lookup in the same list.
func indexChain(chains []broker.Chain, u underlying, class string) (broker.Chain, bool) {
	var other *broker.Chain
	for i, c := range chains {
		if c.TradingClass != class {
			continue
		}
		if c.Exchange == u.Exchange {
			return c, true
		}
		if other == nil {
			other = &chains[i]
		}
	}
	if other != nil {
		return *other, true
	}
	return broker.Chain{}, false
}

// listedClasses names the distinct trading classes of the chains in listing order, at most ten.
func listedClasses(chains []broker.Chain) string {
	var out []string
	seen := map[string]bool{}
	for _, c := range chains {
		if seen[c.TradingClass] {
			continue
		}
		seen[c.TradingClass] = true
		if len(out) == 10 {
			out = append(out, "...")
			break
		}
		out = append(out, c.TradingClass)
	}
	return strings.Join(out, ",")
}
