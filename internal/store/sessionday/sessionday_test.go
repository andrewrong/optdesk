package sessionday

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type ctxKey struct{}

// fakeDB records every database call with the context it got, and replays canned rows to Select.
type fakeDB struct {
	events   []string // "begin", "exec", "commit", "rollback", "select"
	ctxs     []context.Context
	execSQL  []string
	execArgs [][]any
	selSQL   string
	selArgs  []any
	rows     []contracts.SessionDay
	execErr  error
	closed   bool
}

func (f *fakeDB) InTx(ctx context.Context, fn func(execer) error) error {
	f.events = append(f.events, "begin")
	f.ctxs = append(f.ctxs, ctx)
	if err := fn(&fakeTx{f: f}); err != nil {
		f.events = append(f.events, "rollback")
		return err
	}
	f.events = append(f.events, "commit")
	return nil
}

type fakeTx struct{ f *fakeDB }

func (t *fakeTx) Exec(ctx context.Context, sql string, args ...any) error {
	t.f.events = append(t.f.events, "exec")
	t.f.ctxs = append(t.f.ctxs, ctx)
	t.f.execSQL = append(t.f.execSQL, sql)
	t.f.execArgs = append(t.f.execArgs, args)
	return t.f.execErr
}

func (f *fakeDB) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	f.events = append(f.events, "select")
	f.ctxs = append(f.ctxs, ctx)
	f.selSQL, f.selArgs = sql, args
	for _, r := range f.rows {
		if err := scan(rowScanner{r}); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Close() error { f.closed = true; return nil }

type rowScanner struct{ r contracts.SessionDay }

func (s rowScanner) Scan(dest ...any) error {
	*(dest[0].(*string)) = s.r.CalendarID
	*(dest[1].(*time.Time)) = s.r.SessionDay.Time
	*(dest[2].(*bool)) = s.r.IsSession
	*(dest[3].(*time.Time)) = s.r.OpenAt
	*(dest[4].(*time.Time)) = s.r.CloseAt
	*(dest[5].(*bool)) = s.r.EarlyClose
	return nil
}

func day(t *testing.T, s string) types.SerializableDate {
	t.Helper()
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatal(err)
	}
	return types.SerializableDate{Time: d}
}

func utc(y int, m time.Month, d, h, mi int) time.Time {
	return time.Date(y, m, d, h, mi, 0, 0, time.UTC)
}

func TestOpenSessionDayFuncShape(t *testing.T) {
	t.Parallel()
	var f store.OpenSessionDayFunc = Open
	if f == nil {
		t.Fatal("Open is nil")
	}
	var _ store.SessionDayStore = &sessionDayStore{}
}

func TestSQLStrings(t *testing.T) {
	t.Parallel()
	const wantUpsert = "INSERT INTO master.session_day (calendar_id, session_day, is_session, open_at, close_at, early_close) " +
		"VALUES ($1, $2, $3, $4, $5, $6) " +
		"ON CONFLICT (calendar_id, session_day) DO UPDATE SET " +
		"is_session = EXCLUDED.is_session, open_at = EXCLUDED.open_at, " +
		"close_at = EXCLUDED.close_at, early_close = EXCLUDED.early_close"
	const wantSelect = "SELECT calendar_id, session_day, is_session, open_at, close_at, early_close " +
		"FROM rd.session_day_r1 " +
		"WHERE calendar_id = $1 AND session_day >= $2 AND session_day <= $3 " +
		"ORDER BY session_day"
	if upsertSQL != wantUpsert {
		t.Errorf("upsert SQL:\n got %q\nwant %q", upsertSQL, wantUpsert)
	}
	if selectSQL != wantSelect {
		t.Errorf("select SQL:\n got %q\nwant %q", selectSQL, wantSelect)
	}
	if strings.Contains(selectSQL, "SELECT *") {
		t.Error("select uses SELECT *")
	}
}

func TestUpsertOneTransactionContextAndArgs(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	s := &sessionDayStore{db: f}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")
	rows := []contracts.SessionDay{
		{CalendarID: "XNYS", SessionDay: day(t, "2026-11-02"), IsSession: true, OpenAt: utc(2026, 11, 2, 14, 30), CloseAt: utc(2026, 11, 2, 21, 0)},
		{CalendarID: "XNYS", SessionDay: day(t, "2026-11-27"), IsSession: true, EarlyClose: true,
			OpenAt: utc(2026, 11, 27, 14, 30), CloseAt: utc(2026, 11, 27, 18, 0)},
	}
	if err := s.UpsertSessionDays(ctx, rows); err != nil {
		t.Fatal(err)
	}
	want := []string{"begin", "exec", "exec", "commit"}
	if !reflect.DeepEqual(f.events, want) {
		t.Fatalf("events = %v, want %v", f.events, want)
	}
	for i, c := range f.ctxs {
		if c.Value(ctxKey{}) != "marker" {
			t.Errorf("call %d did not get the caller's context", i)
		}
	}
	for _, q := range f.execSQL {
		if q != upsertSQL || !strings.Contains(q, "ON CONFLICT (calendar_id, session_day) DO UPDATE") {
			t.Errorf("exec SQL = %q", q)
		}
	}
	wantArgs := []any{"XNYS", rows[1].SessionDay.Time, true, rows[1].OpenAt, rows[1].CloseAt, true}
	if !reflect.DeepEqual(f.execArgs[1], wantArgs) {
		t.Errorf("args = %v, want %v", f.execArgs[1], wantArgs)
	}
}

func TestUpsertEmptyInputIsNoop(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	s := &sessionDayStore{db: f}
	if err := s.UpsertSessionDays(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSessionDays(context.Background(), []contracts.SessionDay{}); err != nil {
		t.Fatal(err)
	}
	if len(f.events) != 0 {
		t.Errorf("empty input made database calls: %v", f.events)
	}
}

func TestUpsertFailureRollsBackAndIsRetryable(t *testing.T) {
	t.Parallel()
	f := &fakeDB{execErr: errors.New("boom")}
	s := &sessionDayStore{db: f}
	err := s.UpsertSessionDays(context.Background(), []contracts.SessionDay{{CalendarID: "XNYS", SessionDay: day(t, "2026-11-02")}})
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Retryable {
		t.Fatalf("err = %v, want a Retryable TypedError", err)
	}
	if got := f.events[len(f.events)-1]; got != "rollback" {
		t.Errorf("last event = %q, want rollback", got)
	}
}

func TestSessionDaysRangeContextAndOrder(t *testing.T) {
	t.Parallel()
	canned := []contracts.SessionDay{
		{CalendarID: "XNYS", SessionDay: day(t, "2026-11-02"), IsSession: true, OpenAt: utc(2026, 11, 2, 14, 30), CloseAt: utc(2026, 11, 2, 21, 0)},
		{CalendarID: "XNYS", SessionDay: day(t, "2026-11-03"), IsSession: true, OpenAt: utc(2026, 11, 3, 14, 30), CloseAt: utc(2026, 11, 3, 21, 0)},
	}
	f := &fakeDB{rows: canned}
	s := &sessionDayStore{db: f}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")
	got, err := s.SessionDays(ctx, "XNYS", "2026-11-02", "2026-11-03")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, canned) {
		t.Errorf("rows = %v, want %v", got, canned)
	}
	if f.ctxs[0].Value(ctxKey{}) != "marker" {
		t.Error("Select did not get the caller's context")
	}
	for _, frag := range []string{"session_day >= $2", "session_day <= $3", "ORDER BY session_day"} {
		if !strings.Contains(f.selSQL, frag) {
			t.Errorf("select SQL lacks %q: %s", frag, f.selSQL)
		}
	}
	wantArgs := []any{"XNYS", canned[0].SessionDay.Time, canned[1].SessionDay.Time}
	if !reflect.DeepEqual(f.selArgs, wantArgs) {
		t.Errorf("args = %v, want %v", f.selArgs, wantArgs)
	}
}

func TestSessionDaysEmptyAndBadDay(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	s := &sessionDayStore{db: f}
	got, err := s.SessionDays(context.Background(), "XNYS", "2026-11-02", "2026-11-03")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("got %v, %v; want empty non-nil slice and nil error", got, err)
	}
	_, err = s.SessionDays(context.Background(), "XNYS", "not-a-day", "2026-11-03")
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
		t.Errorf("err = %v, want a Permanent TypedError", err)
	}
}

func TestCloseClosesDatabase(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	if err := (&sessionDayStore{db: f}).Close(); err != nil || !f.closed {
		t.Errorf("Close: err=%v closed=%v", err, f.closed)
	}
}
