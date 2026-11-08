package macro_rates

import (
	"fmt"
	"strconv"
)

// The English texts of the page.macro_rates.v3 view model.
const (
	pageTitle    = "Macro rates"
	noPointsText = "no points"
	noValueLabel = "no value"
)

func pageSentence(n int, day string) string {
	return fmt.Sprintf("%d series on %s.", n, day)
}

var seriesTitles = map[string]string{
	"DGS10":  "10-year Treasury yield",
	"DGS3MO": "3-month Treasury yield",
	"SP500":  "S&P 500",
	"VIXCLS": "VIX",
	"VXNCLS": "VXN (Nasdaq-100 volatility)",
	"RVXCLS": "RVX (Russell 2000 volatility)",
	"VXVCLS": "VXV (3-month volatility)",
}

// seriesTitle is the title of a series: a known id has a name, any other is its own id.
func seriesTitle(id string) string {
	if t, ok := seriesTitles[id]; ok {
		return t
	}
	return id
}

// pointText formats a value by unit; a nil value is "no value".
func pointText(unit string, v *float64) string {
	if v == nil {
		return noValueLabel
	}
	switch unit {
	case "percent":
		return strconv.FormatFloat(*v, 'f', 2, 64) + "%"
	case "ratio":
		return strconv.FormatFloat(*v, 'f', 4, 64)
	case "ccy_amount", "index_points":
		return strconv.FormatFloat(*v, 'f', 2, 64)
	default: // count, and any unit the schema does not list
		return strconv.FormatFloat(*v, 'f', -1, 64)
	}
}

func noValueText(date string) string { return "no value on " + date }

func staleGapText(day string) string {
	return fmt.Sprintf("no point within %d days of %s", freshnessLimitDays, day)
}

func staleLatestText(date string) string { return "latest point " + date + " is old" }
