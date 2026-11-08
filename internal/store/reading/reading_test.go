package reading

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

type call struct {
	sql  string
	args []any
}

// fakeDB records every statement and keeps the readings it was given by key, as the table would.
type fakeDB struct {
	calls    []call
	keys     map[string]bool
	events   []string
	rows     map[string][][]any // canned rows of Select by SQL
	selArgs  []any
	closed   int
	failExec string // a statement prefix that fails
}

func newFake() *fakeDB { return &fakeDB{keys: map[string]bool{}, rows: map[string][][]any{}} }

type fakeTx struct{ f *fakeDB }

func (t fakeTx) Exec(_ context.Context, sql string, args ...any) (int64, error) {
	t.f.calls = append(t.f.calls, call{sql, args})
	if t.f.failExec != "" && strings.Contains(sql, t.f.failExec) {
		return 0, errors.New("boom")
	}
	if sql == insertReadingSQL {
		k := ""
		for _, a := range args[:6] {
			k += "|" + toString(a)
		}
		if t.f.keys[k] {
			return 0, nil
		}
		t.f.keys[k] = true
	}
	return 1, nil
}

func toString(a any) string {
	if v, ok := a.(time.Time); ok {
		return v.Format(time.DateOnly)
	}
	return fmt.Sprint(a)
}

// assign copies a canned column into the destination of Scan.
func assign(dest, v any) error {
	d := reflect.ValueOf(dest).Elem()
	if v == nil {
		d.Set(reflect.Zero(d.Type()))
		return nil
	}
	d.Set(reflect.ValueOf(v))
	return nil
}

func (f *fakeDB) InTx(_ context.Context, fn func(execer) error) error {
	f.events = append(f.events, "begin")
	if err := fn(fakeTx{f}); err != nil {
		f.events = append(f.events, "rollback")
		return err
	}
	f.events = append(f.events, "commit")
	return nil
}

func (f *fakeDB) Select(_ context.Context, sql string, args []any, scan func(scanner) error) error {
	f.calls = append(f.calls, call{sql, args})
	f.selArgs = args
	for _, r := range f.rows[sql] {
		if err := scan(rowScanner(r)); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Close() error { f.closed++; return nil }

type rowScanner []any

func (r rowScanner) Scan(dest ...any) error {
	if len(dest) != len(r) {
		return errors.New("column count")
	}
	for i, d := range dest {
		if err := assign(d, r[i]); err != nil {
			return err
		}
	}
	return nil
}

func rec(rev int, state string) contracts.ReadingRecord {
	val := "0.250000000000"
	r := contracts.ReadingRecord{
		Name: "market.ivp_own", Version: 1, SubjectKind: "market", Subject: "VIXCLS",
		Day: types.SerializableDate{Time: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)}, Rev: rev,
		Value: &val, Unit: "decimal", Origin: "compute_figures", Cutoff: "session_close",
		JobRunID: "11111111-1111-1111-1111-111111111111", RecordedAt: time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC),
		RowsSha256: strings.Repeat("a", 64),
	}
	setText(&r.State, state)
	tag := "x"
	r.Inputs = append(r.Inputs, zeroOf(r.Inputs))
	r.Inputs[0].InputTable, r.Inputs[0].CodeVersion, r.Inputs[0].ConventionVersion = "obs.series_point", "h", "c1"
	r.Inputs[0].Keys = []string{"k1", "k2"}
	r.Parameters = append(r.Parameters, zeroOf(r.Parameters))
	r.Parameters[0].Param, r.Parameters[0].ValueTag = "window", &tag
	return r
}

func zeroOf[T any](_ []T) T { var z T; return z }

func count(f *fakeDB, sql string) int {
	n := 0
	for _, c := range f.calls {
		if c.sql == sql {
			n++
		}
	}
	return n
}

func TestInsertSQLIdempotent(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{insertReadingSQL, insertInputSQL, insertInputKeySQL, insertParameterSQL} {
		if !strings.Contains(sql, "ON CONFLICT DO NOTHING") {
			t.Errorf("no ON CONFLICT DO NOTHING in %q", sql)
		}
	}
	f := newFake()
	s := &readingStore{db: f}
	rows := []contracts.ReadingRecord{rec(1, "ok")}
	n, err := s.InsertReadings(context.Background(), rows)
	if err != nil || n != 1 {
		t.Fatalf("first insert = %d, %v", n, err)
	}
	if got := strings.Join(f.events, ","); got != "begin,commit" {
		t.Errorf("events = %s, want one transaction", got)
	}
	if count(f, insertInputSQL) != 1 || count(f, insertInputKeySQL) != 2 || count(f, insertParameterSQL) != 1 {
		t.Errorf("children not written with the record: %d inputs, %d keys, %d params",
			count(f, insertInputSQL), count(f, insertInputKeySQL), count(f, insertParameterSQL))
	}
	n, err = s.InsertReadings(context.Background(), rows)
	if err != nil || n != 0 {
		t.Fatalf("replay = %d, %v; want 0 inserted", n, err)
	}
	if count(f, insertInputSQL) != 1 || count(f, insertParameterSQL) != 1 {
		t.Error("a replay wrote children")
	}
}

func TestInsertRollsBackOnChildFailure(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.failExec = "reading_parameter"
	n, err := (&readingStore{db: f}).InsertReadings(context.Background(), []contracts.ReadingRecord{rec(1, "ok")})
	if err == nil || n != 0 {
		t.Fatalf("got %d, %v; want error and 0", n, err)
	}
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Retryable {
		t.Errorf("error %v is not Retryable", err)
	}
	if got := strings.Join(f.events, ","); got != "begin,rollback" {
		t.Errorf("events = %s", got)
	}
}

func TestInsertEmptyMakesNoCall(t *testing.T) {
	t.Parallel()
	f := newFake()
	n, err := (&readingStore{db: f}).InsertReadings(context.Background(), nil)
	if n != 0 || err != nil || len(f.calls) != 0 || len(f.events) != 0 {
		t.Errorf("got %d, %v, %d calls", n, err, len(f.calls))
	}
}

func TestLatestPicksHighestRev(t *testing.T) {
	t.Parallel()
	f := newFake()
	day := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	val := "0.5"
	f.rows[selectLatestSQL] = [][]any{
		{"market.ivp_own", 1, "market", "VIXCLS", day, 2, &val, "decimal", "", "missing_input", nil, "compute_figures",
			"session_close", "11111111-1111-1111-1111-111111111111", day.Add(21 * time.Hour), strings.Repeat("b", 64)},
	}
	f.rows[selectInputsSQL] = [][]any{
		{1, day, 2, 0, "obs.series_point", "h", "c1"},
		{1, day, 1, 0, "stale", "h", "c1"}, // rev 1 is not the newest: ignored
	}
	f.rows[selectInputKeysSQL] = [][]any{{1, day, 2, 0, "k1"}, {1, day, 2, 0, "k2"}, {1, day, 1, 0, "old"}}
	f.rows[selectParametersSQL] = [][]any{{1, day, 2, "window", nil, strPtr("t")}}
	got, err := (&readingStore{db: f}).LatestReadings(context.Background(), "market.ivp_own", "market", "VIXCLS", "2026-11-01", "2026-11-30")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Rev != 2 || string(got[0].State) != "missing_input" {
		t.Fatalf("got %+v", got)
	}
	if len(got[0].Inputs) != 1 || got[0].Inputs[0].InputTable != "obs.series_point" || strings.Join(got[0].Inputs[0].Keys, ",") != "k1,k2" {
		t.Errorf("inputs = %+v", got[0].Inputs)
	}
	if len(got[0].Parameters) != 1 || *got[0].Parameters[0].ValueTag != "t" {
		t.Errorf("parameters = %+v", got[0].Parameters)
	}
	if !got[0].Day.Equal(day) || got[0].RecordedAt.Location() != time.UTC {
		t.Errorf("day %v recorded %v", got[0].Day, got[0].RecordedAt)
	}
	if !strings.Contains(selectLatestSQL, "rev DESC") || strings.Contains(selectLatestSQL, "rd.reading_r1") {
		t.Errorf("latest must order by rev and not use the winner view: %s", selectLatestSQL)
	}
	if want := (time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)); !f.selArgs[4].(time.Time).Equal(want) {
		t.Errorf("to = %v", f.selArgs[4])
	}
}

func strPtr(s string) *string { return &s }

func TestLatestEmptyAndBadDay(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &readingStore{db: f}
	got, err := s.LatestReadings(context.Background(), "n", "market", "S", "2026-11-01", "2026-11-02")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("got %v, %v; want empty non-nil", got, err)
	}
	if len(f.calls) != 1 {
		t.Errorf("%d queries for no reading, want 1", len(f.calls))
	}
	_, err = s.LatestReadings(context.Background(), "n", "market", "S", "bad", "2026-11-02")
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
		t.Errorf("bad day error = %v", err)
	}
}

func TestCloseClosesPool(t *testing.T) {
	t.Parallel()
	f := newFake()
	if err := (&readingStore{db: f}).Close(); err != nil || f.closed != 1 {
		t.Errorf("Close = %v, closed %d times", err, f.closed)
	}
}

func TestMigration0013Shape(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join(moduleRoot(t), "migrations", "0013_readings.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)
	for _, want := range []string{
		"CREATE SCHEMA IF NOT EXISTS readings", "CREATE TABLE readings.reading ", "CREATE TABLE readings.reading_input ",
		"CREATE TABLE readings.reading_input_key ", "CREATE TABLE readings.reading_parameter ",
		"CREATE VIEW rd.reading_r1", "PRIMARY KEY (name, version, subject_kind, subject, day, rev)",
		"numeric(38,12)", "GRANT USAGE ON SCHEMA readings TO ot_w_readings", "GRANT USAGE ON SCHEMA rd TO ot_w_readings",
		"GRANT SELECT ON rd.reading_r1 TO ot_w_readings, ot_ro_pages", "-- +goose Down",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("migration lacks %q", want)
		}
	}
	for _, unit := range []string{"decimal", "percent", "vol_point", "per_calendar_day", "count", "days", "money", "ratio", "text_tag"} {
		if !strings.Contains(sql, "'"+unit+"'") {
			t.Errorf("enum lacks %q", unit)
		}
	}
	up := strings.SplitN(sql, "-- +goose Down", 2)[0]
	for _, line := range strings.Split(up, "\n") {
		if strings.HasPrefix(line, "GRANT") || strings.Contains(line, "TO ot_w_readings") {
			for _, bad := range []string{"UPDATE", "DELETE", "TRUNCATE", "ALL"} {
				if strings.Contains(line, bad) {
					t.Errorf("grant %q gives %s", line, bad)
				}
			}
		}
	}
	if strings.Contains(sql, "ot_runner") {
		t.Error("migration names ot_runner")
	}
	if !strings.Contains(up, grantFour) {
		t.Error("INSERT, SELECT on the four tables is not granted to ot_w_readings")
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

const grantFour = "GRANT INSERT, SELECT ON readings.reading, readings.reading_input, readings.reading_input_key,\n" +
	"    readings.reading_parameter TO ot_w_readings"
