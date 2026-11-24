package household_page

// The texts of the account page that the external tests assert through the constants.
const (
	TextUnknown       = textUnknown
	TextNoMovers      = textNoMovers
	TextMissingInput  = textMissingInput
	TextStatementFrom = textStatementFrom
	TextStatementNote = textStatementNote
	NoteNotInSnapshot = noteNotInSnapshot
	NoteStressSteps   = noteStressSteps
	TextNoRate        = textNoRate
	PageSentence      = pageSentence
	TextNoStockHeld   = textNoStockHeld
)

// MeaningLines are the twelve meaning lines of the account page.
var MeaningLines = []string{
	meaningOverview, meaningComposition, meaningMovers, meaningTrend, meaningStress, meaningIncome,
	meaningMonthChange, meaningCashAfter, meaningObligations, meaningConcentration, meaningAllocation, meaningAccounts,
}

// GapTexts is the text of every gap reason of the page, and the fallback.
func GapTexts() map[string]string {
	out := map[string]string{"": gapText("")}
	for k := range gapTexts {
		out[k] = gapText(k)
	}
	return out
}
