package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/sched"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

func init() { register("scheduler", schedulerCmd) }

const (
	// exitReplaced is the exit code of ot scheduler after a drain because another build replaced it; a
	// supervisor restarts it.
	exitReplaced = 75

	healthPortEnv     = "OT_SCHED_HEALTH_PORT"
	defaultHealthPort = 8765
	healthHost        = "127.0.0.1"
	healthStaleAfter  = 60 * time.Second

	lockWait  = 60 * time.Second // how long a second ot scheduler retries the lock
	lockEvery = 5 * time.Second
)

// stopOnSignal is deps.notifyContext: the context ends on interrupt or SIGTERM.
func stopOnSignal(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
}

// schedulerCmd is `ot scheduler`: the one resident process that owns the instance fence and runs the ticker.
// It returns an error for every way out but a drain; exitError carries the code, so no os exit happens here
// and every deferred close runs.
func schedulerCmd(a *app, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "status":
			return schedStatusCmd(a, args[1:])
		case "can-restart":
			return schedCanRestartCmd(a, args[1:])
		}
		return fmt.Errorf("usage: ot scheduler")
	}
	if a.profile.Name == "demo" {
		return fmt.Errorf("scheduler needs a real profile; the demo profile opens no store")
	}
	cfg, err := loadConfig(a, true)
	if err != nil {
		return err
	}
	sender, err := telegramSender(a)
	if err != nil {
		return err
	}
	alertsDSN, err := alertsDSN(a)
	if err != nil {
		return err
	}
	rules, err := loadAlertRules(a, "")
	if err != nil {
		return err
	}
	ln, err := listenHealth(a.getenv)
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()
	ctx, stop := a.deps.notifyContext(context.Background())
	defer stop()
	fw := a.deps.newFwClock()
	fence, err := lockFence(ctx, a, fw, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = fence.Close() }() // the ticker closes it on drain too; Close is safe twice
	err = withWiring(ctx, a, a.deps.newClock(), cfg, nil, func(w wiring) error {
		return runScheduler(ctx, a, fw, cfg, fence, ln, sender, w, alertsDSN, rules)
	})
	switch {
	case err == nil:
		return nil
	case errors.Is(err, sched.ErrExecutableChanged):
		return &exitError{code: exitReplaced, err: errors.New("scheduler stopped: another build replaced it")}
	case errors.Is(err, sched.ErrFenceLost):
		return errors.New("scheduler stopped: the instance fence was lost")
	case errors.Is(err, sched.ErrLoopFrozen):
		return errors.New("scheduler stopped: the loop froze")
	}
	return err
}

// lockFence takes the instance fence before anything else is opened. It retries every lockEvery of the clock
// for lockWait while another scheduler holds it. No error text holds a DSN.
func lockFence(ctx context.Context, a *app, clk clockwork.Clock, cfg runOnceConfig) (store.Fence, error) {
	inst, err := a.deps.openFence(ctx, cfg.runnerDSN)
	if err != nil {
		return nil, fmt.Errorf("open instance fence with %s failed", store.DSNEnvRunner)
	}
	start := clk.Now()
	for {
		fence, err := inst.TryLock(ctx, buildIDOf(version))
		if err == nil {
			return fence, nil
		}
		if clk.Since(start) >= lockWait {
			return nil, fmt.Errorf("the scheduler lock is held or cannot be taken after %s: is another ot scheduler running?", lockWait)
		}
		t := clk.NewTimer(lockEvery)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, errors.New("scheduler stopped before it took the lock")
		case <-t.Chan():
		}
	}
}

// runScheduler builds the scheduler over the open stores and runs it until it stops.
func runScheduler(
	ctx context.Context, a *app, fw clockwork.Clock, cfg runOnceConfig, fence store.Fence, ln net.Listener, sender notify.Sender, w wiring,
	alertsDSN string, rules compute.AlertRules,
) error {
	d := a.deps
	sl, err := d.openSchedulerLedger(ctx, cfg.runnerDSN)
	if err != nil {
		return fmt.Errorf("open scheduler ledger with %s failed", store.DSNEnvRunner)
	}
	defer func() { _ = sl.Close() }()
	if err := sl.SetState(ctx, buildIDOf(version)); err != nil {
		return errors.New("record the build in ops.scheduler_state failed")
	}
	disabled := schedulerDisabled(cfg, w)
	var enabled []contracts.Job
	for _, j := range w.jobs {
		if reason, off := disabled[j.ID()]; off {
			a.log.Warn("job left out", "component", "ot", "job", j.ID(), "reason", reason)
			continue
		}
		enabled = append(enabled, j)
	}
	kr := d.newKeyRunner(w.ledger, sl, fw)
	build := func(jobs []contracts.Job) runner.Reconciler {
		rec, _ := d.newReconciler(w.ledger, sl, w.cal, fw, jobs, runner.WindowRule{}, kr)
		return rec
	}
	p := &prober{w: w, log: a.log, hasKey: cfg.fredKey != ""}
	rec := newSwapReconciler(enabled, build, candidateProbe(w, a.log, p.probe), a.log)
	nf, err := newNotifier(a, ledgerGaps{sl}, sender, fw, true)
	if err != nil {
		return err
	}
	sent, err := openSent(ctx, a, alertsDSN)
	if err != nil {
		return err
	}
	defer func() { _ = sent.Close() }()
	an, closeAlerts, err := newAlertNotifier(ctx, a, sent, cfg.readingsDSN, rules, sender, fw, true)
	if err != nil {
		return err
	}
	defer closeAlerts()
	mn, closeMorning, err := newMorningNotifier(ctx, a, cfg.pagesDSN, sent, sender, fw, true)
	if err != nil {
		return err
	}
	defer closeMorning()
	gaps := notifyingGaps{inner: d.newGapSyncer(w.ledger, sl, fw), n: nf, alerts: an, morn: mn, log: a.log}
	s := d.newScheduler(fence, sl, w.ledger, w.cal, fw, rec, gaps, w.jobs, disabled, execIdentity)

	srv := &http.Server{Handler: healthHandler(s, fw), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()
	a.log.Info("scheduler running", "component", "ot", "status", "ok")
	err = s.Run(ctx)
	switch {
	case err == nil, errors.Is(err, sched.ErrExecutableChanged), errors.Is(err, sched.ErrFenceLost), errors.Is(err, sched.ErrLoopFrozen):
		return err
	}
	return errors.New("scheduler stopped with an error; its text is withheld because it may hold a DSN or a key")
}

// execIdentity is the runner.ExecIdentityFunc: size, mtime and inode of the running executable as one string.
func execIdentity() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", errors.New("cannot find the executable")
	}
	fi, err := os.Stat(exe)
	if err != nil {
		return "", errors.New("cannot stat the executable")
	}
	var ino uint64
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		ino = uint64(st.Ino) //nolint:unconvert // the type of Ino differs between platforms
	}
	return fmt.Sprintf("%d-%d-%d", fi.Size(), fi.ModTime().UnixNano(), ino), nil
}

// healthAddr is the address of GET /health: the loopback address (not the name localhost) at the port of
// OT_SCHED_HEALTH_PORT, or the default.
func healthAddr(getenv func(string) string) (string, error) {
	port := defaultHealthPort
	if v := getenv(healthPortEnv); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 65535 {
			return "", fmt.Errorf("%s is not a port number", healthPortEnv)
		}
		port = n
	}
	return net.JoinHostPort(healthHost, strconv.Itoa(port)), nil
}

func listenHealth(getenv func(string) string) (net.Listener, error) {
	addr, err := healthAddr(getenv)
	if err != nil {
		return nil, err
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("scheduler: cannot listen on the health address %s", addr)
	}
	return ln, nil
}

// healthHandler answers GET /health: 200 while the fence is held and the last tick is under 60 seconds old
// on clk, else 503. It reads the scheduler and nothing else.
func healthHandler(s runner.Scheduler, clk clockwork.Clock) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path != "/health":
			http.NotFound(w, r)
		case r.Method != http.MethodGet:
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		case s.FenceHeld() && clk.Since(s.LastTick()) < healthStaleAfter:
			_, _ = w.Write([]byte("ok\n"))
		default:
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
		}
	})
}

// candidateProbe is the probe of the prober with option_candidates added: the line of .env is read once at
// start, so a missing one keeps the job out until the process restarts, as for earnings_refresh.
func candidateProbe(w wiring, log *slog.Logger, next func(context.Context, string) (contracts.Job, string)) func(context.Context, string) (contracts.Job, string) {
	return func(ctx context.Context, id string) (contracts.Job, string) {
		if id != jobOptionCandidates {
			return next(ctx, id)
		}
		if w.candidatesOff != "" {
			return nil, w.candidatesOff
		}
		j, err := w.factory.optionCandidates()
		if err != nil {
			log.Warn("job build failed", "component", "ot", "job", id, "err", err)
			return nil, reasonBuildJob
		}
		return j, ""
	}
}

// schedulerDisabled is startDisabled with option_candidates added when its line is missing and the job is wired.
func schedulerDisabled(cfg runOnceConfig, w wiring) map[string]string {
	out := startDisabled(cfg, w.brokerUp)
	if r := cfg.candidatesOff(); r != "" && w.cand != nil {
		out[jobOptionCandidates] = r
	}
	return out
}
