package barsrefresh

import (
	"context"
	"strconv"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// Run pulls the trades bars of keys in a window that ends at day and upserts every pulled bar by key
// with writtenBy "bars_refresh:<day>". A day before the UTC date of the clock is a repair: the query
// carries End = day and asks backfill_days, and neither the weekly adjusted_last step nor the deepening
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
	var res contracts.JobResult
	for _, k := range keys {
		returned, usable, kerr := r.runKey(ctx, k.SubjectKey)
		res.Returned += returned
		res.Usable += usable
		if kerr != nil && (res.Err == nil || (res.Err.Kind() != contracts.Retryable && kerr.Kind() == contracts.Retryable)) {
			res.Err = kerr
		}
	}
	return res
}

type runState struct {
	j       *job
	up      bar.PriceBarUpserter
	day     contracts.Day
	dayTime time.Time
	past    bool             // the day is before the UTC date of the clock: a repair run
	conIDs  map[string]int64 // con id of a key resolved through the broker in this run
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
	from := r.dayTime.AddDate(0, 0, -r.j.m.BackfillDays).Format(dayLayout)
	days := r.j.m.DailyDays
	backfilled := hasDayFrom(stored, from)
	if !backfilled {
		if !r.j.claimFirstPull(r.day, key) {
			return 0, 0, nil // past the cap: no request, no error
		}
		days = r.j.m.BackfillDays
	}
	if r.past {
		days = r.j.m.BackfillDays
	}
	bars, rows, terr := r.collect(ctx, id, c, stored, days)
	if terr != nil {
		return 0, 0, terr
	}
	if terr := r.upsert(ctx, rows); terr != nil {
		return 0, 0, terr
	}
	if r.past {
		return len(bars), len(bars), nil
	}
	if deferred, terr := r.deepen(ctx, key, id, c, backfilled); terr != nil || deferred {
		return len(bars), len(bars), terr
	}
	return len(bars), len(bars), r.adjusted(ctx, id, c)
}

// collect pulls the window of days days and maps it to rows. When a pulled day restates a stored
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
	rows, terr := r.rows(bars, id, c.Currency)
	if terr != nil || !again {
		return bars, rows, terr
	}
	older, terr := r.olderThanWindow(ctx, c, id, stored, bars)
	return bars, append(rows, older...), terr
}

// pull asks the trades bars of c for days days ending at the run day, and drops the bars after it.
// A run of today sends no End.
func (r *runState) pull(ctx context.Context, c broker.Contract, days int) ([]broker.Bar, error) {
	q := broker.BarQuery{Contract: c, Kind: broker.BarTrades, DurationDays: days}
	if r.past {
		q.End = string(r.day)
	}
	return r.dropAfterDay(r.j.h.Bars(ctx, q))
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

// conID is the contract id of the key: the one it carries, else one ResolveContract answer per run.
func (r *runState) conID(ctx context.Context, key string, t target) (int64, contracts.TypedError) {
	if t.contract.ConID != 0 {
		return t.contract.ConID, nil
	}
	if id, ok := r.conIDs[key]; ok {
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
	return got.Contract.ConID, nil
}

// rows maps the pulled bars to rows of basis trades. A day that is not a date, or a close that is not
// a positive decimal, is Permanent.
func (r *runState) rows(bars []broker.Bar, id int64, currency string) ([]contracts.PriceBar, contracts.TypedError) {
	return r.rowsOf(bars, false, id, currency)
}

func (r *runState) rowsOf(bars []broker.Bar, adjusted bool, id int64, currency string) ([]contracts.PriceBar, contracts.TypedError) {
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
