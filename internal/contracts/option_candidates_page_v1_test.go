package contracts_test

import (
	"strings"
	"testing"
)

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
	g["gap"] = map[string]any{"kind": "gap", "reason": "no_rows", "text": "no rows in 7 days"}
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("group gap no_rows (design Decision 3) is rejected")
	}
}

func TestOptionCandidatesPageNoRunAndUnreadableGapsValid(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["gaps"] = []any{map[string]any{"kind": "gap", "reason": "no_run", "text": "no run yet"}}
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("page gap no_run is rejected")
	}
	g := candidatesGroup(inst)
	table := candidatesTable()
	table["rows"].([]any)[0].(map[string]any)["cells"] = map[string]any{
		"strike": map[string]any{"kind": "gap", "reason": "unreadable", "text": "unreadable"},
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

func tiersBadge() map[string]any {
	return map[string]any{"code": "thin_premium", "label": "thin", "gloss": "premium is thin", "tone": "warn"}
}

func tiersRow() map[string]any {
	return map[string]any{
		"id":      "row-1",
		"company": "AAA",
		"contract": map[string]any{
			"name": "AAA 95 put", "strike": "95", "expiry": "2026-12-18", "dte": "30",
		},
		"margin":     map[string]any{"main": "5.2%", "sub": "to buy price", "gap": nil},
		"below_spot": map[string]any{"main": "8.0%", "sub": nil, "gap": nil},
		"premium":    map[string]any{"main": "1.20", "sub": nil, "gap": nil},
		"badges":     []any{tiersBadge()},
		"notes":      []any{},
		"fields": []any{map[string]any{
			"id": "yield", "label": "Yield", "hint": nil, "value": nil,
			"gap":    map[string]any{"kind": "gap", "reason": "missing_input", "text": "no spot"},
			"badges": []any{},
		}},
		"line": nil,
	}
}

func tiersView() map[string]any {
	return map[string]any{
		"chain_day": "2026-11-18",
		"companies": []any{map[string]any{
			"key": "1", "ticker": "AAA", "spot": "100", "spot_day": "2026-11-18",
			"buy_price": nil, "buy_price_note": nil, "next_earnings": "2026-12-01", "counts": "1 fits",
		}},
		"common":  []any{map[string]any{"code": "delayed", "label": "delayed", "gloss": "quotes are delayed"}},
		"kept":    nil,
		"legend":  []any{map[string]any{"tier": "fits", "title": "Fits", "text": "delta at most 0.3"}},
		"columns": []any{map[string]any{"id": "margin", "label": "Margin", "hint": nil}},
		"tiers": []any{map[string]any{
			"id": "fits", "word": "fits", "icon": "check", "title": "Fits", "explain": "rows that fit",
			"collapsed": false, "rows": []any{tiersRow()},
		}},
	}
}

func tiersFirstRow(inst map[string]any) map[string]any {
	tiers := inst["short_put_tiers"].(map[string]any)["tiers"].([]any)
	return tiers[0].(map[string]any)["rows"].([]any)[0].(map[string]any)
}

func TestOptionCandidatesTiersNullValid(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["short_put_tiers"] = nil
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("short_put_tiers null is invalid")
	}
}

func TestOptionCandidatesTiersAbsentValid(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	delete(inst, "short_put_tiers")
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("an instance without short_put_tiers is invalid")
	}
}

func TestOptionCandidatesTiersOneTierValid(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["short_put_tiers"] = tiersView()
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a tier fits with one row, a badge and a field with a gap is invalid")
	}
}

func TestOptionCandidatesTiersUnknownFieldFails(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["short_put_tiers"] = tiersView()
	tiersFirstRow(inst)["extra"] = "x"
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("an extra field of a TierRow validates")
	}
}

func TestOptionCandidatesTiersNumberFails(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["short_put_tiers"] = tiersView()
	tiersFirstRow(inst)["contract"].(map[string]any)["strike"] = 95.0
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a JSON number where a Text is expected validates")
	}
}

func TestOptionCandidatesTiersEmptyTextFails(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["short_put_tiers"] = tiersView()
	tiersFirstRow(inst)["margin"].(map[string]any)["main"] = ""
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("an empty Pair.main validates")
	}
	inst = candidatesMinimal(t)
	inst["short_put_tiers"] = tiersView()
	tier := inst["short_put_tiers"].(map[string]any)["tiers"].([]any)[0].(map[string]any)
	tier["explain"] = strings.Repeat("x", 201)
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a 201-character explain validates")
	}
}

func TestOptionCandidatesTiersNoteTitleValid(t *testing.T) {
	t.Parallel()
	texts := candidatesMinimal(t)
	view := tiersView()
	view["kept_note"] = "a note"
	view["stored_title"] = "stored tables"
	texts["short_put_tiers"] = view
	if !pageV1Valid(t, optionCandidatesPageV1, texts) {
		t.Fatal("kept_note and stored_title as texts are invalid")
	}
	nulls := candidatesMinimal(t)
	view = tiersView()
	view["kept_note"] = nil
	view["stored_title"] = nil
	nulls["short_put_tiers"] = view
	if !pageV1Valid(t, optionCandidatesPageV1, nulls) {
		t.Fatal("kept_note and stored_title as null are invalid")
	}
}

func valueLineView(reason any) map[string]any {
	return map[string]any{
		"label": "Value line", "text": "$91.20 (review 2026-11-10)", "hint": nil, "reason": reason,
		"gate_label": "Gate", "gate_text": "$90.00", "gate_hint": nil,
	}
}

func valueLineInst(t *testing.T, set func(company map[string]any)) map[string]any {
	t.Helper()
	inst := candidatesMinimal(t)
	view := tiersView()
	set(view["companies"].([]any)[0].(map[string]any))
	inst["short_put_tiers"] = view
	return inst
}

func TestOptionCandidatesValueLineValid(t *testing.T) {
	t.Parallel()
	inst := valueLineInst(t, func(c map[string]any) { c["valuation"] = valueLineView(nil) })
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a valuation with a figure and a null reason is invalid")
	}
}

func TestOptionCandidatesValueLineReasonValid(t *testing.T) {
	t.Parallel()
	inst := valueLineInst(t, func(c map[string]any) { c["valuation"] = valueLineView("no_review") })
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a valuation with reason no_review is invalid")
	}
}

func TestOptionCandidatesValueLineNullValid(t *testing.T) {
	t.Parallel()
	inst := valueLineInst(t, func(c map[string]any) { c["valuation"] = nil })
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a null valuation is invalid")
	}
}

func TestOptionCandidatesNoValuationStillValid(t *testing.T) {
	t.Parallel()
	inst := valueLineInst(t, func(c map[string]any) { delete(c, "valuation") })
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a company without the valuation key is invalid")
	}
}

func TestOptionCandidatesValueLineReasonEnum(t *testing.T) {
	t.Parallel()
	inst := valueLineInst(t, func(c map[string]any) { c["valuation"] = valueLineView("stale") })
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("reason stale validates")
	}
}

func TestOptionCandidatesValueLineUnknownField(t *testing.T) {
	t.Parallel()
	inst := valueLineInst(t, func(c map[string]any) {
		v := valueLineView(nil)
		v["extra"] = "x"
		c["valuation"] = v
	})
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("an extra key of ValueLine validates")
	}
}

func TestOptionCandidatesTierViewIdleFieldsValid(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	view := tiersView()
	view["idle_note"] = "two companies have no buy price"
	view["buy_hint"] = "type the buy price on the notices page"
	view["spot_hint"] = "the spot is the last close"
	inst["short_put_tiers"] = view
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("idle_note, buy_hint and spot_hint as texts are invalid")
	}
	view["idle_note"] = nil
	view["buy_hint"] = nil
	view["spot_hint"] = nil
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("null idle_note, buy_hint and spot_hint are invalid")
	}
}

func TestOptionCandidatesTierViewWithoutIdleFieldsValid(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["short_put_tiers"] = tiersView()
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a tier view without idle_note, buy_hint and spot_hint is invalid")
	}
}
