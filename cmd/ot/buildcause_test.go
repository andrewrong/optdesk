package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/accountcapture"
	"github.com/andrewrong/optdesk/internal/jobs/calendarrefresh"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

// TestJobBuildErrorKeepsCause: each job factory method returns an error that starts with "build " and
// carries the error of the constructor in its chain.
func TestJobBuildErrorKeepsCause(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id    string
		build func(t *testing.T, cause error) error
	}{
		{"earnings_refresh", func(_ *testing.T, cause error) error {
			d := newDeps()
			d.newEarningsRefresh = func(
				_ map[string]sources.EarningsSource, _ store.EarningsDateWriter, _ store.EarningsDateReader, _ []jobs.WatchEntry, _ clock.Clock, _ symbol.Translator,
			) (contracts.Job, error) {
				return nil, cause
			}
			_, err := jobFactory{d: d}.earningsRefresh()
			return err
		}},
		{"calendar_refresh", func(_ *testing.T, cause error) error {
			d := newDeps()
			d.newCalendarRefresh = func(
				clock.SessionSource,
				store.SessionDayStore,
				clock.SessionDayDiffer,
				clock.Clock,
				calendarrefresh.CalendarObserver,
			) (contracts.Job, error) {
				return nil, cause
			}
			_, err := jobFactory{d: d, log: slog.New(slog.DiscardHandler)}.calendarRefresh()
			return err
		}},
		{"compute_figures", func(_ *testing.T, cause error) error {
			d := newDeps()
			d.newComputeFigures = func(
				_ computefigures.ComputeStores, _ compute.Figures, _ compute.Rules, _ compute.Units, _ compute.SeriesOps, _ compute.Ease, _ clock.Clock,
			) (contracts.Job, error) {
				return nil, cause
			}
			_, err := jobFactory{d: d}.computeFigures()
			return err
		}},
		{"option_eod", func(_ *testing.T, cause error) error {
			d := newDeps()
			d.newOptionEodWatch = func(
				broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []jobs.WatchEntry, clock.Clock, symbol.Translator,
			) (contracts.Job, error) {
				return nil, cause
			}
			_, err := jobFactory{d: d}.optionEod(nil)
			return err
		}},
		{"option_eod_jp", func(_ *testing.T, cause error) error {
			d := newDeps()
			d.newOptionEodJP = func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock) (contracts.Job, error) {
				return nil, cause
			}
			d.newOptionEodJPWatch = nil
			_, err := jobFactory{d: d}.optionEodJP(nil)
			return err
		}},
		{"quote_snapshot", func(_ *testing.T, cause error) error {
			d := newDeps()
			d.newQuoteSnapshot = func(broker.Broker, store.InstrumentResolver, store.QuoteSnapshotWriter, clock.Clock) (contracts.Job, error) {
				return nil, cause
			}
			_, err := jobFactory{d: d}.quoteSnapshot(nil)
			return err
		}},
		{"option_candidates", func(_ *testing.T, cause error) error {
			d := newDeps()
			d.newOptionCandidates = func(optioncandidates.CandidateStores, compute.OptionCandidates, []jobs.WatchEntry, clock.Clock) (contracts.Job, error) {
				return nil, cause
			}
			_, err := jobFactory{d: d}.optionCandidates()
			return err
		}},
		{"account_capture", func(t *testing.T, cause error) error {
			r := newAcctRig(t, gwMainB, "a=/s/a.sock,b=/s/b.sock")
			r.a.deps.newAccountCapture = func(accountcapture.Config) (contracts.Job, error) { return nil, cause }
			var extra closerSet
			_, err := r.a.accountCapture(context.Background(), jobFactory{d: r.a.deps}, &extra)
			return err
		}},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			t.Parallel()
			cause := errors.New("cause-" + c.id)
			err := c.build(t, cause)
			if err == nil {
				t.Fatal("no error")
			}
			if !errors.Is(err, cause) || !strings.Contains(err.Error(), "cause-"+c.id) {
				t.Fatalf("err %q does not carry the cause", err)
			}
			if !strings.HasPrefix(err.Error(), "build ") {
				t.Fatalf("err %q does not start with build", err)
			}
		})
	}
}

// TestProberBuiltLogsCause: a failed build returns reasonBuildJob and leaves the cause in the log.
func TestProberBuiltLogsCause(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := &prober{log: slog.New(slog.NewTextHandler(&buf, nil))}
	j, reason := p.built("option_eod", nil, errors.New("cause-built"))
	if j != nil || reason != reasonBuildJob {
		t.Fatalf("got %v %q", j, reason)
	}
	out := buf.String()
	for _, want := range []string{"job build failed", "job=option_eod", "cause-built"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log %q lacks %q", out, want)
		}
	}
}

// TestCandidateProbeLogsCause: a failed option_candidates build returns reasonBuildJob and logs the cause.
func TestCandidateProbeLogsCause(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	d := newDeps()
	d.newOptionCandidates = func(optioncandidates.CandidateStores, compute.OptionCandidates, []jobs.WatchEntry, clock.Clock) (contracts.Job, error) {
		return nil, errors.New("cause-probe")
	}
	probe := candidateProbe(wiring{factory: jobFactory{d: d, log: slog.New(slog.NewTextHandler(&buf, nil))}},
		func(context.Context, string) (contracts.Job, string) { return nil, "" })
	j, reason := probe(context.Background(), jobOptionCandidates)
	if j != nil || reason != reasonBuildJob || !strings.Contains(buf.String(), "cause-probe") {
		t.Fatalf("got %v %q log %q", j, reason, buf.String())
	}
}
