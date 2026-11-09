package optiontrack

import (
	"math/big"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionledger"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const asOf = contracts.Day("2026-11-09")

var defParams = trackapi.TrackParams{MarkMaxAgeSessions: 3, RecentDays: 30}

// pos is an open short put of AAPL, strike 100, expiring 2026-12-18.
func pos(id string, mod func(*compute.LedgerPosition)) compute.LedgerPosition {
	p := compute.LedgerPosition{
		PositionID: id, Alias: "main", Underlying: "AAPL", Status: "open", Right: "P",
		Expiry: "2026-12-18", Strike: "100", Quantity: "-1", Multiplier: "100", Currency: "USD",
		PremiumIncome: "150.0000", Commissions: "1.3000", OpenedOn: "2026-10-01",
	}
	if mod != nil {
		mod(&p)
	}
	return p
}

// quote is the quote of the default position with the given bid, ask and spot.
func quote(bid, ask, spot string) trackapi.TrackQuote {
	return trackapi.TrackQuote{
		Underlying: "AAPL", Expiry: "2026-12-18", Strike: "100", Right: "P", Day: asOf,
		Bid: bid, Ask: ask, UnderlyingClose: spot, Delta: "-0.2", Iv: "0.31",
	}
}

func input(ps []compute.LedgerPosition, qs ...trackapi.TrackQuote) trackapi.TrackInput {
	return trackapi.TrackInput{
		Alias: "main", AsOf: asOf, SessionDays: []contracts.Day{"2026-11-05", "2026-11-06", "2026-11-09"},
		Positions: ps, Quotes: qs,
	}
}

func one(t *testing.T, p compute.LedgerPosition, qs ...trackapi.TrackQuote) trackapi.TrackedOption {
	t.Helper()
	res := New().Track(input([]compute.LedgerPosition{p}, qs...), defParams)
	if len(res.Open) != 1 {
		t.Fatalf("want one open row: %+v", res)
	}
	return res.Open[0]
}

func TestShortsOnlyAreOpenRows(t *testing.T) {
	t.Parallel()
	ps := []compute.LedgerPosition{
		pos("short", nil),
		pos("long", func(p *compute.LedgerPosition) { p.Quantity = "1" }),
		pos("closed", func(p *compute.LedgerPosition) { p.Status = "closed"; p.ClosedOn = "2026-11-01" }),
	}
	res := New().Track(input(ps), defParams)
	if len(res.Open) != 1 || res.Open[0].PositionID != "short" {
		t.Fatalf("open rows: %+v", res.Open)
	}
}

func TestLongLegsCounted(t *testing.T) {
	t.Parallel()
	long := func(p *compute.LedgerPosition) { p.Quantity = "2" }
	res := New().Track(input([]compute.LedgerPosition{pos("a", long), pos("b", long), pos("c", nil)}), defParams)
	if res.LongLegsLeftOut != 2 || len(res.Open) != 1 {
		t.Fatalf("%+v", res)
	}
}

func TestRecentWindow(t *testing.T) {
	t.Parallel()
	closed := func(day contracts.Day, status string) compute.LedgerPosition {
		return pos(string(day), func(p *compute.LedgerPosition) { p.Status = status; p.ClosedOn = day })
	}
	ps := []compute.LedgerPosition{
		closed("2026-10-10", "expired"), // 30 days: in
		closed("2026-10-09", "expired"), // 31 days: out
		closed("2026-11-09", "assigned"),
		pos("longclosed", func(p *compute.LedgerPosition) { p.Status = "closed"; p.Quantity = "1"; p.ClosedOn = "2026-11-08" }),
	}
	res := New().Track(input(ps), defParams)
	if len(res.Recent) != 2 || res.Recent[0].PositionID != "2026-11-09" || res.Recent[1].PositionID != "2026-10-10" {
		t.Fatalf("recent: %+v", res.Recent)
	}
}

func TestOutputOrder(t *testing.T) {
	t.Parallel()
	at := func(id, under, exp, strike string) compute.LedgerPosition {
		return pos(id, func(p *compute.LedgerPosition) { p.Underlying, p.Expiry, p.Strike = under, exp, strike })
	}
	ps := []compute.LedgerPosition{
		at("5", "MSFT", "2026-12-18", "90"), at("4", "AAPL", "2026-12-18", "100"), at("3", "AAPL", "2026-12-18", "95"),
		at("2", "ZZZ", "2026-11-20", "500"), at("1", "AAPL", "2026-12-18", "9.5"), at("0", "AAPL", "2026-12-18", "100"),
	}
	res := New().Track(input(ps), defParams)
	var got []string
	for _, r := range res.Open {
		got = append(got, r.PositionID)
	}
	want := []string{"2", "1", "3", "0", "4", "5"}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("open order %v, want %v", got, want)
		}
	}
	closed := func(id string, day contracts.Day) compute.LedgerPosition {
		return pos(id, func(p *compute.LedgerPosition) { p.Status = "closed"; p.ClosedOn = day })
	}
	r2 := New().Track(input([]compute.LedgerPosition{closed("b", "2026-11-01"), closed("a", "2026-11-01"), closed("c", "2026-11-05")}), defParams)
	if r2.Recent[0].PositionID != "c" || r2.Recent[1].PositionID != "a" || r2.Recent[2].PositionID != "b" {
		t.Fatalf("recent order: %+v", r2.Recent)
	}
}

func TestAssignedKeepsPremiumAndStrikeSeparate(t *testing.T) {
	t.Parallel()
	p := pos("as", func(p *compute.LedgerPosition) {
		p.Status, p.Early, p.ClosedOn, p.RealizedPnL = "assigned", true, "2026-11-05", "148.7000"
	})
	res := New().Track(input([]compute.LedgerPosition{p}), defParams)
	if len(res.Recent) != 1 {
		t.Fatalf("%+v", res)
	}
	r := res.Recent[0]
	if r.PremiumIncome != "150.0000" || r.Strike != "100" || !r.Early || r.RealizedPnL != "148.7000" || r.Status != "assigned" {
		t.Fatalf("%+v", r)
	}
	// No field holds strike minus premium (98.5 per share, 9850 per contract).
	for _, v := range []string{r.Kept, r.Mid, r.Extrinsic, r.KeptPercent, r.Spot, r.OtmPercent, r.Moneyness} {
		if v == "98.5" || v == "98.5000" || v == "9850" || v == "9850.0000" {
			t.Fatalf("a basis was computed: %+v", r)
		}
	}
}

func TestKeptMatchesLedgerUnrealized(t *testing.T) {
	t.Parallel()
	trade := compute.LedgerTrade{TransactionID: "1", Revision: 1, Attributes: map[string]string{
		"assetCategory": "OPT", "symbol": "AAPL", "underlyingSymbol": "AAPL", "conid": "111",
		"strike": "100", "expiry": "20261218", "putCall": "P", "multiplier": "100",
		"quantity": "-2", "proceeds": "300", "ibCommission": "-1.30", "currency": "USD",
		"openCloseIndicator": "O", "tradeDate": "20261001", "orderTime": "20261001;100000",
	}}
	sessions := []contracts.Day{"2026-11-06", "2026-11-09"}
	led := optionledger.New().Replay(compute.LedgerInput{
		Alias: "main", AsOf: asOf, SessionDays: sessions, Trades: []compute.LedgerTrade{trade},
		Marks: []compute.LedgerMark{{Contract: "111", Day: asOf, Bid: "0.40", Ask: "0.55", UnderlyingClose: "104"}},
	}, compute.LedgerParams{MarkMaxAgeSessions: 3})
	lp := led.Positions[0]
	if lp.MarkStatus != "ok" {
		t.Fatalf("ledger: %+v", lp)
	}
	in := trackapi.TrackInput{
		AsOf: asOf, SessionDays: sessions, Positions: led.Positions,
		Quotes: []trackapi.TrackQuote{quote("0.40", "0.55", "104")},
	}
	rows := New().Track(in, defParams).Open
	if len(rows) != 1 {
		t.Fatalf("%+v", rows)
	}
	unreal, _ := new(big.Rat).SetString(lp.UnrealizedPnL)
	comm, _ := new(big.Rat).SetString(lp.Commissions)
	want := new(big.Rat).Add(unreal, comm)
	got, ok := new(big.Rat).SetString(rows[0].Kept)
	if !ok || got.Cmp(want) != 0 {
		t.Fatalf("Kept %q, ledger unrealized %s + commissions %s = %s", rows[0].Kept, lp.UnrealizedPnL, lp.Commissions, want.FloatString(4))
	}
	if rows[0].PremiumIncome != lp.PremiumIncome {
		t.Fatalf("premium %q vs %q", rows[0].PremiumIncome, lp.PremiumIncome)
	}
}
