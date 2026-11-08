package ops

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// recDB is a fakeDB that also records the SQL and arguments of the queries.
type recDB struct {
	fakeDB
	selSQL  []string
	selArgs [][]any
	failTx  error
}

func (f *recDB) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	f.selSQL = append(f.selSQL, sql)
	f.selArgs = append(f.selArgs, args)
	return f.fakeDB.Select(ctx, sql, args, scan)
}

func (f *recDB) Exec(ctx context.Context, sql string, args ...any) error {
	if f.failTx != nil && len(f.execSQL) == 1 {
		return f.failTx
	}
	return f.fakeDB.Exec(ctx, sql, args...)
}

func (f *recDB) InTx(_ context.Context, fn func(querier) error) error {
	f.events = append(f.events, "begin")
	if err := fn(f); err != nil {
		f.events = append(f.events, "rollback")
		return err
	}
	f.events = append(f.events, "commit")
	return nil
}

func TestRenewLeaseUsesDatabaseNow(t *testing.T) {
	t.Parallel()
	exp := time.Date(2026, 11, 8, 9, 0, 30, 0, time.FixedZone("x", 3600))
	f := &recDB{fakeDB: fakeDB{rows: [][]any{{"scheduler-1", exp}}}}
	s := &schedLedger{db: f}
	got, err := s.RenewLease(context.Background(), store.Lease{Owner: "scheduler-1", ExpiresAt: time.Unix(1, 0)}, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.selSQL[0], "now()") {
		t.Errorf("sql lacks now(): %s", f.selSQL[0])
	}
	// the arguments are the owner and the ttl in seconds; no instant of the caller's clock is sent
	if len(f.selArgs[0]) != 2 || f.selArgs[0][0] != "scheduler-1" || f.selArgs[0][1] != 30.0 {
		t.Errorf("args = %v", f.selArgs[0])
	}
	if !got.ExpiresAt.Equal(exp) || got.ExpiresAt.Location() != time.UTC || got.Owner != "scheduler-1" {
		t.Errorf("lease = %+v", got)
	}
}

func TestRenewLeaseHeldByOtherIsRetryable(t *testing.T) {
	t.Parallel()
	s := &schedLedger{db: &recDB{}}
	_, err := s.RenewLease(context.Background(), store.Lease{Owner: "a"}, time.Second)
	if err == nil || kind(t, err) != contracts.Retryable {
		t.Fatalf("err = %v, want Retryable", err)
	}
}

func TestPutGapIdempotent(t *testing.T) {
	t.Parallel()
	f := &recDB{}
	s := &schedLedger{db: f}
	g := store.Gap{DatasetID: "obs.series_point", Day: "2026-11-05", Reason: "expired"}
	for range 2 {
		if err := s.PutGap(context.Background(), g); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.execSQL) != 2 || f.execSQL[0] != f.execSQL[1] {
		t.Fatalf("statements = %v", f.execSQL)
	}
	sql := f.execSQL[0]
	if !strings.Contains(sql, "ON CONFLICT") || !strings.Contains(sql, "cleared_at IS NULL") {
		t.Errorf("not an upsert of the open gap: %s", sql)
	}
	if strings.Contains(sql, "first_seen_at") && strings.Contains(sql, "DO UPDATE SET first_seen_at") {
		t.Errorf("update rewrites first_seen_at: %s", sql)
	}
	a := f.execArgs[0]
	if len(a) != 4 || a[0] != "" || a[1] != "obs.series_point" || a[2] != time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC) || a[3] != "expired" {
		t.Errorf("args = %v", a)
	}
}

func TestPutGapBadDayIsPermanent(t *testing.T) {
	t.Parallel()
	s := &schedLedger{db: &recDB{}}
	err := s.PutGap(context.Background(), store.Gap{Day: "yesterday"})
	if err == nil || kind(t, err) != contracts.Permanent {
		t.Fatalf("err = %v, want Permanent", err)
	}
}

func TestClearGap(t *testing.T) {
	t.Parallel()
	f := &recDB{}
	s := &schedLedger{db: f}
	g := store.Gap{JobID: "job-a", Day: "2026-11-05", Reason: "preflight_failed"}
	if err := s.ClearGap(context.Background(), g); err != nil {
		t.Fatal(err)
	}
	sql := f.execSQL[0]
	if !strings.Contains(sql, "cleared_at = now()") || !strings.Contains(sql, "cleared_at IS NULL") {
		t.Errorf("sql = %s", sql)
	}
	if a := f.execArgs[0]; len(a) != 3 || a[0] != "job-a" || a[1] != "" {
		t.Errorf("args = %v", a)
	}
	// MarkGapReported takes the instant from the database too
	if err := s.MarkGapReported(context.Background(), g); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.execSQL[1], "reported_at = now()") {
		t.Errorf("sql = %s", f.execSQL[1])
	}
}

func TestExpectationFromWindowAndPresent(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	prev := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	due := time.Date(2026, 11, 5, 22, 0, 0, 0, time.UTC)
	f := &recDB{fakeDB: fakeDB{rows: [][]any{
		{"obs.series_point", day, &prev, &due, "partial", false},
		{"obs.option_eod", day, (*time.Time)(nil), (*time.Time)(nil), "", true},
	}}}
	r := &gapReader{db: f}
	got, err := r.Expectations(context.Background(), "2026-11-01", "2026-11-08")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.selSQL[0], "rd.expectation_r1") || strings.Contains(f.selSQL[0], "ops.") {
		t.Errorf("sql = %s", f.selSQL[0])
	}
	if a := f.selArgs[0]; len(a) != 2 || a[0] != time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) || a[1] != time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC) {
		t.Errorf("args = %v", a)
	}
	want := []store.Expectation{
		{DatasetID: "obs.series_point", ExpectedDay: "2026-11-05", PresentDay: "2026-11-04", DueBy: due, Gap: "partial"},
		{DatasetID: "obs.option_eod", ExpectedDay: "2026-11-05", Closed: true},
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if _, err := r.Expectations(context.Background(), "bad", "2026-11-08"); err == nil {
		t.Error("bad day accepted")
	}
}

func TestOpenGapsReadsView(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	first := time.Date(2026, 11, 5, 22, 1, 0, 0, time.UTC)
	rep := first.Add(time.Hour)
	f := &recDB{fakeDB: fakeDB{rows: [][]any{{"", "obs.series_point", day, "expired", first, &rep, (*time.Time)(nil)}}}}
	got, err := (&gapReader{db: f}).OpenGaps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.selSQL[0], "rd.job_gap_r1") {
		t.Errorf("sql = %s", f.selSQL[0])
	}
	want := store.Gap{DatasetID: "obs.series_point", Day: "2026-11-05", Reason: "expired", FirstSeenAt: first, ReportedAt: rep}
	if len(got) != 1 || got[0] != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestPutJobStatesReplacesRows(t *testing.T) {
	t.Parallel()
	f := &recDB{}
	s := &schedLedger{db: f}
	next := time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)
	states := []store.JobState{
		{JobID: "fred", NextStart: next, RestartSensitive: true},
		{JobID: "eod", DisabledReason: "no_credential"},
	}
	if err := s.PutJobStates(context.Background(), states); err != nil {
		t.Fatal(err)
	}
	if len(f.execSQL) != 3 || !strings.HasPrefix(f.execSQL[0], "DELETE FROM ops.scheduler_job") {
		t.Fatalf("statements = %v", f.execSQL)
	}
	if a := f.execArgs[1]; a[0] != "fred" || a[1] != next || a[2] != nil || a[3] != true || a[4] != "" {
		t.Errorf("row 1 args = %v", a)
	}
	if a := f.execArgs[2]; a[0] != "eod" || a[1] != nil || a[4] != "no_credential" {
		t.Errorf("row 2 args = %v", a)
	}
	if got := strings.Join(f.events, ","); got != "begin,commit" {
		t.Errorf("events = %s", got)
	}
	// an empty list still clears the table
	f2 := &recDB{}
	if err := (&schedLedger{db: f2}).PutJobStates(context.Background(), nil); err != nil || len(f2.execSQL) != 1 {
		t.Errorf("empty: err = %v, statements = %v", err, f2.execSQL)
	}
}

func TestPutJobStatesRollsBackOnError(t *testing.T) {
	t.Parallel()
	f := &recDB{failTx: errors.New("boom")}
	err := (&schedLedger{db: f}).PutJobStates(context.Background(), []store.JobState{{JobID: "a"}})
	if err == nil || kind(t, err) != contracts.Retryable {
		t.Fatalf("err = %v, want Retryable", err)
	}
	if got := strings.Join(f.events, ","); got != "begin,rollback" {
		t.Errorf("events = %s", got)
	}
}
