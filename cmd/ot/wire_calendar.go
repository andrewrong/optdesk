package main

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/clock/calendarcli"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Design: docs/design/ot-wire-split.md#Decisions 2 and 7.

func init() {
	registerJob(jobFamily{
		id: "calendar_refresh", order: 80,
		build: func(_ context.Context, _ *app, jc *jobCtx) (contracts.Job, string, error) {
			j, err := jc.f.calendarRefresh()
			return j, "", err
		},
	})
}

// calendarRefresh builds calendar_refresh over the loader store; the command is the uv program of the .env
// line (default uv) and calendarArgs, run by calendarcli.Exec in the default temporary directory.
func (f jobFactory) calendarRefresh() (contracts.Job, error) {
	uv := f.uv
	if uv == "" {
		uv = "uv"
	}
	argv := append([]string{uv}, calendarArgs...)
	src := f.d.newCommandSessionSource(argv, "", calendarcli.Exec)
	// The log handler passes only the keys of profile.logFields, so the change is one value of "status".
	obs := func(c clock.SessionDayChange) {
		f.log.Info("calendar change", "component", "ot", "job", "calendar_refresh",
			"status", fmt.Sprintf("day=%s before=[%s] after=[%s]", c.Day, describeDay(c.Before), describeDay(c.After)))
	}
	j, err := f.d.newCalendarRefresh(src, f.loader, f.d.newDiffer(), f.clk, obs)
	if err != nil {
		return nil, fmt.Errorf("build calendar_refresh job failed: %w", err)
	}
	return j, nil
}

// describeDay renders one session-day row for the log; nil is "none" (the day is absent on that side).
func describeDay(d *contracts.SessionDay) string {
	if d == nil {
		return "none"
	}
	return fmt.Sprintf("session=%t early_close=%t open=%s close=%s", d.IsSession, d.EarlyClose,
		d.OpenAt.UTC().Format(time.RFC3339), d.CloseAt.UTC().Format(time.RFC3339))
}
