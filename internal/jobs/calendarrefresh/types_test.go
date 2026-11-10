package calendarrefresh_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/calendarrefresh"
	"github.com/andrewrong/optdesk/internal/store"
)

type kindErr struct{ kind contracts.ErrorKind }

func (e kindErr) Error() string             { return string(e.kind) }
func (e kindErr) Kind() contracts.ErrorKind { return e.kind }

// sd is a session-day row of the market-zone date day.
func sd(day string) contracts.SessionDay {
	var r contracts.SessionDay
	r.SessionDay.Time, _ = time.Parse("2006-01-02", day)
	return r
}

func dayOf(r contracts.SessionDay) string { return r.SessionDay.Format("2006-01-02") }

// fakeSessionSource answers fixed rows or an error and records the span asked.
type fakeSessionSource struct {
	rows       []contracts.SessionDay
	err        error
	cal        string
	from, to   contracts.Day
	askedTimes int
}

func (s *fakeSessionSource) SessionDays(_ context.Context, cal string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	s.askedTimes++
	s.cal, s.from, s.to = cal, from, to
	return s.rows, s.err
}

// fakeSessionStore holds stored rows and records the upserts.
type fakeSessionStore struct {
	store.SessionDayStore
	stored []contracts.SessionDay
	upsert []contracts.SessionDay
}

func (s *fakeSessionStore) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	return s.stored, nil
}

func (s *fakeSessionStore) UpsertSessionDays(_ context.Context, rows []contracts.SessionDay) error {
	s.upsert = append(s.upsert, rows...)
	return nil
}

// fakeDiffer reports every row of fresh that is not in old as an added day.
type fakeDiffer struct{}

func (fakeDiffer) Diff(old, fresh []contracts.SessionDay) []clock.SessionDayChange {
	have := map[string]bool{}
	for _, r := range old {
		have[dayOf(r)] = true
	}
	var out []clock.SessionDayChange
	for i := range fresh {
		if !have[dayOf(fresh[i])] {
			out = append(out, clock.SessionDayChange{Day: contracts.Day(dayOf(fresh[i])), After: &fresh[i]})
		}
	}
	return out
}

// calendarJob is a fake calendar_refresh job that follows the contract: key {master.session_day, XNYS}, span
// today-7..today+400, an empty fresh answer Permanent, a source error passed on with its kind, one observer
// call per change, and a Postcondition on the span end.
type calendarJob struct {
	src    clock.SessionSource
	st     store.SessionDayStore
	differ clock.SessionDayDiffer
	obs    calendarrefresh.CalendarObserver
}

func (calendarJob) ID() string { return "calendar_refresh" }

func (calendarJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return []contracts.ExpectedKey{{DatasetID: "master.session_day", SubjectKey: "XNYS"}}, "hash", nil
}

func (j calendarJob) Run(ctx context.Context, day contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	from, to := span(day)
	old, err := j.st.SessionDays(ctx, "XNYS", from, to)
	if err != nil {
		return contracts.JobResult{Err: kindErr{contracts.Retryable}}
	}
	fresh, err := j.src.SessionDays(ctx, "XNYS", from, to)
	if err != nil {
		var te contracts.TypedError
		if errors.As(err, &te) {
			return contracts.JobResult{Err: te}
		}
		return contracts.JobResult{Err: kindErr{contracts.Permanent}}
	}
	if len(fresh) == 0 {
		return contracts.JobResult{Err: kindErr{contracts.Permanent}}
	}
	var rows []contracts.SessionDay
	for _, c := range j.differ.Diff(old, fresh) {
		j.obs(c)
		if c.After != nil {
			rows = append(rows, *c.After)
		}
	}
	if len(rows) > 0 {
		if err := j.st.UpsertSessionDays(ctx, rows); err != nil {
			return contracts.JobResult{Returned: len(fresh), Err: kindErr{contracts.Retryable}}
		}
	}
	return contracts.JobResult{Returned: len(fresh), Usable: len(fresh)}
}

func span(day contracts.Day) (contracts.Day, contracts.Day) {
	d, _ := time.Parse("2006-01-02", string(day))
	return contracts.Day(d.AddDate(0, 0, -7).Format("2006-01-02")), contracts.Day(d.AddDate(0, 0, 400).Format("2006-01-02"))
}

func TestCalendarRefreshContract(t *testing.T) {
	t.Parallel()
	src := &fakeSessionSource{rows: []contracts.SessionDay{sd("2026-11-06"), sd("2026-11-09")}}
	st := &fakeSessionStore{stored: []contracts.SessionDay{sd("2026-11-06")}}
	var seen []clock.SessionDayChange
	var obs calendarrefresh.CalendarObserver = func(c clock.SessionDayChange) { seen = append(seen, c) }
	var newJob calendarrefresh.NewCalendarRefreshFunc = func(
		s clock.SessionSource, w store.SessionDayStore, d clock.SessionDayDiffer, _ clock.Clock, o calendarrefresh.CalendarObserver,
	) (contracts.Job, error) {
		return calendarJob{src: s, st: w, differ: d, obs: o}, nil
	}
	j, err := newJob(src, st, fakeDiffer{}, nil, obs)
	if err != nil {
		t.Fatal(err)
	}
	if j.ID() != "calendar_refresh" {
		t.Fatalf("id = %q", j.ID())
	}
	keys, _, err := j.Plan(context.Background(), "2026-11-06")
	if err != nil || len(keys) != 1 || keys[0] != (contracts.ExpectedKey{DatasetID: "master.session_day", SubjectKey: "XNYS"}) {
		t.Fatalf("keys %v, err %v", keys, err)
	}
	got := j.Run(context.Background(), "2026-11-06", keys)
	if got.Err != nil || got.Returned != 2 || got.Usable != 2 {
		t.Fatalf("result %+v", got)
	}
	if src.cal != "XNYS" || src.from != "2026-10-30" || src.to != "2027-12-11" {
		t.Fatalf("span %s %s..%s", src.cal, src.from, src.to)
	}
	if len(seen) != 1 || seen[0].Day != "2026-11-09" || seen[0].Before != nil || len(st.upsert) != 1 {
		t.Fatalf("changes %+v, upserts %+v", seen, st.upsert)
	}
}

func TestCalendarRefreshErrorClasses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  *fakeSessionSource
		want contracts.ErrorKind
	}{
		{"empty answer", &fakeSessionSource{}, contracts.Permanent},
		{"source retryable", &fakeSessionSource{err: kindErr{contracts.Retryable}}, contracts.Retryable},
		{"source permanent", &fakeSessionSource{err: kindErr{contracts.Permanent}}, contracts.Permanent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := &fakeSessionStore{}
			j := calendarJob{src: c.src, st: st, differ: fakeDiffer{}, obs: func(clock.SessionDayChange) {}}
			got := j.Run(context.Background(), "2026-11-06", nil)
			if got.Err == nil || got.Err.Kind() != c.want || got.Usable != 0 || len(st.upsert) != 0 {
				t.Fatalf("result %+v, upserts %v", got, st.upsert)
			}
		})
	}
}
