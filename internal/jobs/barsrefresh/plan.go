package barsrefresh

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Plan returns one key per watch entry and per index, and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys := make([]contracts.ExpectedKey, 0, len(j.keys))
	for _, k := range j.keys {
		keys = append(keys, contracts.ExpectedKey{DatasetID: datasetID, SubjectKey: k})
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s\n%s", j.m.ID, j.m.Version, day, strings.Join(j.keys, "\n"))))
	return keys, hex.EncodeToString(sum[:]), nil
}
