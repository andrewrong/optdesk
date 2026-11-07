package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

func init() { register("run-once", runOnce) }

// maxSocketPath is the length from which a Unix socket path is refused: sun_path holds 104 bytes on macOS.
const maxSocketPath = 104

// runOnceConfig is what run-once needs from $OT_HOME/.env and the home; its values are never printed.
type runOnceConfig struct {
	runnerDSN, seriesDSN, optionEodDSN, quoteSnapshotDSN, masterDSN, fredKey string
	socketPath                                                               string
}

func loadRunOnceConfig(a *app) (runOnceConfig, error) {
	env, err := a.envFile()
	if err != nil {
		return runOnceConfig{}, err
	}
	c := runOnceConfig{
		runnerDSN:        env[store.DSNEnvRunner],
		seriesDSN:        env[store.DSNEnvSeriesFred],
		optionEodDSN:     env[store.DSNEnvOptionEod],
		quoteSnapshotDSN: env[store.DSNEnvQuoteSnapshot],
		masterDSN:        env[store.DSNEnvMaster],
		fredKey:          env["FRED_API_KEY"],
	}
	for _, l := range []struct{ name, v string }{
		{store.DSNEnvRunner, c.runnerDSN}, {store.DSNEnvSeriesFred, c.seriesDSN},
		{store.DSNEnvOptionEod, c.optionEodDSN}, {store.DSNEnvQuoteSnapshot, c.quoteSnapshotDSN},
		{store.DSNEnvMaster, c.masterDSN}, {"FRED_API_KEY", c.fredKey},
	} {
		if l.v == "" {
			return runOnceConfig{}, fmt.Errorf("%s is missing in .env", l.name)
		}
	}
	c.socketPath = filepath.Join(a.getenv("OT_HOME"), "run", "ibkr.sock")
	if len(c.socketPath) >= maxSocketPath {
		return runOnceConfig{}, fmt.Errorf("socket path too long")
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

// reconcileDay opens the stores, wires the jobs and reconciles day; it closes the stores on every path.
func reconcileDay(ctx context.Context, a *app, clk clock.Clock, cfg runOnceConfig, day contracts.Day) error {
	check := func(ledger store.OpsLedger) error {
		cur, err := ledger.Cursor(ctx)
		if err != nil {
			return fmt.Errorf("read reconcile cursor failed")
		}
		if cur.Day != "" && day <= cur.Day {
			return fmt.Errorf("day %s is at or before the cursor %s: nothing to reconcile; use ot backfill --day %s", day, cur.Day, day)
		}
		return nil
	}
	return withWiring(ctx, a, clk, cfg, check, func(w wiring) error {
		rec := a.deps.newRunner(w.ledger, w.cal, clk, w.jobs, runner.WindowRule{})
		return rec.Reconcile(ctx, day)
	})
}

// wiring is what run-once and backfill build from the open stores.
type wiring struct {
	ledger store.OpsLedger
	cal    clock.Calendar
	jobs   []contracts.Job
}

// withWiring opens the stores and the broker, wires fred_series, option_eod and quote_snapshot, then
// calls use; it closes what it opened on every path, last opened first. check runs after the ledger is
// open and before the other stores are opened; it may be nil. A failed dial does not stop the call: the
// option jobs get a broker that answers retryable. No error text holds a DSN, a path or a key.
func withWiring(
	ctx context.Context, a *app, clk clock.Clock, cfg runOnceConfig, check func(store.OpsLedger) error, use func(wiring) error,
) error {
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
	if check != nil {
		if err := check(ledger); err != nil {
			return err
		}
	}
	ser, err := d.openSeries(ctx, cfg.seriesDSN)
	if err != nil {
		return fmt.Errorf("open series store with %s failed", store.DSNEnvSeriesFred)
	}
	defer func() { _ = ser.Close() }()
	eod, err := d.openOptionEod(ctx, cfg.optionEodDSN)
	if err != nil {
		return fmt.Errorf("open option_eod store with %s failed", store.DSNEnvOptionEod)
	}
	defer func() { _ = eod.Close() }()
	snap, err := d.openQuoteSnapshot(ctx, cfg.quoteSnapshotDSN)
	if err != nil {
		return fmt.Errorf("open quote_snapshot store with %s failed", store.DSNEnvQuoteSnapshot)
	}
	defer func() { _ = snap.Close() }()
	instr, err := d.openInstrument(ctx, cfg.masterDSN)
	if err != nil {
		return fmt.Errorf("open instrument store with %s failed", store.DSNEnvMaster)
	}
	defer func() { _ = instr.Close() }()
	src, err := d.newFRED(sources.FREDConfig{BaseURL: fredBaseURL, APIKey: cfg.fredKey, Clock: clk})
	if err != nil {
		return fmt.Errorf("build FRED source failed")
	}
	brk := dialOrStandIn(ctx, a, cfg.socketPath)
	defer func() { _ = brk.Close() }()
	fredJob, err := d.newFredSeries(src, ser, clk)
	if err != nil {
		return fmt.Errorf("build fred_series job failed: %w", err)
	}
	eodJob, err := d.newOptionEod(brk, instr, eod, clk)
	if err != nil {
		return fmt.Errorf("build option_eod job failed")
	}
	snapJob, err := d.newQuoteSnapshot(brk, instr, snap, clk)
	if err != nil {
		return fmt.Errorf("build quote_snapshot job failed")
	}
	return use(wiring{ledger: ledger, cal: d.newCalendar(sd), jobs: []contracts.Job{fredJob, eodJob, snapJob}})
}

// dialOrStandIn dials the broker once; a failure is logged by step name only and gives the stand-in
// broker, whose Close does nothing.
func dialOrStandIn(ctx context.Context, a *app, socketPath string) broker.Broker {
	b, err := a.deps.dialBroker(ctx, socketPath, broker.Options{})
	if err != nil {
		a.log.Error("dial broker failed: the option jobs answer retryable", "component", "ot", "step", "dial broker")
		return unreachableBroker{}
	}
	return b
}
