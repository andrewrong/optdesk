package pagefig_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func tiles(in blockapi.OptionTilesIn) blockapi.OptionTilesOut {
	if in.Day == "" {
		in.Day = contracts.Day("2026-11-13")
	}
	return pagefig.NewOptionTiles().Tiles(in)
}

func lot(id, qty, opened, status string) compute.LedgerPosition {
	return compute.LedgerPosition{PositionID: id, Quantity: qty, OpenedOn: contracts.Day(opened), Status: status}
}

func leg(qty, price string) compute.StructureLeg {
	return compute.StructureLeg{Quantity: qty, OpenPrice: price, Multiplier: "100"}
}

func st(kind, ccy, maxLoss string, legs ...compute.StructureLeg) compute.Structure {
	s := compute.Structure{ID: kind, Kind: kind, Currency: ccy, Legs: legs}
	if maxLoss == "" {
		s.Figures.MaxLoss = compute.FigureValue{Gap: "no_rule"}
	} else {
		s.Figures.MaxLoss = compute.FigureValue{Value: maxLoss}
	}
	return s
}

func TestSoldCountMonth(t *testing.T) {
	t.Parallel()
	o := tiles(blockapi.OptionTilesIn{Positions: []compute.LedgerPosition{
		lot("a:t1:1", "-1", "2026-11-03", "closed"),
		lot("a:t1:2", "-1", "2026-11-03", "open"), // same lot, second piece
		lot("a:t2:1", "-2", "2026-10-30", "open"), // month before
		lot("a:t3:1", "2", "2026-11-04", "open"),  // bought
		lot("a:t4:1", "-1", "2026-11-14", "open"), // after the day
		lot("a:t5:1", "x", "2026-11-05", "open"),  // unparsable
		lot("b:t6:1", "-1", "2026-11-13", "expired"),
	}})
	if o.SoldCount != 2 {
		t.Fatalf("SoldCount = %d", o.SoldCount)
	}
}

func TestOpenPremiumSpreadNet(t *testing.T) {
	t.Parallel()
	o := tiles(blockapi.OptionTilesIn{Structures: []compute.Structure{
		st("put_spread", "USD", "-300", leg("-1", "2.50"), leg("1", "1.00")),
	}})
	if o.OpenPremium != (blockapi.UsdText{Value: "150.00", Dollars: "150"}) || o.OpenPremiumGap != "" {
		t.Fatalf("%+v", o)
	}
}

func TestOpenPremiumRemainingPieceOnly(t *testing.T) {
	t.Parallel()
	o := tiles(blockapi.OptionTilesIn{Structures: []compute.Structure{
		st("short_put", "USD", "-1000", leg("-1", "3.10")), // lot of 3 closed down to 1 left
	}})
	if o.OpenPremium.Value != "310.00" {
		t.Fatalf("%+v", o.OpenPremium)
	}
}

func TestOpenPremiumNetPaid(t *testing.T) {
	t.Parallel()
	o := tiles(blockapi.OptionTilesIn{Structures: []compute.Structure{
		st("other", "USD", "", leg("-1", "1.00"), leg("2", "2.00")),
	}})
	if o.OpenPremiumGap != "no_data" || o.OpenPremium != (blockapi.UsdText{}) {
		t.Fatalf("%+v", o)
	}
}

func TestMaxLossShortPutAndSpread(t *testing.T) {
	t.Parallel()
	o := tiles(blockapi.OptionTilesIn{Structures: []compute.Structure{
		st("short_put", "USD", "-4500.50", leg("-1", "1")),
		st("put_spread", "USD", "-300", leg("-1", "2"), leg("1", "1")),
		st("long_call", "USD", "-100", leg("1", "1")),
	}})
	if o.MaxLossUsd != (blockapi.UsdText{Value: "4900.50", Dollars: "4901"}) || o.MaxLossGap != "" || o.OpenCount != 3 {
		t.Fatalf("%+v", o)
	}
	// long_call has no short leg: not in the premium; 100 + 100 = 200.
	if o.OpenPremium.Value != "200.00" {
		t.Fatalf("premium %+v", o.OpenPremium)
	}
}

func TestMaxLossUncoveredCallGap(t *testing.T) {
	t.Parallel()
	o := tiles(blockapi.OptionTilesIn{Structures: []compute.Structure{
		st("naked_call", "USD", "", leg("-1", "1")),
		st("short_put", "USD", "-200", leg("-1", "1")),
	}})
	if o.MaxLossGap != "no_data" || o.Uncovered != 1 || o.Unnumbered != 0 || o.MaxLossUsd.Value != "200.00" {
		t.Fatalf("%+v", o)
	}
	o = tiles(blockapi.OptionTilesIn{Structures: []compute.Structure{st("naked_call", "USD", "", leg("-1", "1"))}})
	if o.MaxLossGap != "no_data" || o.MaxLossUsd != (blockapi.UsdText{}) {
		t.Fatalf("alone %+v", o)
	}
}

func TestMaxLossUnnumbered(t *testing.T) {
	t.Parallel()
	o := tiles(blockapi.OptionTilesIn{Structures: []compute.Structure{
		st("covered_call", "USD", "", leg("-1", "1")),
		st("other", "USD", "", leg("1", "2"), leg("-1", "1")),
		st("short_put", "USD", "-50", leg("-1", "1")),
	}})
	if o.Unnumbered != 2 || o.MaxLossGap != "" || o.MaxLossUsd.Value != "50.00" {
		t.Fatalf("%+v", o)
	}
}

func TestOtherCurrencyNotSummed(t *testing.T) {
	t.Parallel()
	o := tiles(blockapi.OptionTilesIn{Structures: []compute.Structure{
		st("short_put", "JPY", "-99999", leg("-1", "100")),
		st("short_put", "USD", "-50", leg("-1", "1")),
	}})
	if o.OtherCurrency != 1 || o.OpenCount != 2 || o.MaxLossUsd.Value != "50.00" || o.OpenPremium.Value != "100.00" {
		t.Fatalf("%+v", o)
	}
}

func TestOnlyNonUsdIsUnsupportedGap(t *testing.T) {
	t.Parallel()
	o := tiles(blockapi.OptionTilesIn{Structures: []compute.Structure{
		st("short_put", "JPY", "", leg("-1", "100")),
		st("short_put", "", "", leg("-1", "100")),
	}})
	if o.OpenPremiumGap != "unsupported_currency" || o.MaxLossGap != "unsupported_currency" || o.OtherCurrency != 2 || o.OpenCount != 2 {
		t.Fatalf("%+v", o)
	}
}

func TestUnreadableLegNotSummed(t *testing.T) {
	t.Parallel()
	o := tiles(blockapi.OptionTilesIn{Structures: []compute.Structure{
		st("short_put", "USD", "-70", leg("-1", "bad")),
		st("short_put", "USD", "-50", leg("-1", "1")),
	}})
	if o.Unreadable != 1 || o.MaxLossUsd.Value != "50.00" || o.OpenPremium.Value != "100.00" {
		t.Fatalf("%+v", o)
	}
}

func TestHalfCentRounding(t *testing.T) {
	t.Parallel()
	// Two parts of 0.004: each rounds to 0.00, the exact sum 0.008 rounds to 0.01.
	o := tiles(blockapi.OptionTilesIn{Structures: []compute.Structure{
		st("short_put", "USD", "-0.004", leg("-1", "0.00004")),
		st("short_put", "USD", "-0.004", leg("-1", "0.00004")),
	}})
	if o.MaxLossUsd != (blockapi.UsdText{Value: "0.01", Dollars: "0"}) || o.OpenPremium.Value != "0.01" {
		t.Fatalf("%+v", o)
	}
	// 0.5 dollars rounds half up to 1; 0.005 to 0.01.
	o = tiles(blockapi.OptionTilesIn{Structures: []compute.Structure{st("short_put", "USD", "-0.5", leg("-1", "1"))}})
	if o.MaxLossUsd != (blockapi.UsdText{Value: "0.50", Dollars: "1"}) {
		t.Fatalf("%+v", o.MaxLossUsd)
	}
	o = tiles(blockapi.OptionTilesIn{Structures: []compute.Structure{st("short_put", "USD", "-0.005", leg("-1", "1"))}})
	if o.MaxLossUsd.Value != "0.01" {
		t.Fatalf("%+v", o.MaxLossUsd)
	}
}

func TestEmptyInputZero(t *testing.T) {
	t.Parallel()
	o := tiles(blockapi.OptionTilesIn{})
	zero := blockapi.UsdText{Value: "0.00", Dollars: "0"}
	if o.SoldCount != 0 || o.OpenPremium != zero || o.MaxLossUsd != zero || o.OpenPremiumGap != "" || o.MaxLossGap != "" || o.OpenCount != 0 {
		t.Fatalf("%+v", o)
	}
}
