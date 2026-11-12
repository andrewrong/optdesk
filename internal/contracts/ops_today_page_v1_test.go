package contracts_test

import "testing"

const opsTodayPageV1 = "page.ops_today_page.v1.json"

func opsTodayMinimal(t *testing.T) map[string]any {
	t.Helper()
	return pageV1Minimal(t, opsTodayPageV1, map[string]string{
		"^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$": "2026-11-12",
	})
}

func opsTodayRow() map[string]any {
	return map[string]any{
		"dataset": "master.option_list", "key": "k1", "market": "US", "status": "ok",
		"error_code": nil, "reason_code": "outside_range", "returned_n": 3, "usable_n": 0,
		"detail": "AAPL 2026-12-18",
	}
}

func TestOpsTodayPageValid(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, opsTodayPageV1, opsTodayMinimal(t)) {
		t.Fatal("minimal instance is invalid")
	}
	inst := opsTodayMinimal(t)
	inst["today"] = map[string]any{"rows": []any{opsTodayRow()}}
	if !pageV1Valid(t, opsTodayPageV1, inst) {
		t.Fatal("page with a full row is invalid")
	}
}

func TestOpsTodayPageMissingTodayRefused(t *testing.T) {
	t.Parallel()
	inst := opsTodayMinimal(t)
	delete(inst, "today")
	if pageV1Valid(t, opsTodayPageV1, inst) {
		t.Fatal("a page without today validates")
	}
}

func TestOpsTodayPageUnknownFieldRefused(t *testing.T) {
	t.Parallel()
	inst := opsTodayMinimal(t)
	inst["extra"] = "x"
	if pageV1Valid(t, opsTodayPageV1, inst) {
		t.Fatal("an unknown page field validates")
	}
	inst = opsTodayMinimal(t)
	row := opsTodayRow()
	row["rank"] = 1
	inst["today"] = map[string]any{"rows": []any{row}}
	if pageV1Valid(t, opsTodayPageV1, inst) {
		t.Fatal("an unknown row field validates")
	}
}
