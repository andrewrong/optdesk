package optionopenpg

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/openquote"
)

var listAt = time.Date(2026, 11, 12, 11, 20, 0, 0, time.UTC)

func TestShortAndErrorKeepListedAtAndSession(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"short", "error"} {
		s, f := newStore()
		err := s.PutLists(context.Background(), []openquote.ListRecord{{UnderlyingID: 5, Expiry: "2026-12-18", Class: "AAPL",
			TriedAt: listAt, Outcome: outcome}})
		if err != nil {
			t.Fatal(err)
		}
		if len(f.sqls) != 1 || strings.Contains(f.sqls[0], "listed_at = EXCLUDED") ||
			strings.Contains(f.sqls[0], "listed_session = EXCLUDED") {
			t.Fatalf("%s: statement overwrites the listing: %v", outcome, f.sqls)
		}
		if got := f.args[0][len(f.args[0])-1]; got != outcome {
			t.Errorf("%s: outcome arg = %v", outcome, got)
		}
	}
}

func TestListedWritesEveryColumn(t *testing.T) {
	t.Parallel()
	s, f := newStore()
	err := s.PutLists(context.Background(), []openquote.ListRecord{{UnderlyingID: 5, Expiry: "2026-12-18", Class: "AAPL",
		ListedAt: listAt, ListedSession: "2026-11-12", Count: 516, TriedAt: listAt, Outcome: "listed"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.sqls[0], "listed_session = EXCLUDED.listed_session") || !strings.Contains(f.sqls[0], "count = EXCLUDED.count") {
		t.Fatalf("listed must update all columns: %s", f.sqls[0])
	}
	if f.args[0][5] != 516 {
		t.Errorf("args = %v", f.args[0])
	}
}

func TestPutListRunsNeverReplacesNonEmptyExpected(t *testing.T) {
	t.Parallel()
	s, f := newStore()
	err := s.PutListRuns(context.Background(), []openquote.ListRunRecord{{UnderlyingID: 5, Session: "2026-11-12",
		Expected: []string{"2026-12-18|AAPL"}, Listed: 1, Reason: "none", UpdatedAt: listAt}})
	if err != nil {
		t.Fatal(err)
	}
	q := f.sqls[0]
	for _, want := range []string{"expected IS NULL", "cardinality(master.option_list_run.expected) = 0",
		"cardinality(EXCLUDED.expected) > 0", "ELSE master.option_list_run.expected"} {
		if !strings.Contains(q, want) {
			t.Errorf("guard lacks %q: %s", want, q)
		}
	}
}

func TestPutListRunsWritesListedAsGiven(t *testing.T) {
	t.Parallel()
	s, f := newStore()
	err := s.PutListRuns(context.Background(), []openquote.ListRunRecord{{UnderlyingID: 5, Session: "2026-11-12",
		Expected: []string{}, Listed: 7, Reason: "some_short", UpdatedAt: listAt}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f.sqls[0], "listed = master.option_list_run.listed") || !strings.Contains(f.sqls[0], "listed = EXCLUDED.listed") {
		t.Errorf("listed is not the given value: %s", f.sqls[0])
	}
	if f.args[0][3] != 7 || f.args[0][4] != "some_short" {
		t.Errorf("args = %v", f.args[0])
	}
}

func TestListRunsAsksOnlyTheSession(t *testing.T) {
	t.Parallel()
	s, f := newStore()
	got, err := s.ListRuns(context.Background(), "2026-11-12")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
	d, _ := f.args[0][0].(time.Time)
	if !strings.Contains(f.sqls[0], "WHERE session = $1") || d.Format(time.DateOnly) != "2026-11-12" {
		t.Errorf("query %s args %v", f.sqls[0], f.args[0])
	}
	if _, err := s.ListRuns(context.Background(), "nope"); kind(err) != "Permanent" && err == nil {
		t.Errorf("bad day must fail: %v", err)
	}
}

func TestEmptyListWritesMakeNoDatabaseCall(t *testing.T) {
	t.Parallel()
	s, f := newStore()
	if err := s.PutLists(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if err := s.PutListRuns(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if f.inTx != 0 || len(f.sqls) != 0 {
		t.Fatalf("calls: %d, %v", f.inTx, f.sqls)
	}
}

func TestPutListsWritesBand(t *testing.T) {
	t.Parallel()
	s, f := newStore()
	band := openquote.ListRecord{UnderlyingID: 5, Expiry: "2026-12-18", Class: "AAPL", ListedAt: listAt, ListedSession: "2026-11-12",
		Count: 516, TriedAt: listAt, Outcome: "listed", BandLo: "180.5", BandHi: "220", Sigma: 0.31, SigmaSource: "iv", InBand: 120}
	none := openquote.ListRecord{UnderlyingID: 5, Expiry: "2026-12-24", Class: "AAPL", ListedAt: listAt, ListedSession: "2026-11-12",
		Count: 10, TriedAt: listAt, Outcome: "listed"}
	if err := s.PutLists(context.Background(), []openquote.ListRecord{band, none}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.sqls[0], "band_lo = EXCLUDED.band_lo") || !strings.Contains(f.sqls[0], "sigma_source = EXCLUDED.sigma_source") {
		t.Fatalf("listed must update the band: %s", f.sqls[0])
	}
	deref := func(v any) any {
		switch p := v.(type) {
		case *string:
			if p == nil {
				return nil
			}
			return *p
		case *float64:
			if p == nil {
				return nil
			}
			return *p
		case *int:
			if p == nil {
				return nil
			}
			return *p
		}
		return v
	}
	got := func(a []any) []any {
		out := []any{}
		for _, v := range a[8:] {
			out = append(out, deref(v))
		}
		return out
	}
	if want := []any{"180.5", "220", 0.31, 120, "iv"}; !reflect.DeepEqual(got(f.args[0]), want) {
		t.Errorf("band args = %v, want %v", got(f.args[0]), want)
	}
	if want := []any{nil, nil, nil, nil, nil}; !reflect.DeepEqual(got(f.args[1]), want) {
		t.Errorf("no band must write nulls: %v", got(f.args[1]))
	}
}

// rowsDB replays one row for Select.
type rowsDB struct {
	fakeDB
	row []any
}

type valsScanner []any

func (v valsScanner) Scan(dest ...any) error {
	for i, d := range dest {
		dv := reflect.ValueOf(d).Elem()
		switch {
		case v[i] == nil:
			dv.SetZero()
		case dv.Kind() == reflect.Pointer:
			p := reflect.New(dv.Type().Elem())
			p.Elem().Set(reflect.ValueOf(v[i]))
			dv.Set(p)
		default:
			dv.Set(reflect.ValueOf(v[i]))
		}
	}
	return nil
}

func (r *rowsDB) Select(_ context.Context, _ string, _ []any, scan func(scanner) error) error {
	return scan(valsScanner(r.row))
}

func TestListsReadsBand(t *testing.T) {
	t.Parallel()
	exp := time.Date(2026, 12, 18, 0, 0, 0, 0, time.UTC)
	cols := func(lo, hi, src any, sigma, inBand any) []any {
		return []any{exp, "AAPL", listAt, listAt, 516, listAt, "listed", lo, hi, sigma, inBand, src}
	}
	got, err := (&store{db: &rowsDB{row: cols("180.5", "220", "rv20", 0.31, 120)}}).Lists(context.Background(), 5)
	if err != nil || len(got) != 1 {
		t.Fatalf("%+v, %v", got, err)
	}
	if r := got[0]; r.BandLo != "180.5" || r.BandHi != "220" || r.Sigma != 0.31 || r.InBand != 120 || r.SigmaSource != "rv20" {
		t.Errorf("band = %+v", r)
	}
	got, err = (&store{db: &rowsDB{row: cols(nil, nil, nil, nil, nil)}}).Lists(context.Background(), 5)
	if err != nil || len(got) != 1 {
		t.Fatalf("%+v, %v", got, err)
	}
	if r := got[0]; r.BandLo != "" || r.BandHi != "" || r.Sigma != 0 || r.InBand != 0 || r.SigmaSource != "" {
		t.Errorf("null band must read as none: %+v", r)
	}
}

func TestShortOutcomeKeepsBand(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"short", "error"} {
		s, f := newStore()
		err := s.PutLists(context.Background(), []openquote.ListRecord{{UnderlyingID: 5, Expiry: "2026-12-18", Class: "AAPL",
			TriedAt: listAt, Outcome: outcome, BandLo: "1", BandHi: "2", SigmaSource: "iv"}})
		if err != nil {
			t.Fatal(err)
		}
		for _, col := range []string{"band_lo", "band_hi", "sigma", "in_band", "sigma_source"} {
			if strings.Contains(f.sqls[0], col) {
				t.Errorf("%s: statement touches %s: %s", outcome, col, f.sqls[0])
			}
		}
	}
}
