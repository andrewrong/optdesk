package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestSchedulerLogsEveryLeftOutJob(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
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

// With every line set no job is left out for a missing line: recon_bars, held only for want of its line,
// is neither disabled nor logged.
func TestSchedulerLogsNoLeftOutForSetLine(t *testing.T) {
	t.Parallel()
	cfg := runOnceConfig{fredKey: "k", eventsDSN: "x", reconDSN: "x"}
	disabled := schedulerDisabled(cfg, wiring{brokerUp: true})
	if _, off := disabled[jobReconBars]; off {
		t.Fatalf("disabled %v holds %s with its line set", disabled, jobReconBars)
	}
	var buf bytes.Buffer
	enabled := logLeftOut(slog.New(slog.NewTextHandler(&buf, nil)), []contracts.Job{candJobStub{id: jobFredSeries}}, disabled)
	if strings.Contains(buf.String(), jobReconBars) || len(enabled) != 1 {
		t.Fatalf("logged %q, enabled %v", buf.String(), enabled)
	}
	// The same config without the recon line holds recon_bars with its reason.
	cfg.reconDSN = ""
	if got := schedulerDisabled(cfg, wiring{brokerUp: true})[jobReconBars]; got != reasonNoReconDSN {
		t.Fatalf("recon_bars disabled with %q, want %q", got, reasonNoReconDSN)
	}
}
