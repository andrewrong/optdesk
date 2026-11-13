package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/ops_today_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ops"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

// Design: docs/design/option-open-gap-jp.md#Same-morning visibility (Decision 11, entry 8).

// siteOpsTodayRoute is the address of the page ops_today_page under the site root; opsTodayName is its name for
// ot page.
const (
	siteOpsTodayRoute = "options/today"
	opsTodayName      = "ops_today_page"
	opsTodaySchema    = "page.ops_today_page.v1"
	// siteOpsTodayKey is the key of the run reader that the site opener puts in siteReaders.ext.
	siteOpsTodayKey = "ops_today_runs"
)

// openRunsFunc opens the run store with a DSN (ops.OpenRuns; a test replaces it).
type openRunsFunc func(ctx context.Context, dsn string) (ops.RunStore, error)

// realOpsTodayDeps sets the real opener of the run store (the runs and reasons of a session day).
func realOpsTodayDeps(d *deps) { d.openRuns = ops.OpenRuns }

func init() {
	registerPage(pageFamily{name: opsTodayName, route: siteOpsTodayRoute, order: 110, build: buildOpsToday, named: opsTodayPageNamed})
	siteOpeners[siteOpsTodayKey] = openOpsTodayRuns
}

// readOnlyRuns hides the writer of the run store: the page reads and nothing else.
type readOnlyRuns struct{ runview.Reader }

// openOpsTodayRuns opens the run store with the pages line (the role reads the two rd views only) and gives it
// as a reader. Without an opener or the line it gives no reader and goes on: the page then shows no row.
func openOpsTodayRuns(ctx context.Context, a *app, track func(io.Closer, error) bool) (any, bool) {
	if a.deps.openRuns == nil {
		return nil, true
	}
	env, err := a.envFile()
	if err != nil {
		return nil, false
	}
	if env[pagesDSNName] == "" {
		return nil, true
	}
	s, err := a.deps.openRuns(ctx, env[pagesDSNName])
	if !track(s, err) {
		return nil, false
	}
	return runview.Reader(readOnlyRuns{s}), true
}

// buildOpsToday builds ops_today_page per request for the day of dayFlag or today from the reader of the site (nil:
// no row).
func buildOpsToday(ctx context.Context, _ *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	r, _ := siteReaderOf[runview.Reader](rd, siteOpsTodayKey)
	return newOpsTodayPage(r).Build(ctx, nil, day, clk.Now().UTC())
}

// newOpsTodayPage is the page ops_today_page over r (nil: an empty page). It reads the runs and the reasons of
// the day and the not_wired set, and writes nothing.
func newOpsTodayPage(r runview.Reader) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: opsTodayName, Schema: opsTodaySchema},
		Build: func(ctx context.Context, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			var runs []runview.Row
			var reasons []runview.Reason
			if r != nil {
				var err error
				if runs, err = r.RunsOn(ctx, day); err != nil {
					return nil, err
				}
				if reasons, err = r.ReasonsOn(ctx, day); err != nil {
					return nil, err
				}
				nw, err := r.NotWired(ctx)
				if err != nil {
					return nil, err
				}
				reasons = append(reasons, nw...)
			}
			return ops_today_page.Build(runs, reasons, at)
		},
	}
}

// opsTodayPageNamed returns ops_today_page for ot page and a close function: the run store opened with the pages
// line, none with a missing line or the demo profile.
func opsTodayPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Name == "demo" || a.deps.openRuns == nil {
		return newOpsTodayPage(nil), func() {}, nil
	}
	env, err := a.envFile()
	if err != nil {
		return pages.Page{}, nil, err
	}
	if env[pagesDSNName] == "" {
		return newOpsTodayPage(nil), func() {}, nil
	}
	s, err := a.deps.openRuns(ctx, env[pagesDSNName])
	if err != nil {
		return pages.Page{}, nil, openRunsFailed(pagesDSNName)
	}
	return newOpsTodayPage(readOnlyRuns{s}), func() { _ = s.Close() }, nil
}

// openLedgerWithReasons opens the ledger of a wiring and gives it a reason writer when it has none of its own: the
// ledger that ops.Open returns does not write reasons, the run store of the same runner DSN does. It returns the
// ledger to use and a function that closes what it opened. Without an opener (a test) the ledger is as it is.
func openLedgerWithReasons(ctx context.Context, d deps, dsn string) (store.OpsLedger, func(), error) {
	l, err := d.openLedger(ctx, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open ledger with %s failed", store.DSNEnvRunner)
	}
	if _, ok := l.(runview.ReasonWriter); ok || d.openRuns == nil {
		return l, func() { _ = l.Close() }, nil
	}
	s, err := d.openRuns(ctx, dsn)
	if err != nil {
		_ = l.Close()
		return nil, nil, openRunsFailed(store.DSNEnvRunner)
	}
	both := struct {
		store.OpsLedger
		runview.ReasonWriter
	}{l, s}
	return both, func() { _ = s.Close(); _ = l.Close() }, nil
}

func openRunsFailed(line string) error { return fmt.Errorf("open run store with %s failed", line) }
