package main

import (
	"context"
	"errors"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/strategy_replay_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// Design: docs/design/strategy-replay-page.md#3 (Decisions 1, 2, 4, 5, 8, 12).

// replayPostTimeout is the deadline of one run [T]: the reads stop at it. The run itself is pure and quick.
const replayPostTimeout = 60 * time.Second

// replayGates holds the try-lock of each app: one run at a time.
var replayGates sync.Map

func replayGateOf(a *app) *atomic.Bool {
	g, _ := replayGates.LoadOrStore(a, new(atomic.Bool))
	return g.(*atomic.Bool)
}

// replayPost is one post handler: run is replayRun, a fake in a test.
type replayPost struct {
	run     func(ctx context.Context, a *app, o replayOpts) (replayResult, error)
	timeout time.Duration
}

// postStrategyReplay serves POST /strategy-replay-post. It reads and writes no store beyond what replayRun reads.
func postStrategyReplay(a *app, clk clock.Clock, rd siteReaders, newHandler pages.NewHandlerFunc, w http.ResponseWriter, r *http.Request) {
	replayPost{run: replayRun, timeout: replayPostTimeout}.serve(a, clk, rd, newHandler, w, r)
}

func (p replayPost) serve(a *app, clk clock.Clock, rd siteReaders, newHandler pages.NewHandlerFunc, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !guardOwnerPost(w, r, rd.edit) {
		return
	}
	form := strategy_replay_page.FormInput{
		Lots: r.PostForm.Get("lots"), Alias: strings.TrimSpace(r.PostForm.Get("alias")), AsOf: strings.TrimSpace(r.PostForm.Get("as_of")),
		EarlyKeep: strings.TrimSpace(r.PostForm.Get("early_keep")), EarlyDTE: strings.TrimSpace(r.PostForm.Get("early_dte")),
		MinLots: strings.TrimSpace(r.PostForm.Get("min_lots")),
	}
	buildCtx := context.WithoutCancel(r.Context())
	answer := func(status int, in strategy_replay_page.Input) {
		in.GeneratedAt = generatedAtOf(clk)
		in.Form = &form
		vm, err := strategy_replay_page.Build(buildCtx, in)
		if err != nil {
			a.log.Error("strategy replay build failed", "component", "ot", "status", "failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		newHandler(vm).ServeHTTP(&statusWriter{ResponseWriter: w, code: status}, r)
	}
	notice := func(status int, key string) {
		answer(status, strategy_replay_page.Input{View: strategy_replay_page.ViewForm, Notices: []string{key}})
	}
	opts, key := replayOptsOf(form)
	if key != "" {
		notice(http.StatusUnprocessableEntity, key)
		return
	}
	gate := replayGateOf(a)
	if !gate.CompareAndSwap(false, true) {
		notice(http.StatusOK, strategy_replay_page.NoticeBusy)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), p.timeout)
	defer cancel()
	type outcome struct {
		res replayResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		defer gate.Store(false)
		res, err := p.run(ctx, a, opts)
		done <- outcome{res, err}
	}()
	select {
	case <-ctx.Done():
		notice(http.StatusOK, strategy_replay_page.NoticeTimedOut)
	case o := <-done:
		switch {
		case errors.Is(o.err, context.DeadlineExceeded):
			notice(http.StatusOK, strategy_replay_page.NoticeTimedOut)
		case o.err != nil:
			a.log.Error("strategy replay failed", "component", "ot", "status", "failed", "line", failingLine(o.err))
			notice(http.StatusOK, strategy_replay_page.NoticeReadFailed)
		default:
			answer(http.StatusOK, strategy_replay_page.Input{View: strategy_replay_page.ViewReport, Report: replayReportInput(o.res)})
		}
	}
}

// replayOptsOf turns the posted texts into the options of a run, or the notice key of the first bad field.
func replayOptsOf(f strategy_replay_page.FormInput) (replayOpts, string) {
	o := replayOpts{lots: f.Lots, alias: f.Alias, asOf: f.AsOf, earlyKeep: f.EarlyKeep, earlyDTE: f.EarlyDTE}
	if f.Lots != lotsShortPuts && f.Lots != lotsShortLots {
		return o, strategy_replay_page.NoticeBadLots
	}
	if f.AsOf != "" {
		if _, err := time.Parse(dayLayout, f.AsOf); err != nil {
			return o, strategy_replay_page.NoticeBadAsOf
		}
	}
	if _, ok := new(big.Rat).SetString(f.EarlyKeep); !ok {
		return o, strategy_replay_page.NoticeBadEarlyKeep
	}
	if _, err := strconv.Atoi(f.EarlyDTE); err != nil {
		return o, strategy_replay_page.NoticeBadEarlyDTE
	}
	n, err := strconv.Atoi(f.MinLots)
	if err != nil || n < 0 {
		return o, strategy_replay_page.NoticeBadMinLots
	}
	o.minLots = n
	if _, _, err := replayVariants(o); err != nil {
		if strings.Contains(err.Error(), "early_dte") && !strings.Contains(err.Error(), "early_keep") {
			return o, strategy_replay_page.NoticeBadEarlyDTE
		}
		return o, strategy_replay_page.NoticeBadEarlyKeep
	}
	return o, ""
}

// failingLine names the store line an error of a run mentions, never a value: the errors of a run name the line.
func failingLine(err error) string {
	for _, name := range []string{store.DSNEnvOptionLedger, store.DSNEnvPages, store.DSNEnvRunner, bar.DSNEnvBar} {
		if strings.Contains(err.Error(), name) {
			return name
		}
	}
	return "unknown"
}
