package findingpg

import (
	"strings"
	"testing"
)

// TestNewProbeQueriesAreReadOnly: the two queries of data check 2 and the candidate query read and never write.
func TestNewProbeQueriesAreReadOnly(t *testing.T) {
	t.Parallel()
	for name, q := range map[string]string{
		"earnings": earningsChecksSQL, "bars": barShapesSQL, "candidates": newestCandidateRunsSQL,
	} {
		up := strings.ToUpper(q)
		for _, w := range []string{"INSERT", "UPDATE", "DELETE"} {
			if strings.Contains(up, w) {
				t.Errorf("%s query contains %s", name, w)
			}
		}
	}
}
