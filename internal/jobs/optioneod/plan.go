package optioneod

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"

	"github.com/andrewrong/optdesk/internal/contracts"
)

//go:embed job.toml
var manifestBytes []byte

const datasetID = "obs.option_eod"

// planFrom builds the plan of day from manifest bytes: one key per underlying and the plan hash.
func planFrom(data []byte, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	m, err := parseManifest(data)
	if err != nil {
		return nil, "", err
	}
	keys, hash := m.plan(day)
	return keys, hash, nil
}

// plan lists the keys sorted by symbol (parseManifest sorts the underlyings) and hashes day with the manifest.
func (m manifest) plan(day contracts.Day) ([]contracts.ExpectedKey, string) {
	keys := make([]contracts.ExpectedKey, 0, len(m.Underlying))
	for _, u := range m.Underlying {
		keys = append(keys, contracts.ExpectedKey{DatasetID: m.LedgerDataset, SubjectKey: u.Symbol})
	}
	sum := sha256.Sum256([]byte(string(day) + "\n" + m.canonical()))
	return keys, hex.EncodeToString(sum[:])
}

// Plan returns the keys expected for day and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys, hash := j.m.plan(day)
	return keys, hash, nil
}
