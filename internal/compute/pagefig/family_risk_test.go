package pagefig

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

func frPut(under, cur, strike, n, premium string) blockapi.ObligationIn {
	return blockapi.ObligationIn{Underlying: under, Currency: cur, Strike: strike, Multiplier: "100", Contracts: n, PremiumIncome: premium}
}

func frAcc(alias, cash string, obs ...blockapi.ObligationIn) blockapi.FamilyRiskAccountIn {
	return blockapi.FamilyRiskAccountIn{Alias: alias, Cash: hv(cash), Obligations: obs, Read: true}
}

func TestFamilyRiskNeedAndCashAfter(t *testing.T) {
	t.Parallel()
	r := NewFamilyRisk().FamilyRisk(blockapi.FamilyRiskIn{
		Accounts: []blockapi.FamilyRiskAccountIn{
			frAcc("main", "22000", frPut("AAA", "USD", "180", "1", "")),
			frAcc("wife", "6000", frPut("BBB", "USD", "100", "1", "")),
		},
		Net: hv("1000000"),
	})
	if r.ShortCount != 1 || r.Accounts[0].Short || !r.Accounts[1].Short {
		t.Fatalf("short %+v", r)
	}
	if r.Accounts[0].CashAfter.Value != "4000.00" || r.Accounts[1].CashAfter.Value != "-4000.00" || r.Accounts[1].Need.Value != "10000.00" {
		t.Fatalf("accounts %+v", r.Accounts)
	}
	if r.AfterTotal.Value != "0.00" || r.NeedTotal.Value != "28000.00" || r.CashTotal.Value != "28000" && r.CashTotal.Value != "28000.00" {
		t.Fatalf("totals %+v %+v %+v", r.NeedTotal, r.CashTotal, r.AfterTotal)
	}
	if r.NeedShare.Value != "2.80" || r.AfterShare.Value != "0.00" {
		t.Fatalf("shares %+v %+v", r.NeedShare, r.AfterShare)
	}
}

func TestFamilyRiskYen(t *testing.T) {
	t.Parallel()
	in := blockapi.FamilyRiskIn{
		Accounts: []blockapi.FamilyRiskAccountIn{frAcc("main", "20000", frPut("AAA", "JPY", "15000", "1", ""))},
		Net:      hv("100000"), UsdJpy: "150",
	}
	r := NewFamilyRisk().FamilyRisk(in)
	if r.Accounts[0].Need.Value != "10000.00" || r.NeedTotal.Value != "10000.00" || r.AfterTotal.Value != "10000.00" {
		t.Fatalf("with rate %+v", r)
	}
	in.UsdJpy = ""
	r = NewFamilyRisk().FamilyRisk(in)
	const g = "unsupported_currency"
	if r.Accounts[0].Need.Gap != g || r.NeedTotal.Gap != g || r.AfterTotal.Gap != g || r.Accounts[0].CashAfter.Gap != g {
		t.Fatalf("without rate %+v", r)
	}
}

func TestFamilyRiskUnreadAccount(t *testing.T) {
	t.Parallel()
	bad := frAcc("wife", "6000")
	bad.Read = false
	r := NewFamilyRisk().FamilyRisk(blockapi.FamilyRiskIn{
		Accounts: []blockapi.FamilyRiskAccountIn{frAcc("main", "22000", frPut("AAA", "USD", "180", "1", "")), bad},
		Net:      hv("1000"),
	})
	const g = "missing_input"
	if r.NeedTotal.Gap != g || r.CashTotal.Gap != g || r.AfterTotal.Gap != g {
		t.Fatalf("totals %+v", r)
	}
	if r.Accounts[1].Need.Gap != g || r.Accounts[1].CashAfter.Gap != g || r.Accounts[0].CashAfter.Value != "4000.00" {
		t.Fatalf("accounts %+v", r.Accounts)
	}
}

func TestFamilyRiskCompanies(t *testing.T) {
	t.Parallel()
	r := NewFamilyRisk().FamilyRisk(blockapi.FamilyRiskIn{
		Accounts: []blockapi.FamilyRiskAccountIn{
			frAcc("main", "99999", frPut("AAA", "USD", "180", "1", "100"), frPut("BBB", "USD", "100", "1", "50")),
			frAcc("wife", "99999", frPut("AAA", "USD", "180", "1", "40")),
		},
		Stocks: []blockapi.HeldLineIn{
			{Alias: "main", Underlying: "AAA", Currency: "USD", Value: hv("1000")},
			{Alias: "main", Underlying: "CCC", Currency: "USD", Value: compute.FigureValue{Gap: "missing_quote"}},
		},
		Net: hv("100000"),
	})
	if len(r.Puts) != 2 || r.Puts[0].Underlying != "AAA" || r.Puts[0].Cash.Value != "36000.00" ||
		len(r.Puts[0].Aliases) != 2 || r.Puts[0].Aliases[0] != "main" || r.Puts[0].Aliases[1] != "wife" || r.Puts[0].Premium != "140.00" {
		t.Fatalf("puts %+v", r.Puts)
	}
	if len(r.Companies) != 3 {
		t.Fatalf("companies %+v", r.Companies)
	}
	checkCompanies(t, r.Companies)
}

func checkCompanies(t *testing.T, cs []blockapi.FamilyRiskCompany) {
	t.Helper()
	a, b, c := cs[0], cs[1], cs[2]
	if a.Underlying != "AAA" || a.Stock.Value != "1000.00" || a.Put.Value != "36000.00" || a.Exposure.Value != "37000.00" || a.Share.Value != "37.00" {
		t.Fatalf("AAA %+v", a)
	}
	if b.Underlying != "BBB" || b.Stock.Value != "0.00" || b.Exposure.Value != "10000.00" || b.Share.Value != "10.00" {
		t.Fatalf("BBB %+v", b)
	}
	if c.Underlying != "CCC" || c.Stock.Gap != "missing_quote" || c.Exposure.Gap != "missing_quote" || c.Share.Gap == "" {
		t.Fatalf("CCC %+v", c)
	}
}

func TestFamilyRiskEqualsHousehold(t *testing.T) {
	t.Parallel()
	obs := []blockapi.ObligationIn{put("100", "100", "2", "10"), put("45.5", "100", "1", "5")}
	h := New().Household(blockapi.HouseholdInput{Values: vals("9000", "5000", "USD"), Obligations: obs})
	sum := new(big.Rat)
	for _, l := range h.Obligations {
		v, ok := parse(l.Amount.Value)
		if !ok {
			t.Fatalf("line %+v", l)
		}
		sum.Add(sum, v)
	}
	r := NewFamilyRisk().FamilyRisk(blockapi.FamilyRiskIn{
		Accounts: []blockapi.FamilyRiskAccountIn{frAcc("main", "5000", obs...)}, Net: hv("9000"),
	})
	if got, ok := parse(r.Accounts[0].Need.Value); !ok || got.Cmp(sum) != 0 {
		t.Fatalf("need %+v want %s", r.Accounts[0].Need, sum.RatString())
	}
	if got, want := r.Accounts[0].CashAfter.Value, h.CashAfter[0].Amount.Value; got != want {
		t.Fatalf("cash after %s want %s", got, want)
	}
}

func TestFamilyRiskCap(t *testing.T) {
	t.Parallel()
	var stocks []blockapi.HeldLineIn
	for i := 0; i < 61; i++ {
		stocks = append(stocks, blockapi.HeldLineIn{Alias: "main", Underlying: fmt.Sprintf("S%02d", i), Currency: "USD", Value: hv(fmt.Sprint(100 + i))})
	}
	r := NewFamilyRisk().FamilyRisk(blockapi.FamilyRiskIn{Accounts: []blockapi.FamilyRiskAccountIn{frAcc("main", "1")}, Stocks: stocks, Net: hv("100000")})
	if len(r.Companies) != 60 || r.Hidden != 1 || r.Companies[0].Underlying != "S60" {
		t.Fatalf("rows %d hidden %d first %s", len(r.Companies), r.Hidden, r.Companies[0].Underlying)
	}
}
