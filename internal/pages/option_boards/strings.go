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

	verdictWorth   = "Worth a look"
	verdictPending = "Incomplete data"
	verdictSkip    = "Not eligible"
	gatesPassed    = "All gates passed"

	ivUnknown   = "n/a"
	earnAbsent  = "E none"
	earnMissing = "E n/a"

	noCardText = "No candidate rows for this underlying"
	footerText = "Built from archived close quotes and the candidate table"
	policyText = "Policy: sell put, only inside the bid line"
	trendText  = "Trend n/a"
)
