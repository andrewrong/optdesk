package market_state_page

import (
	"encoding/json"
	"fmt"

	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
)

// pointFormat says how the decimal text of a history point is shifted and rounded for display.
type pointFormat struct {
	unit            string
	shift, decimals int
	suffix          string
}

var (
	fmtPercentShift = pointFormat{unit: unitPercent, shift: 2, decimals: 2, suffix: "%"} // a stored fraction
	fmtPercent      = pointFormat{unit: unitPercent, decimals: 2, suffix: "%"}
	fmtOneDecimal   = pointFormat{unit: unitIndexPoints, decimals: 1}
	fmtTwoDecimals  = pointFormat{unit: unitRatio, decimals: 2}
)

type historyPoint struct {
	Day   string      `json:"day"`
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
}

type history struct {
	Caption  string         `json:"caption"`
	AriaText string         `json:"aria_text"`
	Unit     string         `json:"unit"`
	Points   []historyPoint `json:"points"`
}

// historyCell copies the points of l, shifted and rounded as decimal text, and writes the caption:
// "<n> days, <first> to <last>", or "newest <n> of <N> days, <first> to <last>" when Total exceeds
// the points. Fewer than two points is the Gap no_data; no point is computed here.
func historyCell(l marketstateapi.Line, label string, f pointFormat) any {
	what := label + " history"
	if len(l.Points) < 2 {
		return mkGap(reasonNoData, what+": "+gapNoHistory)
	}
	pts := make([]historyPoint, 0, len(l.Points))
	for _, p := range l.Points {
		v, err := shiftRound(p.Value, f.shift, f.decimals)
		if err != nil {
			return mkGap(reasonMissingInput, reasonText(reasonMissingInput, what))
		}
		pts = append(pts, historyPoint{Day: string(p.Day), Value: json.Number(v), Text: v + f.suffix})
	}
	first, last := pts[0].Day, pts[len(pts)-1].Day
	caption := fmt.Sprintf("%d days, %s to %s", len(pts), first, last)
	if l.Total > len(pts) {
		caption = fmt.Sprintf("newest %d of %d days, %s to %s", len(pts), l.Total, first, last)
	}
	return history{Caption: caption, AriaText: label + " by day: " + caption + ".", Unit: f.unit, Points: pts}
}
