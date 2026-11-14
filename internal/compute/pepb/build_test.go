package pepb_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute/gonumstats"
	"github.com/andrewrong/optdesk/internal/compute/pepb"
	"github.com/andrewrong/optdesk/internal/compute/pepbapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func sd(s string) types.SerializableDate { return types.SerializableDate{Time: day(s)} }

// fact builds one stored fact; start "" is an instant fact.
func fact(taxonomy, concept, unit, form, doc, filed, start, end, val string) contracts.FundamentalFact {
	f := contracts.FundamentalFact{
		Taxonomy: taxonomy, Concept: concept, Unit: unit, Form: form, DocID: doc,
		Filed: sd(filed), PeriodEnd: sd(end), Value: val, Source: "sec", Subject: "cik:0000000001",
	}
	if start != "" {
		s := sd(start)
		f.PeriodStart = &s
	}
	return f
}

func income(form, doc, filed, start, end, val string) contracts.FundamentalFact {
	return fact("us-gaap", "NetIncomeLoss", "USD", form, doc, filed, start, end, val)
}

func equity(doc, filed, end, val string) contracts.FundamentalFact {
	return fact("us-gaap", "StockholdersEquity", "USD", "10-K", doc, filed, "", end, val)
}

func cover(doc, filed, val string) contracts.FundamentalFact {
	return fact("dei", "EntityCommonStockSharesOutstanding", "shares", "10-K", doc, filed, "", filed, val)
}

// baseFacts is 100 shares, a 10-K with net income 1000 and equity 5000, all filed in 2023-02.
func baseFacts() []contracts.FundamentalFact {
	return []contracts.FundamentalFact{
		cover("k1", "2023-02-01", "100"),
		income("10-K", "k1", "2023-02-01", "2022-01-01", "2022-12-31", "1000"),
		equity("k1", "2023-02-01", "2022-12-31", "5000"),
	}
}

// series is n consecutive calendar-day bars from `from`, the close of day i being closeOf(i).
func series(from string, n int, closeOf func(i int) string) []contracts.PriceBar {
	out := make([]contracts.PriceBar, n)
	for i := range out {
		out[i] = contracts.PriceBar{
			Close: closeOf(i), Currency: "USD",
			SessionDay: types.SerializableDate{Time: day(from).AddDate(0, 0, i)},
		}
	}
	return out
}

func flat(c string) func(int) string { return func(int) string { return c } }

func lastDay(b []contracts.PriceBar) contracts.Day {
	return contracts.Day(b[len(b)-1].SessionDay.Format("2006-01-02"))
}

func run(facts []contracts.FundamentalFact, bars []contracts.PriceBar, today contracts.Day) pepbapi.Result {
	return pepb.New(gonumstats.New())(pepbapi.Input{Facts: facts, Bars: bars, Today: today})
}

func runLast(facts []contracts.FundamentalFact, bars []contracts.PriceBar) pepbapi.Result {
	return run(facts, bars, lastDay(bars))
}

func mustEq(t *testing.T, what, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", what, got, want)
	}
}

func pointText(t *testing.T, h []pepbapi.Point, d string) string {
	t.Helper()
	for _, p := range h {
		if string(p.Day) == d {
			return p.Text
		}
	}
	t.Fatalf("no history point for %s", d)
	return ""
}

func TestNoLookAhead(t *testing.T) {
	t.Parallel()
	facts := append(baseFacts(),
		income("10-K", "k2", "2024-03-01", "2023-01-01", "2023-12-31", "2000"),
		cover("k2", "2024-03-01", "100"))
	bars := series("2024-02-20", 20, flat("10")) // to 2024-03-10
	// The page day is 2024-02-29: the 10-K filed on 2024-03-01 and later bars are not used.
	r := run(facts, bars, "2024-02-29")
	mustEq(t, "PE on 2024-02-29", r.PE.Ratio, "1.0")
	mustEq(t, "PE day", string(r.PE.Day), "2024-02-29")
	// From the filing day on the new figure is used, and the days before keep the old one.
	r = run(facts, bars, "2024-03-05")
	mustEq(t, "PE on 2024-03-05", r.PE.Ratio, "0.5")
	mustEq(t, "history 02-29", pointText(t, r.PE.History, "2024-02-29"), "1.0")
	mustEq(t, "history 03-01", pointText(t, r.PE.History, "2024-03-01"), "0.5")
}

func TestHistoryIsAtMost120Points(t *testing.T) {
	t.Parallel()
	for _, n := range []int{60, 120, 121, 300, 500} {
		bars := series("2023-03-01", n, flat("10"))
		h := runLast(baseFacts(), bars).PE.History
		if len(h) == 0 || len(h) > 120 {
			t.Fatalf("n=%d: %d history points", n, len(h))
		}
		if h[len(h)-1].Day != lastDay(bars) {
			t.Fatalf("n=%d: history ends %s, want %s", n, h[len(h)-1].Day, lastDay(bars))
		}
		for i := 1; i < len(h); i++ {
			if h[i-1].Day >= h[i].Day {
				t.Fatalf("n=%d: history not oldest first at %d", n, i)
			}
		}
		if n <= 120 && len(h) != n {
			t.Fatalf("n=%d: %d points, want all", n, len(h))
		}
	}
}

func TestPercentile(t *testing.T) {
	t.Parallel()
	// ratio = close / 10: 102 days at 4.0, 197 at 6.0, today at 5.0; 102 of 300 are below.
	bars := series("2023-03-01", 300, func(i int) string {
		switch {
		case i < 102:
			return "40"
		case i < 299:
			return "60"
		}
		return "50"
	})
	r := runLast(baseFacts(), bars).PE
	mustEq(t, "state", r.State, pepbapi.StateOK)
	mustEq(t, "Percentile", r.Percentile, "34")
	mustEq(t, "PercentileReason", r.PercentileReason, "")
	if r.WindowDays != 300 {
		t.Fatalf("WindowDays = %d", r.WindowDays)
	}
	mustEq(t, "WindowFrom", string(r.WindowFrom), "2023-03-01")
}

func TestUnderMinDaysHasNoPercentile(t *testing.T) {
	t.Parallel()
	r := runLast(baseFacts(), series("2023-03-01", 249, flat("10"))).PE
	mustEq(t, "state", r.State, pepbapi.StateOK)
	mustEq(t, "PercentileReason", r.PercentileReason, "too_few_days")
	mustEq(t, "Percentile", r.Percentile, "")
	mustEq(t, "Ratio", r.Ratio, "1.0")
	r = runLast(baseFacts(), series("2023-03-01", 250, flat("10"))).PE
	mustEq(t, "250 days PercentileReason", r.PercentileReason, "")
}

func TestStaleBarGivesNoFigure(t *testing.T) {
	t.Parallel()
	bars := series("2023-03-01", 10, flat("10")) // newest 2023-03-10
	r := run(baseFacts(), bars, "2023-03-16")    // 6 days old
	for _, x := range []pepbapi.Reading{r.PE, r.PB} {
		mustEq(t, x.Kind+" state", x.State, pepbapi.StateMissing)
		mustEq(t, x.Kind+" reason", x.Reason, "price_stale")
		mustEq(t, x.Kind+" ratio", x.Ratio, "")
	}
	r = run(baseFacts(), bars, "2023-03-15") // 5 days old is still fresh
	mustEq(t, "5 days old", r.PE.State, pepbapi.StateOK)
}

func TestNoPriceGivesNoFigure(t *testing.T) {
	t.Parallel()
	r := run(baseFacts(), nil, "2023-03-16")
	mustEq(t, "reason", r.PE.Reason, "no_price")
	mustEq(t, "PB reason", r.PB.Reason, "no_price")
}

func TestPercentileRoundsHalfUpAndSkipsBadCloses(t *testing.T) {
	t.Parallel()
	// 400 good days, 50 of them below today's (12.5 percent: rounds to 13). Bars with an empty,
	// zero or unparsable close are not days of the window.
	good := series("2023-03-01", 400, func(i int) string {
		switch {
		case i < 50:
			return "40"
		case i == 399:
			return "50" // today
		}
		return "60"
	})
	bad := series("2023-03-01", 3, flat(""))
	bad[1].Close, bad[2].Close = "0", "abc"
	bars := append(append([]contracts.PriceBar{}, good[:399]...), bad...)
	bars = append(bars, good[399])
	r := run(baseFacts(), bars, lastDay(good)).PE
	mustEq(t, "Percentile", r.Percentile, "13")
	if r.WindowDays != 400 {
		t.Fatalf("WindowDays = %d, want 400", r.WindowDays)
	}
}

func TestPricePointsAtPercentile(t *testing.T) {
	t.Parallel()
	// closes 1 to 300, one per day: the 60th and the 150th sorted close are the 20th and 50th
	// percentile by nearest rank.
	bars := series("2023-03-01", 300, func(i int) string { return strconv.Itoa(i + 1) })
	r := runLast(baseFacts(), bars)
	pe := r.PE
	if len(pe.At) != 2 {
		t.Fatalf("PE At = %d", len(pe.At))
	}
	// PE ratio = close/10; price = ratio x TTM 1000 / 100 shares.
	check := func(a pepbapi.AtPercentile, p int, ratio, price, text string) {
		t.Helper()
		if a.Percentile != p {
			t.Fatalf("percentile %d, want %d", a.Percentile, p)
		}
		mustEq(t, "ratio", a.Ratio, ratio)
		mustEq(t, "price", a.Price, price)
		mustEq(t, "text", a.Text, text)
	}
	check(pe.At[0], 20, "6.0", "60.00", "PE at the 20th percentile of 300 days since 2023-03-01: price 60.00 (history, not a buy price)")
	check(pe.At[1], 50, "15.0", "150.00", "PE at the 50th percentile of 300 days since 2023-03-01: price 150.00 (history, not a buy price)")
	// PB ratio = close/50; price = ratio x equity 5000 / 100 shares.
	pb := r.PB
	check(pb.At[0], 20, "1.2", "60.00", "PB at the 20th percentile of 300 days since 2023-03-01: price 60.00 (history, not a buy price)")
	check(pb.At[1], 50, "3.0", "150.00", "PB at the 50th percentile of 300 days since 2023-03-01: price 150.00 (history, not a buy price)")

	// Under 250 days there are no points and the text says why.
	short := runLast(baseFacts(), series("2023-03-01", 100, flat("10")))
	for _, x := range []pepbapi.Reading{short.PE, short.PB} {
		if len(x.At) != 0 {
			t.Fatalf("%s: At = %v", x.Kind, x.At)
		}
		want := fmt.Sprintf("no percentile: %d days since 2023-03-01", 100)
		if !contains(x.Text, want) {
			t.Fatalf("%s text %q lacks %q", x.Kind, x.Text, want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
