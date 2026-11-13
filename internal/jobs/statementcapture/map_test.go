package statementcapture

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const day = "2026-11-05"

func rowOf(a attrs) row { return row{key: "k", a: a} }

func stockAttrs() attrs {
	return attrs{
		"reportDate": "20261105", "levelOfDetail": "SUMMARY", "conid": "1", "assetCategory": "STK", "symbol": "XYZ",
		"currency": "USD", "position": "100", "costBasisPrice": "50.5", "markPrice": "52.25",
	}
}

func optionAttrs() attrs {
	return attrs{
		"reportDate": "20261105", "levelOfDetail": "SUMMARY", "conid": "2", "assetCategory": "OPT", "symbol": "XYZ 261120P00050000",
		"underlyingSymbol": "XYZ", "currency": "USD", "position": "-2", "costBasisPrice": "1.25", "markPrice": "0.9",
		"multiplier": "100", "strike": "50", "expiry": "2026-11-20", "putCall": "C",
	}
}

func mapOne(t *testing.T, a attrs) (mappedDay, error) {
	t.Helper()
	eq := rowOf(attrs{"total": "10", "cash": "1"})
	return mapDay(day, []row{rowOf(a)}, eq, "USD")
}

func TestOptionSymbolIsUnderlyingSymbol(t *testing.T) {
	t.Parallel()
	md, err := mapOne(t, optionAttrs())
	if err != nil || md.positions[0].pos.Symbol != "XYZ" {
		t.Fatalf("%+v %v", md.positions, err)
	}
}

func TestOptionWithoutUnderlyingSymbolRefusesTheDay(t *testing.T) {
	t.Parallel()
	a := optionAttrs()
	delete(a, "underlyingSymbol")
	_, err := mapOne(t, a)
	te, ok := err.(contracts.TypedError) //nolint:errorlint // the mapping returns typedError values
	if !ok || te.Kind() != contracts.Permanent || !strings.Contains(err.Error(), "underlyingSymbol") || !strings.Contains(err.Error(), day) {
		t.Fatalf("err %v", err)
	}
}

func TestStockSymbolIsSymbol(t *testing.T) {
	t.Parallel()
	md, err := mapOne(t, stockAttrs())
	p := md.positions[0].pos
	if err != nil || p.Symbol != "XYZ" || p.SecType != "stock" || p.Multiplier != "1" || p.Quantity != "100" || p.Currency != "USD" {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestOptionAvgCostIsCostTimesMultiplier(t *testing.T) {
	t.Parallel()
	md, err := mapOne(t, optionAttrs())
	if err != nil || md.positions[0].pos.AvgCost != "125" {
		t.Fatalf("%+v %v", md.positions, err)
	}
}

func TestStockAvgCostIsCostPrice(t *testing.T) {
	t.Parallel()
	md, err := mapOne(t, stockAttrs())
	if err != nil || md.positions[0].pos.AvgCost != "50.5" || md.positions[0].mark != "52.25" {
		t.Fatalf("%+v %v", md.positions, err)
	}
}

func TestExpiryAndRightMapping(t *testing.T) {
	t.Parallel()
	for pc, want := range map[string]string{"P": "put", "C": "call"} {
		a := optionAttrs()
		a["putCall"] = pc
		a["expiry"] = "20261120"
		md, err := mapOne(t, a)
		p := md.positions[0].pos
		if err != nil || p.Right != want || p.Expiry != "2026-11-20" || p.Strike != "50" || p.Multiplier != "100" {
			t.Fatalf("%s: %+v %v", pc, p, err)
		}
	}
	a := optionAttrs()
	a["putCall"] = "X"
	if _, err := mapOne(t, a); err == nil {
		t.Fatal("another right must refuse the day")
	}
}

func TestOtherCategoryIsSkippedAndCounted(t *testing.T) {
	t.Parallel()
	a := stockAttrs()
	a["assetCategory"] = "CASH"
	md, err := mapOne(t, a)
	if err != nil || len(md.positions) != 0 || md.skipped.category != 1 {
		t.Fatalf("%+v %v", md, err)
	}
}

func TestNetLiquidationAndCashFromEquitySummary(t *testing.T) {
	t.Parallel()
	md, err := mapDay(day, []row{rowOf(stockAttrs())}, rowOf(attrs{"total": "123.45", "cash": "-6"}), "JPY")
	if err != nil || len(md.values) != 2 {
		t.Fatalf("%+v %v", md, err)
	}
	if md.values[0].Tag != "NetLiquidation" || md.values[0].Value != "123.45" || md.values[0].Currency != "JPY" ||
		md.values[1].Tag != "TotalCashValue" || md.values[1].Value != "-6" || md.values[1].Currency != "JPY" {
		t.Fatalf("values %+v", md.values)
	}
}
