package main

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/jobrequest"
	"github.com/jonboulle/clockwork"
)

// enqPoster is the Poster and Reader of the enqueue path: it stores each request under the next id and answers
// Get from the rows a test gives (a queued row for an id it gives none).
type enqPoster struct {
	mu      sync.Mutex
	reqs    []jobrequest.Request
	outcome jobrequest.EnqueueOutcome
	enqErr  error
	states  map[int64][]jobrequest.Row // successive answers of Get; the last one repeats
	gets    map[int64]int
	dsn     string
	closed  int
}

func (p *enqPoster) Enqueue(_ context.Context, r jobrequest.Request) (jobrequest.Enqueued, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.enqErr != nil {
		return jobrequest.Enqueued{}, p.enqErr
	}
	p.reqs = append(p.reqs, r)
	o := p.outcome
	if o == 0 {
		o = jobrequest.EnqueueCreated
	}
	id := int64(len(p.reqs))
	return jobrequest.Enqueued{Row: jobrequest.Row{ID: id, Request: r, State: jobrequest.StateQueued}, Outcome: o}, nil
}

func (p *enqPoster) Get(_ context.Context, id int64) (jobrequest.Row, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.gets == nil {
		p.gets = map[int64]int{}
	}
	seq := p.states[id]
	if len(seq) == 0 {
		return jobrequest.Row{ID: id, Request: p.reqs[id-1], State: jobrequest.StateQueued}, true, nil
	}
	i := min(p.gets[id], len(seq)-1)
	p.gets[id]++
	row := seq[i]
	row.ID, row.Request = id, p.reqs[id-1]
	return row, true, nil
}

func (p *enqPoster) Recent(context.Context, int) ([]jobrequest.Row, error) { return nil, nil }
func (p *enqPoster) Withdraw(context.Context, int64) error                 { return nil }
func (p *enqPoster) Close() error                                          { p.mu.Lock(); p.closed++; p.mu.Unlock(); return nil }

func (p *enqPoster) jobs() []string {
	var out []string
	for _, r := range p.reqs {
		out = append(out, r.JobID)
	}
	return out
}

// enqueueApp is an app whose request store is p, with a fake clock for --wait; the app clock is
// 2026-11-06 02:00 UTC: 2026-11-05 in New York and 2026-11-06 in Tokyo.
func enqueueApp(t *testing.T, p *enqPoster) (*app, *bytes.Buffer, *clockwork.FakeClock) {
	t.Helper()
	a, _ := testApp(t, "OT_PG_DSN_SETTINGS=settings-dsn\n")
	fc := clockwork.NewFakeClock()
	a.deps.newFwClock = func() clockwork.Clock { return fc }
	a.deps.ext = map[string]any{jobConsoleExtKey: jobRequestOpeners{
		Poster: func(_ context.Context, dsn string) (jobrequest.Poster, error) { p.dsn = dsn; return p, nil },
	}}
	var out bytes.Buffer
	a.stdout = &out
	return a, &out, fc
}

func TestRunOnceEnqueueAllSkipsListedOnlyJobs(t *testing.T) {
	t.Parallel()
	p := &enqPoster{}
	a, out, _ := enqueueApp(t, p)
	if err := runOnce(a, []string{"--day", "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	got := p.jobs()
	for _, want := range []string{"fred_series", "option_eod", "calendar_refresh"} {
		if !slices.Contains(got, want) {
			t.Errorf("no request for %s: %v", want, got)
		}
	}
	for _, none := range []string{"option_open", "option_open_resolve_gap", "option_ledger"} {
		if slices.Contains(got, none) {
			t.Errorf("a listed_only job was posted: %s", none)
		}
	}
	for _, r := range p.reqs {
		if r.Day != "2026-11-05" || r.Force {
			t.Errorf("request %+v", r)
		}
	}
	if p.dsn != "settings-dsn" || p.closed != 1 {
		t.Errorf("dsn %q closed %d", p.dsn, p.closed)
	}
	if !strings.Contains(out.String(), "request 1 job=") || !strings.Contains(out.String(), "outcome=created") {
		t.Errorf("stdout %q", out.String())
	}
}

func TestRunOnceEnqueueAllSkipsTodayOnlyJobsOfAnotherDay(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		day         string
		has, hasNot []string
	}{
		{"2026-11-04", []string{"fred_series", "compute_figures"}, []string{"option_eod", "option_eod_jp", "calendar_refresh"}},
		{"2026-11-05", []string{"option_eod", "fred_series"}, []string{"option_eod_jp", "calendar_hours_jp"}}, // today in New York only
		{"2026-11-06", []string{"option_eod_jp", "fred_series"}, []string{"option_eod", "quote_snapshot"}},    // today in Tokyo only
	} {
		p := &enqPoster{}
		a, _, _ := enqueueApp(t, p)
		if err := runOnce(a, []string{"--day", tc.day}); err != nil {
			t.Fatal(err)
		}
		for _, id := range tc.has {
			if !slices.Contains(p.jobs(), id) {
				t.Errorf("day %s: no request for %s", tc.day, id)
			}
		}
		for _, id := range tc.hasNot {
			if slices.Contains(p.jobs(), id) {
				t.Errorf("day %s: a request for %s", tc.day, id)
			}
		}
	}
}

func TestBackfillEnqueueAllPostsTodayOnlyJobsForAPastDay(t *testing.T) {
	t.Parallel()
	p := &enqPoster{}
	a, _, _ := enqueueApp(t, p)
	if err := backfill(a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	got := p.jobs()
	for _, want := range []string{"option_eod", "option_eod_jp", "fred_series"} {
		if !slices.Contains(got, want) {
			t.Errorf("no request for %s: %v", want, got)
		}
	}
	for _, none := range []string{"option_open", "option_open_plan_jp", "option_ledger"} {
		if slices.Contains(got, none) {
			t.Errorf("a listed_only job was posted: %s", none)
		}
	}
	for _, r := range p.reqs {
		if r.Force {
			t.Errorf("force set without --force: %+v", r)
		}
	}
}

func TestBackfillJobPostsOneRequestWithForce(t *testing.T) {
	t.Parallel()
	p := &enqPoster{}
	a, out, _ := enqueueApp(t, p)
	if err := backfill(a, []string{"--day", "2026-11-04", "--job", "fred_series", "--force"}); err != nil {
		t.Fatal(err)
	}
	want := []jobrequest.Request{{JobID: "fred_series", Day: "2026-11-04", Force: true}}
	if !slices.Equal(p.reqs, want) {
		t.Fatalf("requests %+v, want %+v", p.reqs, want)
	}
	if out.String() != "request 1 job=fred_series day=2026-11-04 outcome=created\n" {
		t.Fatalf("stdout %q", out.String())
	}
}

func TestEnqueueRefusesDatasetAsUnknownFlag(t *testing.T) {
	t.Parallel()
	for name, run := range map[string]func(*app, []string) error{"backfill": backfill, "run-once": runOnce} {
		p := &enqPoster{}
		a, _, _ := enqueueApp(t, p)
		err := run(a, []string{"--day", "2026-11-04", "--dataset", "fred_series"})
		if err == nil || !strings.Contains(err.Error(), "dataset") || len(p.reqs) != 0 {
			t.Errorf("%s: err %v, requests %v", name, err, p.reqs)
		}
	}
}

func TestEnqueueOutcomesThatStoreNothingFail(t *testing.T) {
	t.Parallel()
	for _, o := range []jobrequest.EnqueueOutcome{jobrequest.EnqueueWithdrawing, jobrequest.EnqueueBusy} {
		p := &enqPoster{outcome: o}
		a, _, _ := enqueueApp(t, p)
		if err := backfill(a, []string{"--day", "2026-11-04", "--job", "fred_series"}); err == nil {
			t.Errorf("outcome %d: no error", o)
		}
	}
	p := &enqPoster{outcome: jobrequest.EnqueueExisting}
	a, out, _ := enqueueApp(t, p)
	if err := backfill(a, []string{"--day", "2026-11-04", "--job", "fred_series"}); err != nil || !strings.Contains(out.String(), "outcome=existing") {
		t.Errorf("existing: %v %q", err, out.String())
	}
}

func TestEnqueueErrorsLeakNoDSN(t *testing.T) {
	t.Parallel()
	const secret = "postgres://user:hunter2@host/db"
	for name, run := range map[string]func(*app, []string) error{"backfill": backfill, "run-once": runOnce} {
		for step, setup := range map[string]func(*app, *enqPoster){
			"open": func(a *app, _ *enqPoster) {
				a.deps.ext = map[string]any{jobConsoleExtKey: jobRequestOpeners{
					Poster: func(context.Context, string) (jobrequest.Poster, error) { return nil, errors.New("dial " + secret) },
				}}
			},
			"enqueue": func(_ *app, p *enqPoster) { p.enqErr = errors.New("write " + secret) },
		} {
			p := &enqPoster{}
			a, out, _ := enqueueApp(t, p)
			setup(a, p)
			err := run(a, []string{"--day", "2026-11-04", "--job", "fred_series"})
			if err == nil {
				t.Fatalf("%s %s: no error", name, step)
			}
			if strings.Contains(err.Error(), "hunter2") || strings.Contains(out.String(), "hunter2") || strings.Contains(err.Error(), "settings-dsn") {
				t.Fatalf("%s %s: leak %v / %q", name, step, err, out.String())
			}
		}
	}
}

func TestEnqueueWithoutSettingsDSNOrPosterNamesTheCause(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=p\n")
	if err := backfill(a, []string{"--day", "2026-11-04", "--job", "fred_series"}); err == nil || !strings.Contains(err.Error(), "OT_PG_DSN_SETTINGS") {
		t.Errorf("no dsn: %v", err)
	}
	b, _ := testApp(t, "OT_PG_DSN_SETTINGS=s\n")
	if err := backfill(b, []string{"--day", "2026-11-04", "--job", "fred_series"}); err == nil || !strings.Contains(err.Error(), "no request store") {
		t.Errorf("no poster: %v", err)
	}
}

func row(s jobrequest.State, reason string, lines ...jobrequest.KeyLine) jobrequest.Row {
	return jobrequest.Row{State: s, Reason: reason, Lines: lines}
}

func TestWaitPollsOnTheInjectedClockUntilTheRequestEnds(t *testing.T) {
	t.Parallel()
	p := &enqPoster{states: map[int64][]jobrequest.Row{1: {
		row(jobrequest.StateQueued, ""), row(jobrequest.StateRunning, ""),
		row(jobrequest.StateDone, "completed", jobrequest.KeyLine{Dataset: "obs.series_point", Subject: "DGS10", Group: "ran", Status: "ok", Usable: 3}),
	}}}
	a, out, fc := enqueueApp(t, p)
	done := make(chan error, 1)
	go func() { done <- backfill(a, []string{"--day", "2026-11-04", "--job", "fred_series", "--wait"}) }()
	for range 2 { // one wait of a second after each of the first two reads
		if err := fc.BlockUntilContext(t.Context(), 1); err != nil {
			t.Fatal(err)
		}
		fc.Advance(time.Second)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"key dataset=obs.series_point subject=DGS10 group=ran status=ok", "usable=3",
		"request 1 job=fred_series day=2026-11-04 state=done reason=completed"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout %q lacks %q", out.String(), want)
		}
	}
}

func TestWaitExitStatusFollowsTheEndState(t *testing.T) {
	t.Parallel()
	for state, wantErr := range map[jobrequest.State]bool{
		jobrequest.StateDone: false, jobrequest.StateWithdrawn: false,
		jobrequest.StateFailed: true, jobrequest.StateRefused: true, jobrequest.StateInterrupted: true,
	} {
		p := &enqPoster{states: map[int64][]jobrequest.Row{1: {row(state, "why")}}}
		a, out, _ := enqueueApp(t, p)
		err := backfill(a, []string{"--day", "2026-11-04", "--job", "fred_series", "--wait"})
		if (err != nil) != wantErr {
			t.Errorf("state %s: err %v", state, err)
		}
		if !strings.Contains(out.String(), "state="+string(state)+" reason=why") {
			t.Errorf("state %s: stdout %q", state, out.String())
		}
	}
}

func TestWaitPastTheTimeoutNamesTheConsole(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args    []string
		advance time.Duration
	}{
		{nil, 120 * time.Second}, // the default timeout
		{[]string{"--timeout", "30s"}, 30 * time.Second},
	} {
		p := &enqPoster{}
		a, out, fc := enqueueApp(t, p)
		done := make(chan error, 1)
		args := append([]string{"--day", "2026-11-04", "--job", "fred_series", "--wait"}, tc.args...)
		go func() { done <- backfill(a, args) }()
		if err := fc.BlockUntilContext(t.Context(), 1); err != nil {
			t.Fatal(err)
		}
		fc.Advance(time.Second)
		if err := fc.BlockUntilContext(t.Context(), 1); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			t.Fatalf("gave up before the timeout: %v", err)
		default:
		}
		fc.Advance(tc.advance)
		if err := <-done; err == nil {
			t.Fatalf("%v: exit status zero", tc.args)
		}
		want := "request 1 is still queued: the console may be off (no consumer is running)\n"
		if !strings.Contains(out.String(), want) {
			t.Errorf("%v: stdout %q", tc.args, out.String())
		}
	}
}
