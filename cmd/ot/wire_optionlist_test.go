package main

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

const listDay = "2026-11-09"

// listInst resolves every instrument to id 7.
type listInst struct{ store.InstrumentStore }

func (listInst) ResolveInstrument(context.Context, store.Identifier, store.InstrumentSpec, []store.Identifier) (int64, error) {
	return 7, nil
}

// runsStore holds run rows only.
type runsStore struct {
	openquote.OptionListStore
	runs []openquote.ListRunRecord
}

func (s runsStore) ListRuns(context.Context, contracts.Day) ([]openquote.ListRunRecord, error) {
	return s.runs, nil
}

// withLists is an option open store that also holds lists.
type withLists struct {
	openquote.Stores
	runsStore
}

// listLister is a broker that lists contracts.
type listLister struct{ closeBroker }

func (listLister) ListContracts(context.Context, broker.Contract, contracts.Day, string, string) ([]broker.Contract, error) {
	return nil, nil
}

func (listLister) ListChains(context.Context, broker.Contract) ([]broker.Chain, error) {
	return nil, nil
}

func listHarness(t *testing.T, env map[string]string, up, lister bool) *openHarness {
	t.Helper()
	h := newOpenHarness(t, env, up, nil)
	h.jc.f.inst = listInst{}
	if lister {
		h.jc.brk = listLister{closeBroker{mu: &h.mu, closed: &h.closed}}
	}
	return h
}

func TestOptionListJobRegistered(t *testing.T) {
	t.Parallel()
	ids := familyIDs()
	i, g := slices.Index(ids, jobOptionListPreopen), slices.Index(ids, jobOptionOpenResolveGap)
	if i < 0 || g < 0 || i >= g {
		t.Fatalf("option_list_preopen at %d, gap at %d: %v", i, g, ids)
	}
	if jobOptionListPreopen != "option_list_preopen" || jobFamilies[jobOptionListPreopen].order != 185 {
		t.Fatalf("family %+v", jobFamilies[jobOptionListPreopen])
	}
}

func TestOptionListListedJobAnswersWindowBoundAndPostcondition(t *testing.T) {
	t.Parallel()
	h := listHarness(t, deadEnv, true, true)
	for name, tc := range map[string]struct {
		runs []openquote.ListRunRecord
		want bool
	}{
		"listed":   {[]openquote.ListRunRecord{{UnderlyingID: 7, Session: listDay, Listed: 3}}, true},
		"none":     {[]openquote.ListRunRecord{{UnderlyingID: 7, Session: listDay, Listed: 0}}, false},
		"no names": {nil, false},
	} {
		st := withLists{runsStore: runsStore{runs: tc.runs}}
		j, reason, err := optionListJob(h.a, h.jc, st, "")
		if err != nil || reason != "" {
			t.Fatalf("%s: reason %q err %v", name, reason, err)
		}
		lj, ok := listedOf(j)
		if !ok || lj == nil {
			t.Fatalf("%s: no listedJob in %T", name, j)
		}
		cal, open, closeOff := j.(interface {
			Window() (string, time.Duration, time.Duration)
		}).Window()
		if cal != "XNYS" || open != 0 || closeOff != 0 {
			t.Fatalf("%s: window %q %v %v", name, cal, open, closeOff)
		}
		if got := j.(interface{ RunBound() time.Duration }).RunBound(); got != 47*time.Minute {
			t.Fatalf("%s: run bound %v", name, got)
		}
		if _, _, err := lj.Plan(t.Context(), listDay); err != nil {
			t.Fatal(err)
		}
		got, err := j.(interface {
			Postcondition(ctx context.Context, day, datasetID, subjectKey string) (bool, error)
		}).Postcondition(t.Context(), listDay, "master.option_list", "preopen")
		if err != nil || got != tc.want {
			t.Fatalf("%s: postcondition %v err %v, want %v", name, got, err, tc.want)
		}
	}
}

// assertListHeld checks the held job: the reason, one startup line naming the job and the reason, disabled by the
// scheduler and left out by run-once.
func assertListHeld(t *testing.T, h *openHarness, build func() (contracts.Job, string, error), want string) {
	t.Helper()
	j, reason, err := build()
	if err != nil || j == nil || reason != want {
		t.Fatalf("job %v reason %q err %v", j, reason, err)
	}
	if n := strings.Count(h.log.String(), want); n != 1 || !strings.Contains(h.log.String(), "job="+jobOptionListPreopen+" ") {
		t.Fatalf("%d lines: %s", n, h.log.String())
	}
	w := wiring{brokerUp: h.jc.brokerUp, off: map[string]string{jobOptionListPreopen: reason},
		held: map[string]contracts.Job{jobOptionListPreopen: j}, jobs: []contracts.Job{j}}
	if dis := schedulerDisabled(runOnceConfig{}, w); dis[jobOptionListPreopen] != want {
		t.Fatalf("disabled %v", dis)
	}
	if got := w.runnable(slog.New(slog.DiscardHandler)); len(got) != 0 {
		t.Fatalf("run-once keeps %v", got)
	}
}

func TestOptionListBrokerWithoutContractListerIsHeld(t *testing.T) {
	t.Parallel()
	h := listHarness(t, deadEnv, true, false)
	assertListHeld(t, h, func() (contracts.Job, string, error) {
		return optionListJob(h.a, h.jc, withLists{}, "")
	}, reasonListAbsent)
}

func TestOptionListStoreWithoutOptionListStoreIsHeld(t *testing.T) {
	t.Parallel()
	h := listHarness(t, deadEnv, true, true)
	assertListHeld(t, h, func() (contracts.Job, string, error) {
		return optionListJob(h.a, h.jc, struct{ openquote.Stores }{}, "")
	}, reasonListAbsent)
}

func TestOptionListAbsentStandInsAnswerPermanent(t *testing.T) {
	t.Parallel()
	_, err := absentLister{}.ListContracts(t.Context(), broker.Contract{}, "", "", "")
	te, ok := err.(contracts.TypedError)
	if !ok || te.Kind() != contracts.Permanent || err.Error() != reasonListAbsent {
		t.Fatalf("err %v", err)
	}
}

func TestOptionListMissingDSNLeavesJobOutWithOneLine(t *testing.T) {
	t.Parallel()
	h := listHarness(t, nil, true, true)
	j, reason, err := jobFamilies[jobOptionListPreopen].build(t.Context(), h.a, h.jc)
	if j != nil || reason != "" || err != nil {
		t.Fatalf("job %v reason %q err %v", j, reason, err)
	}
	log := h.log.String()
	if strings.Count(log, "job="+jobOptionListPreopen+" ") != 1 || !strings.Contains(log, "OT_PG_DSN_OPTION_OPEN is not set") {
		t.Fatalf("log: %s", log)
	}
}

func TestOptionListDeadSocketAtStartListsDisabled(t *testing.T) {
	t.Parallel()
	cfg := runOnceConfig{openDSN: deadEnv["OT_PG_DSN_OPTION_OPEN"]}
	w := wiring{brokerUp: false, off: startDisabled(cfg, false)}
	if dis := schedulerDisabled(cfg, w); dis[jobOptionListPreopen] != reasonBrokerDown {
		t.Fatalf("disabled %v", dis)
	}
	if off := startDisabled(cfg, true); off[jobOptionListPreopen] != "" {
		t.Fatalf("socket up but off: %v", off)
	}
	if off := startDisabled(runOnceConfig{}, false); off[jobOptionListPreopen] != "" {
		t.Fatalf("DSN unset but listed: %v", off)
	}
}
