package computefigures

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/compute/rules"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var easeSeriesWant = []string{"WALCL", "WDTGAL", "RRPONTSYD", "DGS10", "DFF", "SP500", "BAMLH0A0HYM2"}

const easeKey = "macro.ease|market|WALCL"

// fakeEase answers with one figure and records what it was asked.
type fakeEase struct {
	fig    compute.EaseFigure
	calls  int
	gotID  string
	gotDay contracts.Day
	gotS   compute.EaseSeries
}

func (e *fakeEase) Ease(id string, s compute.EaseSeries, asOf contracts.Day) compute.EaseFigure {
	e.calls++
	e.gotID, e.gotS, e.gotDay = id, s, asOf
	return e.fig
}

// seriesSpy wraps the series reader of fx: it records each ask and fails the series in fail.
type seriesSpy struct {
	*fx
	asked map[string][2]contracts.Day
	fail  string
}

func (s *seriesSpy) SeriesPoints(ctx context.Context, id string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	s.asked[id] = [2]contracts.Day{from, to}
	if id == s.fail {
		return nil, errors.New("boom")
	}
	return s.fx.SeriesPoints(ctx, id, from, to)
}

func okParts() []compute.EasePart {
	mk := func(in string, d contracts.Day, fig, score float64) compute.EasePart {
		return compute.EasePart{Input: in, DataDay: d, Figure: fig, Score: score, Status: compute.StatusOK}
	}
	return []compute.EasePart{mk("net_liquidity", "2026-11-05", 12.5, 60), mk("us10y", dayD, 4.1, 55.25),
		mk("fed_funds", dayD, 3.9, 50), mk("hy_oas", "2026-11-04", 3.2, 60.4)}
}

func okFigure() compute.EaseFigure {
	return compute.EaseFigure{Figure: compute.Figure{ID: "macro.ease", Version: 2, Day: dayD, DataDay: dayD,
		Status: compute.StatusOK, Value: 56.42, Observations: 4}, Band: compute.EaseSteady, Parts: okParts()}
}

func easeJob2(t *testing.T, s ComputeStores, figs compute.Figures, e compute.Ease) contracts.Job {
	t.Helper()
	j, err := NewWithEase(s, figs, rules.New(), own.NewUnits(), own.NewSeriesOps(), e,
		clockwork.NewFakeClockAt(closeD.Add(20*time.Minute)))
	if err != nil {
		t.Fatalf("NewWithEase: %v", err)
	}
	return j
}

func easeJobOn(t *testing.T, f *fx, e compute.Ease) contracts.Job {
	t.Helper()
	return easeJob2(t, f.rvStores(), newFigs(), e)
}

func numOf(r contracts.ReadingRecord, name string) (float64, bool) {
	for _, p := range r.Parameters {
		if p.Param == name && p.ValueNum != nil {
			return *p.ValueNum, true
		}
	}
	return 0, false
}

func hasParam(r contracts.ReadingRecord, name string) bool {
	for _, p := range r.Parameters {
		if p.Param == name {
			return true
		}
	}
	return false
}

func checkPartParams(t *testing.T, r contracts.ReadingRecord, p compute.EasePart) {
	t.Helper()
	if tagOf(r, p.Input+".data_day") != string(p.DataDay) {
		t.Fatalf("%s.data_day %q", p.Input, tagOf(r, p.Input+".data_day"))
	}
	if v, ok := numOf(r, p.Input+".figure"); !ok || v != p.Figure {
		t.Fatalf("%s.figure %v %v", p.Input, v, ok)
	}
	if v, ok := numOf(r, p.Input+".score"); !ok || v != p.Score {
		t.Fatalf("%s.score %v %v", p.Input, v, ok)
	}
	if hasParam(r, p.Input+".reason") {
		t.Fatalf("%s has a reason though present", p.Input)
	}
}

func TestEaseStoresOkReading(t *testing.T) {
	t.Parallel()
	f := world()
	addIvRv20Inputs(f) // stores the points and rv20 of the session day, so the iv_rv20 keys end without Retryable
	fe := &fakeEase{fig: okFigure()}
	res := runAll(t, easeJobOn(t, f, fe))
	if res.Err != nil || res.Returned != res.Usable {
		t.Fatalf("result %+v", res)
	}
	if fe.calls != 1 || fe.gotID != "macro.ease" || fe.gotDay != dayD {
		t.Fatalf("ease asked %d times with %q on %s", fe.calls, fe.gotID, fe.gotDay)
	}
	r := reading(t, f, "macro.ease", "market", "WALCL")
	if r.Version != 2 || r.Value == nil || *r.Value != "56.42" || r.State != "ok" || string(r.Unit) != "decimal" ||
		r.ReasonCode != nil {
		t.Fatalf("reading %+v", r)
	}
	if tagOf(r, "band") != "steady" {
		t.Fatalf("band %q", tagOf(r, "band"))
	}
	for _, p := range okParts() {
		checkPartParams(t, r, p)
	}
	if len(r.Inputs) != 1 || r.Inputs[0].InputTable != "obs.series_point" ||
		!reflect.DeepEqual(r.Inputs[0].Keys, easeSeriesWant) {
		t.Fatalf("inputs %+v", r.Inputs)
	}
}

func TestEaseMissingStoresReasonParameters(t *testing.T) {
	t.Parallel()
	f := world()
	parts := okParts()
	parts[0] = compute.EasePart{Input: "net_liquidity", Status: compute.StatusMissing, Reason: compute.NoInput}
	parts[3] = compute.EasePart{Input: "hy_oas", Status: compute.StatusMissing, Reason: compute.StaleInput}
	fe := &fakeEase{fig: compute.EaseFigure{Figure: compute.Figure{ID: "macro.ease", Version: 2, Day: dayD,
		Status: compute.StatusMissing, Reason: compute.NoInput}, Parts: parts}}
	runAll(t, easeJobOn(t, f, fe))
	r := reading(t, f, "macro.ease", "market", "WALCL")
	if r.State != "missing_input" || r.Value != nil || r.ReasonCode == nil || *r.ReasonCode != 1 {
		t.Fatalf("reading %+v", r)
	}
	if tagOf(r, "net_liquidity.reason") != "no_input" || tagOf(r, "hy_oas.reason") != "stale_input" {
		t.Fatalf("reason tags %+v", r.Parameters)
	}
	if hasParam(r, "band") {
		t.Fatal("a missing value has no band")
	}
	if !hasParam(r, "us10y.score") || hasParam(r, "net_liquidity.score") {
		t.Fatalf("parameters %+v", r.Parameters)
	}
}

func TestEaseReadsLookbackWindow(t *testing.T) {
	t.Parallel()
	f := world()
	spy := &seriesSpy{fx: f, asked: map[string][2]contracts.Day{}}
	s := f.stores()
	s.Series = spy
	runAll(t, easeJob2(t, s, newFigs(), &fakeEase{fig: okFigure()}))
	for _, id := range easeSeriesWant {
		got, ok := spy.asked[id]
		if !ok || got[0] != shiftDay(dayD, -266) || got[1] != dayD {
			t.Fatalf("%s asked %v (%v), want %s..%s", id, got, ok, shiftDay(dayD, -266), dayD)
		}
	}
}

func TestEaseGivesSeriesToEase(t *testing.T) {
	t.Parallel()
	f := world()
	f.addPoint("DFF", dayD, 3.9)
	fe := &fakeEase{fig: okFigure()}
	runAll(t, easeJobOn(t, f, fe))
	if len(fe.gotS.DFF) != 1 || fe.gotS.DFF[0].Value != 3.9 || len(fe.gotS.WALCL) != 0 {
		t.Fatalf("series given %+v", fe.gotS)
	}
}

func TestEaseStepFailureDoesNotStopOthers(t *testing.T) {
	t.Parallel()
	f := world()
	f.addPoint("VIXCLS", dayD, 20)
	spy := &seriesSpy{fx: f, asked: map[string][2]contracts.Day{}, fail: "DFF"}
	s := f.rvStores()
	s.Series = spy
	j := easeJob2(t, s, newFigs(), &fakeEase{fig: okFigure()})
	res := runAll(t, j)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || res.Usable != res.Returned-1 {
		t.Fatalf("result %+v", res)
	}
	if r := reading(t, f, "market.vix_pctile_3y", "market", "VIXCLS"); r.State != "ok" {
		t.Fatalf("market reading %+v", r)
	}
	post := j.(interface {
		Postcondition(ctx context.Context, day, datasetID, subjectKey string) (bool, error)
	})
	if ok, err := post.Postcondition(context.Background(), string(dayD), "readings.reading", easeKey); ok || err != nil {
		t.Fatalf("ease postcondition %v %v, want false", ok, err)
	}
	if ok, _ := post.Postcondition(context.Background(), string(dayD), "readings.reading", "market.vix_pctile_3y|market|VIXCLS"); !ok {
		t.Fatal("market key must hold")
	}
}

func TestEasePostconditionNamesKey(t *testing.T) {
	t.Parallel()
	f := world()
	j := easeJobOn(t, f, &fakeEase{fig: okFigure()})
	post := j.(interface {
		Postcondition(ctx context.Context, day, datasetID, subjectKey string) (bool, error)
	})
	ctx := context.Background()
	if ok, err := post.Postcondition(ctx, string(dayD), "readings.reading", easeKey); ok || err != nil {
		t.Fatalf("before the run %v %v", ok, err)
	}
	found := false
	for _, k := range plan(t, j) {
		found = found || k.SubjectKey == easeKey
	}
	if !found {
		t.Fatal("plan lacks the ease key")
	}
	runAll(t, j)
	if ok, err := post.Postcondition(ctx, string(dayD), "readings.reading", easeKey); !ok || err != nil {
		t.Fatalf("after the run %v %v", ok, err)
	}
}

func TestEaseKeyRoutedNotToMarketSpec(t *testing.T) {
	t.Parallel()
	f := world()
	fe := &fakeEase{fig: okFigure()}
	j := easeJobOn(t, f, fe)
	res := j.Run(context.Background(), dayD, []contracts.ExpectedKey{{DatasetID: "readings.reading", SubjectKey: easeKey}})
	if res.Err != nil || res.Returned != 1 || res.Usable != 1 || fe.calls != 1 {
		t.Fatalf("result %+v, ease calls %d", res, fe.calls)
	}
	bad := j.Run(context.Background(), dayD, []contracts.ExpectedKey{{DatasetID: "readings.reading", SubjectKey: "macro.ease|market|DFF"}})
	if bad.Err == nil || bad.Err.Kind() != contracts.Permanent {
		t.Fatalf("another ease subject must stay unknown: %+v", bad)
	}
}

func TestMarketFiguresUnchanged(t *testing.T) {
	t.Parallel()
	fill := func() *fx {
		f := world()
		f.addPoint("VIXCLS", dayD, 20)
		f.addPoint("VXVCLS", dayD, 22)
		return f
	}
	a, b := fill(), fill()
	runAll(t, rvJob(t, a, newFigs()))
	runAll(t, easeJobOn(t, b, &fakeEase{fig: okFigure()}))
	for _, k := range plan(t, rvJob(t, a, newFigs())) {
		name, kind, subj, _ := parseKey(k.SubjectKey)
		if !reflect.DeepEqual(reading(t, a, name, kind, subj), reading(t, b, name, kind, subj)) {
			t.Fatalf("%s differs with an Ease", k.SubjectKey)
		}
	}
	// the parameters of the old figures keep their names and order
	names := func(r contracts.ReadingRecord) string {
		var n []string
		for _, p := range r.Parameters {
			n = append(n, p.Param)
		}
		return strings.Join(n, ",")
	}
	if got := names(reading(t, a, "market.iv30_own", "underlying", "SPY")); got != "data_day,observations,stale_rate" {
		t.Fatalf("iv30 parameters %q", got)
	}
	if got := names(reading(t, a, "market.vix_pctile_3y", "market", "VIXCLS")); got != "data_day,observations,years" {
		t.Fatalf("market parameters %q", got)
	}
}

func TestEaseRerunSameRowsNoNewRev(t *testing.T) {
	t.Parallel()
	f := world()
	addIvRv20Inputs(f) // stores the points and rv20 of the session day, so the iv_rv20 keys end without Retryable
	j := easeJobOn(t, f, &fakeEase{fig: okFigure()})
	if first := runAll(t, j); first.Err != nil {
		t.Fatalf("first run %+v", first)
	}
	before := f.inserts
	res := runAll(t, j)
	if res.Err != nil || f.inserts != before || res.Usable != res.Returned {
		t.Fatalf("rerun inserted %d rows (%+v)", f.inserts-before, res)
	}
	if r := reading(t, f, "macro.ease", "market", "WALCL"); r.Rev != 1 {
		t.Fatalf("rev %d", r.Rev)
	}
}

func TestNewWithoutEaseHasNoEaseKey(t *testing.T) {
	t.Parallel()
	f := world()
	j := newJob(t, f, newFigs())
	for _, k := range plan(t, j) {
		if strings.HasPrefix(k.SubjectKey, "macro.ease") {
			t.Fatalf("unexpected key %s", k.SubjectKey)
		}
	}
	res := j.Run(context.Background(), dayD, []contracts.ExpectedKey{{DatasetID: "readings.reading", SubjectKey: easeKey}})
	if res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Fatalf("the ease key must be unknown without an Ease: %+v", res)
	}
	if rows, _ := f.LatestReadings(context.Background(), "macro.ease", "market", "WALCL", dayD, dayD); len(rows) != 0 {
		t.Fatalf("an ease row was written: %+v", rows)
	}
}

var _ store.SeriesReader = (*seriesSpy)(nil)

func TestEaseSpecRefusesSwappedSeries(t *testing.T) {
	t.Parallel()
	e := easeSpec{ID: "macro.ease", Series: []string{"WDTGAL", "WALCL", "RRPONTSYD", "DGS10", "DFF", "SP500", "BAMLH0A0HYM2"}}
	if err := checkEaseSpec(e); err == nil {
		t.Fatal("swapped WALCL/WDTGAL accepted; runEase would feed WDTGAL as WALCL")
	}
}
