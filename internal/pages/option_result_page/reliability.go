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

// idList is "<head>: id, id" or "<head>: none" for no id.
func idList(head string, ids []string) string {
	if len(ids) == 0 {
		return head + "：" + noneText
	}
	more := ""
	if len(ids) > maxIDs {
		ids, more = ids[:maxIDs], " …"
	}
	return head + "：" + strings.Join(ids, ", ") + more
}

// reliabilityOf is the reliability blocks of the page: one of three count lines for each section, then one account-wide
// block that holds the gloss of the reconciliation and the ids of Unresolved and Review, once. The ids are the same in
// every section, so those of the first are written; they are in no figure.
func reliabilityOf(secs []section, rels []compute.ResultReliability) []reliability {
	out := []reliability{}
	for i, r := range rels {
		out = append(out, reliability{Key: secs[i].Key, Heading: secs[i].Heading + " reconciliation", Lines: []string{
			sameLine + strconv.Itoa(r.Equal) + tradeUnit,
			differLine + strconv.Itoa(r.Differ) + tradeUnit,
			noBrokerLine + strconv.Itoa(r.NoBroker) + tradeUnit,
		}})
	}
	if len(rels) == 0 {
		return out
	}
	return append(out, reliability{Key: "account_ids", Heading: "Reconciliation, whole account", Lines: []string{
		reconcileGloss,
		idList(unpairedHead, rels[0].Unresolved),
		idList(reviewHead, rels[0].Review),
	}})
}
