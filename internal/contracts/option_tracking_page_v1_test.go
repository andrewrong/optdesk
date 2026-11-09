package contracts_test

import "testing"

const optionTrackingPageV1 = "page.option_tracking_page.v1.json"

func trackingMinimal(t *testing.T) map[string]any {
	t.Helper()
	inst := pageV1Minimal(t, optionTrackingPageV1, map[string]string{
		"^pos-[A-Za-z0-9:_-]{1,96}$": "pos-1",
		"^chk-[A-Za-z0-9:_-]{1,96}$": "chk-1",
		"^[a-z0-9_-]{1,16}$":         "acct_1",
	})
	acct := trackingAccount(inst)
	acct["open"], acct["recent"], acct["recon"] = nil, nil, nil
	acct["gap"] = pageV1Gap()
	return inst
}

func trackingAccount(inst map[string]any) map[string]any {
	return inst["accounts"].([]any)[0].(map[string]any)
}

func trackingTable(rowID string) map[string]any {
	return map[string]any{
		"columns": []any{map[string]any{"id": "strike", "label": "Strike", "align": "right", "hint": nil}},
		"rows": []any{map[string]any{
			"id":    rowID,
			"cells": map[string]any{"strike": map[string]any{"text": "95", "value": 95.0, "badges": []any{}, "tone": "none"}},
		}},
	}
}

func TestOptionTrackingPageMinimalValid(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, optionTrackingPageV1, trackingMinimal(t)) {
		t.Fatal("minimal instance is invalid")
	}
}

func TestOptionTrackingPageVersionFails(t *testing.T) {
	t.Parallel()
	inst := trackingMinimal(t)
	inst["schema_version"] = 2
	if pageV1Valid(t, optionTrackingPageV1, inst) {
		t.Fatal("version 2 validates")
	}
}

func TestOptionTrackingPageUnknownFieldFails(t *testing.T) {
	t.Parallel()
	inst := trackingMinimal(t)
	inst["extra"] = 1
	if pageV1Valid(t, optionTrackingPageV1, inst) {
		t.Fatal("unknown page field validates")
	}
	inst = trackingMinimal(t)
	trackingAccount(inst)["extra"] = 1
	if pageV1Valid(t, optionTrackingPageV1, inst) {
		t.Fatal("unknown account field validates")
	}
}

func TestOptionTrackingPageOpenWithGapFails(t *testing.T) {
	t.Parallel()
	inst := trackingMinimal(t)
	acct := trackingAccount(inst)
	acct["open"] = trackingTable("pos-1")
	acct["gap"] = pageV1Gap()
	if pageV1Valid(t, optionTrackingPageV1, inst) {
		t.Fatal("account with open and gap validates")
	}
}

func TestOptionTrackingPageOpenTableValid(t *testing.T) {
	t.Parallel()
	inst := trackingMinimal(t)
	acct := trackingAccount(inst)
	acct["open"] = trackingTable("pos-1")
	acct["recon"] = trackingTable("chk-1")
	acct["gap"] = nil
	if !pageV1Valid(t, optionTrackingPageV1, inst) {
		t.Fatal("account with tables and no gap is invalid")
	}
}

func TestOptionTrackingPageNoTableNoGapFails(t *testing.T) {
	t.Parallel()
	inst := trackingMinimal(t)
	acct := trackingAccount(inst)
	acct["open"], acct["recent"], acct["recon"], acct["gap"] = nil, nil, nil, nil
	if pageV1Valid(t, optionTrackingPageV1, inst) {
		t.Fatal("account without table and gap validates")
	}
}

func TestOptionTrackingPageOpenRowIDPrefixFails(t *testing.T) {
	t.Parallel()
	inst := trackingMinimal(t)
	acct := trackingAccount(inst)
	acct["open"] = trackingTable("chk-1")
	acct["gap"] = nil
	if pageV1Valid(t, optionTrackingPageV1, inst) {
		t.Fatal("open row id without pos- prefix validates")
	}
}

func TestOptionTrackingPageUpperCaseAliasFails(t *testing.T) {
	t.Parallel()
	inst := trackingMinimal(t)
	trackingAccount(inst)["alias"] = "Main"
	if pageV1Valid(t, optionTrackingPageV1, inst) {
		t.Fatal("alias with upper-case letter validates")
	}
}
