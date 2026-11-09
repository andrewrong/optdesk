package store_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

// memOptionLedger is a fake of OptionLedgerStore: insert-only, with the revision rule of Decision 5 in
// RecordFlexFetch and the dedupe rule of Decision 6 in ImportFlexTradeRows.
type memOptionLedger struct {
	rows    []store.FlexTradeRow
	fetches []store.FlexFetchRecord
	closed  bool
}

func (m *memOptionLedger) latest(alias, lod, id string) (store.FlexTradeRow, bool) {
	var best store.FlexTradeRow
	found := false
	for _, r := range m.rows {
		if r.Alias == alias && r.LevelOfDetail == lod && r.TransactionID == id && (!found || r.Revision > best.Revision) {
			best, found = r, true
		}
	}
	return best, found
}

func (m *memOptionLedger) RecordFlexFetch(_ context.Context, rec store.FlexFetchRecord, rows []store.FlexTradeRow) (store.FlexInsert, error) {
	var out store.FlexInsert
	for _, r := range rows {
		prev, ok := m.latest(r.Alias, r.LevelOfDetail, r.TransactionID)
		switch {
		case !ok:
			r.Revision = 1
			m.rows = append(m.rows, r)
			out.New++
		case prev.AttributesSHA256 == r.AttributesSHA256:
			out.Unchanged++
		default:
			r.Revision = prev.Revision + 1
			m.rows = append(m.rows, r)
			out.Revised++
		}
	}
	m.fetches = append(m.fetches, rec)
	return out, nil
}

func (m *memOptionLedger) ImportFlexTradeRows(_ context.Context, rows []store.FlexTradeRow) (store.FlexImportResult, error) {
	var out store.FlexImportResult
	for _, r := range rows {
		same, conflict := false, false
		for _, s := range m.rows {
			if s.Alias != r.Alias || s.LevelOfDetail != r.LevelOfDetail || s.TransactionID != r.TransactionID {
				continue
			}
			if s.AttributesSHA256 == r.AttributesSHA256 {
				same = true
			} else if s.Revision == r.Revision {
				conflict = true
			}
		}
		switch {
		case same:
			out.Same++
		case conflict:
			out.Conflict++
		default:
			m.rows = append(m.rows, r)
			out.Inserted++
		}
	}
	return out, nil
}

func (m *memOptionLedger) FlexTradeRows(_ context.Context, alias string) ([]store.FlexTradeRow, error) {
	out := []store.FlexTradeRow{}
	for _, r := range m.rows {
		if r.Alias != alias {
			continue
		}
		if l, _ := m.latest(r.Alias, r.LevelOfDetail, r.TransactionID); l.Revision == r.Revision {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TransactionID < out[j].TransactionID })
	return out, nil
}

func (m *memOptionLedger) LatestOkFetch(_ context.Context, login, queryID string) (store.FlexFetchRecord, bool, error) {
	var best store.FlexFetchRecord
	found := false
	for _, f := range m.fetches {
		if f.Login == login && f.QueryID == queryID && f.Status == "ok" && (!found || f.FinishedAt.After(best.FinishedAt)) {
			best, found = f, true
		}
	}
	return best, found, nil
}

func (m *memOptionLedger) StoredAliases(_ context.Context) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, r := range m.rows {
		if !seen[r.Alias] {
			seen[r.Alias] = true
			out = append(out, r.Alias)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (m *memOptionLedger) Close() error { m.closed = true; return nil }

var (
	_ store.OptionLedgerWriter   = (*memOptionLedger)(nil)
	_ store.OptionLedgerReader   = (*memOptionLedger)(nil)
	_ store.OptionLedgerStore    = (*memOptionLedger)(nil)
	_ store.OpenOptionLedgerFunc = func(context.Context, string) (store.OptionLedgerStore, error) { return &memOptionLedger{}, nil }
)

func ledgerRow(alias, id, hash string) store.FlexTradeRow {
	return store.FlexTradeRow{
		Alias: alias, LevelOfDetail: "EXECUTION", TransactionID: id,
		Attributes: `{"x":"` + hash + `"}`, AttributesSHA256: hash, Source: "flex",
	}
}

// TestOptionLedgerWriterContract drives the fake as the job (RecordFlexFetch, Decision 5) and as the
// import (ImportFlexTradeRows, Decision 6), then reads back through the reader.
func TestOptionLedgerWriterContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := &memOptionLedger{}
	var w store.OptionLedgerWriter = m
	var r store.OptionLedgerReader = m

	if store.DSNEnvOptionLedger != "OT_PG_DSN_OPTION_LEDGER" {
		t.Fatalf("DSNEnvOptionLedger = %q", store.DSNEnvOptionLedger)
	}
	rec := store.FlexFetchRecord{Login: "main", QueryID: "1", Status: "ok", FinishedAt: time.Date(2026, 11, 9, 22, 0, 0, 0, time.UTC)}

	got, err := w.RecordFlexFetch(ctx, rec, []store.FlexTradeRow{ledgerRow("a", "t1", "h1"), ledgerRow("a", "t2", "h2")})
	if err != nil || got != (store.FlexInsert{New: 2}) {
		t.Fatalf("first fetch = %+v, %v; want 2 new", got, err)
	}
	got, err = w.RecordFlexFetch(ctx, rec, []store.FlexTradeRow{ledgerRow("a", "t1", "h1"), ledgerRow("a", "t2", "h2b")})
	if err != nil || got != (store.FlexInsert{Unchanged: 1, Revised: 1}) {
		t.Fatalf("second fetch = %+v, %v; want 1 unchanged, 1 revised", got, err)
	}

	// Decision 6: the import writes a new key, skips an equal hash, counts a changed hash at a taken key.
	res, err := w.ImportFlexTradeRows(ctx, []store.FlexTradeRow{
		func() store.FlexTradeRow {
			x := ledgerRow("a", "t3", "h3")
			x.Revision = 1
			x.Source = "ot_import"
			return x
		}(),
		func() store.FlexTradeRow {
			x := ledgerRow("a", "t1", "h1")
			x.Revision = 1
			x.Source = "ot_import"
			return x
		}(),
		func() store.FlexTradeRow {
			x := ledgerRow("a", "t2", "other")
			x.Revision = 2
			x.Source = "ot_import"
			return x
		}(),
	})
	if err != nil || res != (store.FlexImportResult{Inserted: 1, Same: 1, Conflict: 1}) {
		t.Fatalf("import = %+v, %v; want 1 inserted, 1 same, 1 conflict", res, err)
	}

	rows, err := r.FlexTradeRows(ctx, "a")
	if err != nil || len(rows) != 3 {
		t.Fatalf("rows = %v, %v; want 3", rows, err)
	}
	if rows[1].TransactionID != "t2" || rows[1].Revision != 2 || rows[1].AttributesSHA256 != "h2b" {
		t.Fatalf("t2 = %+v; want the highest revision", rows[1])
	}
	if rows[0].TransactionID != "t1" || rows[2].TransactionID != "t3" {
		t.Fatalf("rows not sorted by transaction id: %v", rows)
	}
}

// TestOptionLedgerReaderContract checks the reader alone: an empty alias gives an empty non-nil slice,
// LatestOkFetch skips a failed fetch, StoredAliases lists each alias once, Close ends the store.
func TestOptionLedgerReaderContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := &memOptionLedger{}
	var s store.OptionLedgerStore = m

	rows, err := s.FlexTradeRows(ctx, "b")
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("empty alias = %v, %v; want empty non-nil slice", rows, err)
	}
	if _, ok, err := s.LatestOkFetch(ctx, "main", "1"); err != nil || ok {
		t.Fatalf("no fetch: ok=%v err=%v", ok, err)
	}
	t1 := time.Date(2026, 11, 8, 22, 0, 0, 0, time.UTC)
	t2 := t1.Add(24 * time.Hour)
	for _, rec := range []store.FlexFetchRecord{
		{Login: "main", QueryID: "1", Status: "ok", FinishedAt: t1},
		{Login: "main", QueryID: "1", Status: "failed", FinishedAt: t2},
	} {
		if _, err := s.RecordFlexFetch(ctx, rec, []store.FlexTradeRow{ledgerRow("a", "t"+rec.Status, "h")}); err != nil {
			t.Fatal(err)
		}
	}
	rec, ok, err := s.LatestOkFetch(ctx, "main", "1")
	if err != nil || !ok || !rec.FinishedAt.Equal(t1) {
		t.Fatalf("LatestOkFetch = %+v, %v, %v; want the ok fetch", rec, ok, err)
	}
	al, err := s.StoredAliases(ctx)
	if err != nil || len(al) != 1 || al[0] != "a" {
		t.Fatalf("aliases = %v, %v", al, err)
	}
	if err := s.Close(); err != nil || !m.closed {
		t.Fatalf("Close = %v, closed=%v", err, m.closed)
	}
}
