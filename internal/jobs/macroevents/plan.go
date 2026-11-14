package macroevents

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
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

const datasetID = "obs.macro_event"

// release is one FRED release the job fetches: the kind stored and the release id of the query.
type release struct{ kind, id string }

// releases are the eight FRED kinds, in the order the job fetches them (Decision 1).
var releases = []release{
	{"cpi", "10"}, {"employment_situation", "50"}, {"pce", "54"}, {"gdp", "53"},
	{"ppi", "46"}, {"retail_sales", "9"}, {"jolts", "192"}, {"h41", "20"},
}

// fileKinds are the five kinds of the hand-kept file, fomc_minutes derived from fomc.
var fileKinds = []string{kindFOMC, kindMinutes, kindBOJ, kindQuarterEnd, kindUSTax}

// allKinds are the 13 kinds of the job, sorted.
func allKinds() []string {
	kinds := slices.Clone(fileKinds)
	for _, r := range releases {
		kinds = append(kinds, r.kind)
	}
	sort.Strings(kinds)
	return kinds
}

type scheduleTable struct {
	Calendar         string `toml:"calendar"`
	Anchor           string `toml:"anchor"`
	Zone             string `toml:"zone"`
	Offset           string `toml:"offset"`
	RunBound         string `toml:"run_bound"`
	RestartSensitive bool   `toml:"restart_sensitive"`
}

type manifest struct {
	Version  int           `toml:"version"`
	ID       string        `toml:"id"`
	Schedule scheduleTable `toml:"schedule"`

	offset, runBound time.Duration
}

func parseManifest(data []byte) (manifest, error) {
	var m manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("macroevents: manifest: %w", err)
	}
	if m.ID == "" {
		return manifest{}, errors.New("macroevents: manifest needs an id")
	}
	s := m.Schedule
	if s.Calendar == "" || s.Anchor != "time" || s.Zone == "" {
		return manifest{}, errors.New("macroevents: manifest: schedule needs a calendar, the time anchor and a zone")
	}
	if _, err := time.LoadLocation(s.Zone); err != nil {
		return manifest{}, errors.New("macroevents: manifest: schedule.zone is unknown")
	}
	var err error
	if m.offset, err = time.ParseDuration(s.Offset); err != nil || m.offset < 0 || m.offset >= 24*time.Hour {
		return manifest{}, errors.New("macroevents: manifest: schedule.offset is not a duration inside one day")
	}
	if m.runBound, err = time.ParseDuration(s.RunBound); err != nil || m.runBound < 0 {
		return manifest{}, errors.New("macroevents: manifest: schedule.run_bound is not a duration")
	}
	return m, nil
}

// plan is one key per kind, sorted by kind, and the hash of the job id, the manifest version, the day and the kinds.
func (m manifest) plan(day contracts.Day) ([]contracts.ExpectedKey, string) {
	kinds := allKinds()
	keys := make([]contracts.ExpectedKey, 0, len(kinds))
	for _, k := range kinds {
		keys = append(keys, contracts.ExpectedKey{DatasetID: datasetID, SubjectKey: k})
	}
	parts := append([]string{m.ID, fmt.Sprint(m.Version), string(day)}, kinds...)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return keys, hex.EncodeToString(sum[:])
}

// Plan returns the 13 keys for day and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys, hash := j.m.plan(day)
	return keys, hash, nil
}

// Schedule is the slot of the manifest as plain values; the scheduler turns it into start instants.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	s := j.m.Schedule
	return s.Calendar, s.Anchor, s.Zone, j.m.offset
}

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.m.runBound }

// RestartSensitive is true when a restart between the slot start and the window close must be avoided.
func (j *job) RestartSensitive() bool { return j.m.Schedule.RestartSensitive }
