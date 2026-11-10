package reconcile

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const finishedMsg = "job run finished"

type logErr struct{ msg string }

func (e logErr) Error() string             { return e.msg }
func (e logErr) Kind() contracts.ErrorKind { return contracts.Permanent }

// advJob advances the fake clock by d inside Run, then answers res. seen gets the log
// text at the moment the run is in progress.
type advJob struct {
	*fakeJob
	clk  *clockwork.FakeClock
	d    time.Duration
	res  contracts.JobResult
	buf  *bytes.Buffer
	seen string
}

func (j *advJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	j.seen = j.buf.String()
	j.clk.Advance(j.d)
	return j.res
}

func newLogRunner(clk clockwork.Clock) (*keyRunner, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	log := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return &keyRunner{ops: newLedger(), clk: clk, log: log}, buf
}

func wantAttrs(t *testing.T, line string, attrs ...string) {
	t.Helper()
	for _, a := range attrs {
		if !strings.Contains(line+" ", " "+a+" ") {
			t.Errorf("line %q lacks %q", line, a)
		}
	}
}

func TestRunKeyLogsOkRun(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	k, buf := newLogRunner(clk)
	job := &advJob{fakeJob: &fakeJob{id: "j1"}, clk: clk, d: 7 * time.Second, buf: buf,
		res: contracts.JobResult{Returned: 5, Usable: 4}}
	if _, err := k.RunKey(context.Background(), job, rkDay, keyA, 2, time.Time{}, time.Time{}, ""); err != nil {
		t.Fatal(err)
	}
	if job.seen != "" {
		t.Fatalf("a line was written before the run ended: %q", job.seen)
	}
	out := buf.String()
	if n := strings.Count(out, finishedMsg); n != 1 {
		t.Fatalf("%d lines, want 1: %q", n, out)
	}
	wantAttrs(t, out[:len(out)-1], "level=INFO", `msg="job run finished"`, "job=j1", "day=2026-11-04",
		"dataset=ds", "subject=a", "attempt=2", "outcome=ok", "returned=5", "usable=4", "duration=7s")
	if strings.Contains(out, "error_code") {
		t.Fatalf("ok run has error_code: %q", out)
	}
}

func TestRunKeyLogsFailedRun(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	k, buf := newLogRunner(clk)
	job := &advJob{fakeJob: &fakeJob{id: "j1"}, clk: clk, d: 3 * time.Second, buf: buf,
		res: contracts.JobResult{Returned: 2, Usable: 0, Err: logErr{msg: "dial tcp 10.0.0.9: hunter2 refused"}}}
	if _, err := k.RunKey(context.Background(), job, rkDay, keyA, 1, time.Time{}, time.Time{}, ""); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if n := strings.Count(out, finishedMsg); n != 1 {
		t.Fatalf("%d lines, want 1: %q", n, out)
	}
	wantAttrs(t, out[:len(out)-1], "job=j1", "day=2026-11-04", "dataset=ds", "outcome=failed",
		"error_code=permanent", "returned=2", "usable=0", "duration=3s")
	if strings.Contains(out, "hunter2") || strings.Contains(out, "dial tcp") {
		t.Fatalf("the error message leaked into the line: %q", out)
	}
}

func TestRunKeyLogsDegradedRunAtDeadline(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	k, buf := newLogRunner(clk)
	job := newBlockJob(contracts.JobResult{Returned: 3, Usable: 1})
	job.id = "j1"
	out := make(chan error, 1)
	go func() {
		_, err := k.RunKey(context.Background(), job, rkDay, keyA, 1, time.Time{}, clk.Now().Add(time.Minute), "")
		out <- err
	}()
	<-job.started
	if err := clk.BlockUntilContext(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Minute)
	if err := <-out; err != nil {
		t.Fatal(err)
	}
	text := buf.String()
	if n := strings.Count(text, finishedMsg); n != 1 {
		t.Fatalf("%d lines, want 1: %q", n, text)
	}
	wantAttrs(t, text[:len(text)-1], "job=j1", "day=2026-11-04", "dataset=ds", "outcome=degraded",
		"returned=3", "usable=1", "duration=1m0s")
}
