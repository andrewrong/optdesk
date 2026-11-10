package positionmarks

import (
	_ "embed"
	"errors"
	"time"
	// The zone database is embedded so that a zone name loads on every machine.
	_ "time/tzdata"

	"github.com/pelletier/go-toml/v2"
)

const (
	anchorOpen  = "session_open"
	anchorClose = "session_close"
	anchorTime  = "time"
)

// scheduleTable is the [schedule] table of job.toml. offset and run_bound are Go durations
// such as "-30m" or "7h30m". A manifest without the table has no schedule.
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
	calendar         string
	anchor           string
	zone             string
	offset           time.Duration
	runBound         time.Duration
	restartSensitive bool
}

func scheduleErr(key, why string) error {
	return errors.New("positionmarks: manifest: schedule." + key + " " + why)
}

// scheduleOf checks the table. Errors name the key and never a value.
func scheduleOf(t scheduleTable) (schedule, error) {
	if t == (scheduleTable{}) {
		return schedule{}, nil
	}
	s := schedule{calendar: t.Calendar, anchor: t.Anchor, zone: t.Zone, restartSensitive: t.RestartSensitive}
	if t.Calendar == "" {
		return schedule{}, scheduleErr("calendar", "is required")
	}
	if t.Offset != "" {
		d, err := time.ParseDuration(t.Offset)
		if err != nil {
			return schedule{}, scheduleErr("offset", "is not a duration")
		}
		s.offset = d
	}
	switch t.Anchor {
	case anchorOpen, anchorClose:
		if t.Zone != "" {
			return schedule{}, scheduleErr("zone", "must be empty for a session anchor")
		}
	case anchorTime:
		if t.Zone == "" {
			return schedule{}, scheduleErr("zone", "is required for the time anchor")
		}
		if _, err := time.LoadLocation(t.Zone); err != nil {
			return schedule{}, scheduleErr("zone", "is unknown")
		}
		if s.offset < 0 || s.offset >= 24*time.Hour {
			return schedule{}, scheduleErr("offset", "must be inside one day for the time anchor")
		}
	default:
		return schedule{}, scheduleErr("anchor", "must be session_open, session_close or time")
	}
	if t.RunBound != "" {
		d, err := time.ParseDuration(t.RunBound)
		if err != nil {
			return schedule{}, scheduleErr("run_bound", "is not a duration")
		}
		if d < 0 {
			return schedule{}, scheduleErr("run_bound", "must not be negative")
		}
		s.runBound = d
	}
	return s, nil
}

// Schedule is the slot of the manifest as plain values; the scheduler turns it into start instants.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	s := j.m.sched
	return s.calendar, s.anchor, s.zone, s.offset
}

// RunBound is the longest time one run may take; 0 when the table gives none.
func (j *job) RunBound() time.Duration { return j.m.sched.runBound }

// RestartSensitive is true when a restart between the slot start and the window close must be avoided.
func (j *job) RestartSensitive() bool { return j.m.sched.restartSensitive }

// manifest is job.toml.
type manifest struct {
	Version  int           `toml:"version"`
	ID       string        `toml:"id"`
	Schedule scheduleTable `toml:"schedule"`

	sched schedule
}

//go:embed job.toml
var manifestBytes []byte

func parseManifest(data []byte) (manifest, error) {
	var m manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return manifest{}, errors.New("positionmarks: manifest: not valid toml")
	}
	if m.ID == "" {
		return manifest{}, errors.New("positionmarks: manifest: id is required")
	}
	s, err := scheduleOf(m.Schedule)
	if err != nil {
		return manifest{}, err
	}
	m.sched = s
	return m, nil
}
