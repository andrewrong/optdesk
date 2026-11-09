package contracts_test

import "testing"

const optionNoticesPageV1 = "page.option_notices_page.v1.json"

func noticesMinimal(t *testing.T) map[string]any {
	t.Helper()
	return pageV1Minimal(t, optionNoticesPageV1, map[string]string{
		"^notice-[A-Za-z0-9_-]{1,64}$":                        "notice-1",
		"^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$": "2026-11-06",
		"^[a-z_]{1,32}$": "early_assign",
	})
}

func noticesBlock() map[string]any {
	return map[string]any{
		"alias": "main", "main": true, "status": "ok", "data_day": "2026-11-06",
		"notices": []any{map[string]any{
			"id": "notice-1", "code": "early_assign", "severity": "warn", "title": "t", "contract": "AAPL 250P",
			"expiry": nil, "facts": []any{map[string]any{"label": "a", "text": "b"}}, "reasons": []any{"r"},
		}},
		"cash_by_date": []any{map[string]any{"day": "2026-11-07", "currency": "USD", "amount_text": "1,000"}},
		"cushion_text": nil,
		"note":         nil,
	}
}

func TestOptionNoticesPageMinimalValid(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, optionNoticesPageV1, noticesMinimal(t)) {
		t.Fatal("minimal instance is invalid")
	}
	inst := noticesMinimal(t)
	inst["accounts"] = []any{noticesBlock()}
	if !pageV1Valid(t, optionNoticesPageV1, inst) {
		t.Fatal("page with a full alias block is invalid")
	}
}

func TestOptionNoticesPageVersionAndUnknownFieldFail(t *testing.T) {
	t.Parallel()
	inst := noticesMinimal(t)
	inst["schema_version"] = 2
	if pageV1Valid(t, optionNoticesPageV1, inst) {
		t.Fatal("version 2 validates")
	}
	inst = noticesMinimal(t)
	inst["extra"] = "x"
	if pageV1Valid(t, optionNoticesPageV1, inst) {
		t.Fatal("an unknown field validates")
	}
	inst = noticesMinimal(t)
	b := noticesBlock()
	b["extra"] = 1
	inst["accounts"] = []any{b}
	if pageV1Valid(t, optionNoticesPageV1, inst) {
		t.Fatal("an unknown field in a block validates")
	}
}

func TestOptionNoticesPageSeverityEnum(t *testing.T) {
	t.Parallel()
	for sev, want := range map[string]bool{"info": true, "warn": true, "act": true, "fatal": false, "": false} {
		inst := noticesMinimal(t)
		b := noticesBlock()
		b["notices"].([]any)[0].(map[string]any)["severity"] = sev
		inst["accounts"] = []any{b}
		if got := pageV1Valid(t, optionNoticesPageV1, inst); got != want {
			t.Fatalf("severity %q: valid=%v, want %v", sev, got, want)
		}
	}
}

func TestOptionNoticesPageCodeIsPattern(t *testing.T) {
	t.Parallel()
	for code, want := range map[string]bool{"new_code": true, "Early": false, "early1": false, "": false} {
		inst := noticesMinimal(t)
		b := noticesBlock()
		b["notices"].([]any)[0].(map[string]any)["code"] = code
		inst["accounts"] = []any{b}
		if got := pageV1Valid(t, optionNoticesPageV1, inst); got != want {
			t.Fatalf("code %q: valid=%v, want %v", code, got, want)
		}
	}
}

func TestOptionNoticesPageMainNoCaptureGapValid(t *testing.T) {
	t.Parallel()
	inst := noticesMinimal(t)
	inst["gaps"] = []any{map[string]any{"kind": "gap", "reason": "no_capture", "text": "no capture for main"}}
	if !pageV1Valid(t, optionNoticesPageV1, inst) {
		t.Fatal("page gap no_capture (Decision 8) is rejected by the schema")
	}
	inst["gaps"] = []any{map[string]any{"kind": "gap", "reason": "No-Capture", "text": "x"}}
	if pageV1Valid(t, optionNoticesPageV1, inst) {
		t.Fatal("a gap reason outside the pattern validates")
	}
}
