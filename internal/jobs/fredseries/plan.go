// Package fredseries is the job that carries FRED series through sources and store.
package fredseries

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
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
	Version int      `toml:"version"`
	ID      string   `toml:"id"`
	Series  []string `toml:"series"`
	Window  window   `toml:"window"`
}

func parseManifest(data []byte) (manifest, error) {
	var m manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("fredseries: manifest: %w", err)
	}
	if m.ID == "" || len(m.Series) == 0 {
		return manifest{}, fmt.Errorf("fredseries: manifest needs an id and a series list")
	}
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
	parts := append([]string{m.ID, fmt.Sprint(m.Version), string(day)}, ids...)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return keys, hex.EncodeToString(sum[:]), nil
}

// Plan returns the keys expected for day and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return j.m.plan(day)
}
