package barpg

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// The readers read the views of the schema rd and never a base table; dates come back as text.
const (
	// selectPriceBarSQL arguments: instrument_id, basis, first day, last day. The view holds the winner epoch.
	selectPriceBarSQL = `SELECT instrument_id, session_day::text, basis, adjust_epoch, open, high, low, close, volume, ` +
		`currency, source, received_at FROM rd.price_bar_r1 ` +
		`WHERE instrument_id = $1 AND basis = $2 AND session_day >= $3::date AND session_day <= $4::date ` +
		`ORDER BY session_day`
	// selectFxRatesSQL arguments: pair, first day, last day.
	selectFxRatesSQL = `SELECT pair, rate_day::text, rate, source, received_at FROM rd.fx_rate_r1 ` +
		`WHERE pair = $1 AND rate_day >= $2::date AND rate_day <= $3::date ORDER BY rate_day`
	// selectLatestFxSQL arguments: pair, as-of day.
	selectLatestFxSQL = `SELECT pair, rate_day::text, rate, source, received_at FROM rd.fx_rate_r1 ` +
		`WHERE pair = $1 AND rate_day <= $2::date ORDER BY rate_day DESC LIMIT 1`
)

// setText stores s in a field of a string-based enum type that contracts does not export.
func setText[T ~string](field *T, s string) { *field = T(s) }

func parseDay(s string) (time.Time, error) { return time.Parse(time.DateOnly, s) }

func checkRange(from, to contracts.Day) error {
	for _, d := range []contracts.Day{from, to} {
		if _, err := parseDay(string(d)); err != nil {
			return permanent(fmt.Errorf("barpg: bad day %q: %w", d, err))
		}
	}
	return nil
}

// PriceBars returns the winner rows of one instrument and basis with session day from to to, inclusive, ascending
// by day.
func (s *Store) PriceBars(ctx context.Context, instrumentID int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	if err := checkRange(from, to); err != nil {
		return nil, err
	}
	out := []contracts.PriceBar{}
	err := s.db.Select(ctx, selectPriceBarSQL, []any{instrumentID, basis, string(from), string(to)}, func(sc scanner) error {
		var (
			r      contracts.PriceBar
			id     int64
			day    string
			bs     string
			epoch  int32
			source string
		)
		if err := sc.Scan(&id, &day, &bs, &epoch, &r.Open, &r.High, &r.Low, &r.Close, &r.Volume, &r.Currency,
			&source, &r.ReceivedAt); err != nil {
			return err
		}
		d, err := parseDay(day)
		if err != nil {
			return err
		}
		r.InstrumentID, r.SessionDay.Time, r.AdjustEpoch = int(id), d, int(epoch)
		setText(&r.Basis, bs)
		setText(&r.Source, source)
		r.ReceivedAt = r.ReceivedAt.UTC()
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("barpg: price bars: %w", err))
	}
	return out, nil
}

func scanFx(sc scanner) (contracts.FxRate, error) {
	var (
		r      contracts.FxRate
		day    string
		source string
	)
	if err := sc.Scan(&r.Pair, &day, &r.Rate, &source, &r.ReceivedAt); err != nil {
		return r, err
	}
	d, err := parseDay(day)
	if err != nil {
		return r, err
	}
	r.RateDay.Time, r.ReceivedAt = d, r.ReceivedAt.UTC()
	setText(&r.Source, source)
	return r, nil
}

// FxRates returns the rates of pair with rate day from to to, inclusive, ascending by day.
func (s *Store) FxRates(ctx context.Context, pair string, from, to contracts.Day) ([]contracts.FxRate, error) {
	if err := checkRange(from, to); err != nil {
		return nil, err
	}
	out := []contracts.FxRate{}
	err := s.db.Select(ctx, selectFxRatesSQL, []any{pair, string(from), string(to)}, func(sc scanner) error {
		r, err := scanFx(sc)
		if err == nil {
			out = append(out, r)
		}
		return err
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("barpg: fx rates: %w", err))
	}
	return out, nil
}

// LatestFxRate returns the rate of the newest rate day on or before asOf; with no such row the answer is the zero
// rate and found false, never a default.
func (s *Store) LatestFxRate(ctx context.Context, pair string, asOf contracts.Day) (contracts.FxRate, bool, error) {
	if err := checkRange(asOf, asOf); err != nil {
		return contracts.FxRate{}, false, err
	}
	var got contracts.FxRate
	found := false
	err := s.db.Select(ctx, selectLatestFxSQL, []any{pair, string(asOf)}, func(sc scanner) error {
		r, err := scanFx(sc)
		if err == nil && !found {
			got, found = r, true
		}
		return err
	})
	if err != nil {
		return contracts.FxRate{}, false, retryable(fmt.Errorf("barpg: latest fx rate: %w", err))
	}
	return got, found, nil
}
