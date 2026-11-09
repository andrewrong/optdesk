package optionledger_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optionledger"
	"github.com/andrewrong/optdesk/internal/store"
)

var importNow = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

// fakeLedger applies Decision 6 over rows keyed by (alias, level, transaction, revision).
type fakeLedger struct {
	mu    sync.Mutex
	rows  []store.FlexTradeRow
	calls int
}

func (f *fakeLedger) RecordFlexFetch(_ context.Context, _ store.FlexFetchRecord, rows []store.FlexTradeRow) (store.FlexInsert, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out store.FlexInsert
	for _, r := range rows {
		latest := -1
		for i, s := range f.rows {
			if s.Alias == r.Alias && s.LevelOfDetail == r.LevelOfDetail && s.TransactionID == r.TransactionID &&
				(latest < 0 || s.Revision > f.rows[latest].Revision) {
				latest = i
			}
		}
		switch {
		case latest < 0:
			r.Revision = 1
			f.rows = append(f.rows, r)
			out.New++
		case f.rows[latest].AttributesSHA256 == r.AttributesSHA256:
			out.Unchanged++
		default:
			r.Revision = f.rows[latest].Revision + 1
			f.rows = append(f.rows, r)
			out.Revised++
		}
	}
	return out, nil
}

func (f *fakeLedger) ImportFlexTradeRows(_ context.Context, rows []store.FlexTradeRow) (store.FlexImportResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	var out store.FlexImportResult
	for _, r := range rows {
		same, keyHeld := false, false
		for _, s := range f.rows {
			if s.Alias != r.Alias || s.LevelOfDetail != r.LevelOfDetail || s.TransactionID != r.TransactionID {
				continue
			}
			if s.AttributesSHA256 == r.AttributesSHA256 {
				same = true
			}
			if s.Revision == r.Revision {
				keyHeld = true
			}
		}
		switch {
		case same:
			out.Same++
		case keyHeld:
			out.Conflict++
		default:
			f.rows = append(f.rows, r)
			out.Inserted++
		}
	}
	return out, nil
}

var mainSecond = map[string]string{"main": "a", "second": "b"}

func runImport(t *testing.T, w store.OptionLedgerWriter, text string, m map[string]string) (jobs.ImportResult, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "in.jsonl")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	return optionledger.ImportOptionTrader(context.Background(), f, w, m, importNow)
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func line(alias, tx, rev, attrs string) string {
	return `{"alias":"` + alias + `","level_of_detail":"EXECUTION","transaction_id":"` + tx + `","revision":` + rev +
		`,"attributes":"` + strings.ReplaceAll(attrs, `"`, `\"`) + `","attributes_sha256":"x","recorded_at":"2026-01-01T00:00:00+00:00"}` + "\n"
}

func TestImportMapsAliases(t *testing.T) {
	t.Parallel()
	w := &fakeLedger{}
	res, err := runImport(t, w, fixture(t, "ot_export.jsonl"), mainSecond)
	if err != nil {
		t.Fatal(err)
	}
	if res != (jobs.ImportResult{Lines: 3, Inserted: 3}) {
		t.Fatalf("result %+v", res)
	}
	got := map[string]int{}
	for _, r := range w.rows {
		got[r.Alias]++
		if r.Source != "ot_import" || !r.RecordedAt.Equal(importNow) {
			t.Errorf("row %+v", r)
		}
	}
	if got["a"] != 2 || got["b"] != 1 || len(got) != 2 {
		t.Fatalf("aliases %v", got)
	}
}

func TestImportUnknownAliasRefusesWholeFile(t *testing.T) {
	t.Parallel()
	w := &fakeLedger{}
	_, err := runImport(t, w, fixture(t, "ot_export.jsonl"), map[string]string{"main": "a"})
	if err == nil {
		t.Fatal("want refusal")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Errorf("error %q does not name the line", err)
	}
	if w.calls != 0 || len(w.rows) != 0 {
		t.Fatalf("written before refusal: %d calls, %d rows", w.calls, len(w.rows))
	}
}

func TestImportBadLineNamesLineNumber(t *testing.T) {
	t.Parallel()
	w := &fakeLedger{}
	_, err := runImport(t, w, fixture(t, "ot_export_bad.jsonl"), mainSecond)
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("error %v", err)
	}
	if strings.Contains(err.Error(), "SECRETVALUE") {
		t.Errorf("error holds content: %q", err)
	}
	if w.calls != 0 {
		t.Fatal("wrote before refusing")
	}
	for name, text := range map[string]string{
		"attributes text not object": line("main", "1", "1", `["x"]`),
		"attributes not valid":       line("main", "1", "1", `{bad`),
		"attributes number value":    line("main", "1", "1", `{"quantity":5}`),
		"not json":                   "hello\n",
	} {
		_, err := runImport(t, w, text, mainSecond)
		if err == nil || !strings.Contains(err.Error(), "line 1") {
			t.Errorf("%s: error %v", name, err)
		}
	}
}

func TestImportRecomputesHash(t *testing.T) {
	t.Parallel()
	w := &fakeLedger{}
	attrs := `{"b":"2","a":"1"}`
	if _, err := runImport(t, w, line("main", "7", "1", attrs), mainSecond); err != nil {
		t.Fatal(err)
	}
	want, err := optionledger.AttributesSHA256(map[string]string{"a": "1", "b": "2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(w.rows) != 1 || w.rows[0].AttributesSHA256 != want || w.rows[0].AttributesSHA256 == "x" {
		t.Fatalf("rows %+v want hash %s", w.rows, want)
	}
}

func TestImportAccountAttributeRefused(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"accountId", "acctAlias"} {
		w := &fakeLedger{}
		text := line("main", "1", "1", `{"symbol":"A"}`) + line("main", "2", "1", `{"`+key+`":"U1","symbol":"A"}`)
		_, err := runImport(t, w, text, mainSecond)
		if err == nil || !strings.Contains(err.Error(), "line 2") {
			t.Fatalf("%s: error %v", key, err)
		}
		if w.calls != 0 {
			t.Fatalf("%s: wrote", key)
		}
	}
}

func TestImportSameRowTwiceCountsSame(t *testing.T) {
	t.Parallel()
	w := &fakeLedger{}
	text := fixture(t, "ot_export.jsonl")
	if _, err := runImport(t, w, text, mainSecond); err != nil {
		t.Fatal(err)
	}
	res, err := runImport(t, w, text, mainSecond)
	if err != nil {
		t.Fatal(err)
	}
	if res != (jobs.ImportResult{Lines: 3, Same: 3}) || len(w.rows) != 3 {
		t.Fatalf("result %+v rows %d", res, len(w.rows))
	}
}

func TestImportThenFetchWritesNothingNew(t *testing.T) {
	t.Parallel()
	w := &fakeLedger{}
	if _, err := runImport(t, w, line("main", "9", "1", `{"symbol":"A","quantity":"1"}`), mainSecond); err != nil {
		t.Fatal(err)
	}
	sum, err := optionledger.AttributesSHA256(map[string]string{"symbol": "A", "quantity": "1"})
	if err != nil {
		t.Fatal(err)
	}
	// a fetched row of the same transaction: same attributes, plus the account keys removed before hashing
	fetchSum, err := optionledger.AttributesSHA256(map[string]string{"symbol": "A", "quantity": "1", "accountId": "U1"})
	if err != nil || fetchSum != sum {
		t.Fatalf("fetch hash %s %v, import hash %s", fetchSum, err, sum)
	}
	ins, err := w.RecordFlexFetch(context.Background(), store.FlexFetchRecord{}, []store.FlexTradeRow{
		{Alias: "a", LevelOfDetail: "EXECUTION", TransactionID: "9", Attributes: `{"symbol":"A","quantity":"1"}`, AttributesSHA256: fetchSum, Source: "flex"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ins != (store.FlexInsert{Unchanged: 1}) || len(w.rows) != 1 {
		t.Fatalf("insert %+v rows %d", ins, len(w.rows))
	}
}

func TestImportConflictCounted(t *testing.T) {
	t.Parallel()
	w := &fakeLedger{}
	if _, err := runImport(t, w, line("main", "5", "1", `{"symbol":"A"}`), mainSecond); err != nil {
		t.Fatal(err)
	}
	res, err := runImport(t, w, line("main", "5", "1", `{"symbol":"B"}`), mainSecond)
	if err != nil {
		t.Fatal(err)
	}
	if res != (jobs.ImportResult{Lines: 1, Conflict: 1}) || len(w.rows) != 1 {
		t.Fatalf("result %+v rows %d", res, len(w.rows))
	}
}

func TestImportPrintsCountsOnly(t *testing.T) {
	t.Parallel()
	w := &fakeLedger{}
	text := line("main", "1", "1", `{"symbol":"SECRETVALUE"}`) + "\n" + line("main", "2", "1", `{"symbol":"SECRETVALUE"}`)
	res, err := runImport(t, w, text, mainSecond)
	if err != nil {
		t.Fatal(err)
	}
	// blank lines are not counted
	if res != (jobs.ImportResult{Lines: 2, Inserted: 2}) {
		t.Fatalf("result %+v", res)
	}
	// a refusal never carries content either
	_, err = runImport(t, w, text+"SECRETVALUE\n", mainSecond)
	if err == nil || strings.Contains(err.Error(), "SECRETVALUE") || strings.Contains(err.Error(), "main") {
		t.Fatalf("error %v", err)
	}
	var buf bytes.Buffer
	buf.WriteString(err.Error())
	if !strings.Contains(buf.String(), "line 4") {
		t.Fatalf("error %q", buf.String())
	}
}
