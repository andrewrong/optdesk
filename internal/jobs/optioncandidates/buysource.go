package optioncandidates

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
	"github.com/andrewrong/optdesk/internal/jobs"
)

// addBuySourceMark appends the one mark that names where the entry's buy price came from: buy_from_valuation for
// the source valuation, buy_from_hand for hand or for an empty source (a price built without the resolver step is
// the list's hand price). An entry with no price adds nothing. The job is the only writer of the two marks.
func addBuySourceMark(c *compute.Candidate, e jobs.WatchEntry) {
	if e.BuyPrice == "" {
		return
	}
	if e.BuySource == buypriceapi.SourceValuation {
		c.Marks = append(c.Marks, buypriceapi.MarkValuation)
		return
	}
	c.Marks = append(c.Marks, buypriceapi.MarkHand)
}
