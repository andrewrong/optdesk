package structures_page

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

type sharePercent struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

type stage struct {
	Name     string       `json:"name"`
	DateText string       `json:"date_text"`
	Elapsed  sharePercent `json:"elapsed"`
	DaysText string       `json:"days_text"`
}

// shareOf is hundredths of a percent as the schema's SharePercent; the arithmetic is on whole numbers.
func shareOf(h int) sharePercent {
	text := fmt.Sprintf("%d.%02d", h/100, h%100)
	return sharePercent{Value: json.Number(text), Text: text + "%", Unit: "share_percent"}
}

// dayCount is the whole days from a to b, on the dates alone.
func dayCount(a, b string) (int, bool) {
	ta, errA := time.Parse(dayLayout, a)
	tb, errB := time.Parse(dayLayout, b)
	if errA != nil || errB != nil {
		return 0, false
	}
	return int(tb.Sub(ta) / (24 * time.Hour)), true
}

// timeline is the three stages of the structure from the day it was opened to its first expiry.
func (b *builder) timeline(asOf contracts.Day) []stage {
	opened := ""
	for _, l := range b.s.Legs {
		if d := string(b.pos[l.PositionID].OpenedOn); d != "" && (opened == "" || d < opened) {
			opened = d
		}
	}
	expiry := earliestExpiry(b.s)
	total, okT := dayCount(opened, expiry)
	gone, okG := dayCount(opened, string(asOf))
	if opened == "" || !okT || !okG {
		b.addGap(mkGap(reasonMissing))
		return []stage{}
	}
	left, _ := dayCount(string(asOf), expiry)
	share := 10000
	if total > 0 {
		share = min(max(gone, 0)*10000/total, 10000)
	}
	todayText := "已持有 " + strconv.Itoa(max(gone, 0)) + " 天，距到期 " + strconv.Itoa(max(left, 0)) + " 天"
	if left < 0 {
		todayText = "已过到期日 " + strconv.Itoa(-left) + " 天"
	}
	return []stage{
		{Name: "开仓", DateText: opened, Elapsed: shareOf(0), DaysText: "第 0 天"},
		{Name: "今天", DateText: string(asOf), Elapsed: shareOf(share), DaysText: todayText},
		{Name: "到期", DateText: expiry, Elapsed: shareOf(10000), DaysText: "共 " + strconv.Itoa(max(total, 0)) + " 天"},
	}
}
