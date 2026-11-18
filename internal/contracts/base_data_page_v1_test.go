package contracts_test

import (
	"strings"
	"testing"
)

const baseDataPageV1 = "page.base_data_page.v1.json"

func baseDataFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	return readRaw(t, optionPageFixturesDir, "page.base_data_page.v1."+name+".json")
}

func baseDataRows(m map[string]any) []map[string]any {
	var rows []map[string]any
	for _, g := range m["groups"].([]any) {
		for _, r := range g.(map[string]any)["rows"].([]any) {
			rows = append(rows, r.(map[string]any))
		}
	}
	return rows
}

// TestBaseDataPageOkValid: the ok fixture passes the schema.
func TestBaseDataPageOkValid(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, baseDataPageV1, baseDataFixture(t, "ok")) {
		t.Fatal("ok fixture refused")
	}
}

// TestBaseDataPageGapValid: the gap fixture passes and has no group and one gap.
func TestBaseDataPageGapValid(t *testing.T) {
	t.Parallel()
	m := baseDataFixture(t, "gap")
	if !pageV1Valid(t, baseDataPageV1, m) {
		t.Fatal("gap fixture refused")
	}
	if len(m["groups"].([]any)) != 0 || len(m["gaps"].([]any)) != 1 {
		t.Fatal("gap fixture must have no groups and one gap")
	}
}

// TestBaseDataPageRejectsBadLevel: a level outside the four, a bad gap reason and a bad repair_path fail.
func TestBaseDataPageRejectsBadLevel(t *testing.T) {
	t.Parallel()
	m := baseDataFixture(t, "ok")
	baseDataRows(m)[0]["level"] = "green"
	if pageV1Valid(t, baseDataPageV1, m) {
		t.Error("level green accepted")
	}
	m = baseDataFixture(t, "ok")
	m["top"].(map[string]any)["level"] = "bad"
	if pageV1Valid(t, baseDataPageV1, m) {
		t.Error("top level bad accepted")
	}
	m = baseDataFixture(t, "gap")
	m["gaps"].([]any)[0].(map[string]any)["reason"] = "other"
	if pageV1Valid(t, baseDataPageV1, m) {
		t.Error("gap reason other accepted")
	}
	m = baseDataFixture(t, "ok")
	m["repair_path"] = "/elsewhere"
	if pageV1Valid(t, baseDataPageV1, m) {
		t.Error("repair_path /elsewhere accepted")
	}
}

// TestBaseDataPageAcceptsCrossCheckSlot: a row with a cross_check validates, one without it validates,
// and so does a top-level cross_checks.
func TestBaseDataPageAcceptsCrossCheckSlot(t *testing.T) {
	t.Parallel()
	m := baseDataFixture(t, "ok")
	rows := baseDataRows(m)
	with, without := 0, 0
	for _, r := range rows {
		if _, ok := r["cross_check"]; ok {
			with++
		} else {
			without++
		}
	}
	if with == 0 || without == 0 {
		t.Fatalf("ok fixture needs a row with and a row without cross_check, got %d and %d", with, without)
	}
	if !pageV1Valid(t, baseDataPageV1, m) {
		t.Fatal("rows with and without cross_check refused")
	}
	m["cross_checks"] = []any{rows[0]["cross_check"]}
	if !pageV1Valid(t, baseDataPageV1, m) {
		t.Fatal("top-level cross_checks refused")
	}
	m = baseDataFixture(t, "ok")
	baseDataRows(m)[0]["cross_check"].(map[string]any)["extra"] = 1
	if pageV1Valid(t, baseDataPageV1, m) {
		t.Fatal("cross_check with an extra key accepted")
	}
}

// TestCrossFixtureValid: the cross fixture passes and carries one bars row with an R6 cross_check
// that has the counts and the skipped count.
func TestCrossFixtureValid(t *testing.T) {
	t.Parallel()
	m := baseDataFixture(t, "cross")
	if !pageV1Valid(t, baseDataPageV1, m) {
		t.Fatal("cross fixture refused")
	}
	rows := baseDataRows(m)
	if len(rows) != 1 || rows[0]["dataset"] != "bars" {
		t.Fatalf("cross fixture needs one bars row, got %d rows", len(rows))
	}
	cc, ok := rows[0]["cross_check"].(map[string]any)
	if !ok || cc["pair_id"] != "R6" {
		t.Fatal("bars row needs a cross_check of pair R6")
	}
	for _, k := range []string{"n", "matched", "diff", "missing", "skipped"} {
		if _, ok := cc[k]; !ok {
			t.Errorf("cross_check lacks %s", k)
		}
	}
}

// TestSkippedNotNegative: skipped below zero, a reason outside the enum and a non-string company fail;
// the five reasons and a company pass; a cross_check without the new fields still passes.
func TestSkippedNotNegative(t *testing.T) {
	t.Parallel()
	cc := func(m map[string]any) map[string]any { return baseDataRows(m)[0]["cross_check"].(map[string]any) }
	m := baseDataFixture(t, "cross")
	cc(m)["skipped"] = -1
	if pageV1Valid(t, baseDataPageV1, m) {
		t.Error("skipped -1 accepted")
	}
	m = baseDataFixture(t, "cross")
	cc(m)["reason"] = "other"
	if pageV1Valid(t, baseDataPageV1, m) {
		t.Error("reason other accepted")
	}
	m = baseDataFixture(t, "cross")
	cc(m)["items"].([]any)[0].(map[string]any)["company"] = 7
	if pageV1Valid(t, baseDataPageV1, m) {
		t.Error("numeric company accepted")
	}
	for _, r := range []string{"no_run", "rate_limited", "network", "timeout", "source_changed"} {
		m = baseDataFixture(t, "cross")
		cc(m)["reason"] = r
		cc(m)["reason_zh"] = "原因"
		if !pageV1Valid(t, baseDataPageV1, m) {
			t.Errorf("reason %s refused", r)
		}
	}
	m = baseDataFixture(t, "cross")
	delete(cc(m), "skipped")
	if !pageV1Valid(t, baseDataPageV1, m) {
		t.Error("cross_check without skipped refused")
	}
}

// TestCrossTextsBounded: company and reason_zh follow the text maximum of 600.
func TestCrossTextsBounded(t *testing.T) {
	t.Parallel()
	m := baseDataFixture(t, "cross")
	cc := baseDataRows(m)[0]["cross_check"].(map[string]any)
	cc["items"].([]any)[0].(map[string]any)["company"] = strings.Repeat("x", 601)
	if pageV1Valid(t, baseDataPageV1, m) {
		t.Error("company of 601 chars accepted")
	}
	m = baseDataFixture(t, "cross")
	baseDataRows(m)[0]["cross_check"].(map[string]any)["reason_zh"] = strings.Repeat("原", 601)
	if pageV1Valid(t, baseDataPageV1, m) {
		t.Error("reason_zh of 601 chars accepted")
	}
}
