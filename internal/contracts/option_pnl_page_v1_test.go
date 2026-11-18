package contracts_test

import "testing"

const optionPnlPageV1 = "page.option_pnl_page.v1.json"

func pnlMinimal(t *testing.T) map[string]any {
	t.Helper()
	inst := pageV1Minimal(t, optionPnlPageV1, map[string]string{
		"^book-[A-Za-z0-9:_-]{1,96}$": "book-1",
		"^lot-[A-Za-z0-9:_-]{1,96}$":  "lot-1",
		"^[a-z0-9_-]{1,16}$":          "acct_1",
	})
	acct := pnlAccount(inst)
	acct["books"], acct["lots"], acct["gap"] = []any{}, []any{}, nil
	return inst
}

func pnlAccount(inst map[string]any) map[string]any {
	return inst["accounts"].([]any)[0].(map[string]any)
}

func pnlLine(id string) map[string]any {
	return map[string]any{"id": id, "label": "Time", "text": "$1.00", "hint": nil, "tone": "none"}
}

func pnlBook(leftOut any) map[string]any {
	return map[string]any{
		"id": "book-usd", "currency": "USD", "counts": "1 lot counted", "left_out": leftOut,
		"headline": pnlLine("headline"), "lines": []any{pnlLine("time")},
	}
}

func pnlLot(lines []any, ivLine any, gap any) map[string]any {
	return map[string]any{
		"id": "lot-1", "title": "AAA put", "summary": "open", "lines": lines,
		"days_line": pnlLine("days"), "iv_line": ivLine, "end_line": nil,
		"recon": []any{}, "badges": []any{}, "gap": gap,
	}
}

func TestOptionPnlPageMinimalValid(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, optionPnlPageV1, pnlMinimal(t)) {
		t.Fatal("minimal instance is invalid")
	}
}

func TestOptionPnlPageVersionFails(t *testing.T) {
	t.Parallel()
	inst := pnlMinimal(t)
	inst["schema_version"] = 2
	if pageV1Valid(t, optionPnlPageV1, inst) {
		t.Fatal("version 2 validates")
	}
}

func TestOptionPnlPageUnknownField(t *testing.T) {
	t.Parallel()
	inst := pnlMinimal(t)
	inst["extra"] = 1
	if pageV1Valid(t, optionPnlPageV1, inst) {
		t.Fatal("unknown page field validates")
	}
	inst = pnlMinimal(t)
	pnlAccount(inst)["extra"] = 1
	if pageV1Valid(t, optionPnlPageV1, inst) {
		t.Fatal("unknown account field validates")
	}
	inst = pnlMinimal(t)
	pnlAccount(inst)["lots"] = []any{pnlLot([]any{pnlLine("time")}, nil, nil)}
	pnlAccount(inst)["lots"].([]any)[0].(map[string]any)["extra"] = 1
	if pageV1Valid(t, optionPnlPageV1, inst) {
		t.Fatal("unknown lot field validates")
	}
	inst = pnlMinimal(t)
	line := pnlLine("time")
	line["tone"] = "loud"
	pnlAccount(inst)["books"] = []any{map[string]any{
		"id": "book-usd", "currency": "USD", "counts": "c", "left_out": nil,
		"headline": line, "lines": []any{},
	}}
	if pageV1Valid(t, optionPnlPageV1, inst) {
		t.Fatal("tone outside the enum validates")
	}
}

func TestOptionPnlPageNoQuotesValid(t *testing.T) {
	t.Parallel()
	inst := pnlMinimal(t)
	pnlAccount(inst)["books"] = []any{pnlBook(nil)}
	pnlAccount(inst)["lots"] = []any{pnlLot([]any{}, nil, nil)}
	if !pageV1Valid(t, optionPnlPageV1, inst) {
		t.Fatal("lot with iv_line null and empty lines is invalid")
	}
}

func TestOptionPnlPageLotGapValid(t *testing.T) {
	t.Parallel()
	inst := pnlMinimal(t)
	pnlAccount(inst)["books"] = []any{pnlBook("1 of 2 lots left out")}
	pnlAccount(inst)["lots"] = []any{pnlLot([]any{}, nil, pageV1Gap())}
	if !pageV1Valid(t, optionPnlPageV1, inst) {
		t.Fatal("lot with gap and book with left_out is invalid")
	}
}

func pnlLedger(viewIDs ...string) map[string]any {
	views := []any{}
	for _, id := range viewIDs {
		views = append(views, map[string]any{
			"id": id, "label": "By month",
			"groups": []any{map[string]any{
				"id": "grp-2026-11", "label": "2026-11", "summary": "6 lots ended",
				"books": []any{pnlBook(nil)},
			}},
		})
	}
	return map[string]any{"title": "Ledger", "note": "2 open lots", "views": views}
}

func TestOptionPnlLedgerValid(t *testing.T) {
	t.Parallel()
	inst := pnlMinimal(t)
	pnlAccount(inst)["ledger"] = pnlLedger("by_month", "by_company")
	if !pageV1Valid(t, optionPnlPageV1, inst) {
		t.Fatal("account with a ledger of two views is invalid")
	}
}

func TestOptionPnlLedgerNullValid(t *testing.T) {
	t.Parallel()
	inst := pnlMinimal(t)
	pnlAccount(inst)["ledger"] = nil
	if !pageV1Valid(t, optionPnlPageV1, inst) {
		t.Fatal("ledger null is invalid")
	}
}

func TestOptionPnlNoLedgerStillValid(t *testing.T) {
	t.Parallel()
	inst := pnlMinimal(t)
	delete(pnlAccount(inst), "ledger")
	if !pageV1Valid(t, optionPnlPageV1, inst) {
		t.Fatal("account without the ledger key is invalid")
	}
}

func TestOptionPnlLedgerViewIDEnum(t *testing.T) {
	t.Parallel()
	inst := pnlMinimal(t)
	pnlAccount(inst)["ledger"] = pnlLedger("by_week")
	if pageV1Valid(t, optionPnlPageV1, inst) {
		t.Fatal("view id by_week validates")
	}
}

func TestOptionPnlLedgerUnknownField(t *testing.T) {
	t.Parallel()
	inst := pnlMinimal(t)
	l := pnlLedger("by_month")
	l["views"].([]any)[0].(map[string]any)["groups"].([]any)[0].(map[string]any)["extra"] = 1
	pnlAccount(inst)["ledger"] = l
	if pageV1Valid(t, optionPnlPageV1, inst) {
		t.Fatal("extra key of Group validates")
	}
}
