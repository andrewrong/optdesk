package option_result_page

import (
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
)

type reliability struct {
	Key     string   `json:"key"`
	Heading string   `json:"heading"`
	Lines   []string `json:"lines"`
}

// maxIDs is how many transaction ids a line names before it ends with an ellipsis.
const maxIDs = 30

// idList is "<head>: id, id" or "" for no id.
func idList(head string, ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	more := ""
	if len(ids) > maxIDs {
		ids, more = ids[:maxIDs], " …"
	}
	return "; " + head + ": " + strings.Join(ids, ", ") + more
}

// reliabilityOf is the three lines of one section: equal, differ, no broker figure. The ids of Unresolved and Review
// are named in the last line and are in no figure.
func reliabilityOf(key, heading string, r compute.ResultReliability) reliability {
	return reliability{Key: key, Heading: heading + " reconciliation", Lines: []string{
		"Same as broker realized: " + strconv.Itoa(r.Equal) + " trade(s)",
		"Differs from broker realized: " + strconv.Itoa(r.Differ) + " trade(s)",
		"No broker figure: " + strconv.Itoa(r.NoBroker) + " trade(s)" + idList("Unpaired closes (not in any figure)", r.Unresolved) +
			idList("Cancels or corrections to review (not in any figure)", r.Review),
	}}
}
