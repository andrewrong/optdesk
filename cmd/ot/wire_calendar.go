package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// envCalendarProject names the optional line of .env that sets the directory of the pyside project.
const envCalendarProject = "OT_CALENDAR_PROJECT"

// calendarCommand is the command of calendar_refresh up to the arguments calendarcli adds:
// `uv run --locked --project <project> python -m optdesk.calendar`; project must be absolute so that
// the child starts from any working directory.
func calendarCommand(uv, project string) []string {
	return []string{uv, "run", "--locked", "--project", project, "python", "-m", "optdesk.calendar"}
}

// hasPyproject is the real deps.hasProject.
func hasPyproject(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "pyproject.toml"))
	return err == nil
}

// calendarProjectDir returns the absolute pyside project directory: the line OT_CALENDAR_PROJECT, or
// pyside under the working directory of ot. It fails when the directory holds no pyproject.toml.
func (f jobFactory) calendarProjectDir() (string, error) {
	dir := f.project
	if dir == "" {
		dir = "pyside"
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("calendar_refresh: resolve the project directory failed, set %s: %w", envCalendarProject, err)
	}
	if !f.d.hasProject(dir) {
		return "", errors.New("calendar_refresh: the pyside project has no pyproject.toml, set the line OT_CALENDAR_PROJECT or run ot from the checkout")
	}
	return dir, nil
}

// calendarRefresh builds calendar_refresh over the loader store; the command is the uv program of the .env
// line (default uv) and calendarCommand, run by calendarcli.Exec in the default temporary directory.
func (f jobFactory) calendarRefresh() (contracts.Job, error) {
	uv := f.uv
	if uv == "" {
		uv = "uv"
	}
	dir, err := f.calendarProjectDir()
	if err != nil {
		return nil, err
	}
	src := f.d.newCommandSessionSource(calendarCommand(uv, dir), "", calendarcli.Exec)
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
