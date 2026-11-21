package market_state_page

import (
	"encoding/json"
	"fmt"

	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
)

type priceChartPoint struct {
	Day          string       `json:"day"`
	Close        json.Number  `json:"close"`
	CloseText    string       `json:"close_text"`
	Ma200        *json.Number `json:"ma200,omitempty"`
	Ma200Text    string       `json:"ma200_text,omitempty"`
	BandLow      *json.Number `json:"band_low,omitempty"`
	BandLowText  string       `json:"band_low_text,omitempty"`
	BandHigh     *json.Number `json:"band_high,omitempty"`
	BandHighText string       `json:"band_high_text,omitempty"`
}

type priceChart struct {
	Caption  string            `json:"caption"`
	AriaText string            `json:"aria_text"`
	Unit     string            `json:"unit"`
	Points   []priceChartPoint `json:"points"`
}

// priceChartOf joins the four lines of h by day over the days of Close and copies their text. A day
// that a line has no point for has no such field. Nil when Close has no point or a text is no number.
func priceChartOf(h marketstateapi.History) *priceChart {
	if len(h.Close.Points) == 0 {
		return nil
	}
	byDay := func(l marketstateapi.Line) map[string]string {
		m := make(map[string]string, len(l.Points))
		for _, p := range l.Points {
			m[string(p.Day)] = p.Value
		}
		return m
	}
	ma, lo, hi := byDay(h.Ma200), byDay(h.BandLow), byDay(h.BandHigh)
	pts := make([]priceChartPoint, 0, len(h.Close.Points))
	for _, p := range h.Close.Points {
		if !decimalText.MatchString(p.Value) {
			return nil
		}
		day := string(p.Day)
		pt := priceChartPoint{Day: day, Close: json.Number(p.Value), CloseText: p.Value}
		set := func(m map[string]string, num **json.Number, text *string) {
			if v, ok := m[day]; ok && decimalText.MatchString(v) {
				n := json.Number(v)
				*num, *text = &n, v
			}
		}
		set(ma, &pt.Ma200, &pt.Ma200Text)
		set(lo, &pt.BandLow, &pt.BandLowText)
		set(hi, &pt.BandHigh, &pt.BandHighText)
		pts = append(pts, pt)
	}
	first, last := pts[0].Day, pts[len(pts)-1].Day
	aria := fmt.Sprintf(chartAriaFmt, len(pts), first, last)
	return &priceChart{Caption: chartLegend, AriaText: aria, Unit: "index_points", Points: pts}
}
