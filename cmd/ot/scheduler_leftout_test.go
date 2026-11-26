package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/profile"
)

func TestSchedulerLogsEveryLeftOutJob(t *testing.T) {
	t.Parallel()
	p, err := profile.Load(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log := slog.New(profile.NewLogHandler(p, slog.NewTextHandler(&buf, nil)))
	built := candJobStub{id: jobBookRisk}
	kept := candJobStub{id: jobFredSeries}
	disabled := map[string]string{
		jobReconBars: reasonNoReconDSN, // never built: not among the jobs
		jobBookRisk:  "held",           // built and left out
	}
	enabled := logLeftOut(log, []contracts.Job{built, kept}, disabled)
	if len(enabled) != 1 || enabled[0].ID() != jobFredSeries {
		t.Fatalf("enabled = %v, want only %s", enabled, jobFredSeries)
	}
	out := buf.String()
	if n := strings.Count(out, "job left out"); n != 2 {
		t.Fatalf("%d left-out lines, want 2:\n%s", n, out)
	}
	if want := `job=recon_bars status="OT_PG_DSN_RECON is not set (run setup pg)"`; !strings.Contains(out, want) {
		t.Fatalf("log lacks %q:\n%s", want, out)
	}
	if n := strings.Count(out, "job="+jobBookRisk+" "); n != 1 {
		t.Fatalf("%s logged %d times, want once:\n%s", jobBookRisk, n, out)
	}
}
