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

func shortPair(main string) map[string]any {
	return map[string]any{"main": main, "sub": nil, "gap": nil}
}

func shortList() map[string]any {
	row := map[string]any{
		"key": "r1", "strike": shortPair("95"), "expiry": shortPair("2026-12-18"), "otm": shortPair("6.1%"),
		"delta": shortPair("0.21"), "premium": shortPair("1.20"), "annualized": shortPair("14%"),
		"buy":        map[string]any{"main": "no buy price", "sub": nil, "gap": pageV1Gap()},
		"buy_source": map[string]any{"code": "buy_source", "label": "your price", "gloss": "typed by you", "tone": "info"},
		"buy_link":   nil,
		"badges":     []any{map[string]any{"code": "wide_spread", "label": "wide spread", "gloss": "bid and ask are far apart", "tone": "warn"}},
	}
	return map[string]any{
		"run_day": "2026-11-18", "counts": "1 of 40", "caveat": nil,
		"filters": []any{map[string]any{"id": "otm", "label": "OTM", "value": "5 to 15%", "hint": "distance"}},
		"dropped": nil, "dropped_items": []any{map[string]any{"code": "no_spot", "label": "no spot", "count": "2", "hint": "no price"}},
		"columns": []any{map[string]any{"id": "strike", "label": "Strike", "hint": nil}},
		"groups": []any{map[string]any{
			"ticker": "AAPL", "spot": "100", "summary": "best 14%", "rows": []any{row},
		}},
		"folded_title": "more",
	}
}

func shortListRow(inst map[string]any) map[string]any {
	g := inst["short_put_list"].(map[string]any)["groups"].([]any)[0].(map[string]any)
	return g["rows"].([]any)[0].(map[string]any)
}

func TestShortListNullValid(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["short_put_list"] = nil
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a null short_put_list is invalid")
	}
}

func TestShortListAbsentValid(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	delete(inst, "short_put_list")
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a page without short_put_list is invalid")
	}
}

func TestShortListOneGroupValid(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["short_put_list"] = shortList()
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("one group with a gap pair, a buy_source badge and a liquidity badge is invalid")
	}
}

func TestShortListUnknownFieldFails(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["short_put_list"] = shortList()
	shortListRow(inst)["extra"] = "x"
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("an extra field of a ShortRow validates")
	}
}

func TestShortListNumberFails(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["short_put_list"] = shortList()
	shortListRow(inst)["strike"].(map[string]any)["main"] = 95.0
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a JSON number for strike.main validates")
	}
}

func buySourceBadge() map[string]any {
	return map[string]any{"code": "buy_source", "label": "your price", "gloss": "typed by you", "tone": "info"}
}

func TestBuySourceValid(t *testing.T) {
	t.Parallel()
	inst := valueLineInst(t, func(c map[string]any) { c["buy_source"] = buySourceBadge() })
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a company with a buy_source badge is invalid")
	}
}

func TestBuySourceNullValid(t *testing.T) {
	t.Parallel()
	inst := valueLineInst(t, func(c map[string]any) { c["buy_source"] = nil })
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a null buy_source is invalid")
	}
}

func TestNoBuySourceStillValid(t *testing.T) {
	t.Parallel()
	inst := valueLineInst(t, func(c map[string]any) { delete(c, "buy_source") })
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a company without the buy_source key is invalid")
	}
}

func TestBuySourceUnknownField(t *testing.T) {
	t.Parallel()
	inst := valueLineInst(t, func(c map[string]any) {
		b := buySourceBadge()
		b["extra"] = "x"
		c["buy_source"] = b
	})
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("an extra key of the buy_source badge validates")
	}
}

func jpLimitCounts() []any {
	return []any{
		map[string]any{"limit": "5", "count": float64(3), "text": "3 within the 5 limit"},
	}
}

func TestJPGroupValid(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	group := candidatesGroup(inst)
	group["currency"] = "JPY"
	group["note"] = "yen quotes"
	group["limit_counts"] = jpLimitCounts()
	inst["short_put_tiers_jpy"] = tiersView()
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a document with the JP group fields and short_put_tiers_jpy is invalid")
	}
	group["currency"] = nil
	group["note"] = nil
	group["limit_counts"] = nil
	inst["short_put_tiers_jpy"] = nil
	inst["short_put_list_jpy"] = nil
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("null JP fields are invalid")
	}
	group["limit_counts"] = jpLimitCounts()
	group["limit_counts"].([]any)[0].(map[string]any)["extra"] = "x"
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("an extra key inside a limit_counts entry is valid")
	}
}

func TestNoJPFieldsStillValid(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a document without the JP fields is invalid")
	}
	if _, ok := candidatesGroup(inst)["currency"]; ok {
		t.Fatal("the minimal group already carries currency")
	}
}

func candidatesGroups(t *testing.T, n int) map[string]any {
	t.Helper()
	inst := candidatesMinimal(t)
	first := candidatesGroup(inst)
	groups := []any{first}
	for i := 1; i < n; i++ {
		g := map[string]any{}
		for k, v := range first {
			g[k] = v
		}
		groups = append(groups, g)
	}
	inst["groups"] = groups
	return inst
}

func TestOptionCandidatesPageFourGroupsValid(t *testing.T) {
	t.Parallel()
	inst := candidatesGroups(t, 4)
	second := inst["groups"].([]any)[1].(map[string]any)
	second["kind"] = "short_put"
	second["currency"] = "JPY"
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a document with four groups, the second the yen short_put, is invalid")
	}
}

func TestOptionCandidatesPageFiveGroupsFail(t *testing.T) {
	t.Parallel()
	if pageV1Valid(t, optionCandidatesPageV1, candidatesGroups(t, 5)) {
		t.Fatal("a document with five groups validates")
	}
}

func zhTier(id string) map[string]any {
	return map[string]any{"id": id, "word": "适合", "glyph": "✓"}
}

func zhValueLine(runState string) map[string]any {
	v := valueLineView(nil)
	v["line_label"] = "估值买入线"
	v["line_text"] = "91.20"
	v["run_state"] = runState
	v["note"] = "复核于 11-10，运行就用它"
	return v
}

func zhCompany(c map[string]any) {
	c["tier"] = zhTier("fits")
	c["count_parts"] = map[string]any{"fits": "1", "check": "0", "grey": "0"}
	c["spot_text"] = "100"
	c["spot_note"] = "存档 11-18"
	c["price_note"] = nil
	c["earnings_text"] = "2026-12-01"
	c["source_badge"] = map[string]any{"code": "buy_source", "label": "手填", "gloss": "你自己填的买入价", "tone": "info"}
	c["valuation"] = zhValueLine("used")
}

func zhInst(t *testing.T, set func(view, company map[string]any)) map[string]any {
	t.Helper()
	inst := candidatesMinimal(t)
	view := tiersView()
	set(view, view["companies"].([]any)[0].(map[string]any))
	inst["short_put_tiers"] = view
	return inst
}

func TestTiersNewFieldsValidate(t *testing.T) {
	t.Parallel()
	inst := zhInst(t, func(view, c map[string]any) {
		zhCompany(c)
		view["strip_labels"] = map[string]any{
			"spot": "现价", "buy_price": "买入价", "next_earnings": "下次财报", "contracts": "合约",
			"fits": "适合", "check": "待查", "grey": "折叠", "grey_hint": "折叠 = 未判定",
		}
		view["card_note"] = "现价是任务为这一天运行时取到的价格。"
	})
	inst["head_labels"] = map[string]any{
		"data_day": "数据日", "run_status": "运行状态", "run_status_text": "完成", "rule_version": "规则版本",
	}
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("an instance with every new field is invalid")
	}
	c := inst["short_put_tiers"].(map[string]any)["companies"].([]any)[0].(map[string]any)
	c["spot_text"], c["spot_note"], c["source_badge"] = nil, nil, nil
	c["price_note"] = "没有买入价"
	c["valuation"].(map[string]any)["note"] = nil
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("null spot_text, spot_note, source_badge and note are invalid")
	}
}

func TestTiersOldShapeStillValidates(t *testing.T) {
	t.Parallel()
	inst := zhInst(t, func(view, c map[string]any) { c["valuation"] = valueLineView(nil) })
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("an instance with only the old fields is invalid")
	}
}

func TestRunStateEnum(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"used", "not_used", "none"} {
		inst := zhInst(t, func(_, c map[string]any) { c["valuation"] = zhValueLine(ok) })
		if !pageV1Valid(t, optionCandidatesPageV1, inst) {
			t.Fatalf("run_state %q is invalid", ok)
		}
	}
	inst := zhInst(t, func(_, c map[string]any) { c["valuation"] = zhValueLine("maybe") })
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("run_state maybe validates")
	}
}

func TestTierIDEnum(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"fits", "check", "grey"} {
		inst := zhInst(t, func(_, c map[string]any) { c["tier"] = zhTier(ok) })
		if !pageV1Valid(t, optionCandidatesPageV1, inst) {
			t.Fatalf("tier id %q is invalid", ok)
		}
	}
	inst := zhInst(t, func(_, c map[string]any) { c["tier"] = zhTier("fit") })
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("tier id fit validates")
	}
}

func headsView() map[string]any {
	return map[string]any{"companies": "公司", "companies_note": "每家公司两行", "tiers": "合约分档"}
}

func TestHeadingsFieldsValidate(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["subtitle"] = "先卖出看跌期权的候选合约"
	view := tiersView()
	view["heads"] = headsView()
	inst["short_put_tiers"] = view
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("an instance with subtitle and heads is invalid")
	}
}

func TestHeadingsOldShapeStillValidates(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	inst["short_put_tiers"] = tiersView()
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("an instance without subtitle and heads is invalid")
	}
}

func TestHeadsNeedAllThree(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	view := tiersView()
	heads := headsView()
	delete(heads, "tiers")
	view["heads"] = heads
	inst["short_put_tiers"] = view
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("heads without tiers validates")
	}
}

func TestHeadsNullFails(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	view := tiersView()
	view["heads"] = nil
	inst["short_put_tiers"] = view
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("heads null validates")
	}
}
