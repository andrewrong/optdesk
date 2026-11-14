package replay_test

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
)

func line(t *testing.T, rep replayapi.Report, variant, currency string) replayapi.Line {
	t.Helper()
	for _, ln := range rep.Lines {
		if ln.Variant == variant && ln.Currency == currency {
			return ln
		}
	}
	t.Fatalf("no line %s in %s", variant, currency)
	return replayapi.Line{}
}

func TestRolledLotsOutsideTotals(t *testing.T) {
	t.Parallel()
	rep := run(t, load(t), 1, early(t, "early_50_21", "21"))
	// USD lots hold prices: L1, L2, L3, L8 (+ rolled L4 apart; L5 not expired, L6 split, L7 no close).
	if h := line(t, rep, "hold", "USD"); h.Lots != 4 || h.KeptShare != "0.3750" {
		t.Errorf("hold = %+v", h)
	}
	var rolled []replayapi.Line
	for _, ln := range rep.Lines {
		if ln.Set == "rolled" {
			rolled = append(rolled, ln)
		}
		if ln.Set != "rolled" && ln.Variant == "actual" {
			t.Errorf("an actual row outside the fates: %+v", ln)
		}
	}
	if len(rolled) != 1 || rolled[0].Lots != 1 || rolled[0].Rolled != 1 || rolled[0].MeanGross != "1" {
		t.Errorf("rolled rows = %+v", rolled)
	}
	// L4 has full quotes and would be priced under the variant, yet stays out of its row.
	if v := line(t, rep, "early_50_21", "USD"); v.Lots != 2 {
		t.Errorf("variant row = %+v", v)
	}
	// the check of the held lots: hold and actual are on one basis, so the difference is zero.
	if c := line(t, rep, "check:held", "USD"); c.Lots != 2 || c.MeanGross != "0" || c.WorstGross != "0" {
		t.Errorf("check = %+v", c)
	}
}

func TestEarlyLineCoversOwnPricedLotsWithHoldBeside(t *testing.T) {
	t.Parallel()
	rep := run(t, load(t), 1, early(t, "early_50_21", "21"))
	v := line(t, rep, "early_50_21", "USD")
	// priced under the rule: L1 (closed at 1.10) and L3 (held, 2.00); credit 4.00.
	want := replayapi.Line{
		Set: "priced:early_50_21", Variant: "early_50_21", ParamHash: v.ParamHash, Currency: "USD",
		Lots: 2, Cycles: 1, ClosedEarly: 1, Held: 1, InTheMoney: 0,
		KeptShare: "0.7750", MeanGross: "1.55", WorstGross: "1.1",
		HoldKeptShare: "1.0000", ActualKeptShare: "0.8000",
	}
	if !reflect.DeepEqual(v, want) || v.ParamHash == "" {
		t.Errorf("line = %+v\nwant   %+v", v, want)
	}
	// one currency per row: the EUR lot is on its own row.
	if e := line(t, rep, "early_50_21", "EUR"); e.Lots != 1 || e.KeptShare != "1.0000" {
		t.Errorf("EUR = %+v", e)
	}
}

func TestUnderMinimumFlag(t *testing.T) {
	t.Parallel()
	f := load(t)
	rep := run(t, f, 2, early(t, "early_50_21", "21"))
	if line(t, rep, "early_50_21", "USD").UnderMinimum {
		t.Error("2 lots with a minimum of 2 is not under")
	}
	if !line(t, rep, "early_50_21", "EUR").UnderMinimum {
		t.Error("1 lot with a minimum of 2 is under")
	}
	rep = run(t, f, 3, early(t, "early_50_21", "21"))
	if !line(t, rep, "early_50_21", "USD").UnderMinimum {
		t.Error("2 lots with a minimum of 3 is under")
	}
}

func TestReportIsDeterministic(t *testing.T) {
	t.Parallel()
	f := load(t)
	vs := []replayapi.Variant{early(t, "early_50_21", "21"), early(t, "early_50_5", "5")}
	a := run(t, f, 2, vs...)
	// the same lots and days in another order give the same report.
	g := f
	g.Lots = append([]replayapi.Lot(nil), f.Lots...)
	for i, j := 0, len(g.Lots)-1; i < j; i, j = i+1, j-1 {
		g.Lots[i], g.Lots[j] = g.Lots[j], g.Lots[i]
	}
	g.SessionDays = append([]contracts.Day(nil), f.SessionDays...)
	for i, j := 0, len(g.SessionDays)-1; i < j; i, j = i+1, j-1 {
		g.SessionDays[i], g.SessionDays[j] = g.SessionDays[j], g.SessionDays[i]
	}
	if b := run(t, g, 2, vs...); !reflect.DeepEqual(a, b) {
		t.Error("report differs with the input order")
	}
	for i := 1; i < len(a.Outcomes); i++ {
		p, c := a.Outcomes[i-1], a.Outcomes[i]
		if p.LotID > c.LotID || (p.LotID == c.LotID && p.Variant >= c.Variant) {
			t.Errorf("outcomes out of order at %d: %+v then %+v", i, p, c)
		}
	}
	if a.Lots != 9 || a.LotsWithQuote != 6 || a.FirstQuoteDay != "2026-10-06" || a.LastQuoteDay != "2026-10-16" || a.QuoteDays != 9 {
		t.Errorf("coverage = %+v", a)
	}
}
