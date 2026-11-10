// Package fundamentals is the job that carries the SEC facts of the watch list into the fact store.
package fundamentals

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	// The zone database is embedded so that a zone name loads on every machine.
	_ "time/tzdata"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/contracts"
)

//go:embed job.toml
var manifestBytes []byte

const (
	factDatasetID = "obs.fundamental_fact"
	anchorOpen    = "session_open"
	anchorClose   = "session_close"
	anchorTime    = "time"
)

type window struct {
	Rule          string `toml:"rule"`
	LatenessHours int    `toml:"lateness_hours"`
	Calendar      string `toml:"calendar"`
}

// scheduleTable is the [schedule] table of job.toml. offset and run_bound are Go durations.
type scheduleTable struct {
	Calendar         string `toml:"calendar"`
	Anchor           string `toml:"anchor"`
	Zone             string `toml:"zone"`
	Offset           string `toml:"offset"`
	RunBound         string `toml:"run_bound"`
	RestartSensitive bool   `toml:"restart_sensitive"`
}

type schedule struct {
	calendar         string
	anchor           string
	zone             string
	offset           time.Duration
	runBound         time.Duration
	restartSensitive bool
}

type manifest struct {
	Version int    `toml:"version"`
	ID      string `toml:"id"`
	// LookbackDays is how many days back the filing days are fetched.
	LookbackDays int           `toml:"lookback_days"`
	Window       window        `toml:"window"`
	Schedule     scheduleTable `toml:"schedule"`

	sched schedule
}

func parseManifest(data []byte) (manifest, error) {
	var m manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("fundamentals: manifest: %w", err)
	}
	if m.ID == "" {
		return manifest{}, errors.New("fundamentals: manifest needs an id")
	}
	if m.LookbackDays < 1 || m.LookbackDays > 31 {
		return manifest{}, fmt.Errorf("fundamentals: lookback_days %d is outside 1..31", m.LookbackDays)
	}
	if m.Window.Rule != "after_close" {
		return manifest{}, errors.New("fundamentals: manifest: window.rule must be after_close")
	}
	if m.Window.LatenessHours < 0 {
		return manifest{}, errors.New("fundamentals: manifest: window.lateness_hours must be at least 0")
	}
	sched, err := scheduleOf(m.Schedule)
	if err != nil {
		return manifest{}, err
	}
	m.sched = sched
	return m, nil
}

func scheduleErr(key, why string) error {
	return errors.New("fundamentals: manifest: schedule." + key + " " + why)
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

// plan builds the keys of day, one per issuer (sorted, as given), and the plan hash.
func (m manifest) plan(day contracts.Day, issuers []string) ([]contracts.ExpectedKey, string) {
	ids := append([]string(nil), issuers...)
	sort.Strings(ids)
	keys := make([]contracts.ExpectedKey, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, contracts.ExpectedKey{DatasetID: factDatasetID, SubjectKey: id})
	}
	parts := append([]string{m.ID, fmt.Sprint(m.Version), string(day), fmt.Sprint(m.LookbackDays)}, ids...)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return keys, hex.EncodeToString(sum[:])
}

// Plan returns the keys expected for day and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys, hash := j.m.plan(day, j.issuers)
	return keys, hash, nil
}

// Window is the window rule of the manifest as plain values; the rule is carried, not applied.
func (j *job) Window() (calendarID string, openOffset, closeOffset time.Duration) {
	return j.m.Window.Calendar, 0, time.Duration(j.m.Window.LatenessHours) * time.Hour
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
