package calendarrefresh

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

type fakeSource struct {
	rows     []contracts.SessionDay
	err      error
	cal      string
	from, to contracts.Day
}

func (s *fakeSource) SessionDays(_ context.Context, cal string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	s.cal, s.from, s.to = cal, from, to
	return s.rows, s.err
}

type fakeStore struct {
	rows    map[string]contracts.SessionDay
	upserts [][]contracts.SessionDay
	err     error
	readErr error
}

func dayKey(r contracts.SessionDay) string { return r.SessionDay.Format("2006-01-02") }

func newStore(rows ...contracts.SessionDay) *fakeStore {
	s := &fakeStore{rows: map[string]contracts.SessionDay{}}
	for _, r := range rows {
		s.rows[dayKey(r)] = r
	}
	return s
}

func (s *fakeStore) SessionDays(_ context.Context, cal string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	var out []contracts.SessionDay
	for k, r := range s.rows {
		if r.CalendarID == cal && k >= string(from) && k <= string(to) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(a, b int) bool { return dayKey(out[a]) < dayKey(out[b]) })
	return out, nil
}

func (s *fakeStore) UpsertSessionDays(_ context.Context, rows []contracts.SessionDay) error {
	if s.err != nil {
		return s.err
	}
	s.upserts = append(s.upserts, rows)
	for _, r := range rows {
		s.rows[dayKey(r)] = r
	}
	return nil
}

func (s *fakeStore) Close() error { return nil }

func (s *fakeStore) written() int {
	n := 0
	for _, u := range s.upserts {
		n += len(u)
	}
	return n
}

type fakeDiffer struct{}

func (fakeDiffer) Diff(old, fresh []contracts.SessionDay) []clock.SessionDayChange {
	o := map[string]contracts.SessionDay{}
	f := map[string]contracts.SessionDay{}
	days := map[string]bool{}
	for _, r := range old {
		o[dayKey(r)] = r
		days[dayKey(r)] = true
	}
	for _, r := range fresh {
		f[dayKey(r)] = r
		days[dayKey(r)] = true
	}
	var keys []string
	for k := range days {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []clock.SessionDayChange
	for _, k := range keys {
		a, inOld := o[k]
		b, inNew := f[k]
		c := clock.SessionDayChange{Day: contracts.Day(k)}
		switch {
		case inOld && inNew && a.IsSession == b.IsSession && a.EarlyClose == b.EarlyClose && a.OpenAt.Equal(b.OpenAt) && a.CloseAt.Equal(b.CloseAt):
			continue
		case inOld && inNew:
			c.Before, c.After = &a, &b
		case inNew:
			c.After = &b
		default:
			c.Before = &a
		}
		out = append(out, c)
	}
	return out
}

func sd(t *testing.T, day string, early bool) contracts.SessionDay {
	t.Helper()
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		t.Fatal(err)
	}
	open := d.Add(14*time.Hour + 30*time.Minute)
	return contracts.SessionDay{
		CalendarID: "XNYS", SessionDay: types.SerializableDate{Time: d}, IsSession: true, EarlyClose: early,
		OpenAt: open, CloseAt: open.Add(6*time.Hour + 30*time.Minute),
	}
}

type rig struct {
	j       *job
	src     *fakeSource
	st      *fakeStore
	changes []clock.SessionDayChange
}

func newRig(t *testing.T, st *fakeStore, fresh []contracts.SessionDay) *rig {
	t.Helper()
	r := &rig{src: &fakeSource{rows: fresh}, st: st}
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	j, err := New(r.src, st, fakeDiffer{}, fakeClock{now}, func(c clock.SessionDayChange) { r.changes = append(r.changes, c) })
	if err != nil {
		t.Fatal(err)
	}
	r.j = j.(*job)
	return r
}

func (r *rig) run() contracts.JobResult {
	keys, _, _ := r.j.Plan(context.Background(), "2026-11-09")
	return r.j.Run(context.Background(), "2026-11-09", keys)
}

func TestUpsertsOnlyChanged(t *testing.T) {
	t.Parallel()
	st := newStore(sd(t, "2026-11-09", false), sd(t, "2026-11-10", false))
	r := newRig(t, st, []contracts.SessionDay{sd(t, "2026-11-09", false), sd(t, "2026-11-10", true), sd(t, "2026-11-11", false)})
	res := r.run()
	if res.Err != nil || res.Returned != 3 {
		t.Fatalf("res = %+v", res)
	}
	if len(st.upserts) != 1 || len(st.upserts[0]) != 2 {
		t.Fatalf("upserts = %v", st.upserts)
	}
	if dayKey(st.upserts[0][0]) != "2026-11-10" || dayKey(st.upserts[0][1]) != "2026-11-11" {
		t.Fatalf("upserted days = %v", st.upserts[0])
	}
}

func TestNoChangeWritesNothing(t *testing.T) {
	t.Parallel()
	rows := []contracts.SessionDay{sd(t, "2026-11-09", false), sd(t, "2026-11-10", false)}
	r := newRig(t, newStore(rows...), rows)
	res := r.run()
	if res.Err != nil || res.Usable != 2 || res.Usable <= 0 {
		t.Fatalf("res = %+v", res)
	}
	if r.st.written() != 0 || len(r.changes) != 0 {
		t.Fatalf("written %d, changes %d", r.st.written(), len(r.changes))
	}
}

func TestNeverDeletes(t *testing.T) {
	t.Parallel()
	st := newStore(sd(t, "2026-11-09", false), sd(t, "2026-11-10", false))
	r := newRig(t, st, []contracts.SessionDay{sd(t, "2026-11-09", false)})
	if res := r.run(); res.Err != nil {
		t.Fatal(res.Err)
	}
	if _, ok := st.rows["2026-11-10"]; !ok || st.written() != 0 {
		t.Fatalf("the stored-only day was touched: rows %v, written %d", st.rows, st.written())
	}
}

func TestObserverOncePerChange(t *testing.T) {
	t.Parallel()
	st := newStore(sd(t, "2026-11-09", false), sd(t, "2026-11-12", false))
	r := newRig(t, st, []contracts.SessionDay{sd(t, "2026-11-09", true), sd(t, "2026-11-10", false)})
	if res := r.run(); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(r.changes) != 3 {
		t.Fatalf("changes = %d, want 3", len(r.changes))
	}
	if r.changes[0].Day != "2026-11-09" || r.changes[1].Day != "2026-11-10" || r.changes[2].Day != "2026-11-12" {
		t.Fatalf("changes = %+v", r.changes)
	}
}

type retryableErr struct{}

func (retryableErr) Error() string             { return "boom" }
func (retryableErr) Kind() contracts.ErrorKind { return contracts.Retryable }

func TestEmptyFreshIsPermanent(t *testing.T) {
	t.Parallel()
	st := newStore(sd(t, "2026-11-09", false))
	r := newRig(t, st, nil)
	res := r.run()
	if res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Fatalf("res = %+v", res)
	}
	if st.written() != 0 {
		t.Fatal("empty answer wrote")
	}
}

func TestSourceErrorKeepsItsClass(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		want contracts.ErrorKind
	}{
		{"retryable", retryableErr{}, contracts.Retryable},
		{"untyped", errors.New("x"), contracts.Permanent},
	} {
		r := newRig(t, newStore(), nil)
		r.src.err = tc.err
		res := r.run()
		if res.Err == nil || res.Err.Kind() != tc.want {
			t.Fatalf("%s: res = %+v", tc.name, res)
		}
	}
}

func TestSpanFromClock(t *testing.T) {
	t.Parallel()
	r := newRig(t, newStore(), []contracts.SessionDay{sd(t, "2026-11-09", false)})
	r.run()
	if r.src.cal != "XNYS" || r.src.from != "2026-11-02" || r.src.to != "2027-12-14" {
		t.Fatalf("span = %s %s..%s", r.src.cal, r.src.from, r.src.to)
	}
}

// span is one row per calendar day from 2026-11-09 to 2027-12-14.
func span(t *testing.T) []contracts.SessionDay {
	t.Helper()
	start := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	var rows []contracts.SessionDay
	for i := 0; i <= daysAfter; i++ {
		rows = append(rows, sd(t, start.AddDate(0, 0, i).Format("2006-01-02"), false))
	}
	return rows
}

func TestPostconditionFalseWithGapInSpan(t *testing.T) {
	t.Parallel()
	st := newStore(sd(t, "2026-11-09", false), sd(t, "2027-12-14", false))
	r := newRig(t, st, nil)
	got, err := r.j.Postcondition(context.Background(), "2026-11-09", datasetID, "XNYS")
	if err != nil || got {
		t.Fatalf("Postcondition = %v, %v; want false: the days between are not stored", got, err)
	}
}

func TestPostconditionFalseUntilSpanEndStored(t *testing.T) {
	t.Parallel()
	st := newStore(sd(t, "2026-11-09", false))
	r := newRig(t, st, span(t))
	ctx := context.Background()
	check := func(want bool) {
		t.Helper()
		got, err := r.j.Postcondition(ctx, "2026-11-09", datasetID, "XNYS")
		if err != nil || got != want {
			t.Fatalf("Postcondition = %v, %v; want %v", got, err, want)
		}
	}
	check(false)
	if res := r.run(); res.Err != nil {
		t.Fatal(res.Err)
	}
	check(true)
	if res := r.run(); res.Err != nil {
		t.Fatal(res.Err)
	}
	check(true)
}

func TestScheduleValues(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("job.toml")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(manifestBytes) {
		t.Fatal("embedded manifest differs from job.toml")
	}
	r := newRig(t, newStore(), nil)
	cal, anchor, zone, offset := r.j.Schedule()
	if cal != "XNYS" || anchor != "session_open" || zone != "America/New_York" || offset != -30*time.Minute {
		t.Fatalf("Schedule() = %q %q %q %v", cal, anchor, zone, offset)
	}
	if r.j.RunBound() != 5*time.Minute || r.j.RestartSensitive() {
		t.Fatalf("RunBound %v, RestartSensitive %v", r.j.RunBound(), r.j.RestartSensitive())
	}
	keys, hash, err := r.j.Plan(context.Background(), "2026-11-09")
	if err != nil || len(keys) != 1 || keys[0] != (contracts.ExpectedKey{DatasetID: "master.session_day", SubjectKey: "XNYS"}) || hash == "" {
		t.Fatalf("Plan = %v %q %v", keys, hash, err)
	}
	if !strings.Contains(r.j.ID(), "calendar_refresh") {
		t.Fatal(r.j.ID())
	}
}

// recHandler is a slog.Handler that keeps the attributes of every record it receives.
type recHandler struct {
	mu   *sync.Mutex
	recs *[]map[string]string
}

func (h recHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h recHandler) Handle(_ context.Context, r slog.Record) error {
	m := map[string]string{"msg": r.Message, "level": r.Level.String()}
	r.Attrs(func(a slog.Attr) bool { m[a.Key] = a.Value.String(); return true })
	h.mu.Lock()
	*h.recs = append(*h.recs, m)
	h.mu.Unlock()
	return nil
}
func (h recHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h recHandler) WithGroup(string) slog.Handler      { return h }

type leakyErr struct {
	kind contracts.ErrorKind
}

func (e leakyErr) Error() string             { return "store said 99999" }
func (e leakyErr) Kind() contracts.ErrorKind { return e.kind }

func TestRunFailureLogsStepKind(t *testing.T) {
	t.Parallel()
	fresh := []contracts.SessionDay{sd(t, "2026-11-09", false)}
	cases := []struct {
		name, step, kind string
		setup            func(st *fakeStore, r *rig)
	}{
		{"read_store typed", "read_store", "retryable", func(st *fakeStore, _ *rig) { st.readErr = leakyErr{contracts.Retryable} }},
		{"read_store untyped", "read_store", "unknown", func(st *fakeStore, _ *rig) { st.readErr = errors.New("store said 99999") }},
		{"read_source", "read_source", "permanent", func(_ *fakeStore, r *rig) { r.src.err = leakyErr{contracts.Permanent} }},
		{"empty_source", "empty_source", "permanent", func(_ *fakeStore, r *rig) { r.src.rows = nil }},
		{"write", "write", "retryable", func(st *fakeStore, _ *rig) { st.err = leakyErr{contracts.Retryable} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			st := newStore()
			r := newRig(t, st, fresh)
			c.setup(st, r)
			var recs []map[string]string
			r.j.log = slog.New(recHandler{&sync.Mutex{}, &recs})
			if res := r.run(); res.Err == nil {
				t.Fatal("want an error")
			}
			if len(recs) != 1 {
				t.Fatalf("records = %v", recs)
			}
			m := recs[0]
			if m["msg"] != "run failed" || m["level"] != "WARN" || m["step"] != c.step || m["kind"] != c.kind || len(m) != 4 {
				t.Fatalf("record = %v", m)
			}
			for _, v := range m {
				if strings.Contains(v, "99999") {
					t.Fatalf("record leaks error text: %v", m)
				}
			}
		})
	}
	t.Run("postcondition", func(t *testing.T) {
		t.Parallel()
		st := newStore()
		r := newRig(t, st, fresh)
		var recs []map[string]string
		r.j.log = slog.New(recHandler{&sync.Mutex{}, &recs})
		if _, err := r.j.Postcondition(context.Background(), "not-a-day", datasetID, "XNYS"); err == nil {
			t.Fatal("want an error")
		}
		st.readErr = leakyErr{contracts.Retryable}
		if _, err := r.j.Postcondition(context.Background(), "2026-11-09", datasetID, "XNYS"); err == nil {
			t.Fatal("want an error")
		}
		if len(recs) != 2 || recs[0]["step"] != "postcondition_day" || recs[0]["kind"] != "unknown" ||
			recs[1]["step"] != "postcondition_read" || recs[1]["kind"] != "retryable" {
			t.Fatalf("records = %v", recs)
		}
	})
}
