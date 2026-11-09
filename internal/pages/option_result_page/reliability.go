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
	return "；" + head + ": " + strings.Join(ids, ", ") + more
}

// reliabilityOf is the three lines of one section: equal, differ, no broker figure. The ids of Unresolved and Review
// are named in the last line and are in no figure.
func reliabilityOf(key, heading string, r compute.ResultReliability) reliability {
	return reliability{Key: key, Heading: heading + " 对账", Lines: []string{
		"与券商已实现一致: " + strconv.Itoa(r.Equal) + " 笔",
		"与券商已实现不一致: " + strconv.Itoa(r.Differ) + " 笔",
		"没有券商数字: " + strconv.Itoa(r.NoBroker) + " 笔" + idList("未配对的平仓(不入任何数字)", r.Unresolved) +
			idList("待复核的撤销或更正(不入任何数字)", r.Review),
	}}
}
