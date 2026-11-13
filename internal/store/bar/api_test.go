package bar_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// fakeStore is an in-memory bar.BarStore that keeps the promises of the interface.
type fakeStore struct {
	bars   []contracts.PriceBar
	rates  []contracts.FxRate
	closed bool
}

var _ bar.BarStore = (*fakeStore)(nil)

func (f *fakeStore) InsertPriceBars(_ context.Context, rows []contracts.PriceBar) (int, error) {
	n := 0
	for _, r := range rows {
		exists := false
		for _, o := range f.bars {
			if o.InstrumentID == r.InstrumentID && o.Basis == r.Basis && o.AdjustEpoch == r.AdjustEpoch &&
				o.SessionDay.Equal(r.SessionDay.Time) {
				exists = true
			}
		}
		if !exists {
			f.bars = append(f.bars, r)
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) PriceBars(_ context.Context, id int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	win := map[string]contracts.PriceBar{}
	for _, b := range f.bars {
		d := b.SessionDay.Format("2006-01-02")
		if int64(b.InstrumentID) != id || string(b.Basis) != basis || d < string(from) || d > string(to) {
			continue
		}
		if w, ok := win[d]; !ok || b.AdjustEpoch > w.AdjustEpoch {
			win[d] = b
		}
	}
	var out []contracts.PriceBar
	for _, b := range win {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SessionDay.Before(out[j].SessionDay.Time) })
	return out, nil
}

func (f *fakeStore) InsertFxRates(_ context.Context, rows []contracts.FxRate) (int, error) {
	n := 0
	for _, r := range rows {
		exists := false
		for _, o := range f.rates {
			if o.Pair == r.Pair && o.RateDay.Equal(r.RateDay.Time) {
				exists = true
			}
		}
		if !exists {
			f.rates = append(f.rates, r)
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) FxRates(_ context.Context, pair string, from, to contracts.Day) ([]contracts.FxRate, error) {
	var out []contracts.FxRate
	for _, r := range f.rates {
		d := r.RateDay.Format("2006-01-02")
		if r.Pair == pair && d >= string(from) && d <= string(to) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RateDay.Before(out[j].RateDay.Time) })
	return out, nil
}

func (f *fakeStore) LatestFxRate(_ context.Context, pair string, asOf contracts.Day) (contracts.FxRate, bool, error) {
	var best contracts.FxRate
	found := false
	for _, r := range f.rates {
		if r.Pair == pair && r.RateDay.Format("2006-01-02") <= string(asOf) &&
			(!found || r.RateDay.After(best.RateDay.Time)) {
			best, found = r, true
		}
	}
	return best, found, nil
}

func (f *fakeStore) Close() error { f.closed = true; return nil }

func day(t *testing.T, s string) types.SerializableDate {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return types.SerializableDate{Time: d}
}

func priceBar(t *testing.T, id int, d string, epoch int, closePx string) contracts.PriceBar {
	t.Helper()
	return contracts.PriceBar{
		InstrumentID: id, Basis: "trades", SessionDay: day(t, d),
		AdjustEpoch: epoch, Close: closePx,
	}
}

func TestBarStoreContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var s bar.BarStore = &fakeStore{}
	rows := []contracts.PriceBar{priceBar(t, 7, "2026-01-05", 1, "10"), priceBar(t, 7, "2026-01-02", 1, "9")}
	if n, err := s.InsertPriceBars(ctx, rows); err != nil || n != 2 {
		t.Fatalf("first insert = %d, %v; want 2", n, err)
	}
	if n, err := s.InsertPriceBars(ctx, rows); err != nil || n != 0 {
		t.Fatalf("insert of existing keys = %d, %v; want 0 (insert-only)", n, err)
	}
	got, err := s.PriceBars(ctx, 7, "trades", "2026-01-01", "2026-01-31")
	if err != nil || len(got) != 2 || got[0].Close != "9" || got[1].Close != "10" {
		t.Fatalf("PriceBars = %+v, %v; want the two days ascending", got, err)
	}
	fx := contracts.FxRate{Pair: "USDJPY", RateDay: day(t, "2026-01-05"), Rate: "157.1"}
	if n, err := s.InsertFxRates(ctx, []contracts.FxRate{fx}); err != nil || n != 1 {
		t.Fatalf("InsertFxRates = %d, %v", n, err)
	}
	rates, err := s.FxRates(ctx, "USDJPY", "2026-01-01", "2026-01-31")
	if err != nil || len(rates) != 1 || rates[0].Rate != "157.1" {
		t.Fatalf("FxRates = %+v, %v", rates, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestBarWinnerIsHighestEpoch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var r bar.PriceBarReader
	w := &fakeStore{}
	r = w
	if _, err := w.InsertPriceBars(ctx, []contracts.PriceBar{
		priceBar(t, 7, "2026-01-02", 1, "old"),
		priceBar(t, 7, "2026-01-05", 1, "kept"),
		priceBar(t, 7, "2026-01-02", 2, "restated"),
	}); err != nil {
		t.Fatal(err)
	}
	got, err := r.PriceBars(ctx, 7, "trades", "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want one per day", len(got))
	}
	if got[0].Close != "restated" || got[0].AdjustEpoch != 2 {
		t.Fatalf("day 1 = %+v, want the epoch 2 row", got[0])
	}
	if got[1].Close != "kept" || got[1].AdjustEpoch != 1 {
		t.Fatalf("day 2 = %+v, want the epoch 1 row", got[1])
	}
}

func TestLatestFxRateNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := &fakeStore{}
	var r bar.FxRateReader = w
	if _, err := w.InsertFxRates(ctx, []contracts.FxRate{{Pair: "USDJPY", RateDay: day(t, "2026-01-05"), Rate: "157.1"}}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := r.LatestFxRate(ctx, "USDJPY", "2026-01-04"); err != nil || ok {
		t.Fatalf("a day before the first rate: ok=%v err=%v; want not found and no fallback", ok, err)
	}
	if _, ok, err := r.LatestFxRate(ctx, "EURUSD", "2026-02-01"); err != nil || ok {
		t.Fatalf("another pair: ok=%v err=%v; want not found", ok, err)
	}
	got, ok, err := r.LatestFxRate(ctx, "USDJPY", "2026-02-01")
	if err != nil || !ok || got.Rate != "157.1" {
		t.Fatalf("later day = %+v, %v, %v; want the newest rate on or before", got, ok, err)
	}
}

// upsertFake keeps the promises of bar.FxRateUpserter: a differing row replaces
// the stored one, an equal row changes nothing, no old value is kept.
type upsertFake struct {
	rates     map[string]string
	writtenBy string
}

var (
	_ bar.FxRateWriter   = (*upsertFake)(nil)
	_ bar.FxRateUpserter = (*upsertFake)(nil)
)

func (f *upsertFake) InsertFxRates(context.Context, []contracts.FxRate) (int, error) { return 0, nil }

func (f *upsertFake) UpsertFxRates(_ context.Context, rows []contracts.FxRate, writtenBy string) (bar.FxWriteCount, error) {
	var c bar.FxWriteCount
	if len(rows) == 0 {
		return c, nil
	}
	if f.rates == nil {
		f.rates = map[string]string{}
	}
	f.writtenBy = writtenBy
	for _, r := range rows {
		k := r.Pair + r.RateDay.Format("2006-01-02")
		if old, ok := f.rates[k]; ok && old == r.Rate {
			c.Unchanged++
			continue
		}
		f.rates[k] = r.Rate
		c.Written++
	}
	return c, nil
}

func TestFxRateUpserterContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &upsertFake{}
	var u bar.FxRateUpserter = f
	if c, err := u.UpsertFxRates(ctx, nil, "job"); err != nil || c != (bar.FxWriteCount{}) {
		t.Fatalf("empty list = %+v, %v; want zero counts", c, err)
	}
	a := contracts.FxRate{Pair: "USDJPY", RateDay: day(t, "2026-01-05"), Rate: "157.1"}
	if c, err := u.UpsertFxRates(ctx, []contracts.FxRate{a}, "fx_refresh 2026-01-05"); err != nil || c.Written != 1 || c.Unchanged != 0 {
		t.Fatalf("first upsert = %+v, %v; want 1 written", c, err)
	}
	b := a
	b.Rate = "158.0"
	c, err := u.UpsertFxRates(ctx, []contracts.FxRate{a, b}, "x")
	if err != nil || c.Written != 1 || c.Unchanged != 1 {
		t.Fatalf("second upsert = %+v, %v; want 1 written, 1 unchanged", c, err)
	}
}
