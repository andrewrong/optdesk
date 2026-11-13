package optioneod

import (
	"context"
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const selDay = "2026-11-13"

func indexManifest(t *testing.T) manifest {
	t.Helper()
	m, err := parseManifest(manifestIndexBytes)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func strikesRange(from, to, step int) []string {
	var out []string
	for s := from; s <= to; s += step {
		out = append(out, strconv.Itoa(s))
	}
	return out
}

func spxChain(expiries []string, strikes []string) broker.Chain {
	return broker.Chain{Exchange: "CBOE", TradingClass: "SPX", Multiplier: "100", Expirations: expiries, Strikes: strikes}
}

func spxRow(cap int, kind string) indexRow {
	return indexRow{Symbol: "SPX", TradingClass: "SPX", StrikeMultiple: 50, MaxContracts: cap, ExpiryKind: kind,
		SigmaIndex: "SPX", SpotIndex: "SPX", SpotDivisor: 1}
}

var spxUnderlying = underlying{Symbol: "SPX", SecType: "IND", Exchange: "CBOE", Currency: "USD"}

func strikeSet(cs []broker.Contract, expiry, right string) []float64 {
	var out []float64
	for _, c := range cs {
		if c.Expiry == expiry && c.Right == right {
			v, _ := strconv.ParseFloat(c.Strike, 64)
			out = append(out, v)
		}
	}
	return out
}

func minMax(v []float64) (float64, float64) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, x := range v {
		lo, hi = math.Min(lo, x), math.Max(hi, x)
	}
	return lo, hi
}

func TestIndexKeepsBothRightsOfEveryStrike(t *testing.T) {
	t.Parallel()
	cs := selectIndex(spxChain([]string{"20261217"}, strikesRange(7000, 8200, 50)), spxUnderlying, spxRow(40, "listed"),
		indexManifest(t), selDay, 7591.86, 0.15)
	calls, puts := strikeSet(cs, "20261217", "C"), strikeSet(cs, "20261217", "P")
	if len(calls) == 0 || len(calls) != len(puts) {
		t.Fatalf("calls %v puts %v", calls, puts)
	}
	for i := range calls {
		if calls[i] != puts[i] {
			t.Fatalf("call %v and put %v differ", calls[i], puts[i])
		}
	}
}

func TestIndexStrikesAreMultiplesOfTheClass(t *testing.T) {
	t.Parallel()
	cs := selectIndex(spxChain([]string{"20261217"}, strikesRange(7000, 8200, 5)), spxUnderlying, spxRow(40, "listed"),
		indexManifest(t), selDay, 7591.86, 0.15)
	if len(cs) == 0 {
		t.Fatal("nothing selected")
	}
	for _, c := range cs {
		if v, _ := strconv.Atoi(c.Strike); v%50 != 0 {
			t.Fatalf("strike %s is not a multiple of 50", c.Strike)
		}
	}
}

func TestIndexSpreadsTheStrikesOverTheBand(t *testing.T) {
	t.Parallel()
	spot := 7591.86
	cs := selectIndex(spxChain([]string{"20261217"}, strikesRange(5000, 10000, 50)), spxUnderlying, spxRow(40, "listed"),
		indexManifest(t), selDay, spot, 0.15)
	got := strikeSet(cs, "20261217", "C")
	if len(got) != 20 {
		t.Fatalf("want 20 strikes, got %d: %v", len(got), got)
	}
	w := spot * 1.8 * 0.15 * math.Sqrt(34.0/365) // about 625
	lo, hi := minMax(got)
	if lo > spot-w+50 || hi < spot+w-50 {
		t.Fatalf("strikes %v..%v do not cover the band %v..%v", lo, hi, spot-w, spot+w)
	}
	// The 20 strikes nearest to spot would span 950 points at most.
	if hi-lo < 1100 {
		t.Fatalf("strikes span only %v", hi-lo)
	}
}

func TestIndexAlwaysTakesTheAtTheMoneyPair(t *testing.T) {
	t.Parallel()
	chain := spxChain([]string{"20261217"}, strikesRange(7000, 8200, 50))
	// Two contracts: one strike, so it is the at-the-money one.
	cs := selectIndex(chain, spxUnderlying, spxRow(2, "listed"), indexManifest(t), selDay, 7591.86, 0.15)
	if got := strikeSet(cs, "20261217", "C"); len(got) != 1 || got[0] != 7600 {
		t.Fatalf("strikes %v, want [7600]", got)
	}
	// A tie keeps the lower strike.
	cs = selectIndex(chain, spxUnderlying, spxRow(2, "listed"), indexManifest(t), selDay, 7575, 0.15)
	if got := strikeSet(cs, "20261217", "P"); len(got) != 1 || got[0] != 7550 {
		t.Fatalf("strikes %v, want [7550]", got)
	}
}

func TestIndexCapIsPerClassAcrossExpiries(t *testing.T) {
	t.Parallel()
	cs := selectIndex(spxChain([]string{"20261120", "20261217", "20270219"}, strikesRange(5000, 10000, 50)), spxUnderlying,
		spxRow(80, "listed"), indexManifest(t), selDay, 7591.86, 0.30)
	if len(cs) != 80 {
		t.Fatalf("want 80 contracts, got %d", len(cs))
	}
	for _, e := range []string{"20261120", "20261217"} {
		if n := len(strikeSet(cs, e, "C")); n != 20 {
			t.Fatalf("expiry %s has %d strikes, want 20", e, n)
		}
	}
	if n := len(strikeSet(cs, "20270219", "C")); n != 0 {
		t.Fatalf("third expiry has %d strikes", n)
	}
}

func TestIndexBandWidensWithSigmaAndExpiry(t *testing.T) {
	t.Parallel()
	chain := spxChain([]string{"20261120", "20261217"}, strikesRange(4000, 12000, 50))
	span := func(sigma float64, expiry string) float64 {
		cs := selectIndex(chain, spxUnderlying, spxRow(80, "listed"), indexManifest(t), selDay, 7591.86, sigma)
		lo, hi := minMax(strikeSet(cs, expiry, "C"))
		return hi - lo
	}
	if lowS, highS := span(0.10, "20261217"), span(0.20, "20261217"); highS <= lowS*1.5 {
		t.Fatalf("sigma 0.20 span %v is not wider than sigma 0.10 span %v", highS, lowS)
	}
	if near, far := span(0.15, "20261120"), span(0.15, "20261217"); far <= near {
		t.Fatalf("far expiry span %v is not wider than near %v", far, near)
	}
}

func TestIndexDropsAStrikeWithOneRightUnlisted(t *testing.T) {
	t.Parallel()
	mk := func(strike, right string) broker.Contract {
		return broker.Contract{Expiry: "20261217", Strike: strike, Right: right}
	}
	got := completePairs([]broker.Contract{mk("7600", "C"), mk("7600", "P"), mk("7650", "C"), mk("7700", "P")})
	if len(got) != 2 || got[0].Strike != "7600" || got[1].Strike != "7600" {
		t.Fatalf("kept %+v, want the pair of 7600 only", got)
	}
}

func TestSigmaOfTwentyLogReturns(t *testing.T) {
	t.Parallel()
	var closes []string
	for i := 0; i < 21; i++ {
		closes = append(closes, []string{"100", "101"}[i%2])
	}
	// Returns alternate +ln(1.01) and -ln(1.01); sample deviation x sqrt(252) = 0.162060 (worked by hand).
	if got := annualisedSigma(closes, 20, 0.2); math.Abs(got-0.162060) > 5e-7 {
		t.Fatalf("sigma %.7f, want 0.162060", got)
	}
	// Only the last 21 closes count.
	if got := annualisedSigma(append([]string{"5000", "10"}, closes...), 20, 0.2); math.Abs(got-0.162060) > 5e-7 {
		t.Fatalf("sigma %.7f with older closes", got)
	}
}

func TestSigmaFallbackOnShortHistory(t *testing.T) {
	t.Parallel()
	closes := make([]string, 20)
	for i := range closes {
		closes[i] = strconv.Itoa(100 + i)
	}
	if got := annualisedSigma(closes, 20, 0.2); got != 0.2 {
		t.Fatalf("sigma %v, want the fallback 0.2", got)
	}
}

type fakeCloses struct {
	closes map[string][]string
	err    error
	from   []string
}

func (f *fakeCloses) IndexCloses(_ context.Context, index string, from, _ contracts.Day) ([]string, error) {
	f.from = append(f.from, string(from))
	return f.closes[index], f.err
}

func TestSigmaFallbackOnReaderError(t *testing.T) {
	t.Parallel()
	j, err := build(manifestIndexBytes, &fakeBroker{}, &fakeResolver{}, &fakeWriter{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakeCloses{err: errors.New("boom")}
	j.closes = fc
	day, _ := time.Parse(time.DateOnly, selDay)
	r := &runState{j: j, day: selDay, dayTime: day}
	if got := r.sigma(context.Background(), spxRow(80, "listed")); got != 0.20 {
		t.Fatalf("sigma %v, want 0.20", got)
	}
	if len(fc.from) != 1 || fc.from[0] != "2026-09-04" {
		t.Fatalf("closes read from %v, want 70 days before the day", fc.from)
	}
}

func TestXspMonthlyIsThirdFridayOrThursdayBefore(t *testing.T) {
	t.Parallel()
	xsp := indexRow{Symbol: "XSP", StrikeMultiple: 5, MaxContracts: 80, ExpiryKind: "third_friday"}
	exp := func(day string, expiries ...string) []string {
		cs := selectIndex(broker.Chain{Exchange: "CBOE", TradingClass: "XSP", Expirations: expiries, Strikes: []string{"750", "760"}},
			underlying{Symbol: "XSP"}, xsp, indexManifest(t), day, 759, 0.2)
		seen := map[string]bool{}
		var out []string
		for _, c := range cs {
			if !seen[c.Expiry] {
				seen[c.Expiry] = true
				out = append(out, c.Expiry)
			}
		}
		return out
	}
	// 19 June 2026 is a holiday: the monthly is Thursday the 18th.
	if got := exp("2026-06-01", "20260617", "20260618", "20260626"); len(got) != 1 || got[0] != "20260618" {
		t.Fatalf("June 2026 monthly %v, want [20260618]", got)
	}
	// Both listed: the Friday.
	if got := exp("2026-12-01", "20261217", "20261218"); len(got) != 1 || got[0] != "20261218" {
		t.Fatalf("December 2026 monthly %v, want [20261218]", got)
	}
}

func TestSpxListedKeepsTheBrokersThursday(t *testing.T) {
	t.Parallel()
	cs := selectIndex(spxChain([]string{"20260618", "20260626"}, strikesRange(7000, 8200, 50)), spxUnderlying, spxRow(80, "listed"),
		indexManifest(t), "2026-06-01", 7600, 0.15)
	if len(strikeSet(cs, "20260618", "C")) == 0 || len(strikeSet(cs, "20260626", "C")) == 0 {
		t.Fatalf("listed kind must keep every expiry of the class, got %+v", cs)
	}
}

func TestRutListedKeepsTheClassMonthlies(t *testing.T) {
	t.Parallel()
	row := indexRow{Symbol: "RUT", StrikeMultiple: 5, MaxContracts: 80, ExpiryKind: "listed"}
	cs := selectIndex(broker.Chain{Exchange: "RUSSELL", TradingClass: "RUT", Expirations: []string{"20261119", "20261217", "20270218"},
		Strikes: strikesRange(2000, 3000, 5)}, underlying{Symbol: "RUT"}, row, indexManifest(t), selDay, 2500, 0.2)
	if len(strikeSet(cs, "20261119", "C")) == 0 || len(strikeSet(cs, "20261217", "C")) == 0 || len(strikeSet(cs, "20270218", "C")) != 0 {
		t.Fatalf("contracts %+v", cs)
	}
}

func TestIndexExpiryIsKeyedAsTheBrokerReports(t *testing.T) {
	t.Parallel()
	r := newIndexRig(t, nil)
	r.b.chains = []broker.Chain{spxChain([]string{"20261217"}, strikesRange(7000, 8200, 5))}
	r.b.price = num(7585)
	r.job.m.Index[0].StrikeMultiple = 5
	if res := r.run("SPX"); res.Err != nil {
		t.Fatal(res.Err)
	}
	var found bool
	for _, c := range r.res.calls {
		if c.spec.Name != "SPX 20261217 C 7585" {
			continue
		}
		found = true
		if len(c.extra) != 1 || c.extra[0].Scheme != "osi" || c.extra[0].Value[:13] != "SPX   261217C" {
			t.Fatalf("identifiers %+v", c.extra)
		}
	}
	if !found {
		t.Fatal("no instrument named SPX 20261217 C 7585 was resolved")
	}
	for _, s := range r.b.resolved {
		if s.Expiry != "20261217" {
			t.Fatalf("contract expiry %s", s.Expiry)
		}
	}
}
