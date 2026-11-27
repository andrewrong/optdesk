package findingpg

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

// TestProbeReturnsCountsOnly: a fact type of the probe holds counts and keys, no free text, so no row of a table can
// travel through it.
func TestProbeReturnsCountsOnly(t *testing.T) {
	t.Parallel()
	allowed := map[string]bool{"Name": true, "Currency": true, "Kind": true, "Reason": true, "JobID": true, "Dataset": true}
	dayType := reflect.TypeFor[contracts.Day]()
	for _, v := range []any{finding.ReadingRepeat{}, finding.LateQuotes{}, finding.CandidateRun{}, finding.OpenGap{}} {
		typ := reflect.TypeOf(v)
		for f := range typ.Fields() {
			if f.Type.Kind() != reflect.String || f.Type == dayType {
				continue
			}
			if !allowed[f.Name] {
				t.Errorf("%s.%s is a string field that is not a name, currency, kind, reason, day or id", typ.Name(), f.Name)
			}
		}
	}
}
