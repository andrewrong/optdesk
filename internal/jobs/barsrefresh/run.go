package barsrefresh

import (
	"context"
	"strconv"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// Run pulls and stores the trades closes of keys. One key's failure does not stop the others; the
// result carries the first failure, a Retryable one before a Permanent one.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	dayTime, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "bad day")}
	}
	r := &runState{j: j, day: day, dayTime: dayTime, conIDs: map[string]int64{}}
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
	day     contracts.Day
	dayTime time.Time
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
	// The whole pair is read: the epoch is that of the pair, not of the backfill window, so a name
	// whose rows are all older than the window still writes at or above its winner epoch.
	stored, err := r.j.r.PriceBars(ctx, id, basisTrades, firstDay, r.day)
	if err != nil {
		return 0, 0, asTyped(err, contracts.Retryable)
	}
	from := r.dayTime.AddDate(0, 0, -r.j.m.BackfillDays).Format(dayLayout)
	epoch := max(maxEpoch(stored), 1)
	days := r.j.m.DailyDays
	backfilled := hasDayFrom(stored, from)
	if !backfilled {
		if !r.j.claimFirstPull(r.day, key) {
			return 0, 0, nil // past the cap: no request, no error
		}
		days = r.j.m.BackfillDays
	}
	pull := func(days int) ([]broker.Bar, error) {
		return r.j.h.Bars(ctx, broker.BarQuery{Contract: c, Kind: broker.BarTrades, DurationDays: days})
	}
	bars, err := pull(days)
	if err != nil {
		return 0, 0, asTyped(err, contracts.Retryable)
	}
	if len(stored) > 0 {
		again, err := restated(stored, bars, r.j.m.RestateTolerance)
		if err != nil {
			return 0, 0, asTyped(err, contracts.Permanent)
		}
		if again {
			epoch++
			if bars, err = pull(r.j.m.BackfillDays); err != nil {
				return 0, 0, asTyped(err, contracts.Retryable)
			}
			stored = nil // every row of the pull goes in under the new epoch
		}
	}
	rows, terr := r.rows(bars, stored, id, epoch, c.Currency)
	if terr != nil {
		return 0, 0, terr
	}
	if len(rows) > 0 {
		if _, err := r.j.w.InsertPriceBars(ctx, rows); err != nil {
			return 0, 0, asTyped(err, contracts.Retryable)
		}
	}
	if deferred, terr := r.deepen(ctx, key, id, c, backfilled); terr != nil || deferred {
		return len(bars), len(bars), terr
	}
	return len(bars), len(bars), r.adjusted(ctx, id, c)
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

// rows maps the pulled bars to rows of epoch, leaving out the days already in stored.
func (r *runState) rows(bars []broker.Bar, stored []contracts.PriceBar, id int64, epoch int, currency string) ([]contracts.PriceBar, contracts.TypedError) {
	have := make(map[string]bool, len(stored))
	for _, s := range stored {
		have[s.SessionDay.Format(dayLayout)] = true
	}
	now := r.j.clk.Now().UTC()
	var out []contracts.PriceBar
	for _, b := range bars {
		d, err := time.Parse(dayLayout, b.Day)
		if err != nil {
			return nil, newErr(contracts.Permanent, "bad bar day")
		}
		if _, err := parseRat(b.Close); err != nil {
			return nil, asTyped(err, contracts.Permanent)
		}
		if have[b.Day] {
			continue
		}
		out = append(out, contracts.PriceBar{
			AdjustEpoch: epoch, Basis: basisTrades, Close: b.Close, Currency: currency,
			High: b.High, Low: b.Low, Open: b.Open, InstrumentID: int(id), ReceivedAt: now,
			SessionDay: types.SerializableDate{Time: d}, Source: sourceIbkr, Volume: b.Volume,
		})
	}
	return out, nil
}
