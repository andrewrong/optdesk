package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/jobrunapi"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/store/jobrun"
	"github.com/jonboulle/clockwork"
)

// slowNotifier holds no lock set and no cancel func: a slow run is a notice only.
type slowNotifier struct {
	deps   notify.SlowDeps
	sender notify.Sender
	clock  clockwork.Clock
	zone   *time.Location
	win    window
	log    *slog.Logger
}

// NewSlow is notify.NewSlowNotifierFunc. It fails only on a bad Config or an incomplete SlowDeps.
func NewSlow(deps notify.SlowDeps, sender notify.Sender, clock clockwork.Clock, cfg notify.Config) (notify.SlowNotifier, error) {
	if sender == nil || clock == nil {
		return nil, errors.New("notify: sender and clock are required")
	}
	if deps.Runs == nil || deps.Mark == nil || deps.Figures == nil {
		return nil, errors.New("notify: SlowDeps.Runs, Mark and Figures are required")
	}
	if cfg.Zone == nil {
		return nil, errors.New("notify: Config.Zone is required")
	}
	from, to := cfg.QuietFrom, cfg.QuietTo
	if from == "" {
		from = notify.DefaultQuietFrom
	}
	if to == "" {
		to = notify.DefaultQuietTo
	}
	w, err := parseWindow(from, to)
	if err != nil {
		return nil, err
	}
	return &slowNotifier{deps: deps, sender: sender, clock: clock, zone: cfg.Zone, win: w, log: slog.Default()}, nil
}

var _ notify.NewSlowNotifierFunc = NewSlow

// NotifySlowRuns makes one pass; see notify.SlowNotifier.
func (n *slowNotifier) NotifySlowRuns(ctx context.Context) (int, error) {
	now := n.clock.Now()
	if quiet(now, n.zone, n.win) {
		return 0, nil
	}
	running, err := n.deps.Runs.Running(ctx)
	if err != nil {
		return 0, fmt.Errorf("notify: read running jobs: %w", err)
	}
	var fresh []jobrun.JobRun
	var in []jobrunapi.Input
	for _, r := range running {
		if !r.SlowNotifiedAt.IsZero() {
			continue
		}
		fresh = append(fresh, r)
		in = append(in, jobrunapi.Input{JobID: r.JobID, StartedAt: r.StartedAt, KeysDone: r.KeysDone, KeysTotal: r.KeysTotal, Step: r.Step})
	}
	if len(fresh) == 0 {
		return 0, nil
	}
	stats, err := n.deps.Runs.Usuals(ctx)
	if err != nil {
		return 0, fmt.Errorf("notify: read usual durations: %w", err)
	}
	usuals := make([]jobrunapi.Usual, len(stats))
	for i, s := range stats {
		usuals[i] = jobrunapi.Usual{JobID: s.JobID, Median: s.Median, Samples: s.Samples}
	}
	figs := n.deps.Figures(now, in, usuals)
	sent := 0
	for i, f := range figs {
		if i >= len(fresh) {
			break
		}
		if !f.Slow {
			continue
		}
		r := fresh[i]
		text := slowRunText(r.JobID, f.ElapsedMinutes, slowUsualText(f.HasUsual, f.UsualMinutes), r.KeysDone, r.KeysTotal)
		if err := n.sender.Send(ctx, text); err != nil {
			return sent, err
		}
		n.log.Info("job_run_slow",
			slog.String("job", r.JobID),
			slog.Int64("elapsed_s", int64(now.Sub(r.StartedAt)/time.Second)),
			slog.Int("usual_s", f.UsualMinutes*60),
			slog.Int("keys_done", r.KeysDone),
			slog.Int("keys_total", r.KeysTotal),
			slog.String("source", r.Source))
		if err := n.deps.Mark.MarkSlowNotified(ctx, r.ID, now); err != nil {
			return sent, fmt.Errorf("notify: mark slow run notified: %w", err)
		}
		sent++
	}
	return sent, nil
}
