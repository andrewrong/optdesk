package reading

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func viewRow(day time.Time, version, rev int, state string) []any {
	val := "0.5"
	return []any{"market.ivp_own", version, "market", "VIXCLS", day, rev, &val, "decimal", "", state, nil, "compute_figures",
		"session_close", "11111111-1111-1111-1111-111111111111", day.Add(21 * time.Hour), strings.Repeat("b", 64)}
}

func TestViewQueryShape(t *testing.T) {
	t.Parallel()
	for _, want := range []string{
		"FROM rd.reading_r1", "WHERE name = $1 AND subject_kind = $2 AND subject = $3 AND day >= $4 AND day <= $5",
		"ORDER BY day, version DESC",
	} {
		if !strings.Contains(selectWinnerSQL, want) {
			t.Errorf("winner statement lacks %q: %s", want, selectWinnerSQL)
		}
	}
	for _, want := range []string{"FROM rd.reading_parameter_r1", "WHERE name = $1 AND subject_kind = $2 AND subject = $3 AND day >= $4 AND day <= $5",
		"ORDER BY day, version, rev, param"} {
		if !strings.Contains(selectViewParametersSQL, want) {
			t.Errorf("parameter statement lacks %q: %s", want, selectViewParametersSQL)
		}
	}
}

func TestViewRowMapping(t *testing.T) {
	t.Parallel()
	f := newFake()
	day := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	f.rows[selectWinnerSQL] = [][]any{viewRow(day, 1, 2, "ok")}
	got, err := (&viewStore{db: f}).WinnerReadings(context.Background(), "market.ivp_own", "market", "VIXCLS", "2026-11-01", "2026-11-30")
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	r := got[0]
	if r.Rev != 2 || string(r.State) != "ok" || string(r.Unit) != "decimal" || *r.Value != "0.5" || !r.Day.Equal(day) {
		t.Errorf("row = %+v", r)
	}
	if r.RecordedAt.Location() != time.UTC {
		t.Errorf("recorded_at not UTC: %v", r.RecordedAt)
	}
	if r.Inputs == nil || len(r.Inputs) != 0 || r.Parameters == nil {
		t.Errorf("inputs %v parameters %v: want empty non-nil", r.Inputs, r.Parameters)
	}
	if want := time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC); !f.selArgs[4].(time.Time).Equal(want) {
		t.Errorf("to = %v", f.selArgs[4])
	}
	for _, c := range f.calls {
		if strings.Contains(c.sql, "readings.reading") {
			t.Errorf("view reader touched a table: %s", c.sql)
		}
	}
}

func TestViewKeepsDataDay(t *testing.T) {
	t.Parallel()
	f := newFake()
	day := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	f.rows[selectWinnerSQL] = [][]any{viewRow(day, 1, 2, "ok")}
	win := 20.0
	f.rows[selectViewParametersSQL] = [][]any{
		{1, day, 1, "data_day", nil, strPtr("2026-11-01")}, // rev 1 is not the winner: ignored
		{1, day, 2, "data_day", nil, strPtr("2026-11-05")},
		{1, day, 2, "window", &win, nil},
	}
	got, err := (&viewStore{db: f}).WinnerReadings(context.Background(), "market.ivp_own", "market", "VIXCLS", "2026-11-01", "2026-11-30")
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	ps := got[0].Parameters
	if len(ps) != 2 || ps[0].Param != "data_day" || ps[0].ValueTag == nil || *ps[0].ValueTag != "2026-11-05" {
		t.Fatalf("parameters = %+v", ps)
	}
}

func TestViewOneRowPerDayAfterVersionBump(t *testing.T) {
	t.Parallel()
	f := newFake()
	d1 := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	// The statement orders each day's rows by version DESC and the reader keeps the first: the fake gives them so.
	f.rows[selectWinnerSQL] = [][]any{
		viewRow(d1, 1, 1, "ok"),
		viewRow(d2, 2, 1, "ok"), viewRow(d2, 1, 3, "ok"),
	}
	f.rows[selectViewParametersSQL] = [][]any{
		{1, d2, 3, "data_day", nil, strPtr("old")},
		{2, d2, 1, "data_day", nil, strPtr("new")},
	}
	got, err := (&viewStore{db: f}).WinnerReadings(context.Background(), "market.ivp_own", "market", "VIXCLS", "2026-11-01", "2026-11-30")
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got[1].Version != 2 || len(got[1].Parameters) != 1 || *got[1].Parameters[0].ValueTag != "new" {
		t.Errorf("day 2 = version %d %+v", got[1].Version, got[1].Parameters)
	}
	if got[0].Version != 1 {
		t.Errorf("day 1 version %d", got[0].Version)
	}
}

func TestViewEmptyIsEmptySlice(t *testing.T) {
	t.Parallel()
	f := newFake()
	got, err := (&viewStore{db: f}).WinnerReadings(context.Background(), "n", "market", "S", "2026-11-01", "2026-11-02")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("got %v, %v; want empty non-nil", got, err)
	}
	if len(f.calls) != 1 {
		t.Errorf("%d queries for no row, want 1", len(f.calls))
	}
}

func TestViewRejectsReversedRange(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &viewStore{db: f}
	for _, c := range [][2]contracts.Day{{"2026-11-02", "2026-11-01"}, {"bad", "2026-11-01"}, {"2026-11-01", "bad"}} {
		_, err := s.WinnerReadings(context.Background(), "n", "market", "S", c[0], c[1])
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
			t.Errorf("%v: error = %v, want Permanent", c, err)
		}
	}
	if len(f.calls) != 0 {
		t.Errorf("%d queries made for a bad range", len(f.calls))
	}
	if err := s.Close(); err != nil || f.closed != 1 {
		t.Errorf("Close = %v, closed %d", err, f.closed)
	}
}
