package optioncandidates

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

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
func toStoreRow(c compute.Candidate, day contracts.Day, ul underlyingRef, ev chainEarnings, now time.Time) (store.Candidate, error) {
	id, err := strconv.Atoi(c.InstrumentID)
	if err != nil {
		return store.Candidate{}, fmt.Errorf("instrument id is not a number")
	}
	mult, err := strconv.Atoi(strings.SplitN(c.Multiplier, ".", 2)[0])
	if err != nil {
		return store.Candidate{}, fmt.Errorf("multiplier of instrument %d is not a number", id)
	}
	right := "put"
	if c.Right == "C" {
		right = "call"
	}
	row := store.Candidate{
		SessionDay: day, Kind: string(c.Kind), InstrumentID: id, Alias: c.Alias,
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
	return row, nil
}

// runIDOf is the run id of a run: a name-based UUID (SHA-1) over the day and the rows as they are written, every
// column of every row except RunID, ComputedAt and SettingsVersion. The same results give the same id, so a rerun is
// skipped by the store key; other results are a new run. The rows are not changed.
func runIDOf(day contracts.Day, rows []store.Candidate) string {
	h := sha1.New()
	h.Write([]byte(day))
	enc := json.NewEncoder(h)
	for _, r := range rows {
		r.RunID, r.ComputedAt, r.SettingsVersion = "", time.Time{}, nil
		_ = enc.Encode(r) // a struct of strings, numbers and pointers to them cannot fail
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, h.Sum(nil)).String()
}

// setRunID stamps the run id of the ranked rows on every row.
func setRunID(day contracts.Day, rows []store.Candidate) {
	id := runIDOf(day, rows)
	for i := range rows {
		rows[i].RunID = id
	}
}

// rankRun sets Rank of the ok rows of the whole run: yield descending, then DTE, strike and instrument id
// ascending (Decision 8). Rows that are not ok keep no rank.
func rankRun(rows []store.Candidate) error {
	var ok []int
	for i, r := range rows {
		if r.Status == "ok" {
			ok = append(ok, i)
		}
	}
	rats := map[int][2]*big.Rat{}
	for _, i := range ok {
		y, strike := new(big.Rat), new(big.Rat)
		if rows[i].AnnualizedYield == nil {
			return fmt.Errorf("an ok row of instrument %d has no yield", rows[i].InstrumentID)
		}
		if _, good := y.SetString(*rows[i].AnnualizedYield); !good {
			return fmt.Errorf("yield of instrument %d is not a number", rows[i].InstrumentID)
		}
		if _, good := strike.SetString(rows[i].Strike); !good {
			return fmt.Errorf("strike of instrument %d is not a number", rows[i].InstrumentID)
		}
		rats[i] = [2]*big.Rat{y, strike}
	}
	sort.SliceStable(ok, func(a, b int) bool {
		x, y := rows[ok[a]], rows[ok[b]]
		if c := rats[ok[a]][0].Cmp(rats[ok[b]][0]); c != 0 {
			return c > 0
		}
		if x.DTE != y.DTE {
			return x.DTE < y.DTE
		}
		if c := rats[ok[a]][1].Cmp(rats[ok[b]][1]); c != 0 {
			return c < 0
		}
		return x.InstrumentID < y.InstrumentID
	})
	for pos, i := range ok {
		n := pos + 1
		rows[i].Rank = &n
	}
	return nil
}
