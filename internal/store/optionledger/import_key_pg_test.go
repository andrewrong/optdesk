//go:build pg

package optionledger

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

var importAt = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

func openImportStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(context.Background(), connURL(t, newDatabase(t), "ot_w_optionledger"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func importRec(login, query, status string) store.FlexFetchRecord {
	return store.FlexFetchRecord{Login: login, QueryID: query, StartedAt: importAt, FinishedAt: importAt.Add(time.Minute),
		Status: status, ParserVersion: "1", RowsParsed: 3}
}

func absentRows(n int) []store.FlexTradeRow {
	var rows []store.FlexTradeRow
	for i := 0; i < n; i++ {
		id := string(rune('a' + i))
		rows = append(rows, store.FlexTradeRow{Alias: "al", LevelOfDetail: "EXECUTION", TransactionID: "t-" + id,
			Attributes: `{"quantity":"1"}`, AttributesSHA256: "h-" + id})
	}
	return rows
}

func TestPGImportRecordsOfOneStartInstantWithTwoAliasesAreBothStored(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openImportStore(t)
	for _, q := range []string{"file:r1:1:alias1", "file:r1:1:alias2"} {
		if _, err := st.RecordFlexFetch(ctx, importRec("import", q, "ok"), nil); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	got, err := st.RecentFetches(ctx, "import", 10)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestPGTwoRefusedFilesOfOnePostAreBothReadAsFailed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openImportStore(t)
	for _, q := range []string{"file:r2:1:-", "file:r2:2:-"} {
		rec := importRec("import", q, "failed")
		rec.ErrorKind, rec.ErrorCode, rec.RowsParsed = "permanent", "not_flex_xml", 0
		if _, err := st.RecordFlexFetch(ctx, rec, nil); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	got, err := st.RecentFetches(ctx, "import", 10)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	for _, r := range got {
		if r.Status != "failed" || r.ErrorCode != "not_flex_xml" {
			t.Errorf("record = %+v", r)
		}
	}
}

func TestPGSameLoginQueryAndStartIsRefusedAndStoresNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openImportStore(t)
	rec := importRec("import", "file:r3:1:alias1", "ok")
	if _, err := st.RecordFlexFetch(ctx, rec, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := st.RecordFlexFetch(ctx, rec, absentRows(2)); err == nil {
		t.Fatal("the second record with the same key was accepted")
	}
	if got, err := st.RecentFetches(ctx, "import", 10); err != nil || len(got) != 1 {
		t.Errorf("records = %+v, %v", got, err)
	}
	if rows, err := st.FlexTradeRows(ctx, "al"); err != nil || len(rows) != 0 {
		t.Errorf("rows = %+v, %v", rows, err)
	}
}

func TestPGReimportRecordReadsBackWithCountsOfZero(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openImportStore(t)
	if _, err := st.RecordFlexFetch(ctx, importRec("import", "file:r4:1:alias1", "ok"), nil); err != nil {
		t.Fatal(err)
	}
	got, err := st.RecentFetches(ctx, "import", 10)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if r := got[0]; r.RowsParsed != 3 || r.RowsNew != 0 || r.RowsRevised != 0 || r.RowsUnchanged != 0 {
		t.Errorf("counts = %+v", r)
	}
}

func TestPGFirstImportRecordReadsBackWithRowsNew(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openImportStore(t)
	if _, err := st.RecordFlexFetch(ctx, importRec("import", "file:r5:1:alias1", "ok"), absentRows(3)); err != nil {
		t.Fatal(err)
	}
	got, err := st.RecentFetches(ctx, "import", 10)
	if err != nil || len(got) != 1 || got[0].RowsNew != 3 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestPGDailyRecordOfALoginNamedImportIsNotListed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openImportStore(t)
	if _, err := st.RecordFlexFetch(ctx, importRec("import", "123456", "ok"), nil); err != nil {
		t.Fatal(err)
	}
	if got, err := st.RecentFetches(ctx, "import", 10); err != nil || len(got) != 0 {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestPGLatestOkFetchNeverTakesAnImportRecord(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openImportStore(t)
	if _, err := st.RecordFlexFetch(ctx, importRec("import", "file:r6:1:alias1", "ok"), nil); err != nil {
		t.Fatal(err)
	}
	if rec, ok, err := st.LatestOkFetch(ctx, "import", "123456"); err != nil || ok {
		t.Errorf("LatestOkFetch = %+v, %v, %v", rec, ok, err)
	}
}
