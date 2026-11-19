//go:build pg

package ops

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

type fRun struct {
	key, day, status, errCode string
	returned, usable, minute  int
}

// fInsert writes run rows of dataset obs.bar; finished_at is the day at 12:00 plus the minute.
func fInsert(t *testing.T, c *pgx.Conn, runs ...fRun) {
	t.Helper()
	for i, r := range runs {
		covExec(t, c, `INSERT INTO ops.capture_run (run_id, session_day, dataset_id, subject_key, status, finished_at,
			returned_n, usable_n, error_code, lease_until)
			VALUES ($1, $2, 'obs.bar', $3, $4, ($2::date + time '12:00')::timestamptz + make_interval(mins => $5), $6, $7, $8,
			timestamptz '2100-01-01')`,
			fmt.Sprintf("r%d-%s-%s", i, r.key, r.day), r.day, r.key, r.status, r.minute, r.returned, r.usable, r.errCode)
	}
}

func failedRun(key, day string, minute int) fRun {
	return fRun{key: key, day: day, status: "done", errCode: "retryable", minute: minute}
}

func emptyRun(key, day string, minute int) fRun {
	return fRun{key: key, day: day, status: "done", returned: 5, minute: minute}
}

func okRun(key, day string, minute int) fRun {
	return fRun{key: key, day: day, status: "done", returned: 5, usable: 5, minute: minute}
}

func failuresRead(t *testing.T, name string, from, to contracts.Day) []coverage.Failure {
	t.Helper()
	r, err := OpenCoverage(context.Background(), connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	got, err := r.(coverage.FailureReader).RepeatedFailures(context.Background(), from, to)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(got, func(i, j int) bool {
		if got[i].Key != got[j].Key {
			return got[i].Key < got[j].Key
		}
		return got[i].Kind < got[j].Kind
	})
	return got
}

func TestRepeatedFailuresPG(t *testing.T) {
	t.Parallel()
	name := newRunsDatabase(t)
	c := connectAs(t, name, "")
	fInsert(t, c,
		// A: failed on 3 days, 4 runs, newest failed.
		failedRun("A", "2026-11-03", 0), failedRun("A", "2026-11-04", 0), failedRun("A", "2026-11-05", 0), failedRun("A", "2026-11-05", 9),
		// B: 5 failed runs on 2 days.
		failedRun("B", "2026-11-03", 0), failedRun("B", "2026-11-03", 1), failedRun("B", "2026-11-03", 2),
		failedRun("B", "2026-11-04", 0), failedRun("B", "2026-11-04", 1),
		// C: 3 failed runs on one day.
		failedRun("C", "2026-11-03", 0), failedRun("C", "2026-11-03", 1), failedRun("C", "2026-11-03", 2),
		// D: failed on 3 days, healed by an ok run.
		failedRun("D", "2026-11-03", 0), failedRun("D", "2026-11-04", 0), failedRun("D", "2026-11-05", 0), okRun("D", "2026-11-06", 0),
		// E: empty on 3 days.
		emptyRun("E", "2026-11-03", 0), emptyRun("E", "2026-11-04", 0), emptyRun("E", "2026-11-05", 0),
		// F: 3 failed days and 3 empty days, newest empty.
		failedRun("F", "2026-11-03", 0), failedRun("F", "2026-11-04", 0), failedRun("F", "2026-11-05", 0),
		emptyRun("F", "2026-11-06", 0), emptyRun("F", "2026-11-07", 0), emptyRun("F", "2026-11-08", 0),
		// G: failed on 3 days, then partial, abandoned, running and a clean nothing: none is the newest run.
		failedRun("G", "2026-11-03", 0), failedRun("G", "2026-11-04", 0), failedRun("G", "2026-11-05", 0),
		fRun{key: "G", day: "2026-11-06", status: "partial", usable: 3, returned: 3},
		fRun{key: "G", day: "2026-11-06", status: "abandoned", usable: 3, returned: 3, minute: 1},
		fRun{key: "G", day: "2026-11-06", status: "running", usable: 3, returned: 3, minute: 2},
		fRun{key: "G", day: "2026-11-06", status: "done", minute: 3},
		// H: partial runs with errors and empties are counted in neither kind.
		fRun{key: "H", day: "2026-11-03", status: "partial", errCode: "retryable"},
		fRun{key: "H", day: "2026-11-04", status: "partial", errCode: "retryable"},
		fRun{key: "H", day: "2026-11-05", status: "abandoned", errCode: "retryable"},
		// I: failed on 3 days, one of them before the window.
		failedRun("I", "2026-10-01", 0), failedRun("I", "2026-11-04", 0), failedRun("I", "2026-11-05", 0),
		// J: failed on 3 days, one of them after the window.
		failedRun("J", "2026-11-04", 0), failedRun("J", "2026-11-05", 0), failedRun("J", "2026-11-20", 0),
	)
	got := failuresRead(t, name, "2026-11-01", "2026-11-12")
	want := []coverage.Failure{
		{Dataset: "obs.bar", Key: "A", Kind: coverage.FailureFailed, Days: 3, Runs: 4, FirstDay: "2026-11-03", LastDay: "2026-11-05", LastClass: "retryable"},
		{Dataset: "obs.bar", Key: "E", Kind: coverage.FailureEmpty, Days: 3, Runs: 3, FirstDay: "2026-11-03", LastDay: "2026-11-05", LastClass: "empty"},
		{Dataset: "obs.bar", Key: "F", Kind: coverage.FailureEmpty, Days: 3, Runs: 3, FirstDay: "2026-11-06", LastDay: "2026-11-08", LastClass: "empty"},
		{Dataset: "obs.bar", Key: "F", Kind: coverage.FailureFailed, Days: 3, Runs: 3, FirstDay: "2026-11-03", LastDay: "2026-11-05", LastClass: "empty"},
		{Dataset: "obs.bar", Key: "G", Kind: coverage.FailureFailed, Days: 3, Runs: 3, FirstDay: "2026-11-03", LastDay: "2026-11-05", LastClass: "retryable"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	// The window cuts I and J: with the whole range both are listed.
	all := failuresRead(t, name, "2026-09-01", "2026-12-01")
	keys := map[string]bool{}
	for _, f := range all {
		keys[f.Key] = true
	}
	if !keys["I"] || !keys["J"] {
		t.Errorf("wide window should list I and J: %+v", all)
	}
}

func TestRepeatedFailuresTieByRunID(t *testing.T) {
	t.Parallel()
	name := newRunsDatabase(t)
	c := connectAs(t, name, "")
	ins := `INSERT INTO ops.capture_run (run_id, session_day, dataset_id, subject_key, status, finished_at,
		returned_n, usable_n, error_code, lease_until) VALUES ($1, $2, 'obs.bar', 'K', 'done',
		timestamptz '2026-11-05 12:00+00', $3, $4, $5, timestamptz '2100-01-01')`
	covExec(t, c, ins, "a1", "2026-11-03", 0, 0, "retryable")
	covExec(t, c, ins, "a2", "2026-11-04", 0, 0, "retryable")
	covExec(t, c, ins, "a3", "2026-11-05", 0, 0, "retryable")
	covExec(t, c, ins, "z9", "2026-11-05", 5, 5, "") // same finished_at, larger run_id: the newest, ok
	if got := failuresRead(t, name, "2026-11-01", "2026-11-12"); len(got) != 0 {
		t.Fatalf("healed key listed: %+v", got)
	}
}

func TestRepeatedFailuresTieFailedRunIDNewest(t *testing.T) {
	t.Parallel()
	name := newRunsDatabase(t)
	c := connectAs(t, name, "")
	ins := `INSERT INTO ops.capture_run (run_id, session_day, dataset_id, subject_key, status, finished_at,
		returned_n, usable_n, error_code, lease_until) VALUES ($1, $2, 'obs.bar', 'K', 'done',
		timestamptz '2026-11-05 12:00+00', $3, $4, $5, timestamptz '2100-01-01')`
	covExec(t, c, ins, "a1", "2026-11-03", 0, 0, "retryable")
	covExec(t, c, ins, "a2", "2026-11-04", 0, 0, "retryable")
	covExec(t, c, ins, "a3", "2026-11-05", 5, 5, "")
	covExec(t, c, ins, "z9", "2026-11-05", 0, 0, "retryable")
	if got := failuresRead(t, name, "2026-11-01", "2026-11-12"); len(got) != 1 {
		t.Fatalf("key K with newest run failed (by run_id) must be listed: %+v", got)
	}
}
