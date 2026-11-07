package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

func init() { register("run-once", runOnce) }

// runOnceConfig is what run-once needs from $OT_HOME/.env; its values are never printed.
type runOnceConfig struct{ runnerDSN, seriesDSN, fredKey string }

func loadRunOnceConfig(a *app) (runOnceConfig, error) {
	env, err := a.envFile()
	if err != nil {
		return runOnceConfig{}, err
	}
	c := runOnceConfig{runnerDSN: env[store.DSNEnvRunner], seriesDSN: env[store.DSNEnvSeriesFred], fredKey: env["FRED_API_KEY"]}
	for name, v := range map[string]string{store.DSNEnvRunner: c.runnerDSN, store.DSNEnvSeriesFred: c.seriesDSN, "FRED_API_KEY": c.fredKey} {
		if v == "" {
			return runOnceConfig{}, fmt.Errorf("%s is missing in .env", name)
		}
	}
	return c, nil
}

func runOnce(a *app, args []string) error {
	fs := flag.NewFlagSet("run-once", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dayFlag := fs.String("day", "", "day YYYY-MM-DD (default: today in "+marketZone+")")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if a.profile.Name == "demo" {
		return fmt.Errorf("run-once needs a real profile; the demo profile opens no store")
	}
	cfg, err := loadRunOnceConfig(a)
	if err != nil {
		return err
	}
	ctx := context.Background()
	clk := a.deps.newClock()
	day, err := dayOrDefault(clk, *dayFlag)
	if err != nil {
		return err
	}
	if err := reconcileDay(ctx, a, clk, cfg, day); err != nil {
		return fmt.Errorf("run-once %s: %w", day, err)
	}
	a.log.Info("run-once "+string(day)+" done", "component", "ot", "status", "ok")
	return nil
}

// reconcileDay opens the stores, wires the job and reconciles day; it closes the stores on every path.
func reconcileDay(ctx context.Context, a *app, clk clock.Clock, cfg runOnceConfig, day contracts.Day) error {
	d := a.deps
	sd, err := d.openSessionDay(ctx, cfg.runnerDSN)
	if err != nil {
		return fmt.Errorf("open session-day store with %s failed", store.DSNEnvRunner)
	}
	defer func() { _ = sd.Close() }()
	ledger, err := d.openLedger(ctx, cfg.runnerDSN)
	if err != nil {
		return fmt.Errorf("open ledger with %s failed", store.DSNEnvRunner)
	}
	defer func() { _ = ledger.Close() }()
	cur, err := ledger.Cursor(ctx)
	if err != nil {
		return fmt.Errorf("read reconcile cursor failed")
	}
	if cur.Day != "" && day <= cur.Day {
		return fmt.Errorf("day %s is at or before the cursor %s: nothing to reconcile", day, cur.Day)
	}
	ser, err := d.openSeries(ctx, cfg.seriesDSN)
	if err != nil {
		return fmt.Errorf("open series store with %s failed", store.DSNEnvSeriesFred)
	}
	defer func() { _ = ser.Close() }()
	src, err := d.newFRED(sources.FREDConfig{BaseURL: fredBaseURL, APIKey: cfg.fredKey, Clock: clk})
	if err != nil {
		return fmt.Errorf("build FRED source failed")
	}
	job, err := d.newFredSeries(src, ser, clk)
	if err != nil {
		return fmt.Errorf("build fred_series job failed: %w", err)
	}
	rec := d.newRunner(ledger, d.newCalendar(sd), clk, []contracts.Job{job}, ruleOf(job))
	return rec.Reconcile(ctx, day)
}

// windowed is the optional method of a job that states its [window] table.
type windowed interface {
	Window() (calendarID string, openOffset, closeOffset time.Duration)
}

// ruleOf is the window rule the job states, or the zero rule when it states none.
func ruleOf(j contracts.Job) runner.WindowRule {
	w, ok := j.(windowed)
	if !ok {
		return runner.WindowRule{}
	}
	id, open, closeOff := w.Window()
	return runner.WindowRule{CalendarID: id, OpenOffset: open, CloseOffset: closeOff}
}
