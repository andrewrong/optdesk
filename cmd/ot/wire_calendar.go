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
	"github.com/andrewrong/optdesk/internal/clock/ibkrhours"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/calendarhours"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/hoursnotice"
	"github.com/andrewrong/optdesk/internal/store/hoursnoticepg"
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

// Design: docs/design/calendar-ibkr-hours.md#8 ot-wire-calendar-hours (Decisions 6, 7, 8 and 9).

// openHoursNotices opens the hours-notice store with the master line; a package variable so that the tests, which
// have no database, install a fake in an init().
var openHoursNotices = func(ctx context.Context, dsn string) (hoursnotice.Store, error) {
	st, err := hoursnoticepg.Open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return st, nil
}

func init() {
	for _, h := range []struct {
		id, market string
		order      int
	}{{"calendar_hours_us", "XNYS", 85}, {"calendar_hours_jp", "XTKS", 86}} {
		registerJob(jobFamily{
			id: h.id, order: h.order,
			// A job whose line is empty is left out whatever the broker does, so the broker reason is for a set line only.
			startOff: func(cfg runOnceConfig, brokerUp bool) string {
				if cfg.masterDSN == "" {
					return ""
				}
				return brokerDownOff(cfg, brokerUp)
			},
			build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
				st := jc.hoursStore(ctx, a, h.id)
				if st == nil {
					return nil, "", nil
				}
				j, err := calendarhours.New(h.market, jc.brk, jc.f.loader, ibkrhours.New(), st, jc.f.clk, a.log)
				if err != nil {
					return nil, "", fmt.Errorf("build %s job failed: %w", h.id, err)
				}
				return j, "", nil
			},
		})
	}
}

// hoursStore is the notice store the two calendar_hours families share: opened once with the master line and closed
// through extra. An empty line or a failed open gives nil and one Warn line that names the job and the line, never
// the DSN; the job is then left out.
func (jc *jobCtx) hoursStore(ctx context.Context, a *app, id string) hoursnotice.Store {
	f := jc.f
	if !f.hoursTried {
		f.hoursTried = true
		if jc.cfg.masterDSN != "" {
			if st, err := openHoursNotices(ctx, jc.cfg.masterDSN); err == nil {
				jc.extra.addCloser(st)
				f.hours = st
			}
		}
	}
	if f.hours == nil {
		status := "open of the notice store with " + store.DSNEnvMaster + " failed"
		if jc.cfg.masterDSN == "" {
			status = store.DSNEnvMaster + " is not set"
		}
		a.log.Warn("calendar hours job not wired", "component", "ot", "job", id, "status", status)
	}
	return f.hours
}

// envCalendarProject names the optional line of .env that sets the directory of the pyside project.
const envCalendarProject = "OT_CALENDAR_PROJECT"

// calendarCommand is the command of calendar_refresh up to the arguments calendarcli adds:
// `uv run --locked --project <project> python -m optdesk.calendar`; project must be absolute so that
// the child starts from any working directory.
func calendarCommand(uv, project string) []string {
	return []string{uv, "run", "--locked", "--project", project, "python", "-m", "optdesk.calendar"}
}

// hasPyproject tells whether dir holds the pyproject.toml of the calendar generator; a package variable so that
// the tests, whose working directory has no pyside directory, can stand in for the checkout.
var hasPyproject = func(dir string) bool {
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
	if !hasPyproject(dir) {
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
