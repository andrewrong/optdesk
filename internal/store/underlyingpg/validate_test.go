package underlyingpg

import (
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

func sp(s string) *string { return &s }

func goodAdd() underlying.Add {
	return underlying.Add{Symbol: "AAPL", Market: "US", Kind: underlying.KindStock, Options: true}
}

func resolvedRow() underlying.Row {
	return underlying.Row{ID: 5, Key: "US:AAPL", Symbol: "AAPL", Market: "US", Kind: underlying.KindStock,
		Exchange: "SMART", Currency: "USD", ConID: 265598, Status: underlying.StatusActive, Options: true}
}

func TestValidateAddRules(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		edit  func(a *underlying.Add)
		field string // "" means valid
	}{
		{"valid us", func(*underlying.Add) {}, ""},
		{"empty issuer is valid", func(a *underlying.Add) { a.Issuer = "" }, ""},
		{"cik issuer", func(a *underlying.Add) { a.Issuer = "cik:0000320193" }, ""},
		{"edinet issuer", func(a *underlying.Add) { a.Issuer = "edinet:E01234" }, ""},
		{"market pattern", func(a *underlying.Add) { a.Market = "HK" }, "market"},
		{"us symbol lower case", func(a *underlying.Add) { a.Symbol = "aapl" }, "symbol"},
		{"us symbol too long", func(a *underlying.Add) { a.Symbol = "ABCDEFGHIJKLM" }, "symbol"},
		{"jp symbol not 4 digits", func(a *underlying.Add) { a.Market, a.Symbol = "JP", "AAPL" }, "symbol"},
		{"kind index", func(a *underlying.Add) { a.Kind = "index" }, "kind"},
		{"cik with 9 digits", func(a *underlying.Add) { a.Issuer = "cik:000032019" }, "issuer"},
		{"issuer without scheme", func(a *underlying.Add) { a.Issuer = "0000320193" }, "issuer"},
		{"price with 5 places", func(a *underlying.Add) { a.BuyPrice = "10.12345" }, "buy_price"},
		{"price zero", func(a *underlying.Add) { a.SalePrice = "0" }, "sale_price"},
		{"sale below buy", func(a *underlying.Add) { a.BuyPrice, a.SalePrice = "100.5", "100.4999" }, "sale_price"},
		{"sale equal buy", func(a *underlying.Add) { a.BuyPrice, a.SalePrice = "100", "100.0" }, ""},
		{"report date not a date", func(a *underlying.Add) { a.ReportDateOverride = "2026-13-40" }, "report_date_override"},
		{"currency not 3 letters", func(a *underlying.Add) { a.Currency = "US" }, "currency"},
		{"name too long", func(a *underlying.Add) { a.Name = string(make([]rune, 81)) + "x" }, "name"},
		{"negative con_id", func(a *underlying.Add) { a.ConID = -1 }, "con_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := goodAdd()
			tc.edit(&a)
			_, err := ValidateAdd(a, nil)
			expectField(t, err, tc.field)
		})
	}
}

func TestValidateAddDuplicateNameKey(t *testing.T) {
	t.Parallel()
	_, err := ValidateAdd(goodAdd(), map[string]bool{"US:AAPL": true})
	expectField(t, err, "symbol")
}

func TestValidateAddDefaultsAndStatus(t *testing.T) {
	t.Parallel()
	r, err := ValidateAdd(underlying.Add{Symbol: "7203", Market: "JP", Kind: underlying.KindStock}, nil)
	if err != nil || r.Exchange != "TSEJ" || r.Currency != "JPY" || r.Key != "JP:7203" || r.Status != underlying.StatusPending {
		t.Fatalf("jp add = %+v, %v", r, err)
	}
	a := goodAdd()
	a.ConID = 265598
	r, err = ValidateAdd(a, nil)
	if err != nil || r.Status != underlying.StatusActive || r.ConID != 265598 || r.Exchange != "SMART" {
		t.Fatalf("add with con_id = %+v, %v; want active", r, err)
	}
}

func TestValidateEditRules(t *testing.T) {
	t.Parallel()
	unresolved := resolvedRow()
	unresolved.Status, unresolved.ConID, unresolved.Reason = underlying.StatusUnresolved, 0, "not listed"
	for _, tc := range []struct {
		name  string
		cur   underlying.Row
		edit  underlying.Edit
		field string
	}{
		{"exchange of an active name is a no", resolvedRow(), underlying.Edit{Exchange: sp("ARCA")}, "exchange"},
		{"currency of an active name is a no", resolvedRow(), underlying.Edit{Currency: sp("JPY")}, "currency"},
		{"status pending is the system's", resolvedRow(), underlying.Edit{Status: sp("pending")}, "status"},
		{"status unresolved is the system's", resolvedRow(), underlying.Edit{Status: sp("unresolved")}, "status"},
		{"active without con_id", unresolved, underlying.Edit{Status: sp("active")}, "status"},
		{"issuer form", resolvedRow(), underlying.Edit{Issuer: sp("sec:1")}, "issuer"},
		{"sale below buy against the stored buy", withBuy(resolvedRow(), "50"), underlying.Edit{SalePrice: sp("49")}, "sale_price"},
		{"report date", resolvedRow(), underlying.Edit{ReportDateOverride: sp("tomorrow")}, "report_date_override"},
		{"pause is fine", resolvedRow(), underlying.Edit{Status: sp("paused")}, ""},
		{"same exchange is no edit", resolvedRow(), underlying.Edit{Exchange: sp("SMART")}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ValidateEdit(tc.cur, tc.edit)
			expectField(t, err, tc.field)
		})
	}
}

func withBuy(r underlying.Row, b string) underlying.Row { r.BuyPrice = b; return r }

func TestValidateEditChanged(t *testing.T) {
	t.Parallel()
	if _, changed, err := ValidateEdit(resolvedRow(), underlying.Edit{Key: "US:AAPL", Note: sp("")}); err != nil || changed {
		t.Errorf("empty note over empty note: changed %v, %v; want unchanged", changed, err)
	}
	next, changed, err := ValidateEdit(resolvedRow(), underlying.Edit{Note: sp("watch")})
	if err != nil || !changed || next.Note != "watch" || next.ConID != 265598 {
		t.Errorf("note edit = %+v, %v, %v", next, changed, err)
	}
}

func expectField(t *testing.T, err error, field string) {
	t.Helper()
	if field == "" {
		if err != nil {
			t.Fatalf("error %v; want valid", err)
		}
		return
	}
	var ie underlying.InvalidError
	if !errors.As(err, &ie) || ie.Field() != field {
		t.Fatalf("error %v; want InvalidError with Field %q", err, field)
	}
}
