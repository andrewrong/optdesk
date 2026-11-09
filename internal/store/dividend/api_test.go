package dividend_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/dividend"
)

type key struct {
	day     time.Time
	id      int
	attempt int
}

// mem is a fake Store: it keeps the first row of a key and reads the highest attempt of each day.
type mem struct {
	rows map[key]contracts.DividendExpectation
}

var _ dividend.OpenFunc = func(context.Context, string) (dividend.Store, error) {
	return &mem{rows: map[key]contracts.DividendExpectation{}}, nil
}

func (m *mem) InsertDividend(_ context.Context, r contracts.DividendExpectation) error {
	k := key{r.SessionDay.Time, r.InstrumentID, r.Attempt}
	if _, ok := m.rows[k]; !ok {
		m.rows[k] = r
	}
	return nil
}

func (m *mem) Dividends(_ context.Context, id int, from, to contracts.Day) ([]contracts.DividendExpectation, error) {
	f, _ := time.Parse(time.DateOnly, string(from))
	t, _ := time.Parse(time.DateOnly, string(to))
	best := map[time.Time]contracts.DividendExpectation{}
	for k, r := range m.rows {
		if k.id != id || k.day.Before(f) || k.day.After(t) {
			continue
		}
		if b, ok := best[k.day]; !ok || r.Attempt > b.Attempt {
			best[k.day] = r
		}
	}
	out := []contracts.DividendExpectation{}
	for _, r := range best {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SessionDay.Before(out[j].SessionDay.Time) })
	return out, nil
}

func (m *mem) Close() error { return nil }

func row(day string, attempt int, status string) contracts.DividendExpectation {
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		panic(err)
	}
	r := contracts.DividendExpectation{
		SessionDay: types.SerializableDate{Time: d}, InstrumentID: 265598, Attempt: attempt, Issuer: "cik:0000320193",
		Currency: "USD", Source: "ibkr_tick_456", ReceivedAt: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
	}
	// The enum type is generated; its UnmarshalJSON sets a word of the dataset.
	if err := r.Status.UnmarshalJSON([]byte(`"` + status + `"`)); err != nil {
		panic(err)
	}
	return r
}

// TestStoreContractFromBothSides drives one Store as the job (Writer) and as a page (Reader).
func TestStoreContractFromBothSides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var w dividend.Writer = &mem{rows: map[key]contracts.DividendExpectation{}}
	var r dividend.Reader = w.(dividend.Store)
	for _, x := range []contracts.DividendExpectation{
		row("2026-11-09", 1, "error"), row("2026-11-09", 2, "no_value"), row("2026-11-10", 1, "no_value"),
		row("2026-11-10", 1, "error"), // same key: stays as the first
	} {
		if err := w.InsertDividend(ctx, x); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.Dividends(ctx, 265598, "2026-11-09", "2026-11-10")
	if err != nil || len(got) != 2 || got[0].Attempt != 2 || got[1].Status != "no_value" {
		t.Fatalf("rows = %+v, %v", got, err)
	}
	none, err := r.Dividends(ctx, 1, "2026-11-09", "2026-11-10")
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("unknown instrument = %v, %v; want an empty slice", none, err)
	}
}
