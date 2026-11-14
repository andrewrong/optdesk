package market_state_page

import (
	"encoding/json"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
)

type rotationPoint struct {
	Day  string      `json:"day"`
	X    json.Number `json:"x"`
	Y    json.Number `json:"y"`
	Text string      `json:"text"`
}

// gapFigure is a gap in points as a figure, or the Gap of its reason.
func gapFigure(value string, reason compute.Reason, what string) any {
	if reason != "" {
		return numGap(reason, what)
	}
	text, err := shiftRound(value, 0, 2)
	if err != nil {
		return mkGap(reasonMissingInput, reasonText(reasonMissingInput, what))
	}
	return mkFigure(text, signed(text)+unitPoints, unitPctPoints, what)
}

// rankCell is "at of of", or the Gap of the reason; an industry fund is never ranked.
func rankCell(p marketstateapi.Position, group, what string) any {
	switch {
	case p.Reason != "":
		return numGap(p.Reason, what)
	case p.At > 0 && p.Of > 0:
		return rank{At: p.At, Of: p.Of}
	case group == GroupIndustry:
		return mkGap(reasonNoData, what+": "+gapNotRanked)
	}
	return mkGap(reasonMissingInput, reasonText(reasonMissingInput, what))
}

// rotationCell copies the spots of a fund, oldest first, as {day, x, y, text}. A spot that the
// result does not hold is never filled from another; an empty list is the Gap missing_input.
func rotationCell(spots []marketstateapi.Spot) any {
	const what = "rotation"
	if len(spots) == 0 {
		return mkGap(reasonMissingInput, reasonText(reasonMissingInput, what))
	}
	out := make([]rotationPoint, 0, len(spots))
	for _, s := range spots {
		x, errX := shiftRound(s.X, 0, 2)
		y, errY := shiftRound(s.Y, 0, 2)
		if errX != nil || errY != nil {
			return mkGap(reasonMissingInput, reasonText(reasonMissingInput, what))
		}
		out = append(out, rotationPoint{Day: string(s.Day), X: json.Number(x), Y: json.Number(y),
			Text: string(s.Day) + ": 63-session " + signed(x) + ", pace " + signed(y)})
	}
	return out
}
