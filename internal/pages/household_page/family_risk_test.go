package household_page_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/pages/household_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// riskAccount answers the stock positions by alias.
type riskAccount struct {
	*fakeAccount
	positions map[string][]store.Position
}

func (r riskAccount) Positions(_ context.Context, alias, _ string) ([]store.Position, error) {
	return r.positions[alias], nil
}

// riskLedger fails to read the ledger of one alias.
type riskLedger struct {
	fakeLedger
	fail string
}

func (l riskLedger) FlexTradeRows(ctx context.Context, alias string) ([]store.FlexTradeRow, error) {
	if alias == l.fail {
		return nil, errors.New("boom")
	}
	return l.fakeLedger.FlexTradeRows(ctx, alias)
}

// riskReplayer gives each alias its open short puts, by the alias of the input.
type riskReplayer map[string][]compute.LedgerPosition

func (r riskReplayer) Replay(in compute.LedgerInput, _ compute.LedgerParams) compute.LedgerResult {
	return compute.LedgerResult{Positions: r[in.Alias]}
}

func usdPut(sym, strike, qty string) compute.LedgerPosition {
	return compute.LedgerPosition{Underlying: sym, Status: "open", Right: "P", Quantity: qty, Strike: strike,
		Multiplier: "100", Currency: "USD", PremiumIncome: "100"}
}

// riskRig is a book of main (cash 42000, 2 AAA puts at 50 = 10000 needed, 100 AAA shares at 50) and wife
// (cash 1000, 1 BBB put at 50 = 5000 needed, so 4000 short), with the family net 105000.
func riskRig() famRig {
	res := famResult("105000.00", "43000.00", "40.95",
		counted("main", "2026-11-10", "100000.00", "42000.00"), counted("wife", "2026-11-10", "5000.00", "1000.00"))
	r := rig(res, "main", "wife")
	r.c.Account = riskAccount{fakeAccount: r.acc, positions: map[string][]store.Position{
		"main": {hstock("AAA", 1, "100", "USD")},
	}}
	r.c.Ledger = riskLedger{}
	r.c.Ledgers = func() compute.OptionLedger {
		return riskReplayer{"main": {usdPut("AAA", "50", "-2")}, "wife": {usdPut("BBB", "50", "-1")}}
	}
	r.c.Marks = heldMarks{"main 2026-11-10": {hmark(1, "50", "USD")}}
	r.c.Holdings = pagefig.NewHoldings
	r.c.FamilyRisk = pagefig.NewFamilyRisk
	return r
}

func block(m map[string]any, name string) map[string]any { return m[name].(map[string]any) }

func blockRows(m map[string]any, name string) []map[string]any {
	var out []map[string]any
	for _, r := range block(m, name)["rows"].([]any) {
		out = append(out, r.(map[string]any))
	}
	return out
}

func rowText(r map[string]any) string { return r["amount"].(map[string]any)["text"].(string) }

func totalTexts(m map[string]any, name string) []string {
	var out []string
	for _, c := range block(m, name)["totals"].([]any) {
		out = append(out, c.(map[string]any)["text"].(string))
	}
	return out
}

func TestFamilyRiskBlocks(t *testing.T) {
	t.Parallel()
	m, b := riskRig().build(t)
	validate(t, b)

	for name, prefix := range map[string]string{"obligations": "家庭风险", "cash_after": "家庭风险", "stress": "家庭风险", "concentration": "家庭风险"} {
		if title := block(m, name)["title"].(string); !strings.HasPrefix(title, prefix) {
			t.Errorf("%s title = %q", name, title)
		}
		if g := block(m, name)["gap"]; g != nil {
			t.Errorf("%s has a gap: %v", name, g)
		}
	}

	checkObligations(t, m)
	checkCashAfter(t, m)
	checkStress(t, m)
	checkConcentration(t, m)
	if !has(strs(m["lead_notes"]), "有 1 个账户现金不够") {
		t.Errorf("lead_notes = %v", m["lead_notes"])
	}
	if has(strs(m["lead_notes"]), "不含") {
		t.Errorf("no account is left out: %v", m["lead_notes"])
	}
	for _, g := range m["gaps"].([]any) {
		if txt := g.(map[string]any)["text"].(string); strings.Contains(txt, "Obligations") && g.(map[string]any)["reason"] == "not_built" {
			t.Errorf("not_built gap for a built block: %v", g)
		}
	}
}

func checkObligations(t *testing.T, m map[string]any) {
	t.Helper()
	ob := blockRows(m, "obligations")
	if len(ob) != 2 || ob[0]["name"] != "AAA 卖出看跌" || rowText(ob[0]) != "$10,000.00" || ob[1]["name"] != "BBB 卖出看跌" || rowText(ob[1]) != "$5,000.00" {
		t.Fatalf("obligations rows = %v", ob)
	}
	if note := ob[0]["note"].(string); note != "账户（main）；权利金 $100.00" {
		t.Errorf("obligation note = %q", note)
	}
	if tt := totalTexts(m, "obligations"); len(tt) != 1 || tt[0] != "$15,000.00" {
		t.Errorf("obligations totals = %v", tt)
	}
}

func checkCashAfter(t *testing.T, m map[string]any) {
	t.Helper()
	ca := blockRows(m, "cash_after")
	if len(ca) != 2 || ca[0]["scope"] != "main" || rowText(ca[0]) != "$32,000.00" || ca[1]["scope"] != "wife" || rowText(ca[1]) != "−$4,000.00" {
		t.Fatalf("cash_after rows = %v", ca)
	}
	if note := ca[1]["note"].(string); !strings.Contains(note, "缺口：现金不够") {
		t.Errorf("short row note = %q", note)
	}
	if note, _ := ca[0]["note"].(string); strings.Contains(note, "缺口") {
		t.Errorf("covered row note = %q", note)
	}
	if tt := totalTexts(m, "cash_after"); len(tt) != 1 || tt[0] != "$28,000.00" {
		t.Errorf("cash_after totals = %v", tt)
	}
}

func checkStress(t *testing.T, m map[string]any) {
	t.Helper()
	st := blockRows(m, "stress")
	if len(st) != 3 || rowText(st[0]) != "$15,000.00" || rowText(st[1]) != "$43,000.00" || rowText(st[2]) != "$28,000.00" || st[0]["share"] == nil {
		t.Fatalf("stress rows = %v", st)
	}
	if !strings.Contains(block(m, "stress")["meaning"].(string), "没有存下标的最新价") {
		t.Errorf("stress meaning = %v", block(m, "stress")["meaning"])
	}
}

func checkConcentration(t *testing.T, m map[string]any) {
	t.Helper()
	co := blockRows(m, "concentration")
	if len(co) != 2 || co[0]["name"] != "AAA" || rowText(co[0]) != "$15,000.00" || co[0]["scope"] != "household" ||
		co[0]["note"] != "股票 $5,000.00 + 指派 $10,000.00" || co[0]["share"] == nil || co[1]["name"] != "BBB" || rowText(co[1]) != "$5,000.00" {
		t.Fatalf("concentration rows = %v", co)
	}
	if !strings.Contains(block(m, "concentration")["meaning"].(string), "现金结算") {
		t.Errorf("concentration meaning lacks the index note")
	}
}

func TestFamilyRiskNilFigure(t *testing.T) {
	t.Parallel()
	r := riskRig()
	r.c.FamilyRisk = nil
	m, _ := r.build(t)
	for _, name := range []string{"obligations", "cash_after", "stress", "concentration"} {
		if g := gapOf(t, m, name); g["reason"] != "not_built" {
			t.Errorf("%s gap = %v", name, g)
		}
	}
}

func TestFamilyRiskAccountReadError(t *testing.T) {
	t.Parallel()
	r := riskRig()
	r.c.Ledger = riskLedger{fail: "wife"}
	m, b := r.build(t)
	validate(t, b)
	ca := blockRows(m, "cash_after")
	if len(ca) != 1 || ca[0]["scope"] != "main" {
		t.Fatalf("cash_after rows = %v", ca)
	}
	for _, name := range []string{"obligations", "cash_after", "stress", "concentration"} {
		g := gapOf(t, m, name)
		if !strings.Contains(g["text"].(string), "账户（wife）读取失败") {
			t.Errorf("%s gap = %v", name, g)
		}
		if tt := totalTexts(m, name); len(tt) != 0 {
			t.Errorf("%s has a total with an account missing: %v", name, tt)
		}
	}
	if ob := blockRows(m, "obligations"); len(ob) != 1 || ob[0]["name"] != "AAA 卖出看跌" {
		t.Errorf("obligations rows = %v", ob)
	}
}

func TestFamilyRiskFirstAliasReadErrorFails(t *testing.T) {
	t.Parallel()
	r := riskRig()
	r.c.Ledger = riskLedger{fail: "main"}
	if _, err := household_page.BuildFamily(context.Background(), r.c, day, at); err == nil {
		t.Fatal("no error for the first alias")
	}
}

func TestFamilyRiskNoMarks(t *testing.T) {
	t.Parallel()
	for name, cut := range map[string]func(*household_page.Config){
		"marks":    func(c *household_page.Config) { c.Marks = nil },
		"holdings": func(c *household_page.Config) { c.Holdings = nil },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := riskRig()
			cut(&r.c)
			m, b := r.build(t)
			validate(t, b)
			if g := gapOf(t, m, "concentration"); g["reason"] != "no_reader" {
				t.Errorf("concentration gap = %v", g)
			}
			for _, n := range []string{"obligations", "cash_after", "stress"} {
				if g := block(m, n)["gap"]; g != nil {
					t.Errorf("%s has a gap: %v", n, g)
				}
				if len(blockRows(m, n)) == 0 {
					t.Errorf("%s has no rows", n)
				}
			}
		})
	}
}

func TestFamilyRiskCountsAccountsLeftOut(t *testing.T) {
	t.Parallel()
	res := famResult("100000.00", "42000.00", "42",
		counted("main", "2026-11-10", "100000.00", "42000.00"), left("wife", "no_data", ""))
	r := riskRig()
	r.c.Family = func() blockapi.FamilyFigure { return fakeFamily{res: res, got: r.in, calls: r.calls} }
	m, _ := r.build(t)
	if !has(strs(m["lead_notes"]), "不含 1 个账户") {
		t.Fatalf("lead_notes = %v", m["lead_notes"])
	}
	if ca := blockRows(m, "cash_after"); len(ca) != 1 || ca[0]["scope"] != "main" {
		t.Errorf("cash_after rows = %v", ca)
	}
}

func TestFamilyRiskMatchesAccountPage(t *testing.T) {
	t.Parallel()
	r := riskRig()
	fam, _ := r.build(t)
	// the page of one account, with the figure that gives its cash after assignment the same arithmetic
	c := r.c
	c.Figures = pagefig.New
	acct, b := build(t, c, "main")
	validate(t, b)
	want := rowText(blockRows(acct, "cash_after")[0])
	if got := rowText(blockRows(fam, "cash_after")[0]); got != want {
		t.Fatalf("family cash_after of main = %s, account page = %s", got, want)
	}
}
