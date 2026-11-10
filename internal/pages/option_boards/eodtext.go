package option_boards

import (
	"math"
	"strconv"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// gapText is shown for a value that is not stored.
const gapText = "缺"

// floatText is the shortest decimal text of a stored value; the value is printed, never parsed. Null, NaN and
// infinity are "缺".
func floatText(v *float64) string {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return gapText
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

// eodText is the text of the end-of-day columns the pages show.
type eodText struct {
	Volume, OpenInterest, Theta, Gamma, Vega string
}

// eodTextOf prints the row; a nil row gives "缺" in every field.
func eodTextOf(e *contracts.OptionEod) eodText {
	if e == nil {
		return eodText{gapText, gapText, gapText, gapText, gapText}
	}
	return eodText{
		Volume:       floatText(e.Volume),
		OpenInterest: floatText(e.OpenInterest),
		Theta:        floatText(e.ThetaPerCalendarDay),
		Gamma:        floatText(e.Gamma),
		Vega:         floatText(e.VegaPerVolPoint),
	}
}

// eodTextFor prints the row of an instrument id from the map of loadEod.
func eodTextFor(m map[int]contracts.OptionEod, instrumentID int) eodText {
	if e, ok := m[instrumentID]; ok {
		return eodTextOf(&e)
	}
	return eodTextOf(nil)
}
