package main

import (
	"bytes"
	"context"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
	"github.com/jonboulle/clockwork"
)

// cuPoster is the fake poster that also remembers each created request as a row requested at the fake clock.
type cuPoster struct {
	*fakePoster
	clk  clockwork.Clock
	rows []jobrequest.Row
	hook func(ctx context.Context)
}

func (p *cuPoster) Recent(context.Context, int) ([]jobrequest.Row, error) { return p.rows, nil }

func (p *cuPoster) Enqueue(ctx context.Context, r jobrequest.Request) (jobrequest.Enqueued, error) {
	res, err := p.fakePoster.Enqueue(ctx, r)
	if err == nil && res.Outcome == jobrequest.EnqueueCreated {
		p.rows = append(p.rows, jobrequest.Row{ID: res.Row.ID, Request: r, RequestedAt: p.clk.Now(), State: jobrequest.StateQueued})
	}
	if p.hook != nil {
		p.hook(ctx)
	}
	return res, err
}

// cuCoverage gives, for every dataset of the page and each of its days, a missing cell for the datasets in gap and
// an ok cell for the others, and the days as sessions.
type cuCoverage struct {
	repairCoverage
	days []contracts.Day
	gap  map[string]coverage.State
	// only is the set of days a gap dataset is missing; empty means all days.
	only map[contracts.Day]bool
}

func (c cuCoverage) Cells(context.Context, contracts.Day, contracts.Day) ([]coverage.Cell, error) {
	var out []coverage.Cell
	done := map[string]bool{}
	for _, j := range dataHealthJobs() {
		for _, ds := range j.Datasets {
			if done[ds] {
				continue
			}
			done[ds] = true
			for _, d := range c.days {
				st := coverage.StateOK
				if g, ok := c.gap[ds]; ok && (len(c.only) == 0 || c.only[d]) {
					st = g
				}
				out = append(out, repairCellOf(ds, d, st))
			}
		}
	}
	return out, nil
}

func (c cuCoverage) Sessions(context.Context, string, contracts.Day, contracts.Day) ([]coverage.Session, error) {
	var out []coverage.Session
	for _, d := range c.days {
		out = append(out, coverage.Session{Calendar: "XNYS", Day: d, IsSession: true})
	}
	return out, nil
}

var cuDays = []contracts.Day{"2026-11-02", "2026-11-03", "2026-11-04", "2026-11-05", "2026-11-06"}

type cuRig struct {
	host *consoleHost
	post *cuPoster
	clk  *clockwork.FakeClock
	out  *bytes.Buffer
}

func newCURig(t *testing.T, cov coverage.Reader) *cuRig {
	t.Helper()
	clk := clockwork.NewFakeClockAt(postNow)
	post := &cuPoster{fakePoster: newFakePoster(), clk: clk}
	out := &bytes.Buffer{}
	h := newConsoleHost(&jcQueue{log: &jcLog{}}, &jcLog{}, nil, clk, slog.New(slog.NewTextHandler(out, nil)), post, cov)
	return &cuRig{host: h, post: post, clk: clk, out: out}
}

func (r *cuRig) enqueuedOf(job string) []string {
	var days []string
	for _, e := range r.post.enqueued {
		if e.JobID == job {
			days = append(days, e.Day)
		}
	}
	return days
}

func TestCatchUpPassEnqueuesGapsOldestFirst(t *testing.T) {
	t.Parallel()
	const ds = "obs.fundamental_fact"
	cov := cuCoverage{days: cuDays, gap: map[string]coverage.State{ds: coverage.StateMissing}}
	r := newCURig(t, cov)
	r.host.catchUpPass(t.Context())
	got := r.enqueuedOf("fundamentals")
	// today (11-06) is not a button yet: the job is not due at the noon of the fake clock.
	want := []string{"2026-11-02", "2026-11-03", "2026-11-04", "2026-11-05"}
	if !slices.Equal(got, want) || len(r.post.enqueued) != len(want) {
		t.Fatalf("enqueued %v, want %v only", r.post.enqueued, want)
	}
	for _, e := range r.post.enqueued {
		if e.Force {
			t.Fatalf("forced request %+v", e)
		}
	}
}

func TestCatchUpPassStopsAtCap(t *testing.T) {
	t.Parallel()
	gap := map[string]coverage.State{}
	for _, j := range dataHealthJobs() {
		for _, ds := range j.Datasets {
			gap[ds] = coverage.StateMissing
		}
	}
	r := newCURig(t, cuCoverage{days: cuDays, gap: gap})
	r.host.catchUpPass(t.Context())
	first := len(r.post.enqueued)
	if first != catchUpCap {
		t.Fatalf("first pass enqueued %d, want %d", first, catchUpCap)
	}
	r.host.catchUpPass(t.Context())
	if rest := len(r.post.enqueued) - first; rest != 5 { // gap days outside the window rule and the newest day of the window jobs; no today of a close-slot job
		t.Fatalf("second pass enqueued %d, want the other 5", rest)
	}
	second := len(r.post.enqueued)
	r.host.catchUpPass(t.Context())
	for _, e := range r.post.enqueued[second:] { // only the next newest day of a window job is left
		if !slices.Contains(windowEndingJobs, e.JobID) {
			t.Fatalf("third pass enqueued %+v", e)
		}
	}
	seen := map[jobrequest.Request]bool{}
	for _, e := range r.post.enqueued {
		if seen[e] {
			t.Fatalf("request %+v enqueued twice", e)
		}
		seen[e] = true
	}
}

func TestCatchUpPassStopsAtTimeout(t *testing.T) {
	t.Parallel()
	gap := map[string]coverage.State{"obs.fundamental_fact": coverage.StateMissing, "account.position_mark": coverage.StateMissing}
	r := newCURig(t, cuCoverage{days: cuDays, gap: gap})
	r.post.hook = func(ctx context.Context) { r.clk.Advance(catchUpTimeout); <-ctx.Done() }
	r.host.catchUpPass(t.Context())
	if len(r.post.enqueued) != 1 {
		t.Fatalf("enqueued %v, want exactly the request that ran the clock out", r.post.enqueued)
	}
}

func TestCatchUpPassOneRequestPerJobDayPerCalendarDay(t *testing.T) {
	t.Parallel()
	states := []jobrequest.State{jobrequest.StateQueued, jobrequest.StateRunning, jobrequest.StateDone, jobrequest.StateFailed,
		jobrequest.StateRefused, jobrequest.StateInterrupted, jobrequest.StateWithdrawn}
	const ds = "obs.fundamental_fact"
	for _, st := range states {
		t.Run(string(st), func(t *testing.T) {
			t.Parallel()
			cov := cuCoverage{days: cuDays, gap: map[string]coverage.State{ds: coverage.StateMissing}, only: map[contracts.Day]bool{"2026-11-04": true}}
			r := newCURig(t, cov)
			r.post.rows = []jobrequest.Row{{ID: 1, State: st, RequestedAt: postNow.Add(-time.Hour),
				Request: jobrequest.Request{JobID: "fundamentals", Day: "2026-11-04"}}}
			r.host.catchUpPass(t.Context())
			if len(r.post.enqueued) != 0 {
				t.Fatalf("state %s: enqueued %v", st, r.post.enqueued)
			}
			r.clk.Advance(24 * time.Hour) // the next New York day: the gap is still there
			r.host.catchUpPass(t.Context())
			if got := r.enqueuedOf("fundamentals"); !slices.Equal(got, []string{"2026-11-04"}) {
				t.Fatalf("state %s next day: enqueued %v", st, r.post.enqueued)
			}
		})
	}
}

func TestCatchUpPassNeverTakesNoValue(t *testing.T) {
	t.Parallel()
	r := newCURig(t, cuCoverage{days: cuDays, gap: map[string]coverage.State{"obs.fundamental_fact": coverage.StateNoValue}})
	r.host.catchUpPass(t.Context())
	if len(r.post.enqueued) != 0 {
		t.Fatalf("enqueued %v", r.post.enqueued)
	}
}

func TestCatchUpPassWindowJobNewestGapOnly(t *testing.T) {
	t.Parallel()
	bars := cuCoverage{days: cuDays, gap: map[string]coverage.State{"obs.price_bar": coverage.StateMissing},
		only: map[contracts.Day]bool{"2026-11-04": true, "2026-11-05": true, "2026-11-06": true}}
	r := newCURig(t, bars)
	r.host.catchUpPass(t.Context())
	if got := r.enqueuedOf("bars_refresh"); !slices.Equal(got, []string{"2026-11-05"}) || len(r.post.enqueued) != 1 {
		t.Fatalf("enqueued %v, want the newest gap day only", r.post.enqueued)
	}
	r.host.catchUpPass(t.Context()) // 11-05 is taken for today: no older day goes through
	if got := r.enqueuedOf("bars_refresh"); !slices.Equal(got, []string{"2026-11-05"}) {
		t.Fatalf("second pass: %v", got)
	}

	st := newCURig(t, cuCoverage{days: cuDays, gap: map[string]coverage.State{"account.capture": coverage.StateMissing},
		only: map[contracts.Day]bool{"2026-11-02": true, "2026-11-03": true, "2026-11-04": true}})
	st.host.catchUpPass(t.Context())
	if got := st.enqueuedOf("statement_capture"); !slices.Equal(got, []string{"2026-11-02", "2026-11-03", "2026-11-04"}) {
		t.Fatalf("statement_capture %v", got)
	}
}

func TestCatchUpNewestRequestedTodayLetsNoOlderDayThrough(t *testing.T) {
	t.Parallel()
	cov := cuCoverage{days: cuDays, gap: map[string]coverage.State{"obs.fx_rate": coverage.StateMissing},
		only: map[contracts.Day]bool{"2026-11-02": true, "2026-11-03": true, "2026-11-04": true}}
	r := newCURig(t, cov)
	r.post.rows = []jobrequest.Row{{ID: 1, State: jobrequest.StateDone, RequestedAt: postNow.Add(-time.Hour),
		Request: jobrequest.Request{JobID: "fx_refresh", Day: "2026-11-04"}}}
	r.host.catchUpPass(t.Context())
	if got := r.enqueuedOf("fx_refresh"); len(got) != 0 {
		t.Fatalf("fx_refresh enqueued %v, want none", got)
	}
}

func TestCatchUpPassLogsCounts(t *testing.T) {
	t.Parallel()
	bars := cuCoverage{days: cuDays, gap: map[string]coverage.State{"obs.price_bar": coverage.StateMissing, "obs.fundamental_fact": coverage.StateMissing},
		only: map[contracts.Day]bool{"2026-11-04": true, "2026-11-05": true, "2026-11-06": true}}
	r := newCURig(t, bars)
	r.post.rows = []jobrequest.Row{{ID: 1, State: jobrequest.StateDone, RequestedAt: postNow,
		Request: jobrequest.Request{JobID: "fundamentals", Day: "2026-11-04"}}}
	r.host.catchUpPass(t.Context())
	line := r.out.String()
	for _, want := range []string{"catch-up", "requested=2", "skipped_today=1", "skipped_window=1"} {
		if !strings.Contains(line, want) {
			t.Fatalf("log %q lacks %q", line, want)
		}
	}
	if strings.Count(line, "catch-up") != 1 {
		t.Fatalf("log %q: want one line", line)
	}
}

func TestClaimNextRunsCatchUpEveryThirtyMinutes(t *testing.T) {
	t.Parallel()
	r := newCURig(t, cuCoverage{days: cuDays, gap: map[string]coverage.State{"obs.fundamental_fact": coverage.StateMissing},
		only: map[contracts.Day]bool{"2026-11-04": true}})
	r.host.set(nil, &jcBackfill{log: &jcLog{}})
	passes := func() int { return strings.Count(r.out.String(), "msg=catch-up") }
	r.host.claimNext(t.Context())
	if passes() != 1 {
		t.Fatalf("passes after start %d", passes())
	}
	r.clk.Advance(29 * time.Minute)
	r.host.claimNext(t.Context())
	if passes() != 1 {
		t.Fatalf("passes after 29 min %d", passes())
	}
	r.clk.Advance(time.Minute)
	r.host.claimNext(t.Context())
	if passes() != 2 {
		t.Fatalf("passes after 30 min %d", passes())
	}
	if len(r.post.enqueued) != 1 {
		t.Fatalf("enqueued %v; the pass runs no job", r.post.enqueued)
	}
}

func TestOpenConsoleHostWithoutCoverageKeepsConsole(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		pagesDSN string
		poster   jobrequest.OpenPosterFunc
	}{
		"no pages dsn": {"", func(context.Context, string) (jobrequest.Poster, error) { return newFakePoster(), nil }},
		"no poster":    {"postgres://pages", nil},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, errb := testApp(t, "")
			q := &jcQueue{log: &jcLog{}}
			a.deps.ext = map[string]any{
				jobConsoleExtKey: jobRequestOpeners{
					Consumer: func(context.Context, string) (jobrequest.Consumer, error) { return q, nil },
					Poster:   tc.poster,
				},
				extDataHealth: dataHealthDeps{openCoverage: func(context.Context, string) (coverage.Reader, error) { return repairCoverage{}, nil }},
			}
			newSyncer := func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.KeyGapSyncer { return q.log }
			host, closeHost := openConsoleHost(t.Context(), a, "postgres://runner", tc.pagesDSN, newSyncer, nil, nil, clockwork.NewFakeClock())
			defer closeHost()
			if host == nil {
				t.Fatal("the console is off")
			}
			if host.post != nil || host.cov != nil {
				t.Fatal("the pass is on")
			}
			if n := strings.Count(errb.String(), "catch-up off"); n != 2 { // the line carries the text twice: message and status
				t.Fatalf("log %q: want one catch-up off line", errb.String())
			}
			host.set(nil, &jcBackfill{log: q.log})
			if took, run := host.claimNext(t.Context()); took || run != nil {
				t.Fatal("claimed a request from an empty queue")
			}
		})
	}
}

func TestCatchUpPassSkipsTodayBeforeCloseSlot(t *testing.T) {
	t.Parallel()
	jobs := []string{"bars_refresh", "iv_history", "account_capture", "position_marks"}
	gap := map[string]coverage.State{}
	for _, j := range dataHealthJobs() {
		if slices.Contains(jobs, j.ID) {
			for _, ds := range j.Datasets {
				gap[ds] = coverage.StateMissing
			}
		}
	}
	const today = "2026-11-06"
	pastBefore := map[string][]string{}
	for _, c := range []struct {
		name  string
		hour  int
		fired bool
	}{{"before the close slot", 6, false}, {"after the close slot", 22, true}} {
		t.Run(c.name, func(t *testing.T) {
			r := newCURig(t, cuCoverage{days: cuDays, gap: gap})
			r.clk.Advance(time.Date(2026, 11, 6, c.hour, 0, 0, 0, time.UTC).Sub(r.clk.Now()))
			r.host.catchUpPass(t.Context())
			for _, j := range jobs {
				got := r.enqueuedOf(j)
				if has := slices.Contains(got, today); has != c.fired {
					t.Errorf("%s: today queued=%v, want %v (%v)", j, has, c.fired, got)
				}
				if j == "bars_refresh" { // a window job queues its newest gap day only: the last close before the slot, today after it
					want := []string{"2026-11-05"}
					if c.fired {
						want = []string{today}
					}
					if !slices.Equal(got, want) {
						t.Errorf("bars_refresh: queued %v, want %v", got, want)
					}
				}
				past := slices.DeleteFunc(slices.Clone(got), func(d string) bool { return d == today })
				if prev, ok := pastBefore[j]; ok && !slices.Contains(windowEndingJobs, j) && !slices.Equal(prev, past) {
					t.Errorf("%s: days before today %v, want %v as before the slot", j, past, prev)
				}
				pastBefore[j] = past
			}
		})
	}
}

func ivHistoryDataset(t *testing.T) string {
	t.Helper()
	for _, j := range dataHealthJobs() {
		if j.ID == "iv_history" && len(j.Datasets) == 1 {
			return j.Datasets[0]
		}
	}
	t.Fatal("iv_history has no single dataset on the page")
	return ""
}

func TestCatchUpQueuesPastDayOfIvHistory(t *testing.T) {
	t.Parallel()
	ds := ivHistoryDataset(t)
	const day = "2026-11-02" // four days before the fake clock's New York day
	cov := cuCoverage{days: cuDays, gap: map[string]coverage.State{ds: coverage.StateMissing}, only: map[contracts.Day]bool{day: true}}
	r := newCURig(t, cov)
	if _, err := r.host.catchUp(t.Context()); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range r.post.enqueued {
		if e.JobID == "iv_history" {
			got = append(got, e.Day)
			if e.Force {
				t.Fatalf("forced request %+v", e)
			}
		}
	}
	if !slices.Equal(got, []string{day}) {
		t.Fatalf("iv_history enqueued %v, want [%s]", got, day)
	}
	before := len(r.post.enqueued)
	n, err := r.host.catchUp(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.post.enqueued) != before || n.skippedToday < 1 {
		t.Fatalf("second pass: enqueued %d more, skippedToday %d", len(r.post.enqueued)-before, n.skippedToday)
	}
	r.clk.Advance(24 * time.Hour)
	if _, err := r.host.catchUp(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := r.enqueuedOf("iv_history"); !slices.Equal(got, []string{day, day}) {
		t.Fatalf("next day: iv_history enqueued %v, want the day again", got)
	}
}

func TestCatchUpPastDayStillSkipsOtherTodayOnly(t *testing.T) {
	t.Parallel()
	jobs := []string{"quote_snapshot", "option_eod", "calendar_hours_us"}
	gap := map[string]coverage.State{}
	for _, j := range dataHealthJobs() {
		if slices.Contains(jobs, j.ID) {
			for _, ds := range j.Datasets {
				gap[ds] = coverage.StateMissing
			}
		}
	}
	r := newCURig(t, cuCoverage{days: cuDays, gap: gap, only: map[contracts.Day]bool{"2026-11-02": true}})
	if _, err := r.host.catchUp(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		if got := r.enqueuedOf(j); len(got) != 0 {
			t.Errorf("%s enqueued %v for a past day", j, got)
		}
	}
}
