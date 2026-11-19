package option_candidates_page

import (
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute/optionshortlistapi"
	"github.com/andrewrong/optdesk/internal/store"
)

// shortlistInputOf is the input of Shortlister.Select: one row per stored short-put row of every status, in
// stored order, with the stored texts unchanged and the key of the row in the tier view. The page reads no
// owner setting and no rule parameter here; a row whose gates or marks cannot be read is passed with the
// mark unreadable_gates and is never dropped by the page.
func shortlistInputOf(rows []store.Candidate, tickers map[int]string) optionshortlistapi.Input {
	in := optionshortlistapi.Input{Rows: make([]optionshortlistapi.Row, len(rows))}
	for i, c := range rows {
		r := optionshortlistapi.Row{
			Key: tierKey(c, i), Company: strconv.Itoa(c.UnderlyingID), Ticker: tickers[c.UnderlyingID],
			Strike: c.Strike, Spot: deref(c.Spot), Delta: deref(c.Delta), Bid: c.BidEod,
			Annualized: deref(c.AnnualizedYield), DTE: strconv.Itoa(c.DTE),
		}
		gates, gErr := parseGates(c.Gates)
		marks, mErr := parseMarks(c.Marks)
		if gErr != nil || mErr != nil {
			r.Marks = []string{unreadableGates}
		} else {
			r.Marks = marks
			r.QuoteReason, r.EventsReason = firstReason(gates, "quote"), firstReason(gates, "events")
		}
		in.Rows[i] = r
	}
	return in
}
