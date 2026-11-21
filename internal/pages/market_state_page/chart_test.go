package market_state_page

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func pts(vals map[string]string, days ...string) marketstateapi.Line {
	l := marketstateapi.Line{}
	for _, d := range days {
		if v, ok := vals[d]; ok {
			l.Points = append(l.Points, marketstateapi.Point{Day: contracts.Day(d), Value: v})
		}
	}
	l.Total = len(l.Points)
	return l
}

func chartResult() marketstateapi.Result {
	r := sampleResult()
	days := []string{"2026-11-11", "2026-11-12", "2026-11-13"}
	r.History.Close = pts(map[string]string{"2026-11-11": "5900.10", "2026-11-12": "5950.25", "2026-11-13": "6000.00"}, days...)
	r.History.Ma200 = pts(map[string]string{"2026-11-12": "5500.50", "2026-11-13": "5510.00"}, days...)
	r.History.BandLow = pts(map[string]string{"2026-11-13": "5800.00"}, days...)
	r.History.BandHigh = pts(map[string]string{"2026-11-13": "6100.00"}, days...)
	return r
}

func TestChartAndVolGap(t *testing.T) {
	t.Parallel()
	schema := compileSchema(t)
	c := fullConfig(t, chartResult())
	c.Indicators = nil
	b, err := Build(t.Context(), c, fakeSeries{}, testDay, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, b)
	v, err := parseJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(v); err != nil {
		t.Fatal(err)
	}
	row := obj(marketRows(m)[0])
	pc := obj(row["price_chart"])
	if pc["caption"] != chartLegend {
		t.Errorf("caption %v", pc["caption"])
	}
	points := list(pc["points"])
	if len(points) != 3 {
		t.Fatalf("%d points", len(points))
	}
	p0, p1, p2 := obj(points[0]), obj(points[1]), obj(points[2])
	if p0["close_text"] != "5900.10" {
		t.Errorf("close_text %v", p0["close_text"])
	}
	for _, k := range []string{"ma200", "ma200_text", "band_low", "band_low_text", "band_high", "band_high_text"} {
		if _, ok := p0[k]; ok {
			t.Errorf("day 1 has %s", k)
		}
	}
	if p1["ma200_text"] != "5500.50" {
		t.Errorf("day 2 %v", p1)
	}
	if _, ok := p1["band_low"]; ok {
		t.Errorf("day 2 has band_low")
	}
	if p2["ma200_text"] != "5510.00" || p2["band_low_text"] != "5800.00" || p2["band_high_text"] != "6100.00" {
		t.Errorf("day 3 %v", p2)
	}
	if _, ok := obj(marketRows(m)[1])["price_chart"]; ok {
		t.Error("another row carries price_chart")
	}
}

func TestChartAbsentWithoutClose(t *testing.T) {
	t.Parallel()
	m := build(t, fullConfig(t, sampleResult()))
	if _, ok := obj(marketRows(m)[0])["price_chart"]; ok {
		t.Error("price_chart without Close points")
	}
}

func TestIndicatorsCopiedIntoInput(t *testing.T) {
	t.Parallel()
	var got marketstateapi.Input
	c := fullConfig(t, sampleResult())
	c.Indicators = fakeIndicators{}
	c.Market = func(in marketstateapi.Input) marketstateapi.Result { got = in; return sampleResult() }
	build(t, c)
	if _, ok := got.Indicators.(fakeIndicators); !ok {
		t.Errorf("Input.Indicators = %T", got.Indicators)
	}
}
