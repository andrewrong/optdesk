package optionopen

import (
	"fmt"
	"testing"
	"time"
)

// rankFixture is 350 names on one expiry of 30 days: 700 around_spot items (two puts a name), 1,500 puts (five a name
// for 100 names, four for the other 250) and 800 calls (three a name for 100 names, two for the other 250).
func rankFixture() []NameInput {
	var out []NameInput
	for i := 0; i < 350; i++ {
		puts, calls := 4, 2
		if i < 100 {
			puts, calls = 5, 3
		}
		cs := chain(30, "P", 100, 101)
		for k := 0; k < puts; k++ {
			cs = append(cs, chain(30, "P", float64(99-k))...)
		}
		for k := 0; k < calls; k++ {
			cs = append(cs, chain(30, "C", float64(102+k))...)
		}
		out = append(out, NameInput{UnderlyingID: int64(i + 1), Symbol: fmt.Sprintf("N%03d", i), Spot: 100.5,
			PriorCloseSigma: 1.0, HasPosition: true, Candidates: cs})
	}
	return out
}

func rankParams(total int) PlanParams {
	p := DefaultPlanParams()
	p.Calls = CallsAll
	p.CeilTotal = total
	return p
}

func countOf(p Plan, class Class, deferred bool) int {
	n := 0
	for _, it := range p.Items {
		if it.Class == class && it.Deferred == deferred {
			n++
		}
	}
	return n
}

func TestCeilTotalDefersTheTailOfTheRank(t *testing.T) {
	t.Parallel()
	got := BuildPlan(testDay, rankParams(2050), rankFixture())
	if n := countOf(got, ClassAroundSpot, false); n != 700 {
		t.Errorf("around_spot kept %d, want 700", n)
	}
	if n := countOf(got, ClassPuts, false); n != 1350 {
		t.Errorf("puts kept %d, want 1350", n)
	}
	if n := countOf(got, ClassCalls, false); n != 0 {
		t.Errorf("calls kept %d, want 0", n)
	}
	if n := countOf(got, ClassPuts, true); n != 150 {
		t.Errorf("puts deferred %d, want 150", n)
	}
	if n := countOf(got, ClassCalls, true); n != 800 {
		t.Errorf("calls deferred %d, want 800", n)
	}
	// The round-robin keeps the nearest puts of each name: the 100 fifth puts (strike 95) go first.
	keptOf := map[string]int{}
	for _, it := range got.Items {
		if it.Class == ClassPuts && !it.Deferred {
			keptOf[it.Symbol]++
			if it.Contract.Strike == 95 {
				t.Errorf("fifth put of %s kept while a nearer put of another name is deferred", it.Symbol)
			}
		}
	}
	for sym, n := range keptOf {
		if n < 3 {
			t.Errorf("%s keeps %d puts, want at least 3", sym, n)
		}
	}
}

func TestCeilTotalCountsTheHeldItemsAndNeverDefersThem(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		total     int
		wantOther int
	}{{"22 held in 2,050", 2050, 2028}, {"ceiling below the held", 10, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			names := rankFixture()
			for k := 0; k < 22; k++ {
				names[0].Held = append(names[0].Held, chain(120, "C", float64(300+k))...)
			}
			got := BuildPlan(testDay, rankParams(tc.total), names)
			held, other := 0, 0
			for _, it := range got.Items {
				switch {
				case it.Class == ClassHeld && it.Deferred:
					t.Errorf("held item deferred: %+v", it)
				case it.Class == ClassHeld:
					held++
				case !it.Deferred:
					other++
				}
			}
			if held != 22 || other != tc.wantOther {
				t.Errorf("held %d (want 22), other kept %d (want %d)", held, other, tc.wantOther)
			}
		})
	}
}

func TestCeilTotalChangesNoRank(t *testing.T) {
	t.Parallel()
	free := BuildPlan(testDay, rankParams(0), rankFixture())
	cut := BuildPlan(testDay, rankParams(2050), rankFixture())
	if len(free.Items) != len(cut.Items) {
		t.Fatalf("items %d and %d", len(free.Items), len(cut.Items))
	}
	kept := 0
	for i := range free.Items {
		a, b := free.Items[i], cut.Items[i]
		if a.Symbol != b.Symbol || a.Contract != b.Contract || a.Rank != b.Rank {
			t.Fatalf("item %d differs: %+v and %+v", i, a, b)
		}
		if a.Deferred {
			t.Fatalf("item %d deferred without a ceiling", i)
		}
		if !b.Deferred {
			kept++
			if kept != i+1 {
				t.Fatalf("kept item at position %d is not in the first of the rank order", i)
			}
		}
	}
	if kept != 2050 {
		t.Errorf("kept %d, want 2050", kept)
	}
}

// bigName is one name of 600 puts over six expiries on a half dollar grid.
func bigName(calls bool) NameInput {
	var cs []Contract
	for _, dte := range []int{20, 25, 30, 35, 40, 45} {
		cs = append(cs, chain(dte, "P", rng(50, 99.5, 0.5)...)...)
		if calls {
			cs = append(cs, chain(dte, "C", rng(102, 226.5, 0.5)...)...)
		}
	}
	return NameInput{UnderlyingID: 1, Symbol: "BIG", Spot: 100.2, PriorCloseSigma: 3, HasPosition: calls, Candidates: cs}
}

func TestUSParamsHaveNoPerStockCapAndNoClassCeiling(t *testing.T) {
	t.Parallel()
	p := DefaultPlanParams()
	p.CeilTotal = 0
	got := BuildPlan(testDay, p, []NameInput{bigName(false)})
	if len(got.Items) <= 300 {
		t.Fatalf("fixture has %d items, want over 300", len(got.Items))
	}
	for _, it := range got.Items {
		if it.Deferred {
			t.Fatalf("deferred without a ceiling: %+v", it)
		}
	}
	// 1,400 calls and more under the US params (no class ceiling): kept while the total allows it.
	p.CeilTotal = 5000 // the total allows every item
	got = BuildPlan(testDay, p, []NameInput{bigName(true)})
	if n := countOf(got, ClassCalls, false); n < 1400 || countOf(got, ClassCalls, true) != 0 {
		t.Errorf("calls kept %d deferred %d, want over 1400 kept and none deferred", n, countOf(got, ClassCalls, true))
	}
}

func TestCeilTotalDefersTheProtectedPutsLast(t *testing.T) {
	t.Parallel()
	cands := append(chain(30, "P", rng(90, 101, 1)...), chain(30, "C", rng(102, 110, 1)...)...)
	n := NameInput{UnderlyingID: 1, Symbol: "ABC", Spot: 100.5, PriorCloseSigma: 1, HasPosition: true, Candidates: cands}
	run := func(total int) Plan {
		p := rankParams(total)
		return BuildPlan(testDay, p, []NameInput{n})
	}
	// Two around_spot items and the six nearest puts (99 to 94) fill a ceiling of 8: the far puts and calls go.
	got := run(8)
	if k := strikesOf(got, ClassPuts, false); len(k) != 6 || !k[94] || !k[99] {
		t.Errorf("kept puts %v, want the six nearest", k)
	}
	if countOf(got, ClassCalls, false) != 0 {
		t.Error("a call kept over a protected put")
	}
	// One less: the farthest protected put goes, and only it.
	if k := strikesOf(run(7), ClassPuts, false); len(k) != 5 || k[94] {
		t.Errorf("kept puts %v, want 99 to 95", k)
	}
}

func TestZeroCeilingMeansNone(t *testing.T) {
	t.Parallel()
	for name, set := range map[string]func(*PlanParams, int){
		"ceil_around_spot": func(p *PlanParams, v int) { p.CeilAroundSpot = v },
		"ceil_puts":        func(p *PlanParams, v int) { p.CeilPuts = v },
		"ceil_calls":       func(p *PlanParams, v int) { p.CeilCalls = v },
		"ceil_per_stock":   func(p *PlanParams, v int) { p.CeilPerStock = v },
		"ceil_total":       func(p *PlanParams, v int) { p.CeilTotal = v },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			deferred := func(v int) int {
				p := rankParams(0)
				set(&p, v)
				n := 0
				for _, it := range BuildPlan(testDay, p, []NameInput{rankFixture()[0]}).Items {
					if it.Deferred {
						n++
					}
				}
				return n
			}
			if deferred(1) == 0 {
				t.Errorf("%s = 1 defers nothing: the ceiling is not applied", name)
			}
			if n := deferred(0); n != 0 {
				t.Errorf("%s = 0 defers %d items, want none", name, n)
			}
		})
	}
}

func TestJPPlanParamsKeepTheirCeilings(t *testing.T) {
	t.Parallel()
	p := JPPlanParams()
	if p.CeilAroundSpot != 350 || p.CeilPuts != 1400 || p.CeilCalls != 1200 || p.CeilPerStock != 60 || p.CeilTotal != 0 {
		t.Errorf("JP ceilings %+v", p)
	}
	if u := DefaultPlanParams(); u.CeilAroundSpot != 0 || u.CeilPuts != 0 || u.CeilCalls != 0 || u.CeilPerStock != 0 || u.CeilTotal != 2050 {
		t.Errorf("US ceilings %+v", u)
	}
}

func TestPlanningTheSameDayTwiceGivesTheSamePlan(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	a := BuildPlan(day, rankParams(2050), rankFixture())
	b := BuildPlan(day, rankParams(2050), rankFixture())
	if len(a.Items) != len(b.Items) {
		t.Fatalf("items %d and %d", len(a.Items), len(b.Items))
	}
	for i := range a.Items {
		if a.Items[i] != b.Items[i] {
			t.Fatalf("item %d differs: %+v and %+v", i, a.Items[i], b.Items[i])
		}
	}
}

func TestLiveBandAppliesNeitherTheTotalNorTheCapOfAStock(t *testing.T) {
	t.Parallel()
	for _, total := range []int{2050, 100} {
		t.Run(fmt.Sprint(total), func(t *testing.T) {
			t.Parallel()
			c := newCore(newFake(100.2))
			c.Params.CeilTotal, c.Params.CeilPerStock = total, 60
			var ts []Target
			id := 1
			for _, dte := range []int{20, 30, 40} {
				for _, k := range rng(50, 99.5, 0.5) {
					id++
					tg := target(1, id, ClassPuts, k, Put)
					tg.Expiry = sessDay.AddDate(0, 0, dte)
					tg.Item.Sigma = 3
					ts = append(ts, tg)
				}
			}
			keep := c.band(sessDay, 100.2, ts)
			if len(ts) != 300 || len(keep) != 300 {
				t.Errorf("%d targets, %d kept by the band, want 300 of 300", len(ts), len(keep))
			}
		})
	}
}
