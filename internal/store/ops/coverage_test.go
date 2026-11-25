package ops

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

func covDay(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestCellsFromRows(t *testing.T) {
	t.Parallel()
	db := &recDB{rows: [][]any{
		{covDay("2026-11-12"), "obs.series_point", 3, 1, 1, 0, 1, []string{"DGS10"}},
		{covDay("2026-11-12"), "obs.bar", 2, 0, 0, 2, 0, []string{}},
	}}
	got, err := (&coverageReader{db: db}).Cells(context.Background(), "2026-11-01", "2026-11-12")
	if err != nil {
		t.Fatal(err)
	}
	want := []coverage.Cell{
		{Day: "2026-11-12", Dataset: "obs.series_point", Keys: 3, OK: 1, NoValue: 1, Missing: 1, MissingKeys: []string{"DGS10"}},
		{Day: "2026-11-12", Dataset: "obs.bar", Keys: 2, Expired: 2, MissingKeys: []string{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if len(db.selArgs[0]) != 2 {
		t.Errorf("args = %v, want the two days", db.selArgs[0])
	}
}

func TestCellsEmptyIsNotNil(t *testing.T) {
	t.Parallel()
	got, err := (&coverageReader{db: &recDB{}}).Cells(context.Background(), "2026-11-01", "2026-11-12")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestCellsBadDay(t *testing.T) {
	t.Parallel()
	if _, err := (&coverageReader{db: &recDB{}}).Cells(context.Background(), "nope", "2026-11-12"); err == nil {
		t.Fatal("want an error for a bad day")
	}
}

func TestSessionsAscending(t *testing.T) {
	t.Parallel()
	db := &recDB{rows: [][]any{
		{"XNYS", covDay("2026-11-13"), true},
		{"XNYS", covDay("2026-11-12"), true},
		{"XNYS", covDay("2026-11-14"), false},
	}}
	got, err := (&coverageReader{db: db}).Sessions(context.Background(), "XNYS", "2026-11-12", "2026-11-14")
	if err != nil {
		t.Fatal(err)
	}
	want := []coverage.Session{
		{Calendar: "XNYS", Day: "2026-11-12", IsSession: true},
		{Calendar: "XNYS", Day: "2026-11-13", IsSession: true},
		{Calendar: "XNYS", Day: "2026-11-14", IsSession: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if db.selArgs[0][0] != "XNYS" {
		t.Errorf("args = %v", db.selArgs[0])
	}
}

func TestNewestObservations(t *testing.T) {
	t.Parallel()
	db := &recDB{rows: [][]any{{"DGS10", covDay("2026-11-12")}}}
	got, err := (&coverageReader{db: db}).NewestObservations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []coverage.Newest{{SeriesID: "DGS10", Day: contracts.Day("2026-11-12")}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	empty, err := (&coverageReader{db: &recDB{}}).NewestObservations(context.Background())
	if err != nil || empty == nil {
		t.Fatalf("empty = %#v, %v", empty, err)
	}
}
