package computefigures

import (
	"context"
	"fmt"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	ibkrName = "market.iv_ibkr"
	ivpIbkr  = "market.ivp_ibkr"
	ivpOwn   = "market.ivp_own"
)

// addIv stores `prior` ok readings of series name on the days before dayD (prior day k holds 0.100 + k/1000)
// and a reading of today.
func addIv(f *fx, name string, prior int, today string) {
	for k := 1; k <= prior; k++ {
		v := fmt.Sprintf("%.6f", 0.1+float64(k)/1000)
		f.readings = append(f.readings, contracts.ReadingRecord{Name: name, SubjectKind: "underlying", Subject: "SPY",
			Day: dateOf(shiftDay(dayD, -k)), State: "ok", Value: sp(v), Rev: 1})
	}
	if today != "" {
		f.readings = append(f.readings, contracts.ReadingRecord{Name: name, SubjectKind: "underlying", Subject: "SPY",
			Day: dateOf(dayD), State: "ok", Value: sp(today), Rev: 1})
	}
}

// runKeys runs only the IVP keys of SPY, so the IV30 step does not rewrite the stored series.
func runKeys(t *testing.T, j contracts.Job, names ...string) {
	t.Helper()
	var keys []contracts.ExpectedKey
	for _, n := range names {
		keys = append(keys, contracts.ExpectedKey{DatasetID: "readings.reading", SubjectKey: n + "|underlying|SPY"})
	}
	if res := j.Run(context.Background(), dayD, keys); res.Err != nil {
		t.Fatalf("run: %v", res.Err)
	}
}

func TestIvpIbkrOkFrom253Bars(t *testing.T) {
	t.Parallel()
	f := world()
	addIv(f, ibkrName, 252, "0.2255") // 125 of the 252 prior values (0.101 .. 0.225) are at or below 0.2255
	runKeys(t, newJob(t, f, realFigs()), ivpIbkr)
	r := reading(t, f, ivpIbkr, "underlying", "SPY")
	if r.State != "ok" || r.Value == nil || *r.Value != "0.496032" {
		t.Fatalf("ivp_ibkr %+v, want ok 0.496032 (125/252)", r)
	}
}

func TestIvpIbkrShortHistoryUnder252(t *testing.T) {
	t.Parallel()
	f := world()
	addIv(f, ibkrName, 251, "0.2255")
	runKeys(t, newJob(t, f, realFigs()), ivpIbkr)
	r := reading(t, f, ivpIbkr, "underlying", "SPY")
	if r.State != "missing_input" || r.Value != nil || r.ReasonCode == nil || *r.ReasonCode != 2 {
		t.Fatalf("ivp_ibkr %+v, want missing short_history (2)", r)
	}
}

func TestIvpIbkrNoBarOfDay(t *testing.T) {
	t.Parallel()
	f := world()
	addIv(f, ibkrName, 252, "")
	runKeys(t, newJob(t, f, realFigs()), ivpIbkr)
	r := reading(t, f, ivpIbkr, "underlying", "SPY")
	if r.State != "missing_input" || r.ReasonCode == nil || *r.ReasonCode != 1 {
		t.Fatalf("ivp_ibkr %+v, want missing no_input (1)", r)
	}
}

func TestIvpIbkrReadsOnlyIbkrSeries(t *testing.T) {
	t.Parallel()
	f := world()
	addIv(f, "market.iv30_own", 252, "0.2255") // a full own series, no IBKR series at all
	runKeys(t, newJob(t, f, realFigs()), ivpIbkr, ivpOwn)
	r := reading(t, f, ivpIbkr, "underlying", "SPY")
	if r.State != "missing_input" || r.ReasonCode == nil || *r.ReasonCode != 1 {
		t.Fatalf("ivp_ibkr %+v, want missing no_input: the own series must not be read", r)
	}
	if own := reading(t, f, ivpOwn, "underlying", "SPY"); own.State != "ok" {
		t.Fatalf("ivp_own %+v, want ok", own)
	}
}

func TestIvpOwnIgnoresIbkrSeries(t *testing.T) {
	t.Parallel()
	run := func(withIbkr bool) contracts.ReadingRecord {
		f := world()
		addIv(f, "market.iv30_own", 252, "0.2255")
		if withIbkr {
			for k := 1; k <= 252; k++ { // a different series, so a read of it would change the figure
				f.readings = append(f.readings, contracts.ReadingRecord{Name: ibkrName, SubjectKind: "underlying", Subject: "SPY",
					Day: dateOf(shiftDay(dayD, -k)), State: "ok", Value: sp("0.900000"), Rev: 1})
			}
			f.readings = append(f.readings, contracts.ReadingRecord{Name: ibkrName, SubjectKind: "underlying", Subject: "SPY",
				Day: dateOf(dayD), State: "ok", Value: sp("0.100000"), Rev: 1})
		}
		runKeys(t, newJob(t, f, realFigs()), ivpOwn)
		return reading(t, f, ivpOwn, "underlying", "SPY")
	}
	without, with := run(false), run(true)
	if without.Value == nil || *without.Value != "0.496032" {
		t.Fatalf("ivp_own without IBKR %+v, want 0.496032", without)
	}
	if with.State != without.State || with.Value == nil || *with.Value != *without.Value || with.RowsSha256 != without.RowsSha256 {
		t.Fatalf("ivp_own with IBKR series %+v differs from %+v", with, without)
	}
}

func TestIvpIbkrKeyPlanned(t *testing.T) {
	t.Parallel()
	n := 0
	for _, k := range plan(t, newJob(t, world(), realFigs())) {
		if k.DatasetID == "readings.reading" && k.SubjectKey == ivpIbkr+"|underlying|SPY" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d keys of market.ivp_ibkr for SPY, want 1", n)
	}
}

func TestIvrNotPlannedForIbkr(t *testing.T) {
	t.Parallel()
	j := newJob(t, world(), realFigs())
	for _, k := range plan(t, j) {
		if k.SubjectKey == "market.ivr_ibkr|underlying|SPY" {
			t.Fatalf("planned %s", k.SubjectKey)
		}
	}
}
