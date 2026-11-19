package main

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/compute/optioncandapi"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/candidatecur"
	"github.com/andrewrong/optdesk/internal/store/openquote"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// jpWriter is a candidate writer that stores the currency; the readers of the job are never called by a build.
type jpWriter struct{ store.CandidateWriter }

func (jpWriter) InsertCandidatesIn(context.Context, []candidatecur.Row) (int, error) { return 0, nil }

type stubOpen struct{ openquote.OptionOpenReader }
type stubEod struct{ store.OptionEodReader }
type stubTerms struct{ store.OptionTermsReader }
type stubNames struct{ store.InstrumentNameReader }
type stubDays struct{ store.SessionDayReader }
type stubCurrencyRule struct{ optioncandapi.CurrencyRule }

func oneJPRow() underlying.Row {
	return listRowsOf(jobs.WatchEntry{Issuer: "edinet:E02144", Market: "JP",
		Contract: broker.Contract{SecType: "STK", Symbol: "7203", ConID: 1}})[0]
}

// jpHarness is a factory with every store the JP job reads but the open store and the writer.
func jpHarness(t *testing.T) *openHarness {
	t.Helper()
	h := newOpenHarness(t, nil, true, nil)
	f := h.jc.f.withList(oneJPRow())
	f.compute.OptionEod, f.compute.Terms, f.compute.Names, f.compute.SessionDays = stubEod{}, stubTerms{}, stubNames{}, stubDays{}
	h.jc.f = &f
	return h
}

func TestJPFamilyRegistered(t *testing.T) {
	t.Parallel()
	fam, ok := jobFamilies[jobOptionCandidatesJP]
	if !ok {
		t.Fatal("option_candidates_jp is not registered")
	}
	if fam.order != 183 || jobFamilies[jobOptionOpenJP].order >= fam.order {
		t.Fatalf("order %d, option_open_jp %d", fam.order, jobFamilies[jobOptionOpenJP].order)
	}
}

// TestJPFamilyScheduled: the hold is gone, so the scheduler lists the family (startDisabled names no reason).
func TestJPFamilyScheduled(t *testing.T) {
	t.Parallel()
	if fam := jobFamilies[jobOptionCandidatesJP]; fam.startOff != nil {
		t.Fatalf("startOff set: %q", fam.startOff(runOnceConfig{openDSN: "x"}, true))
	}
	for _, open := range []bool{true, false} {
		if got, held := startDisabled(runOnceConfig{openDSN: "x"}, open)[jobOptionCandidatesJP]; held {
			t.Fatalf("scheduler disabled reason %q", got)
		}
	}
}

func TestJPLeftOutWithoutOpenStore(t *testing.T) {
	t.Parallel()
	h := jpHarness(t)
	h.jc.f.candidates = jpWriter{}
	h.jc.f.d.newCurrencyRule = func() optioncandapi.CurrencyRule { return stubCurrencyRule{} }
	j, reason, err := jobFamilies[jobOptionCandidatesJP].build(t.Context(), h.a, h.jc)
	if j != nil || reason != "" || err != nil {
		t.Fatalf("job %v reason %q err %v", j, reason, err)
	}
	if n := strings.Count(h.log.String(), "option_candidates_jp not built"); n != 1 || !strings.Contains(h.log.String(), openquote.DSNEnvOptionOpen) {
		t.Fatalf("log %q", h.log.String())
	}
}

func TestJPBuiltOverTheStoresWithTheRuleOfDeps(t *testing.T) {
	t.Parallel()
	h := jpHarness(t)
	h.jc.f.candOpen = stubOpen{}
	h.jc.f.candidates = jpWriter{}
	var rules atomic.Int32
	h.jc.f.d.newCurrencyRule = func() optioncandapi.CurrencyRule { rules.Add(1); return stubCurrencyRule{} }
	j, reason, err := jobFamilies[jobOptionCandidatesJP].build(t.Context(), h.a, h.jc)
	if err != nil || reason != "" || j == nil || j.ID() != jobOptionCandidatesJP {
		t.Fatalf("job %v reason %q err %v", j, reason, err)
	}
	if rules.Load() == 0 {
		t.Fatal("the job was not built with the rule of deps.newCurrencyRule")
	}
}

func TestJPLeftOutWithoutCurrencyWriter(t *testing.T) {
	t.Parallel()
	h := jpHarness(t)
	h.jc.f.candOpen = stubOpen{}
	h.jc.f.candidates = noCandidateWriter{}
	h.jc.f.d.newCurrencyRule = func() optioncandapi.CurrencyRule { return stubCurrencyRule{} }
	if j, _, err := jobFamilies[jobOptionCandidatesJP].build(t.Context(), h.a, h.jc); j != nil || err != nil {
		t.Fatalf("job %v err %v", j, err)
	}
}

func TestJPLeftOutWithoutRuleLogsOneLine(t *testing.T) {
	t.Parallel()
	h := jpHarness(t)
	h.jc.f.candOpen = stubOpen{}
	h.jc.f.candidates = jpWriter{}
	h.jc.f.d.newCurrencyRule = nil
	if j, _, err := jobFamilies[jobOptionCandidatesJP].build(t.Context(), h.a, h.jc); j != nil || err != nil {
		t.Fatalf("job %v err %v", j, err)
	}
	if n := strings.Count(h.log.String(), "option_candidates_jp not built"); n != 1 || !strings.Contains(h.log.String(), reasonJPNoRule) {
		t.Fatalf("log %q", h.log.String())
	}
}
