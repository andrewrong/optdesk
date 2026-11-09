package optioncandidates

import (
	"encoding/json"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// ruleVersion is the rule_version column of every row this job writes.
const ruleVersion = "put_v1"

// FloatToDecimal turns a stored float into decimal text (Decision 4): the shortest text that round-trips, then
// half-even to 4 places, trailing zeros dropped. NaN and infinity give the empty text, which means absent.
func FloatToDecimal(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	r, ok := new(big.Rat).SetString(strconv.FormatFloat(v, 'f', -1, 64))
	if !ok {
		return ""
	}
	return roundHalfEven4(r)
}

// roundHalfEven4 formats r rounded half-even to 4 places with trailing zeros dropped.
func roundHalfEven4(r *big.Rat) string {
	neg := r.Sign() < 0
	scaled := new(big.Rat).Abs(r)
	scaled.Mul(scaled, big.NewRat(10000, 1))
	q, rem := new(big.Int).QuoRem(scaled.Num(), scaled.Denom(), new(big.Int))
	switch cmp := new(big.Int).Mul(rem, big.NewInt(2)).Cmp(scaled.Denom()); {
	case cmp > 0, cmp == 0 && q.Bit(0) == 1:
		q.Add(q, big.NewInt(1))
	}
	if q.Sign() == 0 {
		return "0"
	}
	digits := q.String()
	for len(digits) < 5 {
		digits = "0" + digits
	}
	whole, frac := digits[:len(digits)-4], strings.TrimRight(digits[len(digits)-4:], "0")
	out := whole
	if frac != "" {
		out += "." + frac
	}
	if neg {
		out = "-" + out
	}
	return out
}

// RateRatio is the ratio of a rate in percent, exactly: "4.20" gives "0.042". The second result is false for
// a text that is not a number or a rate outside 0 to 100 percent.
func RateRatio(percent string) (string, bool) {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(percent))
	if !ok || r.Sign() < 0 || r.Cmp(big.NewRat(100, 1)) > 0 {
		return "", false
	}
	r.Quo(r, big.NewRat(100, 1))
	s := r.FloatString(8)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, "."), true
}

func optText(v *float64) string {
	if v == nil {
		return ""
	}
	return FloatToDecimal(*v)
}

func optPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// chainEarnings is what the row of an underlying says about its earnings, kept for the columns.
type chainEarnings struct {
	state string
	next  contracts.Day
}

type gateJSON struct {
	Gate   string `json:"gate"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func gatesText(g []compute.GateResult) string {
	out := make([]gateJSON, 0, len(g))
	for _, r := range g {
		out = append(out, gateJSON{r.Gate, string(r.Status), r.Reason})
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func marksText(m []string) string {
	if m == nil {
		m = []string{}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func fitsText(f map[string]string) string {
	if f == nil {
		f = map[string]string{}
	}
	b, _ := json.Marshal(f) // map keys are sorted by encoding/json
	return string(b)
}

// toStoreRow maps one rule row to a stored row. The ranks are set by rankRun.
func toStoreRow(c compute.Candidate, day contracts.Day, runID string, ul underlyingRef, ev chainEarnings, now time.Time) store.Candidate {
	id, _ := strconv.Atoi(c.InstrumentID)
	mult, _ := strconv.Atoi(strings.SplitN(c.Multiplier, ".", 2)[0])
	right := "put"
	if c.Right == "C" {
		right = "call"
	}
	row := store.Candidate{
		SessionDay: day, RunID: runID, Kind: string(c.Kind), InstrumentID: id, Alias: c.Alias,
		RuleVersion: ruleVersion, Issuer: ul.issuer, UnderlyingID: int(ul.id), Expiry: c.Expiry, DTE: c.DTE,
		Right: right, Strike: c.Strike, Multiplier: mult, BidEod: c.Bid, Ask: optPtr(c.Ask), Spot: optPtr(c.Spot),
		BuyPrice: optPtr(c.BuyPrice), SalePrice: optPtr(c.SalePrice), BreakEven: c.BreakEven,
		AnnualizedYield: optPtr(c.AnnualizedYield), Delta: optPtr(c.Delta), Iv: optPtr(c.IV), Status: c.Status,
		Gates: gatesText(c.Gates), Marks: marksText(c.Marks), Fits: fitsText(c.Fits),
		EarningsState: ev.state, Extrinsic: optPtr(c.Extrinsic), ComputedAt: now,
	}
	if ev.next != "" {
		n := ev.next
		row.NextEarnings = &n
	}
	if c.RankInUnderlying > 0 {
		r := c.RankInUnderlying
		row.RankInUnderlying = &r
	}
	return row
}

// rankRun sets Rank of the ok rows of the whole run: yield descending, then DTE, strike and instrument id
// ascending (Decision 8). Rows that are not ok keep no rank.
func rankRun(rows []store.Candidate) {
	var ok []int
	for i, r := range rows {
		if r.Status == "ok" {
			ok = append(ok, i)
		}
	}
	rat := func(s *string) *big.Rat {
		if s == nil {
			return new(big.Rat)
		}
		v, good := new(big.Rat).SetString(*s)
		if !good {
			return new(big.Rat)
		}
		return v
	}
	sort.SliceStable(ok, func(a, b int) bool {
		x, y := rows[ok[a]], rows[ok[b]]
		if c := rat(x.AnnualizedYield).Cmp(rat(y.AnnualizedYield)); c != 0 {
			return c > 0
		}
		if x.DTE != y.DTE {
			return x.DTE < y.DTE
		}
		if c := rat(&x.Strike).Cmp(rat(&y.Strike)); c != 0 {
			return c < 0
		}
		return x.InstrumentID < y.InstrumentID
	})
	for pos, i := range ok {
		n := pos + 1
		rows[i].Rank = &n
	}
}
