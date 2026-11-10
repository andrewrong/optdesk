package flexsectionpg

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/flexsection"
)

var t0 = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

// fakeDB is an in-memory database that understands the statements of queries.go by their text. A transaction
// works on a copy and is kept only when it returns nil.
type fakeDB struct {
	rows     [][]any // argument lists of the inserted rows
	statuses [][]any
	txs      int
	failOn   string // an insert of a status fails with this text when set
	racer    bool   // every row insert reports that the key was taken
}

type fakeTx struct{ db *fakeDB }

func (f *fakeDB) InTx(_ context.Context, fn func(tx) error) error {
	f.txs++
	rows, statuses := append([][]any(nil), f.rows...), append([][]any(nil), f.statuses...)
	if err := fn(fakeTx{f}); err != nil {
		f.rows, f.statuses = rows, statuses
		return err
	}
	return nil
}

func (f *fakeDB) Close() error { return nil }

func (t fakeTx) Exec(_ context.Context, sql string, args ...any) (int64, error) {
	switch sql {
	case insertRowSQL:
		if t.db.racer {
			return 0, nil
		}
		for _, r := range t.db.rows {
			if reflect.DeepEqual(r[:4], args[:4]) {
				return 0, nil
			}
		}
		t.db.rows = append(t.db.rows, append([]any(nil), args...))
		return 1, nil
	case insertStatusSQL:
		if t.db.failOn != "" {
			return 0, errors.New(t.db.failOn)
		}
		for _, s := range t.db.statuses {
			if reflect.DeepEqual(s[:3], args[:3]) {
				return 0, nil
			}
		}
		t.db.statuses = append(t.db.statuses, append([]any(nil), args...))
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
	case latestRevisionSQL:
		got = f.latest(args)
	case selectRowsSQL:
		got = f.highest(args[0].(string), args[1].(string))
	case selectStatusesSQL:
		got = f.newest(args[0].(string))
	default:
		panic("unexpected query: " + sql)
	}
	for _, r := range got {
		if err := scan(argScanner(r)); err != nil {
			return err
		}
	}
	return nil
}

// latest returns the revision and hash of the highest revision of the key.
func (f *fakeDB) latest(key []any) [][]any {
	var best []any
	for _, r := range f.rows {
		if reflect.DeepEqual(r[:3], key) && (best == nil || best[3].(int) < r[3].(int)) {
			best = r
		}
	}
	if best == nil {
		return nil
	}
	return [][]any{{best[3], best[7]}}
}

// highest returns the rows in the column order of rowColumns.
func (f *fakeDB) highest(alias, section string) [][]any {
	best := map[string][]any{}
	for _, r := range f.rows {
		k := r[2].(string)
		if r[0] == alias && r[1] == section && (best[k] == nil || best[k][3].(int) < r[3].(int)) {
			best[k] = r
		}
	}
	var out [][]any
	for _, r := range best {
		out = append(out, []any{r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7], stamp(r[8])})
	}
	sort.Slice(out, func(i, j int) bool { return out[i][2].(string) < out[j][2].(string) })
	return out
}

// newest returns the statuses in the column order of statusColumns.
func (f *fakeDB) newest(alias string) [][]any {
	best := map[string][]any{}
	for _, s := range f.statuses {
		if s[1] != alias {
			continue
		}
		k := s[2].(string)
		if best[k] == nil || !stamp(s[6]).(time.Time).Before(stamp(best[k][6]).(time.Time)) {
			best[k] = s
		}
	}
	var out [][]any
	for _, s := range best {
		out = append(out, []any{s[0], s[1], s[2], s[3], s[4], s[5], stamp(s[6])})
	}
	sort.Slice(out, func(i, j int) bool { return out[i][2].(string) < out[j][2].(string) })
	return out
}

// stamp turns the optional time of an insert into the time the database would store.
func stamp(v any) any {
	if p := v.(*time.Time); p != nil {
		return *p
	}
	return t0
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

func row(key, hash string) flexsection.Row {
	return flexsection.Row{Alias: "a", Section: "CashTransaction", RowKey: key, Attributes: `{"amount":"` + hash + `"}`,
		AttributesSHA256: hash, RecordedAt: t0}
}

func record(t *testing.T, s *Store, fetch string, rows ...flexsection.Row) flexsection.Insert {
	t.Helper()
	ins, err := s.RecordSections(context.Background(), fetch, rows, nil)
	if err != nil {
		t.Fatal(err)
	}
	return ins
}

func TestNewKeyIsRevisionOne(t *testing.T) {
	t.Parallel()
	s, _ := newStore()
	if got := record(t, s, "f1", row("k1", "h1"), row("k2", "h2")); got != (flexsection.Insert{New: 2}) {
		t.Fatalf("insert = %+v", got)
	}
	rows, err := s.Rows(context.Background(), "a", "CashTransaction")
	if err != nil || len(rows) != 2 || rows[0].Revision != 1 || rows[1].Revision != 1 || rows[0].FetchID != "f1" {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
}

func TestEqualHashNotWritten(t *testing.T) {
	t.Parallel()
	s, db := newStore()
	record(t, s, "f1", row("k1", "h1"))
	if got := record(t, s, "f2", row("k1", "h1")); got != (flexsection.Insert{Unchanged: 1}) {
		t.Fatalf("insert = %+v", got)
	}
	if len(db.rows) != 1 {
		t.Fatalf("%d rows stored", len(db.rows))
	}
}

func TestChangedHashNextRevision(t *testing.T) {
	t.Parallel()
	s, db := newStore()
	record(t, s, "f1", row("k1", "h1"))
	if got := record(t, s, "f2", row("k1", "h2")); got != (flexsection.Insert{Revised: 1}) {
		t.Fatalf("insert = %+v", got)
	}
	if got := record(t, s, "f3", row("k1", "h3")); got != (flexsection.Insert{Revised: 1}) {
		t.Fatalf("insert = %+v", got)
	}
	if len(db.rows) != 3 || db.rows[2][3] != 3 {
		t.Fatalf("rows = %+v", db.rows)
	}
	// the revision the caller carries is ignored
	r := row("k2", "x")
	r.Revision = 9
	record(t, s, "f4", r)
	if db.rows[3][3] != 1 {
		t.Fatalf("revision written = %v", db.rows[3][3])
	}
}

func TestRacerRowNotCounted(t *testing.T) {
	t.Parallel()
	s, db := newStore()
	db.racer = true
	if got := record(t, s, "f1", row("k1", "h1")); got != (flexsection.Insert{}) {
		t.Fatalf("insert = %+v", got)
	}
}

func TestRowsAndStatusesOneTransaction(t *testing.T) {
	t.Parallel()
	s, db := newStore()
	st := []flexsection.Status{{Alias: "a", Section: "CashTransaction", State: "present", RowCount: 1, RecordedAt: t0}}
	if _, err := s.RecordSections(context.Background(), "f1", []flexsection.Row{row("k1", "h1")}, st); err != nil {
		t.Fatal(err)
	}
	if db.txs != 1 || len(db.rows) != 1 || len(db.statuses) != 1 || db.statuses[0][0] != "f1" {
		t.Fatalf("txs=%d rows=%d statuses=%d", db.txs, len(db.rows), len(db.statuses))
	}
	// a failing status rolls the rows back
	db.failOn = "boom"
	_, err := s.RecordSections(context.Background(), "f2", []flexsection.Row{row("k2", "h2")}, st)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
	if len(db.rows) != 1 {
		t.Fatalf("rows kept after rollback: %d", len(db.rows))
	}
	// nothing at all makes no transaction
	before := db.txs
	if ins, err := s.RecordSections(context.Background(), "f3", nil, nil); err != nil || ins != (flexsection.Insert{}) || db.txs != before {
		t.Fatalf("empty call: %+v %v", ins, err)
	}
}

func TestRowWithAccountKeyRefused(t *testing.T) {
	t.Parallel()
	for _, attr := range []string{`{"accountId":"U1"}`, `{"acctAlias":"x","amount":"1"}`, `[]`, `nope`} {
		s, db := newStore()
		good := row("k0", "h0")
		bad := row("k1", "h1")
		bad.Attributes = attr
		_, err := s.RecordSections(context.Background(), "f", []flexsection.Row{good, bad}, nil)
		if err == nil {
			t.Fatalf("%s accepted", attr)
		}
		if db.txs != 0 || len(db.rows) != 0 {
			t.Fatalf("%s: written (txs %d, rows %d)", attr, db.txs, len(db.rows))
		}
	}
}

func TestReaderHighestRevision(t *testing.T) {
	t.Parallel()
	s, _ := newStore()
	record(t, s, "f1", row("b", "h1"), row("a", "h1"))
	record(t, s, "f2", row("a", "h2"))
	other := row("z", "h")
	other.Section = "EquitySummary"
	record(t, s, "f3", other)
	rows, err := s.Rows(context.Background(), "a", "CashTransaction")
	if err != nil || len(rows) != 2 || rows[0].RowKey != "a" || rows[0].Revision != 2 || rows[1].RowKey != "b" || rows[1].Revision != 1 {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
	if none, err := s.Rows(context.Background(), "nobody", "CashTransaction"); err != nil || none == nil || len(none) != 0 {
		t.Fatalf("none = %#v, %v", none, err)
	}
}

func TestStatusNewestPerSection(t *testing.T) {
	t.Parallel()
	s, _ := newStore()
	st := func(fetch, section, state string, at time.Time) flexsection.Status {
		return flexsection.Status{Alias: "a", Section: section, State: state, RecordedAt: at}
	}
	_, err := s.RecordSections(context.Background(), "f1", nil, []flexsection.Status{
		st("f1", "CashTransaction", "absent", t0), st("f1", "EquitySummary", "present", t0)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.RecordSections(context.Background(), "f2", nil, []flexsection.Status{
		st("f2", "CashTransaction", "present", t0.Add(time.Hour))})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Statuses(context.Background(), "a")
	if err != nil || len(got) != 2 || got[0].Section != "CashTransaction" || got[0].State != "present" ||
		got[0].FetchID != "f2" || got[1].State != "present" {
		t.Fatalf("statuses = %+v, %v", got, err)
	}
}

// TestNoAccountColumn shows that what is written holds the alias and the attributes and nothing that names an
// account: nine values for a row, seven for a status, none of them an account key.
func TestNoAccountColumn(t *testing.T) {
	t.Parallel()
	s, db := newStore()
	st := []flexsection.Status{{Alias: "a", Section: "CashTransaction", State: "present"}}
	if _, err := s.RecordSections(context.Background(), "f1", []flexsection.Row{row("k1", "h1")}, st); err != nil {
		t.Fatal(err)
	}
	if len(db.rows[0]) != 9 || db.rows[0][0] != "a" || len(db.statuses[0]) != 7 || db.statuses[0][1] != "a" {
		t.Fatalf("row %v, status %v", db.rows[0], db.statuses[0])
	}
	for _, arg := range append(append([]any(nil), db.rows[0]...), db.statuses[0]...) {
		if v, ok := arg.(string); ok && (strings.Contains(v, "accountId") || strings.Contains(v, "acctAlias")) {
			t.Fatalf("account key written: %q", v)
		}
	}
	rows, _ := s.Rows(context.Background(), "a", "CashTransaction")
	if len(rows) != 1 || rows[0].Alias != "a" {
		t.Fatalf("rows = %+v", rows)
	}
}
