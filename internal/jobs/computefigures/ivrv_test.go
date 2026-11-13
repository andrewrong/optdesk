package computefigures

import (
	"context"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const ivRvName = "market.iv_rv20"

// ivRvPairs are the index names of the step and the FRED series of each.
var ivRvPairs = [][2]string{{"SPX", "VIXCLS"}, {"NDX", "VXNCLS"}, {"RUT", "RVXCLS"}}

// addRv20 stores a market.rv20 reading of day for name, with data_day as given ("" for none).
func (f *fx) addRv20(name string, day contracts.Day, value, dataDay string) {
	rec := contracts.ReadingRecord{Name: rvName, SubjectKind: "underlying", Subject: name, Day: dateOf(day),
		State: "ok", Value: sp(value), Rev: 1}
	if dataDay != "" {
		e := grow(&rec.Parameters)
		e.Param, e.ValueTag = "data_day", sp(dataDay)
	}
	f.readings = append(f.readings, rec)
}

// runIvRvKey runs the one key of name alone, the way the runner does.
func runIvRvKey(t *testing.T, f *fx, name string) contracts.JobResult {
	t.Helper()
	j := newJob(t, f, realFigs())
	return j.Run(context.Background(), dayD, []contracts.ExpectedKey{keyOf(ivRvName, kindUnderlying, name)})
}

func ivRvReading(t *testing.T, f *fx, name string) contracts.ReadingRecord {
	t.Helper()
	return reading(t, f, ivRvName, "underlying", name)
}

func wantRetryable(t *testing.T, res contracts.JobResult, contains ...string) {
	t.Helper()
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("Err %v, want Retryable", res.Err)
	}
	for _, c := range contains {
		if !strings.Contains(res.Err.Error(), c) {
			t.Fatalf("error %q lacks %q", res.Err, c)
		}
	}
}

func TestPlanHasIvRv20Keys(t *testing.T) {
	t.Parallel()
	keys := plan(t, newJob(t, world(), newFigs()))
	var got []string
	for _, k := range keys {
		if strings.HasPrefix(k.SubjectKey, ivRvName+"|") {
			got = append(got, k.SubjectKey)
		}
	}
	want := []string{"market.iv_rv20|underlying|NDX", "market.iv_rv20|underlying|RUT", "market.iv_rv20|underlying|SPX"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("iv_rv20 keys %v, want %v", got, want)
	}
}

func TestIvRv20KeyIsNotAnUnderlyingKey(t *testing.T) {
	t.Parallel()
	f := world()
	res := runIvRvKey(t, f, "SPX")
	if res.Err != nil && strings.Contains(res.Err.Error(), "option rows") {
		t.Fatalf("Err %v: the key was looked up among the option underlyings", res.Err)
	}
	if r := ivRvReading(t, f, "SPX"); r.Name != ivRvName {
		t.Fatalf("reading %+v", r)
	}
}

func TestIvRv20SameDay(t *testing.T) {
	t.Parallel()
	f := world()
	for _, p := range ivRvPairs {
		f.addPoint(p[1], dayD, 20.5)
		f.addRv20(p[0], dayD, "0.162060", string(dayD))
	}
	for _, p := range ivRvPairs {
		if res := runIvRvKey(t, f, p[0]); res.Err != nil {
			t.Fatalf("%s: %+v", p[0], res)
		}
		r := ivRvReading(t, f, p[0])
		if r.State != "ok" || r.Value == nil || *r.Value != "0.042940" || tagOf(r, "data_day") != string(dayD) {
			t.Fatalf("%s reading %+v, want ok 0.042940 of %s", p[0], r, dayD)
		}
	}
}

func TestIvRv20WaitsForFred(t *testing.T) {
	t.Parallel()
	f := world()
	prev := shiftDay(dayD, -1)
	f.addPoint("VIXCLS", prev, 20)
	f.addRv20("SPX", prev, "0.150000", string(prev))
	f.addRv20("SPX", dayD, "0.160000", string(dayD))
	res := runIvRvKey(t, f, "SPX")
	wantRetryable(t, res, string(dayD))
	r := ivRvReading(t, f, "SPX")
	if r.State != "ok" || *r.Value != "0.050000" || tagOf(r, "data_day") != string(prev) || r.Rev != 1 {
		t.Fatalf("reading %+v, want 0.050000 of %s from the rv20 of that day", r, prev)
	}
	f.addPoint("VIXCLS", dayD, 21)
	if res := runIvRvKey(t, f, "SPX"); res.Err != nil {
		t.Fatalf("second run %+v", res)
	}
	r = ivRvReading(t, f, "SPX")
	if r.Rev != 2 || *r.Value != "0.050000" || tagOf(r, "data_day") != string(dayD) {
		t.Fatalf("reading %+v, want rev 2 of %s", r, dayD)
	}
}

// The runner runs iv_rv20 before rv20 of the same day, so neither the rv20 nor the point of D exists yet.
func TestIvRv20Rv20OfDayAbsent(t *testing.T) {
	t.Parallel()
	f := world()
	prev := shiftDay(dayD, -1)
	f.addPoint("VIXCLS", prev, 20)
	f.addRv20("SPX", prev, "0.150000", string(prev))
	wantRetryable(t, runIvRvKey(t, f, "SPX"), string(dayD))
	if r := ivRvReading(t, f, "SPX"); r.State != "ok" || tagOf(r, "data_day") != string(prev) {
		t.Fatalf("reading %+v, want the figure of %s", r, prev)
	}
	// a point of D without an rv20 of D gives the same figure and the same wait
	f.addPoint("VIXCLS", dayD, 21)
	wantRetryable(t, runIvRvKey(t, f, "SPX"), string(dayD))
	if r := ivRvReading(t, f, "SPX"); r.State != "ok" || tagOf(r, "data_day") != string(prev) {
		t.Fatalf("reading %+v, want the figure of %s", r, prev)
	}
}

func TestIvRv20Rv20DataDayMustMatch(t *testing.T) {
	t.Parallel()
	f := world()
	f.addPoint("VIXCLS", dayD, 20)
	f.addRv20("SPX", dayD, "0.160000", string(shiftDay(dayD, -1)))
	runIvRvKey(t, f, "SPX")
	r := ivRvReading(t, f, "SPX")
	if r.State != "missing_input" || r.Value != nil || r.ReasonCode == nil || *r.ReasonCode != 1 {
		t.Fatalf("reading %+v, want missing_input code 1", r)
	}
}

func TestIvRv20NoValueDay(t *testing.T) {
	t.Parallel()
	f := world()
	prev := shiftDay(dayD, -1)
	f.addPoint("VIXCLS", prev, 20)
	f.addPoint("VIXCLS", dayD, 0)
	last := &f.series["VIXCLS"][1]
	last.Status, last.Value = "no_value", nil
	f.addRv20("SPX", prev, "0.150000", string(prev))
	f.addRv20("SPX", dayD, "0.160000", string(dayD))
	wantRetryable(t, runIvRvKey(t, f, "SPX"), string(dayD))
	if r := ivRvReading(t, f, "SPX"); r.State != "ok" || tagOf(r, "data_day") != string(prev) || *r.Value != "0.050000" {
		t.Fatalf("reading %+v, want the figure of %s: a no_value day has no point", r, prev)
	}
}

func TestIvRv20Rv20MissingIsMissing(t *testing.T) {
	t.Parallel()
	f := world()
	f.addPoint("VIXCLS", dayD, 20)
	f.addRv20("SPX", dayD, "0.160000", string(dayD))
	f.readings[0].State, f.readings[0].Value = "missing_input", nil
	runIvRvKey(t, f, "SPX")
	r := ivRvReading(t, f, "SPX")
	if r.State != "missing_input" || r.Value != nil || r.ReasonCode == nil || *r.ReasonCode != 1 {
		t.Fatalf("reading %+v, want missing_input code 1 and no value", r)
	}
}

func TestIvRv20StaleIsMissing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		back int
		code int // 0: a value
	}{{6, 3}, {5, 0}} {
		f := world()
		d := shiftDay(dayD, -tc.back)
		f.addPoint("VIXCLS", d, 20)
		f.addRv20("SPX", d, "0.150000", string(d))
		runIvRvKey(t, f, "SPX")
		r := ivRvReading(t, f, "SPX")
		switch {
		case tc.code == 0 && (r.State != "ok" || *r.Value != "0.050000"):
			t.Fatalf("%d days: %+v, want a value", tc.back, r)
		case tc.code != 0 && (r.State != "missing_input" || r.ReasonCode == nil || *r.ReasonCode != tc.code):
			t.Fatalf("%d days: %+v, want code %d", tc.back, r, tc.code)
		}
	}
}

func TestIvRv20RerunIsNoOp(t *testing.T) {
	t.Parallel()
	f := world()
	f.addPoint("VIXCLS", dayD, 20)
	f.addRv20("SPX", dayD, "0.160000", string(dayD))
	runIvRvKey(t, f, "SPX")
	before := f.inserts
	if res := runIvRvKey(t, f, "SPX"); res.Err != nil || f.inserts != before {
		t.Fatalf("rerun inserted %d rows, result %+v", f.inserts-before, res)
	}
}

// addIvRv20Inputs stores, for the three iv_rv20 names, a FRED point of dayD (unless the series has one) and an
// ok market.rv20 reading of dayD with data_day dayD. Whole-job tests call it so the iv_rv20 keys find their
// session day and end without the Retryable error of Decision 10; the readings carry no origin, so a test
// that counts the rv20 readings of the job counts those with the origin compute_figures.
func addIvRv20Inputs(f *fx) {
	for _, p := range ivRvPairs {
		have := false
		for _, pt := range f.series[p[1]] {
			have = have || dayOf(pt.ObservationDate) == dayD
		}
		if !have {
			f.addPoint(p[1], dayD, 20)
		}
		f.addRv20(p[0], dayD, "0.100000", string(dayD))
	}
}
