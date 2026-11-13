package computefigures

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var volNames = []string{"market.rv20", "market.rv60", "market.rv120", "market.vol_forecast", "market.vol_edge",
	"market.vol_forecast_err"}

// addIvIbkr stores an ok market.iv_ibkr reading of SPY for the day.
func (f *fx) addIvIbkr(day contracts.Day, v string) {
	f.readings = append(f.readings, contracts.ReadingRecord{Name: "market.iv_ibkr", SubjectKind: "underlying", Subject: "SPY",
		Day: dateOf(day), State: "ok", Value: &v, Rev: 1})
}

func volState(t *testing.T, f *fx, name string) (string, int) {
	t.Helper()
	rec := reading(t, f, name, "underlying", "SPY")
	code := 0
	if rec.ReasonCode != nil {
		code = *rec.ReasonCode
	}
	return string(rec.State), code
}

func wantVol(t *testing.T, f *fx, name, state string, code int) {
	t.Helper()
	if s, c := volState(t, f, name); s != state || c != code {
		t.Fatalf("%s: state %s code %d, want %s code %d", name, s, c, state, code)
	}
}

func TestVolStoresAllFigures(t *testing.T) {
	t.Parallel()
	f, b := world(), &fakeBars{}
	b.addCloses("trades", dayD, alternating(160))
	f.addIvIbkr(dayD, "0.3")
	res := runAll(t, hvJob(t, f, b, realFigs()))
	if res.Returned != res.Usable {
		t.Fatalf("result %+v", res)
	}
	for _, n := range volNames {
		wantVol(t, f, n, "ok", 0)
	}
	fc := reading(t, f, "market.vol_forecast", "underlying", "SPY")
	if tagOf(fc, "rv120_day") != string(dayD) || tagOf(fc, "data_day") != string(dayD) {
		t.Fatalf("forecast parameters %+v", fc.Parameters)
	}
	er := reading(t, f, "market.vol_forecast_err", "underlying", "SPY")
	if tagOf(er, "forecast_day") != string(shiftDay(dayD, -20)) || tagOf(er, "rv20_day") != string(dayD) {
		t.Fatalf("error parameters %+v", er.Parameters)
	}
	if tagOf(reading(t, f, "market.vol_edge", "underlying", "SPY"), "implied_day") != string(dayD) {
		t.Fatal("edge has no implied_day")
	}
}

func TestVolShortHistory(t *testing.T) {
	t.Parallel()
	f, b := world(), &fakeBars{}
	b.addCloses("trades", dayD, alternating(70))
	f.addIvIbkr(dayD, "0.3")
	runAll(t, hvJob(t, f, b, realFigs()))
	wantVol(t, f, "market.rv20", "ok", 0)
	wantVol(t, f, "market.rv60", "ok", 0)
	wantVol(t, f, "market.rv120", "missing_input", 2)
	wantVol(t, f, "market.vol_forecast", "missing_input", 1)
	wantVol(t, f, "market.vol_edge", "missing_input", 1)
	wantVol(t, f, "market.vol_forecast_err", "missing_input", 1)
}

func TestVolNoImplied(t *testing.T) {
	t.Parallel()
	f, b := world(), &fakeBars{}
	b.addCloses("trades", dayD, alternating(160))
	f.addIvIbkr(shiftDay(dayD, -1), "0.3") // an earlier day is no implied of the as-of day
	runAll(t, hvJob(t, f, b, realFigs()))
	wantVol(t, f, "market.vol_forecast", "ok", 0)
	wantVol(t, f, "market.vol_edge", "missing_input", 1)
}

func TestVolIndexRv20Once(t *testing.T) {
	t.Parallel()
	f, b := world(), &fakeBars{}
	f.names[1] = "SPX"
	b.addCloses("trades", dayD, alternating(160))
	b.addFor(11, "trades", dayD, alternating(21))
	j := hvJob(t, f, b, realFigs())
	keys := plan(t, j)
	n := map[string]int{}
	for _, k := range keys {
		if strings.HasSuffix(k.SubjectKey, "|underlying|SPX") {
			n[strings.SplitN(k.SubjectKey, "|", 2)[0]]++
		}
	}
	if n["market.rv20"] != 1 || len(n) != 11 {
		t.Fatalf("SPX keys %v, want one market.rv20 and eleven ids", n)
	}
	res := j.Run(t.Context(), dayD, keys)
	if res.Err != nil || res.Returned != res.Usable {
		t.Fatalf("result %+v", res)
	}
	count := map[string]int{}
	for _, r := range f.readings {
		if r.Subject == "SPX" {
			count[r.Name]++
		}
	}
	for _, id := range volNames {
		if count[id] != 1 {
			t.Fatalf("%s has %d readings of SPX, want 1 (%v)", id, count[id], count)
		}
	}
}

func TestVolRoutesRv20OfStock(t *testing.T) {
	t.Parallel()
	f, b := world(), &fakeBars{}
	b.addCloses("trades", dayD, alternating(160))
	runAll(t, hvJob(t, f, b, realFigs()))
	rec := reading(t, f, "market.rv20", "underlying", "SPY")
	hasWindow := false
	for _, p := range rec.Parameters {
		hasWindow = hasWindow || (p.Param == "window" && p.ValueNum != nil && *p.ValueNum == 20)
	}
	if rec.State != "ok" || !hasWindow {
		t.Fatalf("rv20 of SPY %+v, want the realised volatility of 20 sessions", rec)
	}
	ivp := reading(t, f, "market.ivp_own", "underlying", "SPY")
	if ivp.Name != "market.ivp_own" || len(ivp.Inputs) != 1 {
		t.Fatalf("ivp reading %+v", ivp)
	}
}

func TestVolGatedByBars(t *testing.T) {
	t.Parallel()
	j := newJob(t, world(), newFigs())
	for _, k := range plan(t, j) {
		for _, n := range volNames {
			if strings.HasPrefix(k.SubjectKey, n+"|underlying|SPY") {
				t.Fatalf("key %s planned without price bars", k.SubjectKey)
			}
		}
	}
}

// strictFigs refuses a data day older than as-of minus max_age_days, as the core does, and records every
// as-of day of HistoricalVol.
type strictFigs struct {
	compute.Figures
	asOfs map[contracts.Day]bool
	stale int
}

func (s *strictFigs) HistoricalVol(id string, c compute.Series, d contracts.Day) compute.Figure {
	s.asOfs[d] = true
	if len(c) > 0 && daysBetween(c[len(c)-1].Day, d) > 4 {
		s.stale++
	}
	return s.Figures.HistoricalVol(id, c, d)
}

func TestVolErrorAsOf(t *testing.T) {
	t.Parallel()
	f, b := world(), &fakeBars{}
	b.addCloses("trades", dayD, alternating(160))
	s := &strictFigs{Figures: realFigs(), asOfs: map[contracts.Day]bool{}}
	runAll(t, hvJob(t, f, b, s))
	wantVol(t, f, "market.vol_forecast_err", "ok", 0)
	if s.stale != 0 {
		t.Fatalf("%d windows called with a data day older than max_age_days", s.stale)
	}
	if !s.asOfs[shiftDay(dayD, -20)] {
		t.Fatalf("no window called with the day of the past session: %v", s.asOfs)
	}
}
