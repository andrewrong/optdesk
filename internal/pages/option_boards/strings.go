//nolint:unused // used by the page builders of entries 3 and 4 of docs/design/option-page-builders.md
package option_boards

// Texts of the shell of both pages. Option terms stay English, other text is Chinese (option-pages-v22
// Decision 6).
const (
	optionTitle      = "Options"
	stockTitle       = "Stock options"
	candidatesSource = "candidates"

	jpNote         = "Japan options: later design (option-candidates-jp)"
	gapNoReader    = "No candidate reader: cannot show"
	gapNoRows      = "No candidate rows: the candidate job has not stored any yet"
	staleDayPrefix = "Data day "
	staleDaySuffix = ", before today"
	gapStale       = "Candidate rows are not from today"

	verdictWorth    = "Worth a look"
	verdictPending  = "Incomplete data"
	verdictSkip     = "Not eligible"
	gatesPassed     = "All gates passed"
	passedWithMarks = "Passed, with marks: "

	ivUnknown   = "n/a"
	earnAbsent  = "E none"
	earnMissing = "E n/a"

	noCardText = "No candidate rows for this underlying"
	footerText = "Built from archived close quotes and the candidate table"
	policyText = "Policy: sell put, only inside the bid line"
	trendText  = "Trend n/a"
)

// Texts of the volatility view of the stock page (docs/design/vol-edge.md, Interface item 4).
const (
	volMethodText = "Implied is IBKR's implied volatility of the name; its horizon is not stated. " +
		"The forecast is the plain mean of the realised volatility of the last 20, 60 and 120 sessions, " +
		"each annualised with 252 sessions. Implied and the forecast are not the same horizon, " +
		"so the edge is a difference of two numbers of possibly different tenor. " +
		"The figures belong to the name, not to a strike or an expiry, " +
		"and are read the same for a short put and a covered call."

	volTail = " This page does not know the next earnings move, the skew, how well this forecast has fitted this name " +
		"(see the error row), early assignment, or the horizon of implied. One past error is shown, not a track record."

	volAbove    = "Implied volatility is above the forecast by the edge shown." + volTail
	volBelow    = "Implied volatility is below the forecast by the edge shown." + volTail
	volInside   = "Implied volatility is within one volatility point of the forecast." + volTail
	volNoEdge   = "No edge is shown because an input is missing; the rows name it." + volTail
	volDeadBand = "0.010000"
)
