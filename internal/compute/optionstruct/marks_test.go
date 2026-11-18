package optionstruct

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optiontrack"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var sessionDays = []contracts.Day{"2026-11-02", "2026-11-03", "2026-11-04", "2026-11-05", "2026-11-06"}

func baseParams() compute.StructureParams {
	return compute.StructureParams{
		MarkMaxAgeSessions: 1, CaptureTarget: DefaultCaptureTarget,
		ExitDTE: DefaultExitDTE, CommissionDays: 30,
	}
}

// quotesOf reads the quotes of one case of a testdata file.
func quotesOf(t *testing.T, file, name string) []compute.StructureQuote {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []struct {
			Name   string
			Quotes []compute.StructureQuote
		}
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, c := range doc.Cases {
		if c.Name == name {
			return c.Quotes
		}
	}
	t.Fatalf("no case %s in %s", name, file)
	return nil
}

// quote is a quote of AAPL on 2026-11-06 with the underlying close 100.
func quote(right, strike, expiry, bid, ask string) compute.StructureQuote {
	return compute.StructureQuote{
		Underlying: "AAPL", Right: right, Strike: strike, Expiry: expiry, Day: "2026-11-06",
		Bid: bid, Ask: ask, UnderlyingClose: "100",
	}
}

func evalMarked(positions []compute.LedgerPosition, quotes []compute.StructureQuote, p compute.StructureParams) compute.StructureResult {
	return New().Evaluate(compute.StructureInput{
		Alias: "acct", Positions: positions, Quotes: quotes, AsOf: "2026-11-06", SessionDays: sessionDays,
	}, p)
}

// shortPut is the short put 90 of AAPL at 1.50 opened for one contract.
func shortPut() compute.LedgerPosition { return pos("s1", "P", "90", "-1", "1.50") }

func firstOf(t *testing.T, res compute.StructureResult) compute.Structure {
	t.Helper()
	if len(res.Structures) != 1 {
		t.Fatalf("structures = %d, want 1", len(res.Structures))
	}
	return res.Structures[0]
}

func TestMidOfBidAsk(t *testing.T) {
	t.Parallel()
	s := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()},
		[]compute.StructureQuote{quote("P", "90", "2026-12-18", "1.00", "1.20")}, baseParams()))
	if s.Legs[0].Mid != "1.1000" {
		t.Errorf("mid = %q, want 1.1000", s.Legs[0].Mid)
	}
	odd := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()},
		[]compute.StructureQuote{quote("P", "90", "2026-12-18", "1.0001", "1.0002")}, baseParams()))
	if odd.Legs[0].Mid != "1.0002" { // 1.00015 is a tie: half to even gives 1.0002
		t.Errorf("tie mid = %q, want 1.0002", odd.Legs[0].Mid)
	}
}

func TestZeroBidHasNoMark(t *testing.T) {
	t.Parallel()
	s := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()},
		[]compute.StructureQuote{quote("P", "90", "2026-12-18", "0", "0.05")}, baseParams()))
	if s.Legs[0].Mid != "" || s.Figures.UnrealizedPnL != (compute.FigureValue{Gap: "missing_quote"}) {
		t.Errorf("mid %q pnl %+v, want no mark and gap missing_quote", s.Legs[0].Mid, s.Figures.UnrealizedPnL)
	}
}

func TestStaleMarkIsGap(t *testing.T) {
	t.Parallel()
	q := quote("P", "90", "2026-12-18", "1.00", "1.20")
	q.Day = "2026-11-02" // 4 session days before 2026-11-06, the limit is 1
	s := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()}, []compute.StructureQuote{q}, baseParams()))
	if s.Figures.UnrealizedPnL != (compute.FigureValue{Gap: "stale_input"}) || s.Legs[0].Mid != "" {
		t.Errorf("pnl = %+v mid %q, want gap stale_input", s.Figures.UnrealizedPnL, s.Legs[0].Mid)
	}
}

func TestLegPnLSigned(t *testing.T) {
	t.Parallel()
	qs := []compute.StructureQuote{
		quote("P", "90", "2026-12-18", "1.00", "1.20"), // mid 1.10
		quote("P", "85", "2026-12-18", "0.30", "0.50"), // mid 0.40
	}
	res := evalMarked([]compute.LedgerPosition{shortPut(), pos("b1", "P", "85", "1", "0.50")}, qs, baseParams())
	s := firstOf(t, res)
	got := map[string]string{}
	for _, l := range s.Legs {
		got[l.PositionID] = l.LegPnL
	}
	// short: -1 x 100 x (1.10 - 1.50) = +40; long: 1 x 100 x (0.40 - 0.50) = -10; total +30.
	if got["s1"] != "40.0000" || got["b1"] != "-10.0000" || s.Figures.UnrealizedPnL.Value != "30.0000" {
		t.Errorf("leg pnl %v total %+v", got, s.Figures.UnrealizedPnL)
	}
}

func TestEarnedShare(t *testing.T) {
	t.Parallel()
	s := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()},
		[]compute.StructureQuote{quote("P", "90", "2026-12-18", "0.55", "0.65")}, baseParams()))
	if s.Figures.EarnedShare.Value != "0.600000" { // credit 1.50, mark 0.60
		t.Errorf("earned = %+v, want 0.600000", s.Figures.EarnedShare)
	}
	loss := firstOf(t, evalMarked([]compute.LedgerPosition{pos("s1", "P", "90", "-1", "1.00")},
		[]compute.StructureQuote{quote("P", "90", "2026-12-18", "1.15", "1.25")}, baseParams()))
	if loss.Figures.EarnedShare.Value != "-0.200000" {
		t.Errorf("earned = %+v, want -0.200000", loss.Figures.EarnedShare)
	}
}

func TestExpiryDistanceText(t *testing.T) {
	t.Parallel()
	q := quote("P", "50", "2026-12-18", "0.55", "0.65")
	q.UnderlyingClose = "52"
	s := firstOf(t, evalMarked([]compute.LedgerPosition{pos("s1", "P", "50", "-1", "1.50")},
		[]compute.StructureQuote{q}, baseParams()))
	if s.Figures.ExpiryDistance.Value != "0.038462" { // (52 - 50) / 52
		t.Errorf("expiry distance = %+v", s.Figures.ExpiryDistance)
	}
	var strike, be string
	for _, l := range s.Levels {
		switch l.Name {
		case "strike":
			strike = l.Distance
		case "break_even":
			be = l.Distance
		}
	}
	if strike != "-0.038462" || be != "-0.067308" { // break-even 48.5
		t.Errorf("level distances %q %q", strike, be)
	}
}

func TestQuoteMatchedByTerms(t *testing.T) {
	t.Parallel()
	qs := quotesOf(t, "marked.json", "terms")
	s := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()}, qs, baseParams()))
	if s.Legs[0].Mid != "1.1000" { // only the P 90 2026-12-18 AAPL row, whose strike reads 90.0
		t.Errorf("mid = %q, want 1.1000", s.Legs[0].Mid)
	}
	other := firstOf(t, evalMarked([]compute.LedgerPosition{pos("s1", "P", "80", "-1", "1.50")}, qs, baseParams()))
	if other.Figures.UnrealizedPnL != (compute.FigureValue{Gap: "missing_quote"}) {
		t.Errorf("unmatched leg pnl = %+v, want gap missing_quote", other.Figures.UnrealizedPnL)
	}
}

func TestDefaultsNamed(t *testing.T) {
	t.Parallel()
	if DefaultCaptureTarget != "0.50" || DefaultExitDTE != 21 {
		t.Errorf("defaults %q %d", DefaultCaptureTarget, DefaultExitDTE)
	}
	s := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()}, nil, compute.StructureParams{
		CaptureTarget: DefaultCaptureTarget, ExitDTE: DefaultExitDTE,
	}))
	if s.CaptureParam != "0.50" || s.ExitDTEParam != "21" {
		t.Errorf("params %q %q, want the passed ones", s.CaptureParam, s.ExitDTEParam)
	}
}

func TestDecideRule(t *testing.T) {
	t.Parallel()
	decide := func(expiry, bid, ask string) bool {
		p := shortPut()
		p.Expiry = expiry
		return firstOf(t, evalMarked([]compute.LedgerPosition{p},
			[]compute.StructureQuote{quote("P", "90", expiry, bid, ask)}, baseParams())).Decide
	}
	if !decide("2026-12-18", "0.55", "0.65") { // earned 0.60 at 42 days
		t.Error("earned 60 percent: Decide = false")
	}
	if decide("2026-12-18", "1.05", "1.15") { // earned 0.266667 at 42 days
		t.Error("earned 27 percent at 42 days: Decide = true")
	}
	if !decide("2026-11-27", "1.05", "1.15") { // 21 days left
		t.Error("21 days left: Decide = false")
	}
	if decide("2026-11-28", "1.05", "1.15") { // 22 days left
		t.Error("22 days left: Decide = true")
	}
}

func TestCompareOnlySingleShortLeg(t *testing.T) {
	t.Parallel()
	qs := []compute.StructureQuote{
		quote("P", "90", "2026-12-18", "0.55", "0.65"),
		quote("P", "85", "2026-12-18", "0.30", "0.50"),
	}
	single := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()}, qs, baseParams()))
	if single.CaptureParam != "0.50" || single.ExitDTEParam != "21" || single.Figures.CloseCost.Value != "60.0000" {
		t.Errorf("single short: %q %q %+v", single.CaptureParam, single.ExitDTEParam, single.Figures.CloseCost)
	}
	spread := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut(), pos("b1", "P", "85", "1", "0.50")}, qs, baseParams()))
	longPut := firstOf(t, evalMarked([]compute.LedgerPosition{pos("b1", "P", "85", "1", "0.50")}, qs, baseParams()))
	for name, s := range map[string]compute.Structure{"spread": spread, "long": longPut} {
		if s.CaptureParam != "" || s.ExitDTEParam != "" || s.Figures.CloseCost != noData ||
			s.Figures.RollNet != noData || s.Figures.CashAfter != noData {
			t.Errorf("%s has a compare row: %+v", name, s)
		}
	}
}

func TestRecentCommissionWindow(t *testing.T) {
	t.Parallel()
	p := shortPut()
	p.Fills = []compute.LedgerFill{
		{Day: "2026-10-20", Action: "open", Quantity: "-1", Price: "1.50", Commission: "0.6500"},
		{Day: "2026-10-07", Action: "open", Quantity: "-1", Price: "1.50", Commission: "0.1000"},
		{Day: "2026-10-06", Action: "open", Quantity: "-1", Price: "1.50", Commission: "5.0000"},
	}
	s := firstOf(t, evalMarked([]compute.LedgerPosition{p}, nil, baseParams()))
	if s.Figures.RecentCommission.Value != "0.7500" { // 30 days back from 2026-11-06 is 2026-10-07
		t.Errorf("recent commission = %+v, want 0.7500", s.Figures.RecentCommission)
	}
	none := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()}, nil, baseParams()))
	if none.Figures.RecentCommission != noData {
		t.Errorf("no rows = %+v, want no_data", none.Figures.RecentCommission)
	}
}

func TestExpiryDistanceIsShareOfClose(t *testing.T) {
	t.Parallel()
	cases := []struct{ right, strike, close, want string }{
		{"P", "340", "359.24", "0.053558"},
		{"C", "25", "20", "0.250000"},
	}
	for _, c := range cases {
		q := quote(c.right, c.strike, "2026-12-18", "0.55", "0.65")
		q.UnderlyingClose = c.close
		s := firstOf(t, evalMarked([]compute.LedgerPosition{pos("s1", c.right, c.strike, "-1", "1.50")},
			[]compute.StructureQuote{q}, baseParams()))
		if got := s.Figures.ExpiryDistance.Value; got != c.want {
			t.Errorf("%s strike %s close %s: %q want %q", c.right, c.strike, c.close, got, c.want)
		}
	}
}

// TestTrackAndStructureAgree: the two rules give one distance, the percent of the track
// divided by 100 is the ratio of the structure.
func TestTrackAndStructureAgree(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ right, strike, close string }{{"P", "340", "359.24"}, {"C", "25", "20"}, {"P", "380", "367.71"}} {
		lp := pos("s1", c.right, c.strike, "-1", "1.50")
		q := quote(c.right, c.strike, "2026-12-18", "0.55", "0.65")
		q.UnderlyingClose = c.close
		st := firstOf(t, evalMarked([]compute.LedgerPosition{lp}, []compute.StructureQuote{q}, baseParams()))

		res := optiontrack.New().Track(trackapi.TrackInput{
			Alias: "acct", AsOf: "2026-11-06", SessionDays: sessionDays, Positions: []compute.LedgerPosition{lp},
			Quotes: []trackapi.TrackQuote{{
				Underlying: "AAPL", Expiry: "2026-12-18", Strike: c.strike, Right: c.right, Day: "2026-11-06",
				Bid: "0.55", Ask: "0.65", UnderlyingClose: c.close,
			}},
		}, trackapi.TrackParams{MarkMaxAgeSessions: 1})
		if len(res.Open) != 1 {
			t.Fatalf("open rows = %d", len(res.Open))
		}
		pct, _ := new(big.Rat).SetString(res.Open[0].OtmPercent)
		pct.Quo(pct, big.NewRat(100, 1))
		// the percent has two places, so its hundredth is exact to half of 0.0001.
		want, _ := new(big.Rat).SetString(st.Figures.ExpiryDistance.Value)
		diff := new(big.Rat).Sub(pct, want)
		if diff.Abs(diff).Cmp(big.NewRat(1, 20000)) > 0 {
			t.Errorf("%s: track %s/100 = %s, structure %s", c.right, res.Open[0].OtmPercent, pct.FloatString(6), st.Figures.ExpiryDistance.Value)
		}
	}
}
