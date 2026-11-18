package option_candidates_page

import (
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// readingCfg is a page with one ok row, the fakes for the readings and the given view.
func readingCfg(v *fakeView, e *fakeEval) Config {
	c := evConfig([]store.Candidate{okPut(1)}, e)
	c.ATMFor = (&atmCalls{iv: "0.294", strike: "95.00"}).fn
	c.Names = &namesFake{names: map[int]string{7: "AAA"}}
	if v != nil {
		c.Readings = v
	}
	return c
}

func TestReadingsWindow7Days(t *testing.T) {
	t.Parallel()
	for day, used := range map[string]bool{"2026-11-02": true, "2026-11-01": false} {
		v := &fakeView{recs: map[string][]contracts.ReadingRecord{idIVP: {okRec(day, "0.58")}}}
		cell := firstRow(buildOut(t, readingCfg(v, &fakeEval{}), nil)).Cells["ivp"]
		if (cell.Kind != "gap") != used {
			t.Errorf("reading of %s: cell = %+v", day, cell)
		}
	}
}

func TestReadingDataDayBadge(t *testing.T) {
	t.Parallel()
	v := &fakeView{recs: map[string][]contracts.ReadingRecord{idIVP: {okRec("2026-11-06", "0.58")}}}
	cell := firstRow(buildOut(t, readingCfg(v, &fakeEval{}), nil)).Cells["ivp"]
	if !slices.Contains(cell.Badges, "as of 2026-11-06") || cell.Text != "58.0%" {
		t.Errorf("ivp = %+v", cell)
	}
}

func TestBidForecastBadgeDay(t *testing.T) {
	t.Parallel()
	for day, want := range map[string]bool{"2026-11-06": true, "2026-11-09": false} {
		v := &fakeView{recs: map[string][]contracts.ReadingRecord{idVolForecast: {okRec(day, "0.275")}}}
		cell := firstRow(buildOut(t, readingCfg(v, &fakeEval{}), nil)).Cells["bid_vs_forecast"]
		if got := slices.Contains(cell.Badges, "as of "+day); got != want {
			t.Errorf("forecast of %s: badges = %v", day, cell.Badges)
		}
	}
}

func TestReadingGapReason(t *testing.T) {
	t.Parallel()
	code2, code3 := 2, 3
	v := &fakeView{recs: map[string][]contracts.ReadingRecord{
		idVolForecast: {recAt("2026-11-05", "missing_input", nil, &code3), recAt("2026-11-07", "missing_input", nil, &code2)},
	}}
	r := firstRow(buildOut(t, readingCfg(v, &fakeEval{}), nil))
	if g := r.Cells["forecast_vol"]; g.Kind != "gap" || g.Text != "No forecast volatility stored (short_history)" || g.Reason != "missing_input" {
		t.Errorf("forecast_vol = %+v", g)
	}
	if g := r.Cells["ivp"]; g.Kind != "gap" || g.Text != "no_input" {
		t.Errorf("ivp = %+v", g)
	}
}

func TestNilReadingsGap(t *testing.T) {
	t.Parallel()
	f := &fakeEval{fn: func(optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
		e := fullEvidence()
		e.EdgeWhy, e.EdgePerShare = "no_forecast", ""
		return e, nil
	}}
	r := firstRow(buildOut(t, readingCfg(nil, f), nil))
	for _, id := range []string{"forecast_vol", "ivp", "vol_edge"} {
		if g := r.Cells[id]; g.Reason != "no_reader" || g.Text != "No reading reader" {
			t.Errorf("%s = %+v", id, g)
		}
	}
	if g := r.Cells["bid_vs_forecast"]; g.Kind != "gap" || g.Reason != "missing_input" || g.Text != "No forecast volatility stored (no_reader)" {
		t.Errorf("bid_vs_forecast = %+v", g)
	}
}

func TestNilNamesGap(t *testing.T) {
	t.Parallel()
	v := allReadings("2026-11-09")
	c := readingCfg(v, &fakeEval{})
	c.Names = nil
	r := firstRow(buildOut(t, c, nil))
	for _, id := range []string{"forecast_vol", "ivp", "vol_edge"} {
		if g := r.Cells[id]; g.Reason != "no_reader" || g.Text != "No ticker: readings are by ticker" {
			t.Errorf("%s = %+v", id, g)
		}
	}
	if v.reads() != 0 {
		t.Errorf("the reading reader was called %d times", v.reads())
	}
}

func TestReadingsPassedAsText(t *testing.T) {
	t.Parallel()
	f := &fakeEval{}
	v := allReadings("2026-11-09")
	c := readingCfg(v, f)
	c.Evidence = f
	c.Candidates = fakeReader{rows: map[string][]store.Candidate{"2026-11-09/short_put": {okPut(1), okPut(2)}}}
	buildOut(t, c, nil)
	for _, got := range f.rows {
		if got.Forecast != "0.275" || got.ForecastErr != "0.041" || got.IVP != "0.58" || got.VolEdge != "0.037" {
			t.Errorf("row = %+v", got)
		}
	}
	if v.reads() != 4 || v.subjects[0] != "AAA" {
		t.Errorf("reads = %v, want 4 for one name", v.subjects)
	}
}

func TestRateAgeSevenDays(t *testing.T) {
	t.Parallel()
	if maxRateAgeDays != 7 {
		t.Fatalf("maxRateAgeDays = %d", maxRateAgeDays)
	}
	for _, c := range []struct {
		day, want string
	}{{"2026-11-02", "4.3"}, {"2026-11-01", ""}} {
		f := &fakeEval{fn: func(r optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
			e := fullEvidence()
			if r.RatePercent == "" {
				e.EdgeWhy = "no_rate"
			}
			return e, nil
		}}
		v := 4.30
		series := &fakeRate{pts: []contracts.SeriesPoint{point(c.day, &v)}}
		cell := firstRow(buildOut(t, readingCfg(allReadings("2026-11-09"), f), series)).Cells["bid_vs_forecast"]
		if f.rows[0].RatePercent != c.want {
			t.Errorf("rate of %s: passed %q, want %q", c.day, f.rows[0].RatePercent, c.want)
		}
		if gotGap := cell.Kind == "gap"; gotGap != (c.want == "") || (gotGap && cell.Text != "No cash rate within 7 days (DGS3MO)") {
			t.Errorf("rate of %s: cell = %+v", c.day, cell)
		}
	}
}
