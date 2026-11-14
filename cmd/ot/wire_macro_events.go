package main

import (
	"context"
	"fmt"
	"io"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/macroevents"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
	"github.com/andrewrong/optdesk/internal/store/macroeventpg"
)

// Design: docs/design/macro-events.md#Decisions 11.

const (
	jobMacroEvents = "macro_events"

	// extMacroEvents is the key of macroEventsDeps in deps.ext.
	extMacroEvents = "macroevents"
)

// macroEventsDeps is the ext deps of macro_events and of the reader of the macro page. A missing entry or a nil
// field leaves the job out (and the page without events); deps and newDeps are not edited.
type macroEventsDeps struct {
	newJob     macroevents.NewMacroEventsFunc
	newRelease func(baseURL, key string) (macroevents.ReleaseSource, error)
	openWriter func(ctx context.Context, dsn string) (macroevent.Writer, io.Closer, error)
	openReader func(ctx context.Context, dsn string) (macroevent.Reader, io.Closer, error)
}

func init() {
	extDeps[extMacroEvents] = func() any {
		return macroEventsDeps{
			newJob: macroevents.New, newRelease: macroevents.NewRelease,
			openWriter: macroeventpg.OpenWriter, openReader: macroeventpg.OpenReader,
		}
	}
	// order 210 is book_risk's; this is the next free multiple of ten.
	registerJob(jobFamily{
		id: jobMacroEvents, order: 220,
		startOff: func(cfg runOnceConfig, _ bool) string {
			if cfg.fredKey == "" {
				return reasonNoFredKey
			}
			return ""
		},
		build: macroEventsJob,
	})
}

// noKeyReleases is the source of a job held for an empty key: the job exists for the schedule, and no run reaches it.
type noKeyReleases struct{}

func (noKeyReleases) Dates(context.Context, string, contracts.Day) ([]contracts.Day, error) {
	return nil, fmt.Errorf("macro_events: %s", reasonNoFredKey)
}

// macroEventsJob builds the job on the key and base URL of fred_series and a writer on the series store, closed
// through jc.extra. It is nil without the ext deps or the series line, and held with reasonNoFredKey for an empty key.
func macroEventsJob(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
	md, ok := jc.f.d.ext[extMacroEvents].(macroEventsDeps)
	if !ok || md.newJob == nil || md.newRelease == nil || md.openWriter == nil {
		return nil, "", nil
	}
	if jc.cfg.seriesDSN == "" {
		a.log.Warn("job left out", "component", "ot", "job", jobMacroEvents, "status", store.DSNEnvSeriesFred+" is not set")
		return nil, "", nil
	}
	var src macroevents.ReleaseSource = noKeyReleases{}
	reason := reasonNoFredKey
	if jc.cfg.fredKey != "" {
		s, err := md.newRelease(fredBaseURL, jc.cfg.fredKey)
		if err != nil {
			return nil, "", fmt.Errorf("build macro_events source failed: %w", err)
		}
		src, reason = s, ""
	}
	w, c, err := md.openWriter(ctx, jc.cfg.seriesDSN)
	if err != nil {
		return nil, "", fmt.Errorf("open macro event store with %s failed", store.DSNEnvSeriesFred)
	}
	jc.extra.addCloser(c)
	j, err := md.newJob(src, w, jc.f.clk)
	if err != nil {
		return nil, "", fmt.Errorf("build macro_events job failed: %w", err)
	}
	return j, reason, nil
}
