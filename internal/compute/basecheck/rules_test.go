package basecheck

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
)

func TestEveryRuleHasThresholds(t *testing.T) {
	t.Parallel()
	for _, r := range basecheckapi.Rules {
		id := r.Row + "." + r.Rule
		th, ok := Thresholds[id]
		switch r.Source {
		case "sql", "ledger", "derived":
			if !ok {
				t.Errorf("rule %s (%s) has no threshold entry", id, r.Source)
				continue
			}
			if r.Rule == "age" && th.Warn.Kind == None {
				t.Errorf("age rule %s has no limit", id)
			}
		}
	}
}

func TestNoteRuleHasNoThreshold(t *testing.T) {
	t.Parallel()
	for _, r := range basecheckapi.Rules {
		if r.Source != "note" {
			continue
		}
		if _, ok := Thresholds[r.Row+"."+r.Rule]; ok {
			t.Errorf("note rule %s.%s must have no threshold", r.Row, r.Rule)
		}
	}
}

func TestBarJumpHasNoFailThreshold(t *testing.T) {
	t.Parallel()
	th, ok := Thresholds["bars.bar_jump"]
	if !ok {
		t.Fatal("bars.bar_jump has no entry")
	}
	if th.Fail.Kind != None {
		t.Errorf("bars.bar_jump fail = %v, want none", th.Fail)
	}
	if th.Warn.Kind != Any {
		t.Errorf("bars.bar_jump warn = %v, want any", th.Warn)
	}
}

func TestEveryThresholdHasSource(t *testing.T) {
	t.Parallel()
	known := map[string]bool{}
	for _, r := range basecheckapi.Rules {
		known[r.Row+"."+r.Rule] = true
	}
	for id, th := range Thresholds {
		if !known[id] {
			t.Errorf("threshold %s names no rule of basecheckapi.Rules", id)
		}
		if th.Source != "def" && th.Source != "trial" {
			t.Errorf("threshold %s source = %q, want def or trial", id, th.Source)
		}
	}
}

func TestCalendarlessAgeUsesDays(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"rates.age", "calendar.age"} {
		th := Thresholds[id]
		if th.Warn.Kind != Days || th.Fail.Kind != Days {
			t.Errorf("%s = %+v, want calendar days (Decision 6: no subject calendar)", id, th)
		}
	}
}
