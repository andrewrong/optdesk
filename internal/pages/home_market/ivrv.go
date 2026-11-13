package home_market

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	ivRvMaxAgeDays = 5 // the age limit of market.iv_rv20, its rule's max_age_days; not freshDays
	codeStaleInput = 3 // reason_code of a stored stale_input reading
)

// ivRvCell is the implied minus realised cell: the stored market.iv_rv20 fraction times 100, one
// decimal, half even, signed. The text names the data day when it is not the page's day d.
// A stored stale reading or a data day more than ivRvMaxAgeDays before d is stale_input, a
// missing or unusable reading is missing_input, no reading is no_data.
func ivRvCell(name string, r *contracts.ReadingRecord, d time.Time) any {
	if r == nil {
		return mkGap("no_data", name+" "+ivRvNoRow)
	}
	day, hasDay := dataDayOf(r)
	if string(r.State) == "missing_input" && r.ReasonCode != nil && *r.ReasonCode == codeStaleInput {
		return ivRvStale(name, day, hasDay)
	}
	if string(r.State) != "ok" || r.Value == nil || !hasDay {
		return mkGap("missing_input", name+" "+ivRvMissing)
	}
	if daysBetween(day, d) > ivRvMaxAgeDays {
		return ivRvStale(name, day, true)
	}
	text, err := shiftRound(*r.Value, 2, 1)
	if err != nil {
		return mkGap("missing_input", name+" "+ivRvMissing)
	}
	shown := text
	if !strings.HasPrefix(text, "-") && strings.Trim(text, "0.") != "" {
		shown = "+" + text
	}
	if !day.Equal(d) {
		shown += " (" + day.Format("01-02") + ")"
	}
	return volPoints{Kind: "value", Value: json.Number(text), Text: shown, Unit: "vol_points"}
}

func ivRvStale(name string, day time.Time, hasDay bool) gap {
	text := name + " " + ivRvStaleText
	if hasDay {
		text += ", data day " + day.Format("01-02")
	}
	return mkGap("stale_input", text)
}

// dataDayOf parses the data_day parameter of a reading; false when absent or not a date.
func dataDayOf(r *contracts.ReadingRecord) (time.Time, bool) {
	for _, p := range r.Parameters {
		if p.Param != "data_day" || p.ValueTag == nil {
			continue
		}
		if t, err := time.Parse(dayLayout, *p.ValueTag); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
