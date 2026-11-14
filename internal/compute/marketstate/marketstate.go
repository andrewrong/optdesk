// Package marketstate computes the figures of the market state page: the S&P 500 drawdown from its
// 60-session high, the sector gaps against SPY with their ranks and rotation tails, and the daily
// history of the five market rows. Pure: no I/O, no clock, no store. Gaps and the drawdown are exact
// decimal (math/big); the two registered histories call the Figures in Input and take their float64
// arithmetic as it is. Results are text. Design: docs/design/market-state.md Decisions 4 to 8, 15.
package marketstate

import (
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// TrialParams are the trial constants of the figures; marketstateapi.Params documents each.
func TrialParams() marketstateapi.Params {
	return marketstateapi.Params{
		HighSessions: 60, ShortSessions: 21, LongSessions: 63, MeanSessions: 200,
		StaleWeekdays: 3, MaxAgeDays: 4, ChartYears: 3, ChartMax: 800, TailStep: 5, TailPoints: 4,
	}
}

var _ marketstateapi.Func = Compute

// Compute is the one implementation of marketstateapi.Func.
func Compute(in marketstateapi.Input) marketstateapi.Result {
	r := marketstateapi.Result{}
	if len(in.Benchmark.Days) > 0 {
		r.EndDay = in.Benchmark.Days[0]
	}
	r.Drawdown = drawdown(in)
	r.Funds, r.Industry = sectors(in)
	r.History = history(in)
	return r
}

func gapNum(r compute.Reason) liquidityapi.Num { return liquidityapi.Num{Reason: r} }

// ratOf is the exact decimal of the shortest text of v; ok is false for a non-finite value.
func ratOf(v float64) (*big.Rat, bool) {
	return new(big.Rat).SetString(strconv.FormatFloat(v, 'f', -1, 64))
}

// levelText is the shortest exact decimal text of a stored level.
func levelText(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func parseRat(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	return new(big.Rat).SetString(s)
}

func parseDay(d contracts.Day) (time.Time, bool) {
	t, err := time.Parse(time.DateOnly, string(d))
	return t, err == nil
}

// ageDays is the calendar days from then to now; ok is false when either is not a day.
func ageDays(now, then contracts.Day) (int, bool) {
	a, ok1 := parseDay(now)
	b, ok2 := parseDay(then)
	if !ok1 || !ok2 {
		return 0, false
	}
	return int(a.Sub(b).Hours() / 24), true
}

// weekdaysBehind counts the weekdays after day up to and including newest; a bad day is stale.
func weekdaysBehind(day, newest contracts.Day, stale int) int {
	a, ok1 := parseDay(day)
	b, ok2 := parseDay(newest)
	if !ok1 || !ok2 {
		return stale + 1
	}
	n := 0
	for d := a.AddDate(0, 0, 1); !d.After(b); d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			n++
		}
	}
	return n
}

// roundText is x rounded half to even at places decimals, as text with exactly that many.
func roundText(x *big.Rat, places int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	n := new(big.Int).Mul(x.Num(), scale)
	q, m := new(big.Int).DivMod(n, x.Denom(), new(big.Int)) // floor, m >= 0
	twice := new(big.Int).Lsh(m, 1)
	switch c := twice.Cmp(x.Denom()); {
	case c > 0, c == 0 && q.Bit(0) == 1:
		q.Add(q, big.NewInt(1))
	}
	return new(big.Rat).SetFrac(q, scale).FloatString(places)
}

// highOf is the highest of the closes s[i-n+1..i] and the index of its newest occurrence.
func highOf(s compute.Series, i, n int) (*big.Rat, int, bool) {
	var best *big.Rat
	at := -1
	for j := i - n + 1; j <= i; j++ {
		v, ok := ratOf(s[j].Value)
		if !ok || v.Sign() <= 0 {
			return nil, 0, false
		}
		if best == nil || v.Cmp(best) >= 0 {
			best, at = v, j
		}
	}
	return best, at, true
}

// drawdownAt is the close of s[i] against the highest of the n closes ending there, percent, 2 places.
func drawdownAt(s compute.Series, i, n int) (text string, high contracts.Day, reason compute.Reason) {
	if n < 1 || i+1 < n {
		return "", "", compute.ShortHistory
	}
	h, at, ok := highOf(s, i, n)
	c, ok2 := ratOf(s[i].Value)
	if !ok || !ok2 {
		return "", "", compute.BadInput
	}
	x := new(big.Rat).Quo(c, h)
	x.Sub(x, big.NewRat(1, 1))
	x.Mul(x, big.NewRat(100, 1))
	return roundText(x, 2), s[at].Day, ""
}

// drawdown is the newest SP500 close against the highest of the newest HighSessions closes.
func drawdown(in marketstateapi.Input) liquidityapi.Num {
	s := in.SP500
	if len(s) == 0 {
		return gapNum(compute.NoInput)
	}
	last := len(s) - 1
	if age, ok := ageDays(in.Day, s[last].Day); !ok || age > in.Params.MaxAgeDays {
		return gapNum(compute.StaleInput)
	}
	text, high, reason := drawdownAt(s, last, in.Params.HighSessions)
	if reason != "" {
		return gapNum(reason)
	}
	return liquidityapi.Num{Value: text, Day: s[last].Day, Aux: high}
}
