package market_page

import (
	"strings"
	"time"
)

// ivrvMaxAgeDays is the oldest data_day, in calendar days before the page's day, that
// the implied minus realised cell still shows.
const ivrvMaxAgeDays = 5

// ivRvCell is the iv_minus_rv20 cell of one index: the stored market.iv_rv20 fraction
// times 100 in vol points, signed in the text, with no threshold or tone. When its
// data_day is not the page's day d the text names the day. A stored stale_input, or a
// data_day more than five days before d, is a stale_input Gap; a reading without a usable
// figure or day is missing_input; no reading is no_data.
func ivRvCell(name string, r reading, d time.Time) any {
	if r.rec == nil {
		return mkGap(reasonNoData, name+" "+ivRvNoRow)
	}
	if r.failure() == "stale" {
		return ivRvStaleGap(name, r.dataDay)
	}
	v, ok := r.usable()
	if !ok || r.dataDay.IsZero() {
		return mkGap(reasonMissingInput, name+" "+ivRvMissing)
	}
	if r.dataDay.Before(d.AddDate(0, 0, -ivrvMaxAgeDays)) {
		return ivRvStaleGap(name, r.dataDay)
	}
	text, err := shiftRound(v, 2, 1)
	if err != nil {
		return mkGap(reasonMissingInput, name+" "+ivRvMissing)
	}
	shown := text
	if !strings.HasPrefix(text, "-") && text != "0.0" {
		shown = "+" + text
	}
	if !r.dataDay.Equal(d) {
		shown += " (" + r.dataDay.Format("01-02") + ")"
	}
	return mkFigure(text, shown, unitVolPoints)
}

func ivRvStaleGap(name string, day time.Time) gap {
	if day.IsZero() {
		return mkGap(reasonStaleInput, name+" "+ivRvStaleNoDay)
	}
	return mkGap(reasonStaleInput, name+" "+ivRvStale+day.Format("01-02"))
}
