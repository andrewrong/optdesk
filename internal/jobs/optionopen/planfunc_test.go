package optionopen

import (
	"math"
	"testing"
	"time"
)

var testDay = time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)

// chain builds contracts of one expiry dte days after testDay for the strikes
// and rights given. Instrument ids come from the strike so they are stable.
func chain(dte int, rights string, strikes ...float64) []Contract {
	var out []Contract
	for _, s := range strikes {
		for _, r := range rights {
			out = append(out, Contract{
				InstrumentID: int64(dte)*1_000_000 + int64(s*100)*10 + int64(r%7),
				Expiry:       testDay.AddDate(0, 0, dte), Strike: s, Right: Right(string(r)),
			})
		}
	}
	return out
}

func rng(from, to, step float64) []float64 {
	var out []float64
	for s := from; s <= to+1e-9; s += step {
		out = append(out, math.Round(s*100)/100)
	}
	return out
}

func strikesOf(p Plan, class Class, deferred bool) map[float64]bool {
	m := map[float64]bool{}
	for _, it := range p.Items {
		if it.Class == class && it.Deferred == deferred {
			m[it.Contract.Strike] = true
		}
	}
	return m
}

func count(p Plan, class Class, deferred bool) int { return len(strikesOf(p, class, deferred)) }

func wide() PlanParams {
	p := DefaultPlanParams()
	p.CeilPerStock = 10000
	return p
}

func TestSigmaSourcesAndBandBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		prior, last float64
		sigma       float64
		src         SigmaSource
	}{
		{"prior close", 0.30, 0.50, 0.30, SigmaPriorClose},
		{"last known", 0, 0.50, 0.50, SigmaLastKnown},
		{"default", 0, 0, 0.30, SigmaDefault},
	} {
		got := BuildPlan(testDay, wide(), []NameInput{{UnderlyingID: 1, Symbol: "AAA", Spot: 100,
			PriorCloseSigma: tc.prior, LastKnownSigma: tc.last}}).Names[0]
		if got.Sigma != tc.sigma || got.SigmaSource != tc.src {
			t.Errorf("%s: got %v %s", tc.name, got.Sigma, got.SigmaSource)
		}
	}
	// sigma 0.30, 30 days: put_k band floor is 88.66; 89 is inside, 88 outside.
	n := NameInput{UnderlyingID: 1, Symbol: "AAA", Spot: 100, PriorCloseSigma: 0.30,
		Candidates: chain(30, "P", rng(88, 101, 1)...)}
	got := BuildPlan(testDay, wide(), []NameInput{n})
	puts := strikesOf(got, ClassPuts, false)
	if !puts[89] || puts[88] || len(puts) != 11 {
		t.Errorf("puts %v", puts)
	}
	around := strikesOf(got, ClassAroundSpot, false)
	if !around[100] || !around[101] || len(around) != 2 {
		t.Errorf("around %v", around)
	}
	if lo, _ := Band(100, 0.30, 30, 1.4); math.Abs(lo-88.6) > 0.1 {
		t.Errorf("band lo %v", lo)
	}
}

func TestNarrowBandGetsSixPuts(t *testing.T) {
	t.Parallel()
	n := NameInput{UnderlyingID: 1, Symbol: "AAA", Spot: 100, PriorCloseSigma: 0.01,
		Candidates: chain(30, "PC", rng(80, 110, 1)...)}
	got := BuildPlan(testDay, wide(), []NameInput{n})
	puts := strikesOf(got, ClassPuts, false)
	if len(puts) != 6 || !puts[99] || !puts[94] || puts[93] {
		t.Errorf("puts %v", puts)
	}
	if count(got, ClassCalls, false) != 0 {
		t.Error("calls for a name without position")
	}
}

func TestOneDollarGridCap25AndCallsForHeldNames(t *testing.T) {
	t.Parallel()
	n := NameInput{UnderlyingID: 1, Symbol: "AAA", Spot: 100.4, PriorCloseSigma: 1.0, HasPosition: true,
		Candidates: chain(30, "PC", rng(40, 160, 1)...)}
	got := BuildPlan(testDay, wide(), []NameInput{n})
	p, c := count(got, ClassPuts, false), count(got, ClassCalls, false)
	// puts and calls hold strikes only; the cap counts both rights of the strike once each.
	var cls int
	for _, it := range got.Items {
		if it.Class == ClassPuts || it.Class == ClassCalls {
			cls++
		}
	}
	if cls != 25 || p == 0 || c == 0 {
		t.Errorf("cap: %d items, %d put strikes, %d call strikes", cls, p, c)
	}
	// the farthest strikes dropped first: the nearest below the spot survive.
	if !strikesOf(got, ClassPuts, false)[99] || strikesOf(got, ClassPuts, false)[67] {
		t.Error("far puts kept")
	}
	// the same name without a position has no calls.
	n.HasPosition = false
	if count(BuildPlan(testDay, wide(), []NameInput{n}), ClassCalls, false) != 0 {
		t.Error("calls without position")
	}
}

func TestFiveDollarStrikesBeyondThreePercentForSPY(t *testing.T) {
	t.Parallel()
	n := NameInput{UnderlyingID: 1, Symbol: "SPY", Spot: 500.4, PriorCloseSigma: 0.2,
		Candidates: chain(30, "P", rng(450, 502, 1)...)}
	puts := strikesOf(BuildPlan(testDay, wide(), []NameInput{n}), ClassPuts, false)
	if !puts[499] || !puts[490] || puts[484] || !puts[485] {
		t.Errorf("puts %v", puts)
	}
}

func TestHeldOutsideBandAndCeilingsAlwaysIncluded(t *testing.T) {
	t.Parallel()
	held := chain(120, "C", 300)
	p := wide()
	p.CeilAroundSpot, p.CeilPuts, p.CeilCalls = 0, 0, 0
	n := NameInput{UnderlyingID: 1, Symbol: "AAA", Spot: 100, Held: held,
		Candidates: append(chain(30, "P", rng(90, 101, 1)...), held...)}
	got := BuildPlan(testDay, p, []NameInput{n})
	if got.Items[0].Class != ClassHeld || got.Items[0].Deferred || got.Items[0].Rank != 1 {
		t.Fatalf("held %+v", got.Items[0])
	}
	for _, it := range got.Items[1:] {
		if it.Class == ClassHeld || !it.Deferred {
			t.Errorf("non-held not deferred under zero ceilings: %+v", it)
		}
	}
	if len(got.Names) != 1 {
		t.Error("underlying must always be planned")
	}
}

func TestStoredBandNearerOfKAndDrop(t *testing.T) {
	t.Parallel()
	p := DefaultPlanParams()
	lo, hi := StoredBand(p, 100, 0.30, 30) // k = 1.75 gives 86.0 / 116.2, nearer than 75 / 125
	if math.Abs(lo-86.02) > 0.05 || math.Abs(hi-116.2) > 0.05 {
		t.Errorf("k band %v %v", lo, hi)
	}
	lo, hi = StoredBand(p, 100, 1.0, 30) // k = 1.75 gives 60.5 / 165, so -25 percent / +25 percent
	if math.Abs(lo-75) > 1e-9 || math.Abs(hi-125) > 1e-9 {
		t.Errorf("drop band %v %v", lo, hi)
	}
}

func TestHalfDollarGridPerStockCap60(t *testing.T) {
	t.Parallel()
	p := DefaultPlanParams() // cap 60 per stock
	n := NameInput{UnderlyingID: 1, Symbol: "AAA", Spot: 100.2, PriorCloseSigma: 1.0,
		Candidates: chain(30, "P", rng(70, 101, 0.5)...)}
	got := BuildPlan(testDay, p, []NameInput{n})
	var total int
	for _, it := range got.Items {
		if !it.Deferred {
			total++
		}
	}
	if total != 60 {
		t.Fatalf("kept %d", total)
	}
	def := strikesOf(got, ClassPuts, true)
	if len(def) != 2 || !def[70] || !def[70.5] {
		t.Errorf("far puts must drop first: %v", def)
	}
	if count(got, ClassAroundSpot, true) != 0 {
		t.Error("around_spot deferred")
	}
	// a cap of 4: the 6 nearest puts stay, around_spot stays.
	p.CeilPerStock = 4
	got = BuildPlan(testDay, p, []NameInput{n})
	if count(got, ClassPuts, false) != 6 || count(got, ClassAroundSpot, false) != 2 || !strikesOf(got, ClassPuts, false)[99.5] {
		t.Errorf("minimum puts: %v", strikesOf(got, ClassPuts, false))
	}
}

func TestOrderClassTurnsAndDailyRotation(t *testing.T) {
	t.Parallel()
	mk := func(sym string, id int64, spot float64) NameInput {
		return NameInput{UnderlyingID: id, Symbol: sym, Spot: spot, PriorCloseSigma: 0.3, HasPosition: true,
			Candidates: chain(30, "PC", rng(spot-8, spot+8, 1)...)}
	}
	names := []NameInput{mk("BBB", 2, 50.5), mk("AAA", 1, 50.5)}
	day1 := BuildPlan(testDay, wide(), names)
	day2 := BuildPlan(testDay.AddDate(0, 0, 1), wide(), names)
	order := map[Class]int{ClassAroundSpot: 0, ClassPuts: 1, ClassCalls: 2}
	for i := 1; i < len(day1.Items); i++ {
		if order[day1.Items[i].Class] < order[day1.Items[i-1].Class] {
			t.Fatalf("class order broken at %d", i)
		}
	}
	// names take turns inside a class
	for i := 0; i+1 < len(day1.Items); i += 2 {
		if day1.Items[i].Class == day1.Items[i+1].Class && day1.Items[i].Symbol == day1.Items[i+1].Symbol && i < 4 {
			t.Fatalf("one name twice in a row at %d", i)
		}
	}
	if day1.Items[0].Symbol == day2.Items[0].Symbol {
		t.Error("starting name must rotate by day")
	}
	// inside a class, each name's turns go nearest the spot first
	var last float64 = -1
	for _, it := range day1.Items {
		if it.Class == ClassPuts && it.Symbol == "AAA" {
			d := math.Abs(it.Contract.Strike - 50.5)
			if d < last {
				t.Fatal("not nearest first")
			}
			last = d
		}
	}
}

func TestCeilingsDeferPerClassKeepingClass(t *testing.T) {
	t.Parallel()
	p := wide()
	p.CeilAroundSpot, p.CeilPuts, p.CeilCalls = 3, 8, 2
	n := NameInput{UnderlyingID: 1, Symbol: "AAA", Spot: 100.5, PriorCloseSigma: 0.5, HasPosition: true,
		Candidates: chain(30, "PC", rng(85, 115, 1)...)}
	got := BuildPlan(testDay, p, []NameInput{n})
	kept := map[Class]int{}
	def := map[Class]int{}
	for _, it := range got.Items {
		if it.Deferred {
			def[it.Class]++
		} else {
			kept[it.Class]++
		}
	}
	if kept[ClassAroundSpot] != 3 || kept[ClassPuts] != 8 || kept[ClassCalls] != 2 {
		t.Errorf("kept %v", kept)
	}
	if def[ClassAroundSpot] != 1 || def[ClassPuts] == 0 || def[ClassCalls] == 0 {
		t.Errorf("deferred %v", def)
	}
	seenDeferred := false
	for _, it := range got.Items {
		if it.Deferred {
			seenDeferred = true
		} else if seenDeferred {
			t.Fatal("deferred items must rank last")
		}
	}
	// a put and a call at one strike are two items
	if len(strikesOf(got, ClassAroundSpot, false)) != 2 {
		t.Error("around_spot is the put and the call")
	}
}

func TestExpiryWindowUSAndJP(t *testing.T) {
	t.Parallel()
	var cs []Contract
	for _, d := range []int{13, 14, 19, 20, 45, 46, 90, 91} {
		cs = append(cs, chain(d, "P", 100, 101)...)
	}
	n := NameInput{UnderlyingID: 1, Symbol: "AAA", Spot: 100.5, Candidates: cs}
	dtes := func(p PlanParams) map[int]bool {
		m := map[int]bool{}
		for _, it := range BuildPlan(testDay, p, []NameInput{n}).Items {
			m[dteOf(testDay, it.Contract.Expiry)] = true
		}
		return m
	}
	us := dtes(DefaultPlanParams())
	if len(us) != 2 || !us[20] || !us[45] {
		t.Errorf("US %v", us)
	}
	jp := dtes(JPPlanParams())
	if len(jp) != 6 || !jp[14] || !jp[90] || jp[13] || jp[91] {
		t.Errorf("JP %v", jp)
	}
}

func TestCeilingDefersProtectedMinimumPutsLast(t *testing.T) {
	t.Parallel()
	p := wide()
	p.CeilPuts = 12
	cands := append(chain(30, "PC", rng(70, 130, 5)...), chain(31, "PC", rng(80, 110, 1)...)...)
	got := BuildPlan(testDay, p, []NameInput{{UnderlyingID: 1, Symbol: "ABC", Spot: 100, Candidates: cands}})
	protectedDeferred, unprotectedKept := false, false
	for _, it := range got.Items {
		if it.Class != ClassPuts {
			continue
		}
		d := testDay.AddDate(0, 0, 30)
		if it.Contract.Expiry.Equal(d) && it.Contract.Strike <= 85 && it.Deferred {
			protectedDeferred = true // one of E1's six minimum puts
		}
		if !it.Contract.Expiry.Equal(d) && it.Contract.Strike <= 93 && !it.Deferred {
			unprotectedKept = true // E2 put beyond its six nearest
		}
	}
	if protectedDeferred && unprotectedKept {
		t.Error("a protected minimum put was deferred before an unprotected put of the run")
	}
}
