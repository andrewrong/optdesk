package option_notices_page

import (
	"reflect"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/atombender/go-jsonschema/pkg/types"
)

func guardRow(alias, status, gates string) store.Candidate {
	return store.Candidate{Kind: kindHeldCallGuard, Alias: alias, Issuer: "ACME", Expiry: "2026-12-18",
		Strike: "100.00", Status: status, Gates: gates}
}

func TestGuardRowsOnlyBlockedKind(t *testing.T) {
	t.Parallel()
	cands := &fakeCandidates{rows: map[string][]store.Candidate{"2026-11-09/held_call_guard": {
		guardRow("main", "blocked", `[{"gate":"events","status":"block","reason":"earnings_before_expiry"},`+
			`{"gate":"x","status":"unknown","reason":"no_rate"},{"gate":"y","status":"pass"}]`),
		guardRow("other", "blocked", `[{"gate":"events","status":"block","reason":"strike_floor"}]`),
		guardRow("main", "ok", `[]`),
		guardRow("main", "blocked", `not json`),
	}}}
	a := fakeAccount{positions: map[string][]store.Position{"main/2026-11-09": {stock("ACME")}}}
	rule := &fakeRule{}
	c := testConfig(a, rule)
	c.Candidates = cands
	buildPage(t, c)
	if !reflect.DeepEqual(cands.calls, []string{"2026-11-09/held_call_guard"}) {
		t.Errorf("calls = %v", cands.calls)
	}
	want := []compute.NoticeGuard{
		{Symbol: "ACME", Expiry: "2026-12-18", Strike: "100.00", Status: "blocked", Reasons: []string{"earnings_before_expiry"}},
		{Symbol: "ACME", Expiry: "2026-12-18", Strike: "100.00", Status: "ok", Reasons: []string{}},
		{Symbol: "ACME", Expiry: "2026-12-18", Strike: "100.00", Status: "blocked", Reasons: []string{}},
	}
	if !reflect.DeepEqual(rule.inputs[0].Guards, want) {
		t.Errorf("guards = %+v", rule.inputs[0].Guards)
	}
}

func TestNoGuardRowsNote(t *testing.T) {
	t.Parallel()
	a := fakeAccount{positions: map[string][]store.Position{"main/2026-11-09": {stock("ACME")}}}
	p := buildPage(t, testConfig(a, &fakeRule{}))
	if p.Accounts[0].Note == nil || *p.Accounts[0].Note != noteNoGuard {
		t.Errorf("note = %v", p.Accounts[0].Note)
	}
}

func TestInputIsPlainValues(t *testing.T) {
	t.Parallel()
	opt := shortPut("ACME", "2026-11-13")
	a := fakeAccount{
		positions: map[string][]store.Position{
			"main/2026-11-09": {opt, stock("ACME")},
			"main/2026-11-06": {opt},
		},
		values: map[string][]store.AccountValue{"main/2026-11-09": {{Tag: "Cushion", Value: "0.12", Currency: "USD"}}},
	}
	rd := types.SerializableDate{Time: time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)}
	rule := &fakeRule{}
	c := testConfig(a, rule)
	c.Earnings = fakeEarnings{rows: map[string][]contracts.EarningsDate{"ACME": {
		{Status: "listed", ReportDate: &types.SerializableDate{Time: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)}},
		{Status: "listed", ReportDate: &rd},
		{Status: "absent"},
	}}}
	buildPage(t, c)
	next := contracts.Day("2026-11-11")
	wantPos := []compute.NoticePosition{
		{Symbol: "ACME", SecType: "OPT", Expiry: "2026-11-13", Strike: "95", Right: "P", Multiplier: "100", Quantity: "-1", Currency: "USD"},
	}
	got := rule.inputs[0]
	if got.Day != buildDay || got.Alias != "main" || got.CaptureDay != "2026-11-09" || got.PrevDay != "2026-11-06" {
		t.Errorf("input = %+v", got)
	}
	if !reflect.DeepEqual(got.PrevPositions, wantPos) || len(got.Positions) != 2 || got.Positions[0] != wantPos[0] {
		t.Errorf("positions = %+v prev = %+v", got.Positions, got.PrevPositions)
	}
	if !reflect.DeepEqual(got.Earnings, []compute.NoticeEarnings{{Symbol: "ACME", Next: &next}}) {
		t.Errorf("earnings = %+v", got.Earnings)
	}
	if !reflect.DeepEqual(got.Values, []compute.NoticeValue{{Tag: "Cushion", Value: "0.12", Currency: "USD"}}) {
		t.Errorf("values = %+v", got.Values)
	}
	if rule.params[0] != c.Params {
		t.Errorf("params = %+v", rule.params[0])
	}
}
