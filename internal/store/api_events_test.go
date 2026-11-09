package store_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenEarningsDateStoreFunc = func(context.Context, string) (store.EarningsDateStore, error) {
	return newMemEarnings(), nil
}

type earningsKey struct {
	day     time.Time
	issuer  string
	attempt int
}

// memEarnings is a fake EarningsDateStore: it keeps the first row of a key (ON CONFLICT DO NOTHING) and answers
// EarningsDates with the highest attempt of each day, ascending by day.
type memEarnings struct {
	rows map[earningsKey]contracts.EarningsDate
}

func newMemEarnings() *memEarnings {
	return &memEarnings{rows: map[earningsKey]contracts.EarningsDate{}}
}

func (m *memEarnings) InsertEarningsDate(_ context.Context, r contracts.EarningsDate) error {
	k := earningsKey{r.ObservedOn.Time, r.Issuer, r.Attempt}
	if _, ok := m.rows[k]; !ok {
		m.rows[k] = r
	}
	return nil
}

func (m *memEarnings) EarningsDates(_ context.Context, issuer string, from, to contracts.Day) ([]contracts.EarningsDate, error) {
	f, _ := time.Parse(time.DateOnly, string(from))
	t, _ := time.Parse(time.DateOnly, string(to))
	best := map[time.Time]contracts.EarningsDate{}
	for k, r := range m.rows {
		if k.issuer != issuer || k.day.Before(f) || k.day.After(t) {
			continue
		}
		if b, ok := best[k.day]; !ok || r.Attempt > b.Attempt {
			best[k.day] = r
		}
	}
	out := []contracts.EarningsDate{}
	for _, r := range best {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ObservedOn.Before(out[j].ObservedOn.Time) })
	return out, nil
}

func (m *memEarnings) Close() error { return nil }

func earningsRow(day string, attempt int, errored bool) contracts.EarningsDate {
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		panic(err)
	}
	row := contracts.EarningsDate{
		ObservedOn: types.SerializableDate{Time: d}, Issuer: "cik:0000320193", Attempt: attempt, Symbol: "AAPL",
		Market: "US", Status: "absent", Source: "alphavantage", ReceivedAt: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
	}
	if errored {
		row.Status = "error"
	}
	return row
}

// TestEarningsDateContractFromBothSides drives one EarningsDateStore as the job (writer) and as a page (reader).
func TestEarningsDateContractFromBothSides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var w store.EarningsDateWriter = newMemEarnings()
	s := w.(store.EarningsDateStore)
	var r store.EarningsDateReader = s

	for _, row := range []contracts.EarningsDate{
		earningsRow("2026-11-09", 1, true),
		earningsRow("2026-11-09", 2, false),
		earningsRow("2026-11-10", 1, false),
		earningsRow("2026-11-10", 1, true), // same key: stays as the first
	} {
		if err := w.InsertEarningsDate(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.EarningsDates(ctx, "cik:0000320193", "2026-11-09", "2026-11-10")
	if err != nil || len(got) != 2 {
		t.Fatalf("%d rows, %v; want 2 (one per day)", len(got), err)
	}
	if got[0].Attempt != 2 || got[1].Status != "absent" {
		t.Errorf("rows = %+v", got)
	}
	none, err := r.EarningsDates(ctx, "cik:0000789019", "2026-11-09", "2026-11-10")
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("unknown issuer = %v, %v; want an empty slice", none, err)
	}
}

func TestEarningsDSNEnvName(t *testing.T) {
	t.Parallel()
	if store.DSNEnvEvents != "OT_PG_DSN_EVENTS" {
		t.Errorf("DSNEnvEvents = %q", store.DSNEnvEvents)
	}
}
