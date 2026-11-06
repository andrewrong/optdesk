package ops

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeDB records the statements and replays canned rows (one slice of column values per row).
type fakeDB struct {
	execSQL  []string
	execArgs [][]any
	rows     [][]any
	events   []string
}

func (f *fakeDB) Exec(_ context.Context, sql string, args ...any) error {
	f.execSQL = append(f.execSQL, sql)
	f.execArgs = append(f.execArgs, args)
	return nil
}

type rowScanner struct{ vals []any }

func (s rowScanner) Scan(dest ...any) error {
	for i, d := range dest {
		reflect.ValueOf(d).Elem().Set(reflect.ValueOf(s.vals[i]))
	}
	return nil
}

func (f *fakeDB) Select(_ context.Context, _ string, _ []any, scan func(scanner) error) error {
	for _, r := range f.rows {
		if err := scan(rowScanner{r}); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) InTx(_ context.Context, fn func(querier) error) error {
	f.events = append(f.events, "begin")
	if err := fn(f); err != nil {
		f.events = append(f.events, "rollback")
		return err
	}
	f.events = append(f.events, "commit")
	return nil
}

func (f *fakeDB) Close() error { return nil }

func kind(t *testing.T, err error) contracts.ErrorKind {
	t.Helper()
	var te contracts.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("err = %v, want a TypedError", err)
	}
	return te.Kind()
}

func TestRecordRunZeroTimesAreNull(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	l := &ledger{db: f}
	lease := time.Date(2026, 11, 5, 21, 0, 0, 0, time.UTC)
	r := store.Run{ID: "2026-11-05/obs.series_point/DGS10/1", Day: "2026-11-05",
		Key:    contracts.ExpectedKey{DatasetID: "obs.series_point", SubjectKey: "DGS10"},
		Status: store.RunRunning, LeaseUntil: lease, ReturnedN: 3, UsableN: 2, ErrorCode: "retryable", ReasonCode: "late"}
	if err := l.RecordRun(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	a := f.execArgs[0]
	// run_id, session_day, dataset_id, subject_key, status, lease_until, heartbeat_at, finished_at, returned_n, usable_n, error_code, reason_code
	if a[0] != r.ID || a[4] != "running" || a[5] != lease || a[6] != nil || a[7] != nil ||
		a[8] != 3 || a[9] != 2 || a[10] != "retryable" || a[11] != "late" {
		t.Errorf("args = %v", a)
	}
}

func TestRunsReadsNullAsZeroTime(t *testing.T) {
	t.Parallel()
	lease := time.Date(2026, 11, 5, 21, 0, 0, 0, time.UTC)
	var none *time.Time
	f := &fakeDB{rows: [][]any{{"r1", "obs.series_point", "DGS10", "running", &lease, none, none, 0, 0, "", ""}}}
	got, err := (&ledger{db: f}).Runs(context.Background(), "2026-11-05")
	if err != nil {
		t.Fatal(err)
	}
	want := store.Run{ID: "r1", Day: "2026-11-05", Key: contracts.ExpectedKey{DatasetID: "obs.series_point", SubjectKey: "DGS10"},
		Status: store.RunRunning, LeaseUntil: lease}
	if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if !got[0].HeartbeatAt.IsZero() || !got[0].FinishedAt.IsZero() {
		t.Error("NULL did not read back as the zero time")
	}
}

func TestWindowMissingIsNoValue(t *testing.T) {
	t.Parallel()
	_, err := (&ledger{db: &fakeDB{}}).Window(context.Background(), "obs.series_point", "2026-11-26")
	if k := kind(t, err); k != contracts.NoValue {
		t.Errorf("kind = %v, want NoValue", k)
	}
}

func TestWindowRoundTripKeyAndNull(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	l := &ledger{db: f}
	w := store.Window{DatasetID: "obs.series_point", Day: "2026-11-05", OpenAt: time.Date(2026, 11, 5, 14, 30, 0, 0, time.UTC)}
	if err := l.PutWindow(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	a := f.execArgs[0]
	if a[0] != "obs.series_point" || a[2] != w.OpenAt || a[3] != nil {
		t.Errorf("args = %v", a)
	}
	open := w.OpenAt
	var none *time.Time
	f.rows = [][]any{{&open, none}}
	got, err := l.Window(context.Background(), w.DatasetID, w.Day)
	if err != nil || !reflect.DeepEqual(got, w) {
		t.Errorf("got %+v, %v; want %+v", got, err, w)
	}
}

func TestAcquireHeldIsRetryable(t *testing.T) {
	t.Parallel()
	l := &ledger{db: &fakeDB{}} // no row returned: another owner holds the lease
	_, err := l.Acquire(context.Background(), "runner-a", time.Minute)
	if k := kind(t, err); k != contracts.Retryable {
		t.Errorf("kind = %v, want Retryable", k)
	}
	exp := time.Date(2026, 11, 5, 21, 1, 0, 0, time.UTC)
	l = &ledger{db: &fakeDB{rows: [][]any{{"runner-a", exp}}}}
	got, err := l.Acquire(context.Background(), "runner-a", time.Minute)
	if err != nil || got != (store.Lease{Owner: "runner-a", ExpiresAt: exp}) {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestPutExpectedReplacesPlanInOneTransaction(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	keys := []contracts.ExpectedKey{{DatasetID: "obs.series_point", SubjectKey: "DGS10"}}
	if err := (&ledger{db: f}).PutExpected(context.Background(), "2026-11-05", "h1", keys); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.events, []string{"begin", "commit"}) || len(f.execSQL) != 2 ||
		f.execSQL[0] != deleteExpectedSQL || f.execSQL[1] != insertExpectedSQL {
		t.Errorf("events %v, sql %v", f.events, f.execSQL)
	}
}

func TestBadDayIsPermanent(t *testing.T) {
	t.Parallel()
	_, err := (&ledger{db: &fakeDB{}}).Runs(context.Background(), "nope")
	if k := kind(t, err); k != contracts.Permanent {
		t.Errorf("kind = %v, want Permanent", k)
	}
}
