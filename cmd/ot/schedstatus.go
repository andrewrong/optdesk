package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

// schedStatusCmd is `ot scheduler status`: it prints the build, the non-terminal runs, the next slot of each
// job and the open gaps, read through store.SchedulerLedger only. It returns an exitError with code 1 while
// a gap is open or a job is left out, and nil when green. No dataset store and no broker is opened.
func schedStatusCmd(a *app, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: ot scheduler status")
	}
	st, now, err := readSchedStatus(a)
	if err != nil {
		return err
	}
	writeSchedStatus(a.stdout, st, now)
	if reasons := redReasons(st); len(reasons) > 0 {
		return &exitError{code: 1, err: errors.New("scheduler status is red: " + strings.Join(reasons, "; "))}
	}
	return nil
}

// schedCanRestartCmd is `ot scheduler can-restart`: nil when a hand restart is safe, else an exitError with
// code 1 whose text is the reason.
func schedCanRestartCmd(a *app, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: ot scheduler can-restart")
	}
	st, now, err := readSchedStatus(a)
	if err != nil {
		return err
	}
	if reason := restartBlocker(st, now); reason != "" {
		return &exitError{code: 1, err: errors.New("not now: " + reason)}
	}
	_, _ = fmt.Fprintln(a.stdout, "ok: a restart is safe now")
	return nil
}

// readSchedStatus opens the scheduler ledger with the runner DSN alone, reads the status and closes it.
func readSchedStatus(a *app) (store.SchedulerStatus, time.Time, error) {
	if a.profile.Name == "demo" {
		return store.SchedulerStatus{}, time.Time{}, errors.New("scheduler needs a real profile; the demo profile opens no store")
	}
	env, err := a.envFile()
	if err != nil {
		return store.SchedulerStatus{}, time.Time{}, err
	}
	dsn := env[store.DSNEnvRunner]
	if dsn == "" {
		return store.SchedulerStatus{}, time.Time{}, fmt.Errorf("%s is missing in .env", store.DSNEnvRunner)
	}
	ctx := context.Background()
	sl, err := a.deps.openSchedulerLedger(ctx, dsn)
	if err != nil {
		return store.SchedulerStatus{}, time.Time{}, fmt.Errorf("open scheduler ledger with %s failed", store.DSNEnvRunner)
	}
	defer func() { _ = sl.Close() }()
	st, err := sl.Status(ctx)
	if err != nil {
		return store.SchedulerStatus{}, time.Time{}, errors.New("read the scheduler status failed")
	}
	return st, a.deps.newFwClock().Now(), nil
}

// redReasons lists why the status is red: one entry per open gap and per left-out job.
func redReasons(st store.SchedulerStatus) []string {
	var out []string
	for _, g := range st.Gaps {
		out = append(out, "open gap "+gapLabel(g))
	}
	for _, j := range st.Jobs {
		if j.DisabledReason != "" {
			out = append(out, "job "+j.JobID+" is left out: "+j.DisabledReason)
		}
	}
	return out
}

// gapLabel names a gap by its dataset for a key gap and by its job for a preflight gap.
func gapLabel(g store.Gap) string {
	who := g.DatasetID
	if who == "" {
		who = g.JobID
	}
	return fmt.Sprintf("%s %s %s", g.Reason, who, g.Day)
}

// restartBlocker is the reason a restart is not safe at now, or "": a non-terminal run, or a restart-sensitive
// job between its slot start and its window close. ops.scheduler_job holds the next slot after now and the first
// window end after now, so the window is open when that end comes before the next slot (or no slot follows).
func restartBlocker(st store.SchedulerStatus, now time.Time) string {
	if n := len(st.Runs); n > 0 {
		return fmt.Sprintf("%d run(s) not finished, first %s", n, st.Runs[0].ID)
	}
	for _, j := range st.Jobs {
		if !j.RestartSensitive || !j.WindowEnd.After(now) {
			continue
		}
		if j.NextStart.IsZero() || !j.NextStart.Before(j.WindowEnd) {
			return fmt.Sprintf("job %s is restart-sensitive and its window closes at %s", j.JobID, j.WindowEnd.UTC().Format(time.RFC3339))
		}
	}
	return ""
}

func writeSchedStatus(w io.Writer, st store.SchedulerStatus, now time.Time) {
	build := st.Build
	if build == "" {
		build = "(none)"
	}
	p := func(format string, args ...any) { _, _ = fmt.Fprintf(w, format+"\n", args...) }
	p("build: %s", build)
	p("runs: %d", len(st.Runs))
	for _, r := range st.Runs {
		hb := "(none)"
		if !r.HeartbeatAt.IsZero() {
			hb = now.Sub(r.HeartbeatAt).Round(time.Second).String() + " ago"
		}
		p("  %s %s day %s heartbeat %s", r.ID, r.Status, r.Day, hb)
	}
	jobs := slices.Clone(st.Jobs)
	slices.SortFunc(jobs, func(a, b store.JobState) int { return strings.Compare(a.JobID, b.JobID) })
	p("jobs: %d", len(jobs))
	for _, j := range jobs {
		next := "(none)"
		if !j.NextStart.IsZero() {
			next = j.NextStart.UTC().Format(time.RFC3339)
		}
		line := fmt.Sprintf("  %s next %s", j.JobID, next)
		if j.DisabledReason != "" {
			line += " left out: " + j.DisabledReason
		}
		p("%s", line)
	}
	p("gaps: %d", len(st.Gaps))
	for _, g := range st.Gaps {
		line := "  " + gapLabel(g)
		if !g.ReportedAt.IsZero() {
			line += " said at " + g.ReportedAt.UTC().Format(time.RFC3339)
		}
		p("%s", line)
	}
}
