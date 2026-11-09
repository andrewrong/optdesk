package contracts_test

import "testing"

const optionCandidatesPageV1 = "page.option_candidates_page.v1.json"

func candidatesMinimal(t *testing.T) map[string]any {
	t.Helper()
	return pageV1Minimal(t, optionCandidatesPageV1, map[string]string{"^cand-[A-Za-z0-9_-]{1,64}$": "cand-1"})
}

func candidatesGroup(inst map[string]any) map[string]any {
	return inst["groups"].([]any)[0].(map[string]any)
}

func candidatesTable() map[string]any {
	return map[string]any{
		"columns": []any{map[string]any{"id": "strike", "label": "Strike", "align": "right", "hint": nil}},
		"rows": []any{map[string]any{
			"id":      "cand-1",
			"cells":   map[string]any{"strike": map[string]any{"text": "95", "value": 95.0, "badges": []any{}, "tone": "none"}},
			"reasons": []any{},
		}},
	}
}

func TestOptionCandidatesPageMinimalValid(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, optionCandidatesPageV1, candidatesMinimal(t)) {
		t.Fatal("minimal instance is invalid")
	}
}

func TestOptionCandidatesPageVersionAndUnknownFieldFail(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["schema_version"] = 2
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("version 2 validates")
	}
	inst = candidatesMinimal(t)
	inst["extra"] = "x"
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("an unknown field validates")
	}
}

func TestOptionCandidatesPageGroupMainAndGap(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	g := candidatesGroup(inst)
	g["main"] = candidatesTable()
	g["gap"] = nil
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("group with a main table and no gap is invalid")
	}
	g["gap"] = pageV1Gap()
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("group with both main and gap validates")
	}
	g["main"], g["gap"] = nil, nil
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("group with neither a table nor a gap validates")
	}
}

func TestOptionCandidatesPageReasonGate(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	g := candidatesGroup(inst)
	table := candidatesTable()
	reason := map[string]any{"gate": "spread", "status": "block", "code": "too_wide", "text": "spread too wide"}
	table["rows"].([]any)[0].(map[string]any)["reasons"] = []any{reason}
	g["other"], g["gap"] = table, nil
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("Reason with a lower case gate is invalid")
	}
	reason["gate"] = "Spread"
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("Reason with an uppercase gate validates")
	}
}

func TestOptionCandidatesPageNoRowsGapValid(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	g := candidatesGroup(inst)
	g["main"], g["other"] = nil, nil
	g["gap"] = map[string]any{"kind": "gap", "reason": "no_rows", "text": "7 天内无行"}
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("group gap no_rows (design Decision 3) is rejected")
	}
}

func TestOptionCandidatesPageNoRunAndUnreadableGapsValid(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["gaps"] = []any{map[string]any{"kind": "gap", "reason": "no_run", "text": "尚无运行"}}
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("page gap no_run is rejected")
	}
	g := candidatesGroup(inst)
	table := candidatesTable()
	table["rows"].([]any)[0].(map[string]any)["cells"] = map[string]any{
		"strike": map[string]any{"kind": "gap", "reason": "unreadable", "text": "无法读取"},
	}
	g["main"], g["gap"] = table, nil
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("gap cell unreadable is rejected")
	}
	inst["gaps"] = []any{map[string]any{"kind": "gap", "reason": "bogus", "text": "x"}}
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("an unknown gap reason validates")
	}
}
