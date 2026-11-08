package computefigures

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// scheduleTable is the [schedule] table of job.toml; offset and run_bound are Go durations.
type scheduleTable struct {
	Calendar         string `toml:"calendar"`
	Anchor           string `toml:"anchor"`
	Zone             string `toml:"zone"`
	Offset           string `toml:"offset"`
	RunBound         string `toml:"run_bound"`
	RestartSensitive bool   `toml:"restart_sensitive"`
}

// schedule is the checked table as plain values.
type schedule struct {
	calendar, anchor, zone string
	offset, runBound       time.Duration
	restartSensitive       bool
}

func scheduleOf(t scheduleTable) (schedule, error) {
	if t.Calendar == "" || t.Anchor == "" {
		return schedule{}, errors.New("computefigures: manifest: schedule needs calendar and anchor")
	}
	s := schedule{calendar: t.Calendar, anchor: t.Anchor, zone: t.Zone, restartSensitive: t.RestartSensitive}
	var err error
	if t.Offset != "" {
		if s.offset, err = time.ParseDuration(t.Offset); err != nil {
			return schedule{}, errors.New("computefigures: manifest: schedule.offset is not a duration")
		}
	}
	if t.RunBound != "" {
		if s.runBound, err = time.ParseDuration(t.RunBound); err != nil || s.runBound < 0 {
			return schedule{}, errors.New("computefigures: manifest: schedule.run_bound is not a duration")
		}
	}
	return s, nil
}

// Schedule is the slot of the manifest as plain values.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	return j.sched.calendar, j.sched.anchor, j.sched.zone, j.sched.offset
}

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.sched.runBound }

// RestartSensitive is true when a restart between the slot start and the window close must be avoided.
func (j *job) RestartSensitive() bool { return j.sched.restartSensitive }

// Postcondition is true when the key has a reading row for the as-of day, ok or missing.
func (j *job) Postcondition(ctx context.Context, day, datasetID, subjectKey string) (bool, error) {
	if datasetID != datasetReadings {
		return false, fmt.Errorf("computefigures: unknown dataset %q", datasetID)
	}
	name, kind, subj, ok := parseKey(subjectKey)
	if !ok {
		return false, fmt.Errorf("computefigures: bad subject key %q", subjectKey)
	}
	d := dayValue(day)
	rows, err := j.stores.Readings.LatestReadings(ctx, name, kind, subj, d, d)
	if err != nil {
		return false, fmt.Errorf("computefigures: readings: %w", err)
	}
	return len(rows) > 0, nil
}
