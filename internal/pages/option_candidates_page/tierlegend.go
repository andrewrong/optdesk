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

// legendOf is the four legend items as short sentences with the values of the rule in force: the delta
// bound, whether the cash figure is built, and the row counts.
func legendOf(res optiontierapi.Result, f tierFacts, n tierCounts) []legendItem {
	cash, cashClauseShort := cashNoFigure, legendCashMark
	if res.CashBuilt {
		cash, cashClauseShort = legendCashText, legendCashBuilt
	}
	return []legendItem{
		{Tier: tierFitsID, Title: legendFitsTitle + strconv.Itoa(n.fits),
			Text: legendFitsHead + res.Params.DeltaMax + legendFitsMid + cashClauseShort + legendFitsTail},
		{Tier: tierCheckID, Title: legendCheckTitle + strconv.Itoa(n.check), Text: legendCheckText},
		{Tier: legendGreyTier, Title: legendGreyTitle + strconv.Itoa(n.grey), Text: legendGreyText},
		{Tier: legendCashTier, Title: legendCashTitle, Text: cash},
	}
}
