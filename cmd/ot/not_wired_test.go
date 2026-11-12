package main

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/runview"
)

func TestNotWiredWritesKeyNameOnly(t *testing.T) {
	t.Parallel()
	h := newOpenHarness(t, nil, true, nil)
	h.jc.cfg.openDSN = ""
	h.jc.notWired = newNotWiredRecorder(h.reasons, h.jc.f.clk)
	for _, id := range []string{jobOptionOpenPlan, jobOptionListZone} {
		if j, _, err := jobFamilies[id].build(t.Context(), h.a, h.jc); j != nil || err != nil {
			t.Fatalf("%s: job %v err %v", id, j, err)
		}
	}
	// A line with a value (a DSN) is never in the row.
	h.jc.cfg.openDSN = "postgres://secret-user:secret-pass@host/db"
	h.jc.f.inst = nil
	if j, _, err := jobFamilies[jobOptionOpenPlanJP].build(t.Context(), h.a, h.jc); j != nil || err != nil {
		t.Fatalf("job %v err %v", j, err)
	}
	if err := h.jc.notWired.flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(h.reasons.calls) != 1 {
		t.Fatalf("%d writes, want one", len(h.reasons.calls))
	}
	c := h.reasons.calls[0]
	want := map[string]string{jobOptionOpenPlan: "OT_PG_DSN_OPTION_OPEN", jobOptionListZone: "OT_PG_DSN_OPTION_OPEN", jobOptionOpenPlanJP: "OT_PG_DSN_MASTER"}
	if len(c.rows) != len(want) {
		t.Fatalf("rows %+v", c.rows)
	}
	for _, r := range c.rows {
		if r.Detail != want[r.Scope] || r.Code != "not_wired" || strings.Contains(r.Detail, "secret") || strings.Contains(r.Detail, " ") {
			t.Errorf("row %+v", r)
		}
	}
}

func TestNotWiredFixedDayClearedByNextStart(t *testing.T) {
	t.Parallel()
	h := newOpenHarness(t, nil, true, nil)
	first := newNotWiredRecorder(h.reasons, h.jc.f.clk)
	first.record(jobOptionOpenPlan, "OT_PG_DSN_OPTION_OPEN")
	second := newNotWiredRecorder(h.reasons, h.jc.f.clk) // the next start: every job is wired
	for i, n := range []*notWiredRecorder{first, second} {
		if err := n.flush(t.Context()); err != nil {
			t.Fatalf("start %d: %v", i, err)
		}
	}
	if len(h.reasons.calls) != 2 {
		t.Fatalf("%d writes", len(h.reasons.calls))
	}
	for i, c := range h.reasons.calls {
		if c.day != runview.NotWiredDay || c.dataset != "ops.not_wired" || c.subject != "scheduler" {
			t.Errorf("start %d: key %v %q %q", i, c.day, c.dataset, c.subject)
		}
	}
	if got := h.reasons.calls[0].rows; len(got) != 1 || got[0].Scope != jobOptionOpenPlan {
		t.Errorf("first start rows %+v", got)
	}
	if got := h.reasons.calls[1].rows; got == nil || len(got) != 0 {
		t.Errorf("second start must replace the set with an empty one, got %#v", got)
	}
}

func TestNotWiredOnlyFromScheduler(t *testing.T) {
	t.Parallel()
	for _, scheduler := range []bool{false, true} {
		h := newOpenHarness(t, nil, true, nil)
		cfg := runOnceConfig{scheduler: scheduler}
		jc := newJobCtx(cfg, h.jc.f, h.jc.brk, true, nil, &closerSet{}, h.reasons)
		if jc.scheduler != scheduler || (jc.notWired != nil) != scheduler || jc.reasons != runview.ReasonWriter(h.reasons) {
			t.Fatalf("scheduler %v: ctx %+v", scheduler, jc)
		}
		if j, _, err := jobFamilies[jobOptionOpen].build(t.Context(), h.a, jc); j != nil || err != nil {
			t.Fatalf("job %v err %v", j, err)
		}
		if err := jc.notWired.flush(t.Context()); err != nil {
			t.Fatal(err)
		}
		if got := len(h.reasons.calls); got != map[bool]int{false: 0, true: 1}[scheduler] {
			t.Errorf("scheduler %v: %d writes", scheduler, got)
		}
	}
}
