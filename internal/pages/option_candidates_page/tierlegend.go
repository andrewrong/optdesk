package option_candidates_page

import (
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

type legendItem struct {
	Tier  string `json:"tier"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// tierCounts are the numbers of rows of the Fits tier, the Check tier and all grey tiers.
type tierCounts struct{ fits, check, grey int }

func deltaSource(set bool) string {
	if set {
		return deltaSourceSet
	}
	return deltaSourceFixed
}

func cashClause(built bool) string {
	if built {
		return cashClauseBuilt
	}
	return cashClauseMark
}

// explainFits is the Chinese sentence of the Fits tier with the bound of the rule in force.
func explainFits(res optiontierapi.Result, f tierFacts) string {
	return explainFitsHead + res.Params.DeltaMax + explainFitsMid + deltaSource(f.deltaSet) + explainFitsClose +
		cashClause(res.CashBuilt) + explainFitsTail
}

// cashDayOf is the capture day of the first alias that has one, the chain day when none has.
func cashDayOf(f tierFacts) string {
	for _, a := range f.in.Accounts {
		if a.CaptureDay != "" {
			return string(a.CaptureDay)
		}
	}
	return string(f.in.Day)
}

// legendOf is the four legend items of Decision 11 with the values of the rule in force: the delta bound
// and its source, the thin-premium line, the cash age and the row counts.
func legendOf(res optiontierapi.Result, f tierFacts, n tierCounts) []legendItem {
	cash := cashNoFigure
	if res.CashBuilt {
		cash = cashBasisHead + cashDayOf(f) + cashBasisTail + cashAgeHead + strconv.Itoa(res.Params.MaxCashAgeDays) + cashAgeTail
	}
	return []legendItem{
		{Tier: tierFitsID, Title: legendFitsTitle + countOf(n.fits),
			Text: explainFits(res, f) + thinSentenceHead + res.Params.ThinDelta + thinSentenceTail},
		{Tier: tierCheckID, Title: legendCheckTitle + countOf(n.check), Text: explainCheck},
		{Tier: legendGreyTier, Title: legendGreyTitle + countOf(n.grey), Text: explainGrey},
		{Tier: legendCashTier, Title: legendCashTitle, Text: cash},
	}
}

func countOf(n int) string { return " (" + strconv.Itoa(n) + ")" }
