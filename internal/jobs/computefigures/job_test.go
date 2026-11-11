package computefigures

import (
	"context"
	"encoding/hex"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/compute/rules"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeFigs answers like the core in the way the job can see: an empty input is no_input, a short one
// short_history, an old one stale_input (by the rule's MaxAgeDays), else ok with a value of the newest point.
type fakeFigs struct {
	rules    compute.Rules
	override *float64
	ivSeries compute.Series
}

func newFigs() *fakeFigs { return &fakeFigs{rules: rules.New()} }

func (g *fakeFigs) fig(id string, asOf contracts.Day) (compute.Rule, compute.Figure) {
	r, _ := g.rules.Rule(id)
	return r, compute.Figure{ID: id, Version: r.Version, Day: asOf, Status: compute.StatusOK}
}

func miss(f compute.Figure, why compute.Reason) compute.Figure {
	f.Status, f.Reason, f.Value = compute.StatusMissing, why, 0
	return f
}

func (g *fakeFigs) fromSeries(id string, s compute.Series, asOf contracts.Day) compute.Figure {
	r, f := g.fig(id, asOf)
	if len(s) == 0 {
		return miss(f, compute.NoInput)
	}
	last := s[len(s)-1]
	f.DataDay, f.Observations, f.Value = last.Day, len(s), last.Value
	if g.override != nil {
		f.Value = *g.override
	}
	if daysBetween(last.Day, asOf) > r.MaxAgeDays {
		return miss(f, compute.StaleInput)
	}
	return f
}

func (g *fakeFigs) HistoricalVol(id string, s compute.Series, d contracts.Day) compute.Figure {
	return g.fromSeries(id, s, d)
}

func (g *fakeFigs) Iv30(id string, chain []compute.ExpiryQuotes, rate compute.Point, asOf contracts.Day) compute.Figure {
	_, f := g.fig(id, asOf)
	f.StaleRate = rate.Day != asOf
	if len(chain) == 0 {
		return miss(f, compute.NoInput)
	}
	f.DataDay, f.Observations, f.Value = asOf, len(chain), 0.2
	if g.override != nil {
		f.Value = *g.override
	}
	return f
}

func (g *fakeFigs) IvPercentile(id string, s compute.Series, d contracts.Day) compute.Figure {
	g.ivSeries = s
	if len(s) < 2 {
		_, f := g.fig(id, d)
		f.DataDay = d
		return miss(f, compute.ShortHistory)
	}
	return g.fromSeries(id, s, d)
}

func (g *fakeFigs) IvRank(id string, s compute.Series, d contracts.Day) compute.Figure {
	return g.IvPercentile(id, s, d)
}

func (g *fakeFigs) MovingAverageGap(id string, s compute.Series, d contracts.Day) compute.Figure {
	return g.fromSeries(id, s, d)
}

func (g *fakeFigs) PercentileInYears(id string, s compute.Series, d contracts.Day) compute.Figure {
	return g.fromSeries(id, s, d)
}

func (g *fakeFigs) Ratio(id string, a, b compute.Series, d contracts.Day) compute.Figure {
	return g.fromSeries(id, a, d)
}

var marketIDs = []string{"market.vix_term_ratio", "market.vix_pctile_3y", "market.vxn_pctile_3y", "market.rvx_pctile_3y",
	"market.sp500_pctile_3y", "market.sp500_vs_200d"}

func plan(t *testing.T, j contracts.Job) []contracts.ExpectedKey {
	t.Helper()
	keys, _, err := j.Plan(context.Background(), dayD)
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

func runAll(t *testing.T, j contracts.Job) contracts.JobResult {
	t.Helper()
	return j.Run(context.Background(), dayD, plan(t, j))
}

func reading(t *testing.T, f *fx, name, kind, subj string) contracts.ReadingRecord {
	t.Helper()
	rows, _ := f.LatestReadings(context.Background(), name, kind, subj, dayD, dayD)
	if len(rows) != 1 {
		t.Fatalf("%s|%s|%s: %d readings, want 1", name, kind, subj, len(rows))
	}
	return rows[0]
}

func TestPlanKeys(t *testing.T) {
	t.Parallel()
	j := newJob(t, world(), newFigs())
	keys := plan(t, j)
	if len(keys) != 4+len(marketIDs) {
		t.Fatalf("keys %d, want 10 (4 per underlying, 6 market)", len(keys))
	}
	if !sort.SliceIsSorted(keys, func(a, b int) bool { return keys[a].SubjectKey < keys[b].SubjectKey }) {
		t.Fatalf("keys not sorted: %+v", keys)
	}
	seen := map[string]bool{}
	for _, k := range keys {
		if k.DatasetID != "readings.reading" || seen[k.SubjectKey] {
			t.Fatalf("bad or duplicate key %+v", k)
		}
		seen[k.SubjectKey] = true
	}
	for _, want := range []string{"market.iv30_own|underlying|SPY", "market.ivp_own|underlying|SPY", "market.ivr_own|underlying|SPY",
		"market.vix_term_ratio|market|VIXCLS", "market.sp500_vs_200d|market|SP500"} {
		if !seen[want] {
			t.Fatalf("missing key %s", want)
		}
	}
}

func TestRunStoresOkAndMissing(t *testing.T) {
	t.Parallel()
	f := world()
	f.addPoint("VIXCLS", dayD, 20)
	f.addPoint("VXVCLS", dayD, 22)
	j := newJob(t, f, newFigs())
	res := runAll(t, j)
	if res.Err != nil || res.Returned != 10 || res.Usable != 10 {
		t.Fatalf("result %+v, want 10 of 10 and no error", res)
	}
	iv := reading(t, f, "market.iv30_own", "underlying", "SPY")
	if iv.State != "ok" || iv.Value == nil || *iv.Value != "0.200000" || iv.Rev != 1 {
		t.Fatalf("iv30 %+v", iv)
	}
	vr := reading(t, f, "market.vix_term_ratio", "market", "VIXCLS")
	if vr.State != "ok" || tagOf(vr, "data_day") != string(dayD) {
		t.Fatalf("vix ratio %+v", vr)
	}
	sp := reading(t, f, "market.sp500_vs_200d", "market", "SP500")
	if sp.State != "missing_input" || sp.Value != nil || sp.ReasonCode == nil || *sp.ReasonCode != 1 {
		t.Fatalf("sp500 %+v, want missing no_input", sp)
	}
}

func TestRerunSameInputsWritesNothing(t *testing.T) {
	t.Parallel()
	f := world()
	f.addPoint("VIXCLS", dayD, 20)
	j := newJob(t, f, newFigs())
	runAll(t, j)
	before := f.inserts
	res := runAll(t, j)
	if f.inserts != before || res.Usable != res.Returned {
		t.Fatalf("rerun inserted %d rows (result %+v), want none", f.inserts-before, res)
	}
}

func TestChangedInputWritesNextRev(t *testing.T) {
	t.Parallel()
	f := world()
	f.addPoint("VIXCLS", dayD, 20)
	j := newJob(t, f, newFigs())
	runAll(t, j)
	f.addPoint("VIXCLS", shiftDay(dayD, -1), 19) // a late point changes the rows read
	runAll(t, j)
	got := reading(t, f, "market.vix_pctile_3y", "market", "VIXCLS")
	if got.Rev != 2 {
		t.Fatalf("rev %d, want 2", got.Rev)
	}
	if other := reading(t, f, "market.vxn_pctile_3y", "market", "VXNCLS"); other.Rev != 1 {
		t.Fatalf("unchanged key rev %d, want 1", other.Rev)
	}
}

func TestIvpReadsStoredIv30(t *testing.T) {
	t.Parallel()
	f := world()
	g := newFigs()
	j := newJob(t, f, g)
	for _, d := range []contracts.Day{"2026-11-04", "2026-11-05"} {
		f.readings = append(f.readings, contracts.ReadingRecord{Name: "market.iv30_own", SubjectKind: "underlying", Subject: "SPY",
			Day: dateOf(d), State: "ok", Value: sp("0.25"), Rev: 1})
	}
	runAll(t, j)
	if ivp := reading(t, f, "market.ivp_own", "underlying", "SPY"); ivp.State != "ok" {
		t.Fatalf("ivp %+v, want ok from the stored IV30", ivp)
	}
	if len(g.ivSeries) != 3 || g.ivSeries[2].Day != dayD || g.ivSeries[0].Value != 0.25 {
		t.Fatalf("series given to IVP %+v, want the two stored days and today's", g.ivSeries)
	}
	// a missing IV30 gives a missing IVP with no_input
	f2 := world()
	f2.series["DGS3MO"] = nil
	f2.addRate("2026-10-01", 4)
	runAll(t, newJob(t, f2, newFigs()))
	ivp := reading(t, f2, "market.ivp_own", "underlying", "SPY")
	if ivp.State != "missing_input" || ivp.ReasonCode == nil || *ivp.ReasonCode != 1 {
		t.Fatalf("ivp %+v, want missing no_input", ivp)
	}
}

func sp(s string) *string { return &s }

func TestFirstRunOnEmptyStores(t *testing.T) {
	t.Parallel()
	f := newFx()
	j := newJob(t, f, newFigs())
	keys := plan(t, j)
	res := j.Run(context.Background(), dayD, keys)
	if res.Err != nil || res.Usable != len(keys) {
		t.Fatalf("result %+v for %d keys", res, len(keys))
	}
	for _, k := range keys {
		name, kind, subj, _ := parseKey(k.SubjectKey)
		r := reading(t, f, name, kind, subj)
		if r.State != "missing_input" || r.ReasonCode == nil || (*r.ReasonCode != 1 && *r.ReasonCode != 2) {
			t.Fatalf("%s: %+v, want missing with reason 1 or 2", k.SubjectKey, r)
		}
	}
	if len(keys) != len(marketIDs) {
		t.Fatalf("keys %d, want only the market keys on empty stores", len(keys))
	}
}

func TestFredLagStaysOk(t *testing.T) {
	t.Parallel()
	f := world()
	f.addPoint("VIXCLS", shiftDay(dayD, -5), 20)
	f.addPoint("VXNCLS", shiftDay(dayD, -9), 20)
	runAll(t, newJob(t, f, newFigs()))
	ok := reading(t, f, "market.vix_pctile_3y", "market", "VIXCLS")
	if ok.State != "ok" || tagOf(ok, "data_day") != string(shiftDay(dayD, -5)) {
		t.Fatalf("5 days old %+v, want ok with data_day", ok)
	}
	old := reading(t, f, "market.vxn_pctile_3y", "market", "VXNCLS")
	if old.State != "missing_input" || old.ReasonCode == nil || *old.ReasonCode != 3 {
		t.Fatalf("9 days old %+v, want missing_input with reason_code 3", old)
	}
}

// marketWorld is the world with the session days of the week of dayD: the 5th and the 6th, the 6th a session
// unless holiday is set.
func marketWorld(holiday bool) *fx {
	f := world()
	f.sessions = nil
	d5 := shiftDay(dayD, -1)
	f.addSession(d5, true, closeD.AddDate(0, 0, -1).Add(-6*time.Hour-30*time.Minute), closeD.AddDate(0, 0, -1))
	if holiday {
		f.addSession(dayD, false, time.Time{}, time.Time{})
	} else {
		f.addSession(dayD, true, closeD.Add(-6*time.Hour-30*time.Minute), closeD)
	}
	return f
}

func TestMarketInputBehindCalendarIsRetryable(t *testing.T) {
	t.Parallel()
	f := marketWorld(false)
	f.addPoint("VIXCLS", shiftDay(dayD, -1), 20)
	res := runAll(t, newJob(t, f, newFigs()))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("Err %v, want Retryable", res.Err)
	}
	msg := res.Err.Error()
	for _, want := range []string{"VIXCLS", string(shiftDay(dayD, -1)), string(dayD), "waits for"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q lacks %q", msg, want)
		}
	}
	r := reading(t, f, "market.vix_pctile_3y", "market", "VIXCLS")
	if r.State != "ok" || tagOf(r, "data_day") != string(shiftDay(dayD, -1)) {
		t.Fatalf("reading %+v, want stored ok with the older data_day", r)
	}
}

func TestMarketInputCurrentIsDone(t *testing.T) {
	t.Parallel()
	f := marketWorld(false)
	f.addPoint("VIXCLS", dayD, 20)
	f.addPoint("VXVCLS", dayD, 22)
	f.addPoint("VXNCLS", dayD, 20)
	f.addPoint("RVXCLS", dayD, 20)
	f.addPoint("SP500", dayD, 20)
	res := runAll(t, newJob(t, f, newFigs()))
	if res.Err != nil {
		t.Fatalf("Err %v, want none", res.Err)
	}
}

func TestMarketInputBehindOnHolidayIsDone(t *testing.T) {
	t.Parallel()
	f := marketWorld(true)
	d5 := shiftDay(dayD, -1)
	for _, id := range []string{"VIXCLS", "VXVCLS", "VXNCLS", "RVXCLS", "SP500"} {
		f.addPoint(id, d5, 20)
	}
	res := runAll(t, newJob(t, f, newFigs()))
	if res.Err != nil {
		t.Fatalf("Err %v, want none: the as-of day is no session", res.Err)
	}
}

func TestPostconditionRowPerKey(t *testing.T) {
	t.Parallel()
	f := world()
	j := newJob(t, f, newFigs()).(interface {
		Postcondition(ctx context.Context, day, datasetID, subjectKey string) (bool, error)
	})
	ctx := context.Background()
	key := "market.iv30_own|underlying|SPY"
	if ok, err := j.Postcondition(ctx, string(dayD), "readings.reading", key); ok || err != nil {
		t.Fatalf("before the run: %v %v, want false", ok, err)
	}
	runAll(t, j.(contracts.Job))
	if ok, err := j.Postcondition(ctx, string(dayD), "readings.reading", key); !ok || err != nil {
		t.Fatalf("after the run: %v %v, want true", ok, err)
	}
	if ok, _ := j.Postcondition(ctx, "2026-11-09", "readings.reading", key); ok {
		t.Fatal("another day has no row")
	}
	if _, err := j.Postcondition(ctx, string(dayD), "readings.reading", "bad"); err == nil {
		t.Fatal("a malformed key is an error")
	}
}

func TestScheduleValues(t *testing.T) {
	t.Parallel()
	j := newJob(t, world(), newFigs()).(interface {
		Schedule() (string, string, string, time.Duration)
		RunBound() time.Duration
		RestartSensitive() bool
	})
	cal, anchor, zone, off := j.Schedule()
	if cal != "XNYS" || anchor != "session_close" || zone != "" || off != 20*time.Minute {
		t.Fatalf("schedule %s %s %q %v", cal, anchor, zone, off)
	}
	var m manifest
	if err := toml.Unmarshal(manifestBytes, &m); err != nil || m.Schedule.Offset != "20m" || m.Schedule.Calendar != "XNYS" ||
		m.Schedule.Anchor != "session_close" {
		t.Fatalf("job.toml schedule %+v err %v", m.Schedule, err)
	}
	if j.RunBound() != 10*time.Minute || j.RestartSensitive() {
		t.Fatalf("bound %v sensitive %v", j.RunBound(), j.RestartSensitive())
	}
}

func TestManifestHasNoNumbers(t *testing.T) {
	t.Parallel()
	var raw map[string]any
	if err := toml.Unmarshal(manifestBytes, &raw); err != nil {
		t.Fatal(err)
	}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				if k == "schedule" && path == "" {
					continue
				}
				walk(path+"."+k, e)
			}
		case []map[string]any:
			for _, e := range x {
				walk(path, e)
			}
		case []any:
			for _, e := range x {
				walk(path, e)
			}
		case string:
		default:
			t.Errorf("%s holds %T %v, want text only", path, v, v)
		}
	}
	walk("", raw)
}

func TestReadingCarriesBothHashes(t *testing.T) {
	t.Parallel()
	f := world()
	runAll(t, newJob(t, f, newFigs()))
	iv := reading(t, f, "market.iv30_own", "underlying", "SPY")
	rule, _ := rules.New().Rule("market.iv30_own")
	if len(iv.Inputs) == 0 {
		t.Fatal("no inputs")
	}
	for _, in := range iv.Inputs {
		if in.CodeVersion != rule.Hash || in.ConventionVersion != rule.Convention {
			t.Fatalf("input %+v, want rule hash %s", in, rule.Hash)
		}
	}
	if b, err := hex.DecodeString(iv.RowsSha256); err != nil || len(b) != 32 || iv.RowsSha256 != strings.ToLower(iv.RowsSha256) {
		t.Fatalf("rows_sha256 %q is not 64 hex", iv.RowsSha256)
	}
	if iv.Cutoff != "session_close" || iv.Origin != "compute_figures" || iv.JobRunID == "" {
		t.Fatalf("reading %+v", iv)
	}
}

func TestValueIsRoundedText(t *testing.T) {
	t.Parallel()
	f := world()
	g := newFigs()
	g.override = ptr(0.0078125)
	f.addPoint("VIXCLS", dayD, 20)
	runAll(t, newJob(t, f, g))
	r := reading(t, f, "market.vix_pctile_3y", "market", "VIXCLS")
	if r.Value == nil || *r.Value != "0.007812" {
		t.Fatalf("value %v, want 0.007812 (half-even)", r.Value)
	}
}

func ptr(v float64) *float64 { return &v }

func TestPlanKeysUniqueWhenNamesCollide(t *testing.T) {
	t.Parallel()
	f := world()
	f.latest = append(f.latest, store.OptionEodLatest{UnderlyingID: 2, SessionDay: dayD})
	f.names[2] = "SPY"
	row := eodRow(200, fp(1), fp(2), nil, closeD)
	row.UnderlyingID = 2
	f.eod[2] = []contracts.OptionEod{row}
	j := newJob(t, f, newFigs())
	seen := map[string]bool{}
	for _, k := range plan(t, j) {
		if seen[k.SubjectKey] {
			t.Fatalf("duplicate key %s", k.SubjectKey)
		}
		seen[k.SubjectKey] = true
	}
	for _, want := range []string{"market.iv30_own|underlying|underlying:1", "market.iv30_own|underlying|underlying:2"} {
		if !seen[want] {
			t.Fatalf("missing key %s", want)
		}
	}
}

func TestUnknownMethodRejected(t *testing.T) {
	t.Parallel()
	j := &job{m: manifest{Iv30: "market.iv30_own", Ivp: "market.ivp_own", Ivr: "market.ivr_own",
		Market: []marketSpec{{ID: "market.vix_pctile_3y", Method: "typo", Series: []string{"VIXCLS"}}}},
		rules: rules.New(), rule: map[string]compute.Rule{}}
	if err := j.findRules(); err == nil {
		t.Fatal("an unknown method must be an error")
	}
}

func TestDroppedRowsAreCounted(t *testing.T) {
	t.Parallel()
	f := world()
	delete(f.terms, 101) // a row without a current terms version
	_, _, n, err := ivInputCounted(context.Background(), testInputs(f), 1, dayD)
	if err != nil || n != 1 {
		t.Fatalf("no-terms count %d err %v, want 1", n, err)
	}
	f.latest = append(f.latest, store.OptionEodLatest{UnderlyingID: 3, SessionDay: "2026-11-09"})
	_, dropped, err := listSubjects(context.Background(), f.stores(), dayD)
	if err != nil || dropped != 1 {
		t.Fatalf("later-only count %d err %v, want 1", dropped, err)
	}
}

func TestNewWithEaseNilPlansLikeNew(t *testing.T) {
	t.Parallel()
	f := world()
	a := newJob(t, f, newFigs())
	b, err := NewWithEase(f.stores(), newFigs(), rules.New(), own.NewUnits(), own.NewSeriesOps(), nil,
		clockwork.NewFakeClockAt(closeD.Add(20*time.Minute)))
	if err != nil {
		t.Fatal(err)
	}
	ka, ha, _ := a.Plan(context.Background(), dayD)
	kb, hb, _ := b.Plan(context.Background(), dayD)
	if ha != hb || !reflect.DeepEqual(ka, kb) {
		t.Fatalf("plans differ: %v %s vs %v %s", ka, ha, kb, hb)
	}
}
