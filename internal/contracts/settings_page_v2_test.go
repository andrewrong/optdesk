package contracts_test

import (
	"strings"
	"testing"
)

const settingsPageV2 = "page.settings_page.v2.json"

var settingsPatterns = map[string]string{
	"^[0-9]{1,12}$":  "1",
	"^[0-9a-f]{64}$": strings.Repeat("a", 64),
	"^[a-z_]{1,48}$": "dte_min",
	patDateTime:      "2026-01-02T03:04:05+08:00",
}

func settingsMinimal(t *testing.T) map[string]any {
	t.Helper()
	return pageV1Minimal(t, settingsPageV2, settingsPatterns)
}

func settingsForm() map[string]any {
	return map[string]any{"post_path": "/owner-settings", "expires": "1", "mac": strings.Repeat("a", 64)}
}

func settingsParam(kind string) map[string]any {
	return map[string]any{
		"key": "dte_min", "label": "Min days to expiry", "unit": "days", "kind": kind,
		"value": "", "default": "7", "min": "1", "max": "60",
	}
}

func TestSettingsPageV2MinimalValid(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, settingsPageV2, settingsMinimal(t)) {
		t.Fatal("minimal instance is invalid")
	}
}

func TestSettingsPageV2FullValid(t *testing.T) {
	t.Parallel()
	inst := settingsMinimal(t)
	inst["form"] = settingsForm()
	inst["confirm"] = map[string]any{
		"rows": []any{map[string]any{"label": "Min days to expiry", "old": "7", "new": "10"}},
		"form": settingsForm(),
	}
	inst["history"] = []any{
		map[string]any{"at": "2026-01-02T03:04:05+08:00", "kind": "param", "name": "Min days to expiry", "value": "10",
			"key": "dte_min", "instrument_id": nil, "excluded": nil},
		map[string]any{"at": "2026-01-02T03:04:05+08:00", "kind": "exclusion", "name": "AMD", "value": "",
			"key": nil, "instrument_id": 5, "excluded": true},
	}
	inst["params"] = []any{settingsParam("int"), settingsParam("decimal")}
	inst["rows"] = []any{map[string]any{"instrument_id": 5, "symbol": "AMD", "excluded": true}}
	inst["notice"] = map[string]any{"kind": "saved", "text": "Saved"}
	if !pageV1Valid(t, settingsPageV2, inst) {
		t.Fatal("full instance is invalid")
	}
}

func TestSettingsPageV2Version1Fails(t *testing.T) {
	t.Parallel()
	inst := settingsMinimal(t)
	inst["schema_version"] = 1
	if pageV1Valid(t, settingsPageV2, inst) {
		t.Fatal("version 1 validates")
	}
}

func TestSettingsPageV2PostPathIsConstant(t *testing.T) {
	t.Parallel()
	for name, path := range map[string]string{"plan": "/plan/price-lines", "empty": "", "slash": "/owner-settings/"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			inst := settingsMinimal(t)
			f := settingsForm()
			f["post_path"] = path
			inst["form"] = f
			if pageV1Valid(t, settingsPageV2, inst) {
				t.Fatalf("post_path %q validates", path)
			}
		})
	}
}

func TestSettingsPageV2MacIs64LowerHex(t *testing.T) {
	t.Parallel()
	for name, mac := range map[string]string{
		"short":    strings.Repeat("a", 63),
		"upper":    strings.Repeat("A", 64),
		"not_hex":  strings.Repeat("g", 64),
		"too_long": strings.Repeat("a", 65),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			inst := settingsMinimal(t)
			f := settingsForm()
			f["mac"] = mac
			inst["form"] = f
			if pageV1Valid(t, settingsPageV2, inst) {
				t.Fatalf("mac %q validates", mac)
			}
		})
	}
}

func TestSettingsPageV2ParamKindIsIntOrDecimal(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]bool{"int": true, "decimal": true, "float": false} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			inst := settingsMinimal(t)
			inst["params"] = []any{settingsParam(kind)}
			if got := pageV1Valid(t, settingsPageV2, inst); got != want {
				t.Fatalf("kind %s valid = %v, want %v", kind, got, want)
			}
		})
	}
}

func TestSettingsPageV2Rejects(t *testing.T) {
	t.Parallel()
	cases := map[string]func(m map[string]any){
		"extra field":       func(m map[string]any) { m["extra"] = 1 },
		"bad key":           func(m map[string]any) { p := settingsParam("int"); p["key"] = "Dte-Min"; m["params"] = []any{p} },
		"21 history rows":   func(m map[string]any) { m["history"] = make([]any, 21) },
		"bad notice kind":   func(m map[string]any) { m["notice"] = map[string]any{"kind": "warn", "text": "x"} },
		"confirm no form":   func(m map[string]any) { m["confirm"] = map[string]any{"rows": []any{}} },
		"history bad kind":  func(m map[string]any) { m["history"] = []any{map[string]any{"kind": "other"}} },
		"numeric param val": func(m map[string]any) { p := settingsParam("int"); p["default"] = 7; m["params"] = []any{p} },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			inst := settingsMinimal(t)
			edit(inst)
			if pageV1Valid(t, settingsPageV2, inst) {
				t.Fatal("instance validates")
			}
		})
	}
}
