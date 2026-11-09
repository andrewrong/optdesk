package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenFactStoreFunc = func(context.Context, string) (store.FactStore, error) { return newMemFacts(), nil }

// factKey is the eight-column primary key of obs.fundamental_fact; an absent period_start is the zero day,
// as period_start_key holds 0001-01-01 in the table.
type factKey struct {
	source, subject, docID, taxonomy, concept, unit string
	start, end                                      time.Time
}

// memFacts is a fake FactStore: it keeps the first row of a key (ON CONFLICT DO NOTHING) and answers
// FactsFiledBy with every row of the subject filed on or before the cutoff, the filter being the store's.
type memFacts struct {
	keys map[factKey]bool
	rows []contracts.FundamentalFact
}

func newMemFacts() *memFacts { return &memFacts{keys: map[factKey]bool{}} }

func (m *memFacts) InsertFacts(_ context.Context, rows []contracts.FundamentalFact) error {
	for _, r := range rows {
		k := factKey{string(r.Source), r.Subject, r.DocID, r.Taxonomy, r.Concept, r.Unit, time.Time{}, r.PeriodEnd.Time}
		if r.PeriodStart != nil {
			k.start = r.PeriodStart.Time
		}
		if !m.keys[k] {
			m.keys[k] = true
			m.rows = append(m.rows, r)
		}
	}
	return nil
}

func (m *memFacts) FactsFiledBy(_ context.Context, source, subject string, cutoff contracts.Day) ([]contracts.FundamentalFact, error) {
	c, _ := time.Parse(time.DateOnly, string(cutoff))
	out := []contracts.FundamentalFact{}
	for _, r := range m.rows {
		if string(r.Source) == source && r.Subject == subject && !r.Filed.After(c) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memFacts) Close() error { return nil }

func factDay(s string) types.SerializableDate {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return types.SerializableDate{Time: t}
}

func mkFact(subject, doc, concept, start, end, filed, value string) contracts.FundamentalFact {
	f := contracts.FundamentalFact{
		Source: "sec", Subject: subject, DocID: doc, Taxonomy: "us-gaap", Concept: concept, Unit: "USD",
		Currency: "USD", PeriodEnd: factDay(end), Value: value, Form: "10-K", Filed: factDay(filed),
		RetrievedAt: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC), RawSha256: "00",
	}
	if start != "" {
		d := factDay(start)
		f.PeriodStart = &d
	}
	return f
}

// TestFactContractFromBothSides drives one FactStore as the figures job (writer) and as the page (reader).
func TestFactContractFromBothSides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var w store.FactWriter = newMemFacts()
	s := w.(store.FactStore)
	var r store.FactReader = s

	duration := mkFact("cik:0000320193", "a1", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2025-02-01", "100")
	instant := mkFact("cik:0000320193", "a1", "Assets", "", "2024-12-31", "2025-02-01", "900")
	recast := mkFact("cik:0000320193", "a2", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2026-02-01", "95")
	other := mkFact("cik:0000789019", "b1", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2025-02-01", "7")

	// Writer side: a rerun adds nothing, for a duration and for an instant fact.
	for i := 0; i < 2; i++ {
		if err := w.InsertFacts(ctx, []contracts.FundamentalFact{duration, instant, recast, other}); err != nil {
			t.Fatal(err)
		}
	}

	// Reader side: every row of the subject filed on or before the cutoff, none of another subject,
	// and the recast beside the original once it is filed.
	got, err := r.FactsFiledBy(ctx, "sec", "cik:0000320193", "2025-12-31")
	if err != nil || len(got) != 2 {
		t.Fatalf("by 2025-12-31: %d rows, %v; want 2 (duration, instant)", len(got), err)
	}
	got, err = r.FactsFiledBy(ctx, "sec", "cik:0000320193", "2026-02-01")
	if err != nil || len(got) != 3 {
		t.Fatalf("by 2026-02-01: %d rows, %v; want 3 (the recast is kept beside the original)", len(got), err)
	}
	got, err = r.FactsFiledBy(ctx, "sec", "cik:0000999999", "2026-12-31")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("unknown subject: %v, %v; want an empty slice", got, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestFactsFiledBySourceParam: the reader takes the source beside the subject and returns only that source's rows.
func TestFactsFiledBySourceParam(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := newMemFacts()
	var r store.FactReader = m
	sec := mkFact("cik:0000320193", "a1", "Assets", "", "2024-12-31", "2025-02-01", "900")
	edinet := sec
	edinet.Source = "edinet"
	if err := m.InsertFacts(ctx, []contracts.FundamentalFact{sec, edinet}); err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{"sec", "edinet"} {
		got, err := r.FactsFiledBy(ctx, src, "cik:0000320193", "2026-11-09")
		if err != nil || len(got) != 1 || string(got[0].Source) != src {
			t.Errorf("source %s: %v, %v; want its one row", src, got, err)
		}
	}
	if got, _ := r.FactsFiledBy(ctx, "fmp", "cik:0000320193", "2026-11-09"); len(got) != 0 {
		t.Errorf("unknown source returned %d rows", len(got))
	}
}
