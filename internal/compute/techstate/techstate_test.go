package techstate_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/techstate"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var day0 = time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)

// days gives n consecutive calendar days newest first, so "common days" are easy to read.
func days(n int) []contracts.Day {
	out := make([]contracts.Day, n)
	for i := range out {
		out[i] = contracts.Day(day0.AddDate(0, 0, -i).Format("2006-01-02"))
	}
	return out
}

// series builds closes newest first: closes[i] is the value i sessions back. f maps i to the close text.
func series(n int, f func(i int) string) blockapi.CloseIn {
	c := make([]string, n)
	for i := range c {
		c[i] = f(i)
	}
	return blockapi.CloseIn{Name: "x", Symbol: "X", Closes: c, Days: days(n)}
}

func flat(v string) func(int) string { return func(int) string { return v } }

// in builds an input where SPX is flat at 100 and NDX is given by f.
func in(n int, f func(int) string) blockapi.TechIn {
	return blockapi.TechIn{
		Day: "2026-11-13", NDX: series(n, f), SPX: series(n, flat("100")), Params: techstate.TrialParams(),
	}
}

// windows gives NDX a 1m return r1 and a 3m return r3 (percent) on c0 and c1, so the raw states of the two
// newest days agree and the fold publishes it. SPX is flat at 100, so the gaps are r1 and r3.
func windows(r1, r3 float64) func(int) string {
	return func(i int) string {
		switch i {
		case 21, 22:
			return fmt.Sprintf("%.6f", 100/(1+r1/100))
		case 63, 64:
			return fmt.Sprintf("%.6f", 100/(1+r3/100))
		}
		return "100"
	}
}

func TestTechLaggingBothWindows(t *testing.T) {
	t.Parallel()
	got := techstate.Classify(in(70, windows(-2, -3)))
	if got.State != "LAGGING" || got.Rank != -1 || got.Reason != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestTechLaggingHardOnLongGap(t *testing.T) {
	t.Parallel()
	got := techstate.Classify(in(70, windows(-2, -6)))
	if got.State != "LAGGING HARD" || got.Rank != -2 {
		t.Fatalf("got %+v", got)
	}
}

func TestTechLeadingMirror(t *testing.T) {
	t.Parallel()
	got := techstate.Classify(in(70, windows(2, 3)))
	if got.State != "LEADING" || got.Rank != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestTechInLineInsideBand(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ r1, r3 float64 }{{0.5, -0.5}, {-5, 0.5}, {-5, -0.5}, {0.5, -9}} {
		got := techstate.Classify(in(70, windows(tc.r1, tc.r3)))
		if got.State != "IN LINE" || got.Rank != 0 {
			t.Fatalf("%+v: got %+v", tc, got)
		}
	}
}

// exact NDX closes: c0 = 100, c_n = 100/(1+r) with r a rational that gives a gap of exactly -1.00 or just above.
func TestTechBandEdgeIsExact(t *testing.T) {
	t.Parallel()
	// SPX is flat, so each gap is (c0/100 - 1) * 100: c0 = 99 gives exactly -1.00, c0 = 99.0001 gives -0.9999,
	// which the text rounds to -1.00.
	mk := func(c0 string) blockapi.TechIn {
		return in(70, func(i int) string {
			if i <= 1 {
				return c0
			}
			return "100"
		})
	}
	at := techstate.Classify(mk("99"))
	if at.State != "LAGGING" || at.Gap1m != "-1.00" {
		t.Fatalf("exactly -1.00: %+v", at)
	}
	near := techstate.Classify(mk("99.0001"))
	if near.State != "IN LINE" || near.Raw != "IN LINE" || near.Gap1m != "-1.00" {
		t.Fatalf("-0.9999 rounds to -1.00 but is IN LINE: %+v", near)
	}
}

// raw state per day is steered by the closes: day i's raw state looks at c_i, c_{i+21}, c_{i+63}. Level 100 for
// every close except the newest `lag` days of NDX, which are 90: those days are far below the older ones.
func lagFor(lag int) blockapi.TechIn {
	return in(80, func(i int) string {
		if i < lag {
			return "90"
		}
		return "100"
	})
}

func TestTechOneRawDayDoesNotChangeState(t *testing.T) {
	t.Parallel()
	// Only c0 is low: raw(c0) is LAGGING HARD (-10 and -10), the nine days behind are IN LINE.
	got := techstate.Classify(lagFor(1))
	if got.Raw != "LAGGING HARD" || got.State != "IN LINE" {
		t.Fatalf("got %+v", got)
	}
}

func TestTechTwoRawDaysChangeState(t *testing.T) {
	t.Parallel()
	got := techstate.Classify(lagFor(2))
	if got.Raw != "LAGGING HARD" || got.State != "LAGGING HARD" {
		t.Fatalf("got %+v", got)
	}
}

func TestTechUsesCommonDaysOnly(t *testing.T) {
	t.Parallel()
	x := in(70, windows(-2, -6))
	// An extra NDX-only day at the front: a close of 1 that would make everything look huge if it counted.
	extra := contracts.Day(day0.AddDate(0, 0, 1).Format("2006-01-02"))
	x.NDX.Days = append([]contracts.Day{extra}, x.NDX.Days...)
	x.NDX.Closes = append([]string{"1"}, x.NDX.Closes...)
	x.Day = extra
	got := techstate.Classify(x)
	if got.State != "LAGGING HARD" || got.DataDay != "2026-11-13" {
		t.Fatalf("got %+v", got)
	}
}

func TestTechRankOfEachState(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		r1, r3 float64
		state  string
		rank   int
	}{{2, 3, "LEADING", 1}, {0, 0, "IN LINE", 0}, {-2, -3, "LAGGING", -1}, {-2, -6, "LAGGING HARD", -2}} {
		got := techstate.Classify(in(70, windows(tc.r1, tc.r3)))
		if got.State != tc.state || got.Rank != tc.rank {
			t.Fatalf("%+v: got %+v", tc, got)
		}
	}
}

func TestTechFewerThan64CommonDaysIsShortHistory(t *testing.T) {
	t.Parallel()
	got := techstate.Classify(in(63, flat("100")))
	if got.Reason != compute.ShortHistory || got.State != "" || got.DataDay != "2026-11-13" {
		t.Fatalf("got %+v", got)
	}
	if got := techstate.Classify(in(64, flat("100"))); got.Reason != "" || got.RawDays != 1 {
		t.Fatalf("64 days: %+v", got)
	}
}

func TestTechStaleAfterMaxAge(t *testing.T) {
	t.Parallel()
	x := in(70, flat("100"))
	x.Day = "2026-11-18" // 5 days after c0: still fresh
	if got := techstate.Classify(x); got.Reason != "" {
		t.Fatalf("5 days: %+v", got)
	}
	x.Day = "2026-11-19"
	got := techstate.Classify(x)
	if got.Reason != compute.StaleInput || got.DataDay != "2026-11-13" || got.State != "" {
		t.Fatalf("6 days: %+v", got)
	}
}

func TestTechBadCloseIsBadInput(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"", "abc", "0", "-5", "1e3", "1/2", "NaN", "1.2.3"} {
		x := in(70, flat("100"))
		x.SPX.Closes[3] = bad
		if got := techstate.Classify(x); got.Reason != compute.BadInput || got.State != "" {
			t.Fatalf("close %q: %+v", bad, got)
		}
	}
}

func TestTechNoClosesIsNoInput(t *testing.T) {
	t.Parallel()
	x := in(70, flat("100"))
	x.SPX = blockapi.CloseIn{}
	got := techstate.Classify(x)
	if got.Reason != compute.NoInput || got.DataDay != "" {
		t.Fatalf("got %+v", got)
	}
	if got := techstate.Classify(blockapi.TechIn{Params: techstate.TrialParams()}); got.Reason != compute.NoInput {
		t.Fatalf("empty: %+v", got)
	}
}

func TestTechRawDaysCountsTheFold(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ n, want int }{{64, 1}, {66, 3}, {73, 10}, {200, 10}} {
		if got := techstate.Classify(in(tc.n, flat("100"))); got.RawDays != tc.want {
			t.Fatalf("%d closes: raw days %d, want %d", tc.n, got.RawDays, tc.want)
		}
	}
}

func TestTechEvidenceTextsRoundHalfEven(t *testing.T) {
	t.Parallel()
	// ret(NDX) = (c0/c_n - 1)*100 with SPX flat. c_n = 100: c0 = 100.125 gives 0.125 -> 0.12 (half-even),
	// c0 = 100.375 gives 0.375 -> 0.38, c0 = 99.875 gives -0.125 -> -0.12, c0 = 100 gives 0.00.
	for _, tc := range []struct{ c0, want string }{
		{"100.125", "0.12"}, {"100.375", "0.38"}, {"99.875", "-0.12"}, {"99.625", "-0.38"}, {"100", "0.00"}, {"100.001", "0.00"},
		{"99.999", "0.00"},
	} {
		x := in(70, func(i int) string {
			if i == 0 {
				return tc.c0
			}
			return "100"
		})
		got := techstate.Classify(x)
		if got.NDX1m != tc.want || got.Gap1m != tc.want || got.NDX3m != tc.want || got.Gap3m != tc.want {
			t.Fatalf("c0 %s: %+v, want %s", tc.c0, got, tc.want)
		}
		if got.SPX1m != "0.00" || got.SPX3m != "0.00" {
			t.Fatalf("spx: %+v", got)
		}
	}
}

func TestTrialParams(t *testing.T) {
	t.Parallel()
	want := blockapi.TechParams{Band: "1.00", Hard: "5.00", Short: 21, Long: 63, Fold: 10, MaxAgeDays: 5}
	if got := techstate.TrialParams(); got != want {
		t.Fatalf("got %+v", got)
	}
}
