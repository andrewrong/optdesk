package barsrefresh

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// A key with a yfinance route (a USD or JPY stock or ETF, once the job has a yfinance source) asks IB for no
// daily bar: history.go gives its trades and adjusted_last rows. The rest of this comment is the IB path of
// a key with no route.
//
// Run pulls the trades bars of keys in a window that ends at day and upserts every pulled bar by key
// with writtenBy "bars_refresh:<day>". A day before the UTC date of the clock is a repair: the query
// carries End = day and asks the gap of the key, and neither the weekly adjusted_last step nor the deepening
// step runs. One key's failure does not stop the others; the result carries the first failure, a
// Retryable one before a Permanent one.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	dayTime, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "bad day")}
	}
	up, ok := j.w.(bar.PriceBarUpserter)
	if !ok {
		return contracts.JobResult{Err: newErr(contracts.Retryable, "the bar store is not available (%s is not set): no bar can be written", bar.DSNEnvBar)}
	}
	r := &runState{j: j, up: up, day: day, dayTime: dayTime, past: string(day) < j.clk.Now().UTC().Format(dayLayout),
		conIDs: map[string]int64{}}
	r.rep, _ = j.w.(bar.PriceBarReplacer)
	var res contracts.JobResult
	add := func(returned, usable int, kerr contracts.TypedError) {
		res.Returned += returned
		res.Usable += usable
		if kerr != nil && (res.Err == nil || (res.Err.Kind() != contracts.Retryable && kerr.Kind() == contracts.Retryable)) {
			res.Err = kerr
		}
	}
	for _, k := range keys {
		add(r.runKey(ctx, k.SubjectKey))
	}
	return res
}

type runState struct {
	j       *job
	up      bar.PriceBarUpserter
	rep     bar.PriceBarReplacer // nil when the store has no ReplacePriceBars: a full yfinance pull then fails Retryable
	yfDown  bool                 // yfinance answered rate_limited in this run: the later keys are not asked
	day     contracts.Day
	dayTime time.Time
	past    bool             // the day is before the UTC date of the clock: a repair run
	conIDs  map[string]int64 // con id of a key resolved through the broker in this run

	retries int // history retries made in this run: at most histDownAfter
}

// firstDay is before any stored bar: the read that starts there gives every winner row of the pair.
const firstDay = contracts.Day("1900-01-01")

// hasDayFrom reports whether a row has a session day on or after from (YYYY-MM-DD).
func hasDayFrom(rows []contracts.PriceBar, from string) bool {
	for _, r := range rows {
		if r.SessionDay.Format(dayLayout) >= from {
			return true
		}
	}
	return false
}

// claimFirstPull reports whether key may start (or keep) a first pull today: at most
// backfill_names_per_run keys do.
func (j *job) claimFirstPull(day contracts.Day, key string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	set := j.started[day]
	if set == nil {
		set = map[string]bool{}
		j.started[day] = set
	}
	if set[key] {
		return true
	}
	if len(set) >= j.m.BackfillNamesPerRun {
		return false
	}
	set[key] = true
	return true
}

func (r *runState) runKey(ctx context.Context, key string) (returned, usable int, terr contracts.TypedError) {
	t, ok := r.j.byKey[key]
	if !ok {
		return 0, 0, newErr(contracts.Permanent, "unknown key %q", key)
	}
	conID, terr := r.conID(ctx, key, t)
	if terr != nil {
		return 0, 0, terr
	}
	c := t.contract
	c.ConID = conID
	id, err := r.j.res.ResolveInstrument(ctx, store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(conID, 10)}, t.spec, nil)
	if err != nil {
		return 0, 0, asTyped(err, contracts.Permanent)
	}
	stored, err := r.j.r.PriceBars(ctx, id, basisTrades, firstDay, r.day)
	if err != nil {
		return 0, 0, asTyped(err, contracts.Retryable)
	}
	if pair, ok := r.routeOf(c); ok {
		return r.yfinanceKey(ctx, key, id, c, pair, stored) // no IB history request for a key with a route
	}
	from := r.dayTime.AddDate(0, 0, -r.j.m.BackfillDays).Format(dayLayout)
	var days int
	backfilled := hasDayFrom(stored, from)
	complete := backfilled && dailyComplete(stored, r.dayTime) // nothing to ask: deepen and adjusted below keep their own gates
	if !backfilled {
		if !r.j.claimFirstPull(r.day, key) {
			return 0, 0, newErr(contracts.Retryable, "first-pull cap reached") // no request, no row: the ledger closes it expired
		}
		days = r.j.m.BackfillDays
	} else {
		days = windowDays(stored, r.dayTime, r.j.m.DailyDays, r.j.m.BackfillDays) // only the gap, or from the oldest yfinance row
	}
	var bars []broker.Bar
	if !complete {
		var rows []contracts.PriceBar
		bars, rows, terr = r.collect(ctx, id, c, stored, days)
		if terr != nil {
			return 0, 0, terr
		}
		if terr := r.upsert(ctx, rows); terr != nil {
			return 0, 0, terr
		}
	}
	if r.past {
		return len(bars), len(bars), nil
	}
	if deferred, terr := r.deepen(ctx, key, id, c, backfilled); terr != nil || deferred {
		return len(bars), len(bars), terr
	}
	return len(bars), len(bars), r.adjusted(ctx, id, c)
}

// collect pulls the window of `days` days and maps it to rows. When a pulled day restates a stored
// one, the window is the whole backfill and the rows of the older stored days are added; nothing is
// written here.
func (r *runState) collect(
	ctx context.Context, id int64, c broker.Contract, stored []contracts.PriceBar, days int,
) ([]broker.Bar, []contracts.PriceBar, contracts.TypedError) {
	bars, err := r.pull(ctx, c, days)
	if err != nil {
		return nil, nil, asTyped(err, contracts.Retryable)
	}
	var again bool
	if len(stored) > 0 {
		if again, err = restated(stored, bars, r.j.m.RestateTolerance); err != nil {
			return nil, nil, asTyped(err, contracts.Permanent)
		}
	}
	if again && days != r.j.m.BackfillDays {
		if bars, err = r.pull(ctx, c, r.j.m.BackfillDays); err != nil {
			return nil, nil, asTyped(err, contracts.Retryable)
		}
	}
	rows, terr := r.rows(bars, false, id, c.Currency)
	if terr != nil || !again {
		return bars, rows, terr
	}
	older, terr := r.olderThanWindow(ctx, c, id, stored, bars)
	return bars, append(rows, older...), terr
}

// historyAttempts is the number of Bars calls one pull makes for a window: one call and one retry after
// retryPause. A gateway timeout is usually over after a pause; a run makes at most histDownAfter retries.
const historyAttempts = 2

// retryPause is the wait before the retry of a Retryable history call.
const retryPause = 60 * time.Second

// defaultPause waits d on a timer, or returns the error of ctx when it ends first.
func defaultPause(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// pull asks the trades bars of c for `days` days ending at the run day, and drops the bars after it.
// A run of today sends no End.
func (r *runState) pull(ctx context.Context, c broker.Contract, days int) ([]broker.Bar, error) {
	q := broker.BarQuery{Contract: c, Kind: broker.BarTrades, DurationDays: days}
	if r.past {
		q.End = string(r.day)
	}
	var bars []broker.Bar
	var err error
	for attempt := range historyAttempts {
		if attempt > 0 {
			if r.retries >= histDownAfter {
				break
			}
			r.retries++
			if r.j.pause(ctx, retryPause) != nil {
				break
			}
		}
		if bars, err = r.j.h.Bars(ctx, q); err == nil {
			return r.dropAfterDay(bars, nil)
		}
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != contracts.Retryable {
			break
		}
	}
	return nil, err
}

func (r *runState) dropAfterDay(bars []broker.Bar, err error) ([]broker.Bar, error) {
	if err != nil {
		return nil, err
	}
	out := make([]broker.Bar, 0, len(bars))
	for _, b := range bars {
		if b.Day <= string(r.day) {
			out = append(out, b)
		}
	}
	return out, nil
}

// upsert writes rows in one call with writtenBy "bars_refresh:<day>"; no rows make no call.
func (r *runState) upsert(ctx context.Context, rows []contracts.PriceBar) contracts.TypedError {
	if len(rows) == 0 {
		return nil
	}
	if _, err := r.up.UpsertPriceBars(ctx, rows, "bars_refresh:"+string(r.day)); err != nil {
		return asTyped(err, contracts.Retryable)
	}
	return nil
}

// conID is the contract id of the key: the one it carries, else the one the job kept from an earlier answer, else a new ResolveContract answer.
func (r *runState) conID(ctx context.Context, key string, t target) (int64, contracts.TypedError) {
	if t.contract.ConID != 0 {
		return t.contract.ConID, nil
	}
	if id, ok := r.conIDs[key]; ok {
		return id, nil
	}
	r.j.mu.Lock()
	id, ok := r.j.conIDs[key]
	r.j.mu.Unlock()
	if ok {
		return id, nil
	}
	got, err := r.j.b.ResolveContract(ctx, t.contract)
	if err != nil {
		return 0, asTyped(err, contracts.Retryable)
	}
	if !got.Listed || got.Contract.ConID == 0 {
		return 0, newErr(contracts.Permanent, "%s is not listed", key)
	}
	r.conIDs[key] = got.Contract.ConID
	r.j.mu.Lock()
	r.j.conIDs[key] = got.Contract.ConID
	r.j.mu.Unlock()
	return got.Contract.ConID, nil
}

// rows maps the pulled bars to rows of basis trades, or of adjusted_last when adjusted. A day that is
// not a date, or a close that is not a positive decimal, is Permanent.
func (r *runState) rows(bars []broker.Bar, adjusted bool, id int64, currency string) ([]contracts.PriceBar, contracts.TypedError) {
	now := r.j.clk.Now().UTC()
	out := make([]contracts.PriceBar, 0, len(bars))
	for _, b := range bars {
		d, err := time.Parse(dayLayout, b.Day)
		if err != nil {
			return nil, newErr(contracts.Permanent, "bad bar day")
		}
		if v, err := parseRat(b.Close); err != nil {
			return nil, asTyped(err, contracts.Permanent)
		} else if v.Sign() <= 0 {
			return nil, newErr(contracts.Permanent, "close %q is not positive", b.Close)
		}
		row := contracts.PriceBar{
			Basis: basisTrades, Close: b.Close, Currency: currency,
			High: b.High, Low: b.Low, Open: b.Open, InstrumentID: int(id), ReceivedAt: now,
			SessionDay: types.SerializableDate{Time: d}, Source: sourceIbkr, Volume: b.Volume,
		}
		if adjusted {
			row.Basis = basisAdjusted
		}
		out = append(out, row)
	}
	return out, nil
}
