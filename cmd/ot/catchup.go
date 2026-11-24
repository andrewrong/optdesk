package main

import (
	"context"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/data_health_page"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// Design: docs/design/auto-catch-up.md#4 (Decisions 1, 3, 4 and 7).

// The trial limits of the catch-up pass (Decision 3 and 4).
const (
	// catchUpEvery is the least gap between two passes.
	catchUpEvery = 30 * time.Minute
	// catchUpCap is the most requests one pass enqueues.
	catchUpCap = 40
	// catchUpTimeout bounds one pass.
	catchUpTimeout = 2 * time.Minute
	// catchUpRecent is how many requests the pass reads to find the ones of today.
	catchUpRecent = 1000
)

// windowEndingJobs are the four jobs whose one run fetches a window ending at the day (bars_refresh, fx_refresh,
// fred_series and iv_history), so one request for the newest gap day also fetches the older ones (Decisions 5 and 8).
var windowEndingJobs = []string{"bars_refresh", "fx_refresh", "fred_series", "iv_history"}

// catchUpCounts is what one pass did.
type catchUpCounts struct{ requested, skippedToday, skippedWindow int }

// ctxPoster stops the posts of a pass whose context has ended, so the timeout holds between two requests.
type ctxPoster struct{ jobrequest.Poster }

func (p ctxPoster) Enqueue(ctx context.Context, r jobrequest.Request) (jobrequest.Enqueued, error) {
	if err := ctx.Err(); err != nil {
		return jobrequest.Enqueued{}, err
	}
	return p.Poster.Enqueue(ctx, r)
}

// catchUpDue says whether a pass is due now and, when it is, marks it started. A host without the poster or the
// coverage reader never is.
func (h *consoleHost) catchUpDue() bool {
	if h.post == nil || h.cov == nil {
		return false
	}
	now := h.clk.Now()
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.lastPass.IsZero() && now.Sub(h.lastPass) < catchUpEvery {
		return false
	}
	h.lastPass = now
	return true
}

// catchUpPass queues the repairable gap days of every dataset as ordinary requests: the days the owner's repair
// would queue, minus those of a (job, day) already requested on today's New York day, and, for a job that fetches a
// window ending at the day, all but the newest. It enqueues at most catchUpCap requests, runs under catchUpTimeout
// and runs no job. A pass that stops early is continued by the next one.
func (h *consoleHost) catchUpPass(ctx context.Context) {
	ctx, cancel := clockTimeout(ctx, h.clk, catchUpTimeout)
	defer cancel()
	n, err := h.catchUp(ctx)
	if h.log == nil {
		return
	}
	if err != nil {
		h.log.Warn("catch-up stopped", "component", "ot", "requested", n.requested)
		return
	}
	h.log.Info("catch-up", "component", "ot", "requested", n.requested, "skipped_today", n.skippedToday, "skipped_window", n.skippedWindow)
}

func (h *consoleHost) catchUp(ctx context.Context) (n catchUpCounts, err error) {
	today, err := dayOrDefault(h.clk, "")
	if err != nil {
		return n, err
	}
	taken, err := h.requestedToday(ctx, today)
	if err != nil {
		return n, err
	}
	now := h.clk.Now().UTC()
	st := ctxPoster{h.post}
	seen := map[string]bool{}
	for _, j := range dataHealthJobs() {
		for _, ds := range j.Datasets {
			if seen[ds] {
				continue
			}
			seen[ds] = true
			jobs := repairJobsOf(ds)
			for i := range jobs {
				left := catchUpCap - n.requested
				if left <= 0 {
					return n, nil
				}
				cells, err := repairCells(ctx, h.cov, &jobs[i], ds, now, today)
				if err != nil {
					return n, err
				}
				// newest first: a newest day already requested today lets no older day through.
				if slices.Contains(windowEndingJobs, jobs[i].ID) {
					cells = newestGapOnly(cells, &n)
				}
				cells = dropTaken(cells, jobs[i].ID, taken, &n)
				q, _, err := repairGapsFor(ctx, cells, &jobs[i], st, left)
				n.requested += q
				if err != nil {
					return n, err
				}
			}
		}
	}
	return n, nil
}

// requestedToday is the set of (job, day) with a request, in any state, made on the New York day today.
func (h *consoleHost) requestedToday(ctx context.Context, today contracts.Day) (map[string]bool, error) {
	rows, err := h.post.Recent(ctx, catchUpRecent)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(marketZone)
	if err != nil {
		return nil, err
	}
	taken := map[string]bool{}
	for _, r := range rows {
		if contracts.Day(r.RequestedAt.In(loc).Format(dayLayout)) == today {
			taken[r.Request.JobID+"|"+r.Request.Day] = true
		}
	}
	return taken, nil
}

// dropTaken removes the gap cells of a (job, day) already requested today.
func dropTaken(cells []data_health_page.Cell, job string, taken map[string]bool, n *catchUpCounts) []data_health_page.Cell {
	gaps := gapSet(cells)
	out := cells[:0:0]
	for _, c := range cells {
		if gaps[c.Day] && taken[job+"|"+string(c.Day)] {
			n.skippedToday++
			continue
		}
		out = append(out, c)
	}
	return out
}

// newestGapOnly removes every gap cell but the newest one.
func newestGapOnly(cells []data_health_page.Cell, n *catchUpCounts) []data_health_page.Cell {
	gaps := data_health_page.GapDays(cells)
	if len(gaps) < 2 {
		return cells
	}
	newest := gaps[len(gaps)-1]
	gs := gapSet(cells)
	out := cells[:0:0]
	for _, c := range cells {
		if gs[c.Day] && c.Day != newest {
			n.skippedWindow++
			continue
		}
		out = append(out, c)
	}
	return out
}

func gapSet(cells []data_health_page.Cell) map[contracts.Day]bool {
	m := map[contracts.Day]bool{}
	for _, d := range data_health_page.GapDays(cells) {
		m[d] = true
	}
	return m
}
