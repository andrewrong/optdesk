// Package optionshortlist implements optionshortlistapi.Shortlister: the rule that keeps, orders and
// counts the short-put rows of the candidates list. Exact rationals (math/big), no clock, no I/O, no
// rule parameter. Design: docs/design/candidates-page-v2.md#Decisions.
package optionshortlist

import (
	"math/big"
	"slices"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/optionshortlistapi"
)

// The bounds of the list are constants, not rule parameters
// (docs/design/candidates-page-v2.md#Decisions, 6).
const (
	// OTMMinPct: the strike is at least this many percent below spot
	// (docs/design/candidates-page-v2.md#Decisions, 7).
	OTMMinPct = "5"
	// OTMMaxPct: the strike is at most this many percent below spot
	// (docs/design/candidates-page-v2.md#Decisions, 7).
	OTMMaxPct = "15"
	// DeltaMin: smallest absolute delta (docs/design/candidates-page-v2.md#Decisions, 8).
	DeltaMin = "0.15"
	// DeltaMax: largest absolute delta (docs/design/candidates-page-v2.md#Decisions, 8).
	DeltaMax = "0.30"
	// PerUnderlying: most rows shown per underlying (docs/design/candidates-page-v2.md#Decisions, 10).
	PerUnderlying = 3
)

type shortlister struct{}

// New returns the rule.
func New() optionshortlistapi.Shortlister { return shortlister{} }

// kept is a row that passed every check with the figures the ordering needs.
type kept struct {
	row           optionshortlistapi.Row
	ann, strike   *big.Rat
	otm, absDelta string
	liquidity     string
}

func rat(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, "/") {
		return nil, false
	}
	return new(big.Rat).SetString(s)
}

func mustRat(s string) *big.Rat { r, _ := new(big.Rat).SetString(s); return r }

// decimal writes r with at most n decimals (half away from zero), trailing zeros trimmed.
func decimal(r *big.Rat, n int) string {
	s := r.FloatString(n)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

func has(list []string, v string) bool {
	return slices.Contains(list, v)
}

// judge returns the code of the first check that drops the row, or "" with the kept figures.
func judge(r optionshortlistapi.Row) (string, kept) {
	k := kept{row: r}
	bid, bidOK := rat(r.Bid)
	ann, annOK := rat(r.Annualized)
	if has(r.Marks, markUnreadableGates) || (strings.TrimSpace(r.Bid) != "" && !bidOK) || !annOK {
		return codeBadRow, k
	}
	k.ann = ann
	if !bidOK || bid.Sign() <= 0 || noBidQuote[r.QuoteReason] {
		return codeNoBid, k
	}
	if has(r.Marks, markNoOpenInterest) || r.QuoteReason == quoteLowOpenInterest {
		return codeNoOpenInterest, k
	}
	strike, sOK := rat(r.Strike)
	spot, pOK := rat(r.Spot)
	if !sOK || !pOK || strike.Sign() <= 0 || spot.Sign() <= 0 {
		return codeNoSpot, k
	}
	k.strike = strike
	pct := new(big.Rat).Sub(spot, strike)
	pct.Quo(pct, spot)
	pct.Mul(pct, big.NewRat(100, 1))
	if pct.Cmp(mustRat(OTMMinPct)) < 0 {
		return codeOTMLow, k
	}
	if pct.Cmp(mustRat(OTMMaxPct)) > 0 {
		return codeOTMHigh, k
	}
	delta, dOK := rat(r.Delta)
	if !dOK {
		return codeNoDelta, k
	}
	abs := new(big.Rat).Abs(delta)
	if abs.Cmp(mustRat(DeltaMin)) < 0 || abs.Cmp(mustRat(DeltaMax)) > 0 {
		return codeDeltaOut, k
	}
	k.otm, k.absDelta = decimal(pct, 4), decimal(abs, 4)
	if r.QuoteReason == quoteWideSpread {
		k.liquidity = quoteWideSpread
	}
	return "", k
}

// before orders yield high to low, then ticker, strike, key.
func before(a, b kept) bool {
	if c := a.ann.Cmp(b.ann); c != 0 {
		return c > 0
	}
	if a.row.Ticker != b.row.Ticker {
		return a.row.Ticker < b.row.Ticker
	}
	if c := a.strike.Cmp(b.strike); c != 0 {
		return c < 0
	}
	return a.row.Key < b.row.Key
}

// Select applies the rule; it never returns an error.
func (shortlister) Select(in optionshortlistapi.Input) (optionshortlistapi.Result, error) {
	res := optionshortlistapi.Result{
		Scanned: len(in.Rows),
		Bounds: optionshortlistapi.Bounds{
			OTMMinPct: OTMMinPct, OTMMaxPct: OTMMaxPct, DeltaMin: DeltaMin, DeltaMax: DeltaMax,
			PerUnderlying: PerUnderlying,
		},
	}
	counts := map[string]int{}
	var pass []kept
	var dMin, dMax *big.Rat
	var dMinT, dMaxT string
	for _, r := range in.Rows {
		if r.EventsReason == eventsStaleChain {
			res.StaleChain++
		}
		if d, ok := rat(r.DTE); ok {
			if dMin == nil || d.Cmp(dMin) < 0 {
				dMin, dMinT = d, decimal(d, 4)
			}
			if dMax == nil || d.Cmp(dMax) > 0 {
				dMax, dMaxT = d, decimal(d, 4)
			}
		}
		code, k := judge(r)
		if code != "" {
			counts[code]++
			continue
		}
		pass = append(pass, k)
	}
	res.DTEMin, res.DTEMax = dMinT, dMaxT
	sort.SliceStable(pass, func(i, j int) bool { return before(pass[i], pass[j]) })

	perCompany := map[string]int{}
	groups := map[string]*optionshortlistapi.Group{}
	var order []string
	for _, k := range pass {
		c := k.row.Company
		if perCompany[c] >= PerUnderlying {
			counts[codeOverCap]++
			continue
		}
		perCompany[c]++
		g := groups[c]
		if g == nil {
			g = &optionshortlistapi.Group{Company: c, Best: k.row.Annualized}
			groups[c] = g
			order = append(order, c) // pass is sorted, so the first row of a group is its best
		}
		g.Rows = append(g.Rows, optionshortlistapi.Placed{
			Key: k.row.Key, OTM: k.otm, AbsDelta: k.absDelta, Liquidity: k.liquidity,
		})
		res.Shown++
	}
	for _, c := range order {
		res.Groups = append(res.Groups, *groups[c])
	}
	for _, code := range dropOrder {
		if n := counts[code]; n > 0 {
			res.Dropped = append(res.Dropped, optionshortlistapi.Drop{Code: code, N: n})
		}
	}
	return res, nil
}
