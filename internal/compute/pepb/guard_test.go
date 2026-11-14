package pepb_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/pepbapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func countsFacts(counts ...[2]string) []contracts.FundamentalFact {
	out := []contracts.FundamentalFact{
		income("10-K", "k0", "2022-12-01", "2021-01-01", "2021-12-31", "1000"),
		equity("k0", "2022-12-01", "2021-12-31", "5000"),
	}
	for i, c := range counts {
		out = append(out, cover("c"+string(rune('a'+i)), c[0], c[1]))
	}
	return out
}

func TestShareCountJumpGivesNoFigure(t *testing.T) {
	t.Parallel()
	facts := countsFacts(
		[2]string{"2023-01-10", "100"}, [2]string{"2023-04-10", "100"},
		[2]string{"2023-07-10", "400"}, [2]string{"2023-10-10", "400"})
	// A bar of 2023-07-08 is priced with 100 shares, and the 400 filed on 07-10 is known by the
	// page day 07-12 (the bar is 4 days old): the day has no figure.
	bars := series("2023-01-01", 189, flat("10")) // to 2023-07-08
	r := run(facts, bars, "2023-07-12")
	for _, x := range []pepbapi.Reading{r.PE, r.PB} {
		mustEq(t, x.Kind+" reason", x.Reason, "share_count_jump")
		mustEq(t, x.Kind+" ratio", x.Ratio, "")
	}
	// A day priced with the first 100 is a gap too.
	facts = countsFacts([2]string{"2023-01-10", "100"}, [2]string{"2023-07-10", "400"})
	r = run(facts, series("2023-01-20", 20, flat("10")), "2023-02-10")
	mustEq(t, "first 100", r.PE.Reason, "") // the 400 is not yet filed on the page day: nothing to compare
	// Days from the first 400 on have figures and the window starts there.
	facts = countsFacts(
		[2]string{"2023-01-10", "100"}, [2]string{"2023-04-10", "100"},
		[2]string{"2023-07-10", "400"}, [2]string{"2023-10-10", "400"})
	bars = series("2023-01-01", 305, flat("10")) // to 2023-11-01
	r = runLast(facts, bars)
	for _, x := range []pepbapi.Reading{r.PE, r.PB} {
		mustEq(t, x.Kind+" state", x.State, pepbapi.StateOK)
		mustEq(t, x.Kind+" window from", string(x.WindowFrom), "2023-07-10")
		if x.WindowDays != 115 {
			t.Fatalf("%s WindowDays = %d, want 115", x.Kind, x.WindowDays)
		}
		if len(x.History) == 0 || string(x.History[0].Day) != "2023-07-10" {
			t.Fatalf("%s history starts %v", x.Kind, x.History[:1])
		}
	}
	// 400 shares x 10 / 1000.
	mustEq(t, "PE", r.PE.Ratio, "4.0")
}

func TestNewestCountHasNoGuard(t *testing.T) {
	t.Parallel()
	// Equal counts: every day has a figure back to the first filing.
	facts := countsFacts([2]string{"2023-01-10", "100"}, [2]string{"2023-04-10", "100"})
	r := runLast(facts, series("2023-01-10", 100, flat("10"))).PE
	mustEq(t, "state", r.State, pepbapi.StateOK)
	mustEq(t, "window from", string(r.WindowFrom), "2023-01-10")
	// The newest count is not compared with a later one: a bar priced with it has a figure,
	// whatever the bars did after the filing (a split since then is not seen).
	facts = countsFacts([2]string{"2023-01-10", "100"}, [2]string{"2023-04-10", "101"})
	r = runLast(facts, series("2023-04-10", 20, flat("10"))).PE
	mustEq(t, "newest state", r.State, pepbapi.StateOK)
	mustEq(t, "newest shares", r.Shares, "101")
	// A change of 20 percent or less is not a jump.
	facts = countsFacts([2]string{"2023-01-10", "100"}, [2]string{"2023-04-10", "120"})
	r = runLast(facts, series("2023-01-10", 100, flat("10"))).PE
	mustEq(t, "20 percent window from", string(r.WindowFrom), "2023-01-10")
}

func TestLineCarriesSharesAndFiledDay(t *testing.T) {
	t.Parallel()
	facts := countsFacts([2]string{"2023-01-10", "100"}, [2]string{"2023-04-10", "120"})
	r := runLast(facts, series("2023-05-01", 5, flat("10")))
	for _, x := range []pepbapi.Reading{r.PE, r.PB} {
		mustEq(t, x.Kind+" shares", x.Shares, "120")
		mustEq(t, x.Kind+" shares filed", string(x.SharesFiled), "2023-04-10")
		if !contains(x.Text, "on 120 shares filed 2023-04-10") {
			t.Fatalf("%s text %q", x.Kind, x.Text)
		}
	}
	// 10 x 120 / 1000
	mustEq(t, "PE", r.PE.Ratio, "1.2")
}
