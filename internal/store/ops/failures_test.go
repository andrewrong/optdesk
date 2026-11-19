package ops

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/coverage"
)

type errSelectDB struct{ fakeDB }

func (errSelectDB) Select(context.Context, string, []any, func(scanner) error) error {
	return errors.New("boom")
}

func TestRepeatedFailuresMapsRowsAndArgs(t *testing.T) {
	t.Parallel()
	db := &recDB{fakeDB: fakeDB{rows: [][]any{
		{"obs.bar", "AAPL", "failed", 3, 4, covDay("2026-11-03"), covDay("2026-11-12"), "retryable"},
		{"obs.bar", "AAPL", "empty", 5, 5, covDay("2026-11-01"), covDay("2026-11-11"), "empty"},
	}}}
	got, err := (&coverageReader{db: db}).RepeatedFailures(context.Background(), "2026-10-30", "2026-11-12")
	if err != nil {
		t.Fatal(err)
	}
	want := []coverage.Failure{
		{Dataset: "obs.bar", Key: "AAPL", Kind: coverage.FailureFailed, Days: 3, Runs: 4,
			FirstDay: "2026-11-03", LastDay: "2026-11-12", LastClass: "retryable"},
		{Dataset: "obs.bar", Key: "AAPL", Kind: coverage.FailureEmpty, Days: 5, Runs: 5,
			FirstDay: "2026-11-01", LastDay: "2026-11-11", LastClass: "empty"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if len(db.selArgs) != 1 || len(db.selArgs[0]) != 3 {
		t.Fatalf("args = %v, want from, to, min days", db.selArgs)
	}
	if db.selArgs[0][0] != covDay("2026-10-30") || db.selArgs[0][1] != covDay("2026-11-12") || db.selArgs[0][2] != coverage.FailureMinDays {
		t.Errorf("args = %v", db.selArgs[0])
	}
	if sql := db.selSQL[0]; !strings.Contains(sql, "rd.capture_run_r1") || strings.Contains(sql, "ops.") {
		t.Errorf("query must read rd.capture_run_r1 only: %s", sql)
	}
	if len(db.execSQL) != 0 {
		t.Errorf("the method must only read, exec = %v", db.execSQL)
	}
}

func TestRepeatedFailuresEmptyIsNotNil(t *testing.T) {
	t.Parallel()
	got, err := (&coverageReader{db: &recDB{}}).RepeatedFailures(context.Background(), "2026-11-01", "2026-11-12")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestRepeatedFailuresBadDayAndReadError(t *testing.T) {
	t.Parallel()
	if _, err := (&coverageReader{db: &recDB{}}).RepeatedFailures(context.Background(), "nope", "2026-11-12"); err == nil {
		t.Fatal("want an error for a bad day")
	}
	if _, err := (&coverageReader{db: &errSelectDB{}}).RepeatedFailures(context.Background(), "2026-11-01", "2026-11-12"); err == nil {
		t.Fatal("want the read error")
	}
}
