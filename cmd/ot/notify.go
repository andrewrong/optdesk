package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

func init() { register("notify", notifyCmd) }

// notifyCmd is `ot notify --once [--dry-run] [--quiet]`: one pass of the notifier for the owner to test the
// token. It takes no lock and starts no site. With --dry-run the text goes to stdout and no gap is marked.
// Without --quiet the quiet hours are off, so a test works at any hour; --quiet keeps the default hours.
func notifyCmd(a *app, args []string) error {
	fs := flag.NewFlagSet("notify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	once := fs.Bool("once", false, "make one pass and exit")
	dry := fs.Bool("dry-run", false, "print the text; mark no gap")
	quiet := fs.Bool("quiet", false, "keep the quiet hours")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || !*once {
		return errors.New("usage: ot notify --once [--dry-run] [--quiet]")
	}
	if a.profile.Name == "demo" {
		return errors.New("notify needs a real profile; the demo profile opens no store")
	}
	cfg, err := loadConfig(a, true)
	if err != nil {
		return err
	}
	var sender notify.Sender
	if *dry {
		sender = a.deps.newDryRunSender(a.stdout)
	} else if sender, err = telegramSender(a); err != nil {
		return err
	}
	ctx := context.Background()
	fw := a.deps.newFwClock()
	return withWiring(ctx, a, a.deps.newClock(), cfg, nil, func(wiring) error {
		sl, err := a.deps.openSchedulerLedger(ctx, cfg.runnerDSN)
		if err != nil {
			return fmt.Errorf("open scheduler ledger with %s failed", store.DSNEnvRunner)
		}
		defer func() { _ = sl.Close() }()
		var src notify.GapSource = ledgerGaps{sl}
		if *dry {
			src = readOnlyGaps{src}
		}
		n, err := newNotifier(a, src, sender, fw, *quiet)
		if err != nil {
			return err
		}
		sent, err := n.NotifyGaps(ctx)
		if err != nil {
			return fmt.Errorf("notify failed (%s)", errorClass(err))
		}
		_, _ = fmt.Fprintf(a.stdout, "notify: %d gap(s) reported\n", sent)
		return nil
	})
}

// telegramSender reads the token and the chat id from .env and builds the sender. A missing line is an error
// that names the variable and nothing else.
func telegramSender(a *app) (notify.Sender, error) {
	env, err := a.envFile()
	if err != nil {
		return nil, err
	}
	for _, name := range []string{notify.TokenEnv, notify.ChatIDEnv} {
		if env[name] == "" {
			return nil, fmt.Errorf("%s is missing in .env", name)
		}
	}
	s, err := a.deps.newTelegramSender(env[notify.TokenEnv], env[notify.ChatIDEnv], notify.TelegramOptions{})
	if err != nil {
		return nil, errors.New("build the Telegram sender failed")
	}
	return s, nil
}

// newNotifier builds the notifier in the owner's zone; quiet false turns the quiet hours off (an empty window).
func newNotifier(a *app, src notify.GapSource, s notify.Sender, clk clockwork.Clock, quiet bool) (notify.Notifier, error) {
	n, err := a.deps.newNotifier(src, s, clk, notifyConfig(a, quiet))
	if err != nil {
		return nil, errors.New("build the notifier failed")
	}
	return n, nil
}

// notifyConfig is the one Config of both notifiers, gaps and alerts, so they share the quiet window and the zone.
func notifyConfig(a *app, quiet bool) notify.Config {
	c := notify.Config{Zone: a.profile.Zone}
	if !quiet {
		c.QuietFrom, c.QuietTo = "00:00", "00:00"
	}
	return c
}

// alertsDSN reads the line OT_PG_DSN_ALERTS of .env through deps.readEnv; a missing line names the key only.
func alertsDSN(a *app) (string, error) {
	env, err := a.envFile()
	if err != nil {
		return "", err
	}
	if env[store.DSNEnvAlerts] == "" {
		return "", fmt.Errorf("%s is missing in .env", store.DSNEnvAlerts)
	}
	return env[store.DSNEnvAlerts], nil
}

// newAlertNotifier opens the sent store (alerts DSN) and the winner reader (readings DSN) and builds the alert
// notifier over the same Config as the gap notifier. The reader is WinnerReadings of the view and no other
// reader, so the alert reads the rows the page reads. The returned close function closes both stores;
// no error text holds a DSN.
func newAlertNotifier(
	ctx context.Context, a *app, alertsDSN, readingsDSN string, rules compute.AlertRules, s notify.Sender, clk clockwork.Clock, quiet bool,
) (notify.AlertNotifier, func(), error) {
	sent, err := a.deps.openSent(ctx, alertsDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("open sent store with %s failed", store.DSNEnvAlerts)
	}
	view, err := a.deps.openReadingView(ctx, readingsDSN)
	if err != nil {
		_ = sent.Close()
		return nil, nil, fmt.Errorf("open reading view with %s failed", store.DSNEnvReadings)
	}
	closeAll := func() { _ = sent.Close(); _ = view.Close() }
	n, err := a.deps.newAlertNotifier(notify.AlertDeps{
		Rules: rules, Evaluate: a.deps.evaluateAlerts, Read: view.WinnerReadings, Sent: sent,
	}, s, clk, notifyConfig(a, quiet))
	if err != nil {
		closeAll()
		return nil, nil, errors.New("build the alert notifier failed")
	}
	return n, closeAll, nil
}

// errorClass names an error without its text, which may hold a secret.
func errorClass(err error) string {
	if errors.Is(err, notify.ErrSend) {
		return "send"
	}
	return "other"
}

// ledgerGaps adapts the scheduler ledger to notify.GapSource.
type ledgerGaps struct{ sl store.SchedulerLedger }

func (l ledgerGaps) OpenGaps(ctx context.Context) ([]store.Gap, error) {
	st, err := l.sl.Status(ctx)
	return st.Gaps, err
}

func (l ledgerGaps) MarkGapReported(ctx context.Context, g store.Gap, _ time.Time) error {
	return l.sl.MarkGapReported(ctx, g) // the time is the database's now()
}

// readOnlyGaps is the view of a dry run: it reads the gaps and marks none.
type readOnlyGaps struct{ src notify.GapSource }

func (r readOnlyGaps) OpenGaps(ctx context.Context) ([]store.Gap, error) { return r.src.OpenGaps(ctx) }

func (readOnlyGaps) MarkGapReported(context.Context, store.Gap, time.Time) error { return nil }

// notifyingGaps is the gap syncer the scheduler gets: it runs the gap pass, then one notify pass. A failed or
// panicking notify pass is logged by class and never reaches the ticker.
type notifyingGaps struct {
	inner  runner.GapSyncer
	n      notify.Notifier
	alerts notify.AlertNotifier // nil: no alert pass
	log    *slog.Logger
}

func (g notifyingGaps) Sync(ctx context.Context, day contracts.Day, disabled map[string]string) error {
	err := g.inner.Sync(ctx, day, disabled)
	g.notify(ctx)
	g.notifyAlerts(ctx)
	return err
}

func (g notifyingGaps) notify(ctx context.Context) {
	defer func() {
		if recover() != nil {
			g.log.Error("notify panicked", "component", "ot", "class", "panic")
		}
	}()
	if _, err := g.n.NotifyGaps(ctx); err != nil {
		g.log.Warn("notify failed", "component", "ot", "class", errorClass(err))
	}
}

// notifyAlerts is the alert pass, after the gap pass; it fails and panics on its own, apart from the gap pass.
func (g notifyingGaps) notifyAlerts(ctx context.Context) {
	if g.alerts == nil {
		return
	}
	defer func() {
		if recover() != nil {
			g.log.Error("alert notify panicked", "component", "ot", "class", "panic")
		}
	}()
	if _, err := g.alerts.NotifyAlerts(ctx); err != nil {
		g.log.Warn("alert notify failed", "component", "ot", "class", errorClass(err))
	}
}
