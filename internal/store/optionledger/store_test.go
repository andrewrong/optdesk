package optionledger

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

var fixedNow = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

// fakeDB is an in-memory database that understands the statements of queries.go by their text. A transaction works
// on a copy and is kept only when it returns nil.
type fakeDB struct {
	rows    [][]any // the argument lists of the inserted rows, recorded_at filled in
	fetches [][]any
	txs     int
	failOn  string // an insert of the fetch record fails when set
}

type fakeTx struct{ db *fakeDB }

func (f *fakeDB) InTx(_ context.Context, fn func(tx) error) error {
	f.txs++
	rows, fetches := append([][]any(nil), f.rows...), append([][]any(nil), f.fetches...)
	if err := fn(fakeTx{f}); err != nil {
		f.rows, f.fetches = rows, fetches
		return err
	}
	return nil
}

func (f *fakeDB) Close() error { return nil }

func (t fakeTx) Exec(_ context.Context, sql string, args ...any) (int64, error) {
	switch sql {
	case insertRowSQL:
		for _, r := range t.db.rows {
			if reflect.DeepEqual(r[:4], args[:4]) {
				return 0, nil // ON CONFLICT DO NOTHING
			}
		}
		a := append([]any(nil), args...)
		if a[7].(*time.Time) == nil {
			a[7] = &fixedNow
		}
		t.db.rows = append(t.db.rows, a)
		return 1, nil
	case insertFetchSQL:
		if t.db.failOn != "" {
			return 0, errors.New(t.db.failOn)
		}
		t.db.fetches = append(t.db.fetches, args)
		return 1, nil
	}
	panic("unexpected statement: " + sql)
}

func (t fakeTx) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	return t.db.Select(ctx, sql, args, scan)
}

func (f *fakeDB) Select(_ context.Context, sql string, args []any, scan func(scanner) error) error {
	var got [][]any
	switch sql {
	case latestRevisionSQL, allRevisionsSQL:
		got = f.revisions(args, sql == latestRevisionSQL)
	case selectRowsSQL:
		got = f.highest(args[0].(string))
	case selectLatestOkSQL:
		got = f.latestOk(args)
	case selectAliasesSQL:
		got = f.aliases()
	default:
		panic("unexpected query: " + sql)
	}
	return replay(got, scan)
}

func (f *fakeDB) revisions(key []any, latestOnly bool) [][]any {
	var got [][]any
	for _, r := range f.rows {
		if reflect.DeepEqual(r[:3], key) {
			got = append(got, []any{r[3], r[5]})
		}
	}
	sort.Slice(got, func(i, j int) bool { return got[i][0].(int) > got[j][0].(int) })
	if latestOnly && len(got) > 1 {
		got = got[:1]
	}
	return got
}

func (f *fakeDB) highest(alias string) [][]any {
	best := map[string][]any{}
	for _, r := range f.rows {
		k := r[1].(string) + "/" + r[2].(string)
		if r[0] == alias && (best[k] == nil || best[k][3].(int) < r[3].(int)) {
			best[k] = r
		}
	}
	var picked [][]any
	for _, r := range best {
		picked = append(picked, r)
	}
	sort.Slice(picked, func(i, j int) bool { return picked[i][2].(string) < picked[j][2].(string) })
	var got [][]any
	for _, r := range picked {
		got = append(got, []any{r[0], r[1], r[2], r[3], r[4], r[5], r[6], *r[7].(*time.Time)})
	}
	return got
}

func (f *fakeDB) latestOk(key []any) [][]any {
	var got [][]any
	for _, a := range f.fetches {
		if a[0] == key[0] && a[1] == key[1] && a[6] == "ok" {
			got = append(got, a)
		}
	}
	sort.Slice(got, func(i, j int) bool { return got[i][5].(time.Time).After(got[j][5].(time.Time)) })
	if len(got) > 1 {
		got = got[:1]
	}
	return got
}

func (f *fakeDB) aliases() [][]any {
	seen := map[string]bool{}
	var got [][]any
	for _, r := range f.rows {
		if a := r[0].(string); !seen[a] {
			seen[a] = true
			got = append(got, []any{a})
		}
	}
	sort.Slice(got, func(i, j int) bool { return got[i][0].(string) < got[j][0].(string) })
	return got
}

func replay(rows [][]any, scan func(scanner) error) error {
	for _, r := range rows {
		if err := scan(argScanner(r)); err != nil {
			return err
		}
	}
	return nil
}

type argScanner []any

func (a argScanner) Scan(dest ...any) error {
	for i, d := range dest {
		reflect.ValueOf(d).Elem().Set(reflect.ValueOf(a[i]))
	}
	return nil
}

func newStore() (*Store, *fakeDB) {
	db := &fakeDB{}
	return &Store{db: db}, db
}

func row(tx, hash string) store.FlexTradeRow {
	return store.FlexTradeRow{
		Alias: "a", LevelOfDetail: "EXECUTION", TransactionID: tx,
		Attributes:       `{"assetCategory":"OPT","quantity":"-1","transactionID":"` + tx + `"}`,
		AttributesSHA256: hash,
	}
}

func fetchRec() store.FlexFetchRecord {
	return store.FlexFetchRecord{
		Login: "main", QueryID: "123", FirstDay: "2026-10-01", LastDay: "2026-11-08",
		StartedAt: fixedNow, FinishedAt: fixedNow.Add(time.Minute), Status: "ok", ParserVersion: "1",
		Statements: 1, RowsParsed: 3,
	}
}

func TestRecordNewRowIsRevisionOne(t *testing.T) {
	t.Parallel()
	s, db := newStore()
	got, err := s.RecordFlexFetch(context.Background(), fetchRec(), []store.FlexTradeRow{row("t1", "h1")})
	if err != nil || got != (store.FlexInsert{New: 1}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	if len(db.rows) != 1 || db.rows[0][3] != 1 || db.rows[0][6] != "flex" {
		t.Fatalf("stored = %v", db.rows)
	}
	if db.rows[0][7].(*time.Time) != &fixedNow {
		t.Error("recorded_at was not left to the database")
	}
	// A revision given by the caller is ignored.
	r := row("t2", "h2")
	r.Revision = 9
	if _, err := s.RecordFlexFetch(context.Background(), fetchRec(), []store.FlexTradeRow{r}); err != nil || db.rows[1][3] != 1 {
		t.Fatalf("revision = %v, %v", db.rows[1][3], err)
	}
}

func TestRecordEqualHashNotWritten(t *testing.T) {
	t.Parallel()
	s, db := newStore()
	ctx := context.Background()
	rows := []store.FlexTradeRow{row("t1", "h1")}
	if _, err := s.RecordFlexFetch(ctx, fetchRec(), rows); err != nil {
		t.Fatal(err)
	}
	got, err := s.RecordFlexFetch(ctx, fetchRec(), rows)
	if err != nil || got != (store.FlexInsert{Unchanged: 1}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	if len(db.rows) != 1 {
		t.Errorf("%d rows stored, want 1", len(db.rows))
	}
}

func TestRecordChangedHashNextRevision(t *testing.T) {
	t.Parallel()
	s, db := newStore()
	ctx := context.Background()
	for _, h := range []string{"h1", "h2", "h3"} {
		if _, err := s.RecordFlexFetch(ctx, fetchRec(), []store.FlexTradeRow{row("t1", h)}); err != nil {
			t.Fatal(err)
		}
	}
	if len(db.rows) != 3 || db.rows[1][3] != 2 || db.rows[2][3] != 3 {
		t.Fatalf("stored = %v", db.rows)
	}
	// Going back to the first hash is a change of the latest revision, so it is revision 4.
	got, err := s.RecordFlexFetch(ctx, fetchRec(), []store.FlexTradeRow{row("t1", "h1")})
	if err != nil || got != (store.FlexInsert{Revised: 1}) || db.rows[3][3] != 4 {
		t.Fatalf("got %+v, %v, rows %v", got, err, db.rows)
	}
}

func TestFetchAndRowsOneTransaction(t *testing.T) {
	t.Parallel()
	s, db := newStore()
	db.failOn = "boom"
	_, err := s.RecordFlexFetch(context.Background(), fetchRec(), []store.FlexTradeRow{row("t1", "h1"), row("t2", "h2")})
	if err == nil || db.txs != 1 {
		t.Fatalf("err = %v, transactions = %d", err, db.txs)
	}
	if len(db.rows) != 0 || len(db.fetches) != 0 {
		t.Errorf("a failed fetch left %d rows and %d records", len(db.rows), len(db.fetches))
	}
	db.failOn = ""
	got, err := s.RecordFlexFetch(context.Background(), fetchRec(), []store.FlexTradeRow{row("t1", "h1"), row("t2", "h2"), row("t1", "h1")})
	if err != nil || db.txs != 2 || len(db.fetches) != 1 || len(db.rows) != 2 {
		t.Fatalf("got %+v, %v, transactions %d, records %d, rows %d", got, err, db.txs, len(db.fetches), len(db.rows))
	}
	// The record carries the counts of this call, not those the caller guessed.
	if f := db.fetches[0]; f[14] != 2 || f[15] != 0 || f[16] != 1 {
		t.Errorf("counts new/revised/unchanged = %v/%v/%v", f[14], f[15], f[16])
	}
	// No rows still writes the record.
	if _, err := s.RecordFlexFetch(context.Background(), fetchRec(), nil); err != nil || len(db.fetches) != 2 {
		t.Errorf("no rows: %v, records %d", err, len(db.fetches))
	}
}

func imported(tx, hash string, rev int) store.FlexTradeRow {
	r := row(tx, hash)
	r.Revision, r.Source = rev, "flex"
	return r
}

func TestImportSameHashSkipped(t *testing.T) {
	t.Parallel()
	s, db := newStore()
	ctx := context.Background()
	if _, err := s.RecordFlexFetch(ctx, fetchRec(), []store.FlexTradeRow{row("t1", "h1")}); err != nil {
		t.Fatal(err)
	}
	// Same hash at another revision number is the same transaction state: not written.
	got, err := s.ImportFlexTradeRows(ctx, []store.FlexTradeRow{imported("t1", "h1", 5), imported("t2", "h2", 1), imported("t2", "h2", 1)})
	if err != nil || got != (store.FlexImportResult{Inserted: 1, Same: 2}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	if len(db.rows) != 2 || db.rows[1][6] != "ot_import" || db.rows[1][3] != 1 {
		t.Errorf("stored = %v", db.rows)
	}
	before := db.txs
	if got, err := s.ImportFlexTradeRows(ctx, nil); err != nil || got != (store.FlexImportResult{}) || db.txs != before {
		t.Errorf("no rows: %+v, %v, transactions %d", got, err, db.txs-before)
	}
}

func TestImportConflictCountedNotStored(t *testing.T) {
	t.Parallel()
	s, db := newStore()
	ctx := context.Background()
	if _, err := s.RecordFlexFetch(ctx, fetchRec(), []store.FlexTradeRow{row("t1", "h1")}); err != nil {
		t.Fatal(err)
	}
	got, err := s.ImportFlexTradeRows(ctx, []store.FlexTradeRow{imported("t1", "other", 1), imported("t1", "h9", 2)})
	if err != nil || got != (store.FlexImportResult{Inserted: 1, Conflict: 1}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	if len(db.rows) != 2 || db.rows[1][3] != 2 {
		t.Errorf("stored = %v", db.rows)
	}
}

func TestReaderHighestRevision(t *testing.T) {
	t.Parallel()
	s, _ := newStore()
	ctx := context.Background()
	if got, err := s.FlexTradeRows(ctx, "a"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty = %#v, %v", got, err)
	}
	for _, in := range []store.FlexTradeRow{row("t2", "h1"), row("t1", "h1"), row("t2", "h2")} {
		in.Alias = "a"
		if _, err := s.RecordFlexFetch(ctx, fetchRec(), []store.FlexTradeRow{in}); err != nil {
			t.Fatal(err)
		}
	}
	other := row("t3", "x")
	other.Alias = "b"
	if _, err := s.RecordFlexFetch(ctx, fetchRec(), []store.FlexTradeRow{other}); err != nil {
		t.Fatal(err)
	}
	got, err := s.FlexTradeRows(ctx, "a")
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got[0].TransactionID != "t1" || got[1].TransactionID != "t2" || got[1].Revision != 2 || got[1].AttributesSHA256 != "h2" ||
		got[1].Source != "flex" || !got[1].RecordedAt.Equal(fixedNow) {
		t.Errorf("rows = %+v", got)
	}
	if al, err := s.StoredAliases(ctx); err != nil || !reflect.DeepEqual(al, []string{"a", "b"}) {
		t.Errorf("aliases = %v, %v", al, err)
	}
	for _, want := range []string{"rd.flex_trade_row_r1", "DISTINCT ON", "revision DESC"} {
		if !strings.Contains(selectRowsSQL, want) {
			t.Errorf("the read query lacks %q", want)
		}
	}
}

func TestLatestOkFetchMapsRecord(t *testing.T) {
	t.Parallel()
	s, _ := newStore()
	ctx := context.Background()
	if _, ok, err := s.LatestOkFetch(ctx, "main", "123"); err != nil || ok {
		t.Fatalf("empty: %v, %v", ok, err)
	}
	bad := fetchRec()
	bad.Status, bad.StartedAt, bad.FinishedAt = "failed", fixedNow.Add(2*time.Hour), fixedNow.Add(3*time.Hour)
	for _, r := range []store.FlexFetchRecord{fetchRec(), bad} {
		if _, err := s.RecordFlexFetch(ctx, r, nil); err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err := s.LatestOkFetch(ctx, "main", "123")
	if err != nil || !ok {
		t.Fatalf("got %v, %v", ok, err)
	}
	want := fetchRec()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("record = %+v\nwant     %+v", got, want)
	}
}

func TestNoAccountColumn(t *testing.T) {
	t.Parallel()
	for _, typ := range []reflect.Type{reflect.TypeOf(store.FlexTradeRow{}), reflect.TypeOf(store.FlexFetchRecord{})} {
		for i := 0; i < typ.NumField(); i++ {
			if n := strings.ToLower(typ.Field(i).Name); strings.Contains(n, "account") || strings.Contains(n, "acct") {
				t.Errorf("%s has the field %s", typ.Name(), typ.Field(i).Name)
			}
		}
	}
	for _, sql := range []string{tradeColumns, fetchColumns, insertRowSQL, insertFetchSQL} {
		if l := strings.ToLower(sql); strings.Contains(l, "account") || strings.Contains(l, "acct") {
			t.Errorf("statement %q names an account", sql)
		}
	}
}

func TestStoreRefusesAccountAttributes(t *testing.T) {
	t.Parallel()
	s, db := newStore()
	ctx := context.Background()
	for _, key := range []string{"accountId", "acctAlias"} {
		bad := row("t1", "h1")
		bad.Attributes = `{"` + key + `":"X","quantity":"1"}`
		good := row("t2", "h2")
		if _, err := s.RecordFlexFetch(ctx, fetchRec(), []store.FlexTradeRow{good, bad}); err == nil {
			t.Errorf("%s: fetch accepted", key)
		}
		bad.Revision = 1
		if _, err := s.ImportFlexTradeRows(ctx, []store.FlexTradeRow{bad}); err == nil {
			t.Errorf("%s: import accepted", key)
		}
	}
	if db.txs != 0 || len(db.rows) != 0 {
		t.Errorf("a refused row reached the database: %d transactions, %d rows", db.txs, len(db.rows))
	}
	notObject := row("t3", "h3")
	notObject.Attributes = `["accountId"]`
	if _, err := s.RecordFlexFetch(ctx, fetchRec(), []store.FlexTradeRow{notObject}); err == nil {
		t.Error("attributes that are not an object were accepted")
	}
}

func TestStoredAttributesHoldNoAccountKey(t *testing.T) {
	t.Parallel()
	s, db := newStore()
	ctx := context.Background()
	if _, err := s.RecordFlexFetch(ctx, fetchRec(), []store.FlexTradeRow{row("t1", "h1"), row("t2", "h2")}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportFlexTradeRows(ctx, []store.FlexTradeRow{imported("t3", "h3", 1)}); err != nil {
		t.Fatal(err)
	}
	if len(db.rows) != 3 {
		t.Fatalf("%d rows", len(db.rows))
	}
	for _, r := range db.rows {
		var m map[string]any
		if err := json.Unmarshal([]byte(r[4].(string)), &m); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"accountId", "acctAlias"} {
			if _, ok := m[k]; ok {
				t.Errorf("stored attributes hold %s", k)
			}
		}
	}
}
