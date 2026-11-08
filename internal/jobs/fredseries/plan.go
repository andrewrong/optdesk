// Package fredseries is the job that carries FRED series through sources and store.
package fredseries

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/contracts"
)

//go:embed job.toml
var manifestBytes []byte

const seriesDatasetID = "obs.series_point"

type window struct {
	Rule          string `toml:"rule"`
	LatenessHours int    `toml:"lateness_hours"`
	Calendar      string `toml:"calendar"`
}

type manifest struct {
	Version  int      `toml:"version"`
	ID       string   `toml:"id"`
	Series   []string `toml:"series"`
	Windowed []string `toml:"windowed"`
	// LookbackDays is how many days back the windowed series are fetched.
	LookbackDays int           `toml:"lookback_days"`
	Window       window        `toml:"window"`
	Schedule     scheduleTable `toml:"schedule"`

	sched schedule
}

func parseManifest(data []byte) (manifest, error) {
	var m manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("fredseries: manifest: %w", err)
	}
	if m.ID == "" || len(m.Series) == 0 {
		return manifest{}, fmt.Errorf("fredseries: manifest needs an id and a series list")
	}
	for _, id := range m.Windowed {
		if !slices.Contains(m.Series, id) {
			return manifest{}, fmt.Errorf("fredseries: windowed id %q is not in series", id)
		}
	}
	if (len(m.Windowed) > 0 || m.LookbackDays != 0) && (m.LookbackDays < 1 || m.LookbackDays > 31) {
		return manifest{}, fmt.Errorf("fredseries: lookback_days %d is outside 1..31", m.LookbackDays)
	}
	sched, err := scheduleOf(m.Schedule)
	if err != nil {
		return manifest{}, err
	}
	m.sched = sched
	return m, nil
}

// planFrom builds the plan of day from manifest bytes: one key per series and the plan hash.
func planFrom(data []byte, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	m, err := parseManifest(data)
	if err != nil {
		return nil, "", err
	}
	return m.plan(day)
}

func (m manifest) plan(day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	ids := append([]string(nil), m.Series...)
	sort.Strings(ids)
	keys := make([]contracts.ExpectedKey, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, contracts.ExpectedKey{DatasetID: seriesDatasetID, SubjectKey: id})
	}
	windowed := append([]string(nil), m.Windowed...)
	sort.Strings(windowed)
	parts := append([]string{m.ID, fmt.Sprint(m.Version), string(day)}, ids...)
	parts = append(parts, "windowed", strings.Join(windowed, ","), fmt.Sprint(m.LookbackDays))
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return keys, hex.EncodeToString(sum[:]), nil
}

// Plan returns the keys expected for day and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return j.m.plan(day)
}
