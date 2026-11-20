package home_market

import (
	"encoding/json"
	"strings"
	"time"
)

// indexVolSeries maps an index of the table to the stored series of its volatility index.
var indexVolSeries = map[string]string{"SPX": "VIXCLS", "NDX": "VXNCLS", "RUT": "RVXCLS"}

// indexIvCell is the Index IV cell of an index: the newest of pts (newest first, the stored points
// of its series) as vol points with one decimal, half even, and the source text
// "FRED <series> <MM-DD>". A Gap: no_data without a point or for a name outside the map, stale
// beyond freshDays before d, missing_input for a negative point. The cell has its own day: it
// never reads the Difference cell.
func indexIvCell(name string, pts []point, d time.Time) any {
	series, mapped := indexVolSeries[name]
	if !mapped {
		return mkGap("no_data", name+indexIvUnmapped)
	}
	if len(pts) == 0 {
		return mkGap("no_data", name+indexIvNoRow)
	}
	day := pts[0].date.Format("01-02")
	if daysBetween(pts[0].date, d) > freshDays {
		return mkGap("stale", name+indexIvStale+day)
	}
	text, err := shiftRound(pts[0].text, 0, 1)
	if err != nil || strings.HasPrefix(text, "-") {
		return mkGap("missing_input", name+indexIvBad)
	}
	return volPoints{Kind: "value", Value: json.Number(text), Text: text, Unit: "vol_points", Source: "FRED " + series + " " + day}
}
