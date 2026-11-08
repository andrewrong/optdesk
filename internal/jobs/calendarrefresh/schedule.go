package calendarrefresh

import (
	"errors"
	"time"
	// The zone database is embedded so that a zone name loads on every machine.
	_ "time/tzdata"
)

// scheduleTable is the [schedule] table of job.toml. offset and run_bound are Go durations
// such as "-30m" or "5m".
type scheduleTable struct {
	Calendar         string `toml:"calendar"`
	Anchor           string `toml:"anchor"`
	Zone             string `toml:"zone"`
	Offset           string `toml:"offset"`
	RunBound         string `toml:"run_bound"`
	RestartSensitive bool   `toml:"restart_sensitive"`
}

// schedule is the checked [schedule] table as plain values.
type schedule struct {
	calendar string
	anchor   string
	zone     string
	offset   time.Duration
	runBound time.Duration
	loc      *time.Location
}

func scheduleErr(key, why string) error {
	return errors.New("calendarrefresh: manifest: schedule." + key + " " + why)
}

// scheduleOf checks the table. Errors name the key and never a value.
func scheduleOf(t scheduleTable) (schedule, error) {
	if t.Calendar == "" {
		return schedule{}, scheduleErr("calendar", "is required")
	}
	switch t.Anchor {
	case "session_open", "session_close", "time":
	default:
		return schedule{}, scheduleErr("anchor", "must be session_open, session_close or time")
	}
	loc, err := time.LoadLocation(t.Zone)
	if t.Zone == "" || err != nil {
		return schedule{}, scheduleErr("zone", "is required and must be known")
	}
	offset, err := time.ParseDuration(t.Offset)
	if err != nil {
		return schedule{}, scheduleErr("offset", "is not a duration")
	}
	bound, err := time.ParseDuration(t.RunBound)
	if err != nil || bound < 0 {
		return schedule{}, scheduleErr("run_bound", "is not a non-negative duration")
	}
	return schedule{calendar: t.Calendar, anchor: t.Anchor, zone: t.Zone, offset: offset, runBound: bound, loc: loc}, nil
}

// Schedule is the slot of the manifest as plain values; the scheduler turns it into start instants.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	return j.sched.calendar, j.sched.anchor, j.sched.zone, j.sched.offset
}

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.sched.runBound }

// RestartSensitive is false: the refresh is an idempotent diff of a few seconds.
func (j *job) RestartSensitive() bool { return false }
