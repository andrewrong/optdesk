package quotesnapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const datasetID = "obs.quote_snapshot"

// plan builds one key per slot, on every day (the job has no calendar), and the plan hash.
func (m manifest) plan(day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys := make([]contracts.ExpectedKey, 0, len(m.Slots))
	for _, s := range m.Slots {
		keys = append(keys, contracts.ExpectedKey{DatasetID: datasetID, SubjectKey: s})
	}
	ins := make([]string, 0, len(m.Instruments))
	for _, in := range m.Instruments {
		ins = append(ins, fmt.Sprintf("%s:%d", in.Symbol, in.ConID))
	}
	sort.Strings(ins)
	parts := append([]string{m.ID, fmt.Sprint(m.Version), string(day), strings.Join(m.Slots, ",")}, ins...)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return keys, hex.EncodeToString(sum[:]), nil
}

// Plan returns the keys expected for day and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return j.m.plan(day)
}
