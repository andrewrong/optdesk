// Package optioncand is the rule behind compute.OptionCandidates for short puts, covered
// calls and held-call guards: the gates in order, the status rule and the ranking
// by annualized yield, on exact rationals. It performs no I/O and reads no clock; every date and every age comes from the input.
package optioncand

import (
	"math/big"

	"github.com/andrewrong/optdesk/internal/compute"
)

const statusOK = "ok"

type candidates struct{}

// New returns the rule. It evaluates the kinds named in the input; an empty list means short_put only.
func New() compute.OptionCandidates { return candidates{} }

// Evaluate returns the rows of each wanted kind (puts, then covered calls, then held-call
// guards), each kind ranked on its own with ok rows first.
func (candidates) Evaluate(in compute.CandidateInput, p compute.CandidateParams) []compute.Candidate {
	spot, spotOK := parseDec(in.Spot)
	var puts, calls, guards []scored
	wantPut, wantCall := wants(in.Kinds, compute.KindShortPut), wants(in.Kinds, compute.KindCoveredCall)
	held := map[string]bool{}
	if wants(in.Kinds, compute.KindHeldCallGuard) {
		for _, id := range in.OpenShortIDs {
			held[id] = true
		}
	}
	for _, q := range in.Quotes {
		dte, ok := dayDiff(in.ChainDay, q.Expiry)
		strike, strikeOK := parseDec(q.Strike)
		if !ok || !strikeOK || strike.Sign() <= 0 {
			continue
		}
		inWindow := dte >= 1 && dte >= p.MinDTE && dte <= p.MaxDTE
		switch q.Right {
		case "P":
			if wantPut && inWindow {
				puts = append(puts, evaluatePut(in, p, q, dte, strike, spot, spotOK))
			}
		case "C":
			if wantCall && inWindow {
				calls = append(calls, evaluateCall(in, p, q, dte, strike, spot, spotOK)...)
			}
			if held[q.InstrumentID] {
				guards = append(guards, evaluateHeldGuard(in, p, q, dte, strike)...)
			}
		}
	}
	out := order(puts)
	out = append(out, order(calls)...)
	return append(out, order(guards)...)
}

func wants(kinds []compute.CandidateKind, kind compute.CandidateKind) bool {
	if len(kinds) == 0 {
		return kind == compute.KindShortPut
	}
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}

func evaluatePut(in compute.CandidateInput, p compute.CandidateParams, q compute.OptionQuote, dte int, strike, spot *big.Rat, spotOK bool) scored {
	bid, hasBid := parseDec(q.Bid)
	ask, hasAsk := parseDec(q.Ask)
	premium := bid
	if !hasBid {
		premium = new(big.Rat)
	}
	yield := annualized(premium, strike, dte)
	events, mark := eventsGate(in, p, q.Expiry)
	funding, fits := fundingGate(in, strike, q.Multiplier)
	gates := []compute.GateResult{
		buyPriceGate(in, strike, premium),
		quoteGate(q, p, bid, ask, hasBid, hasAsk),
		events,
		funding,
		cashRateGate(in, p, yield),
	}
	var marks []string
	if spotOK && strike.Cmp(spot) > 0 {
		marks = append(marks, "itm")
	}
	if mark != "" {
		marks = append(marks, mark)
	}
	if _, ok := parseDec(in.BuyPrice); !ok {
		marks = append(marks, "no_buy_price")
	}
	if q.OpenInterest == 0 {
		marks = append(marks, "no_open_interest")
	}
	row := compute.Candidate{
		Kind: compute.KindShortPut, Underlying: in.Underlying, InstrumentID: q.InstrumentID,
		Expiry: q.Expiry, DTE: dte, Right: "P", Strike: q.Strike, Multiplier: q.Multiplier,
		Bid: q.Bid, Ask: q.Ask, Spot: in.Spot, BuyPrice: in.BuyPrice, SalePrice: in.SalePrice,
		AnnualizedYield: formatFixed(yield, 6), Delta: q.Delta, IV: q.IV,
		Status: status(gates), Gates: gates, Marks: marks, Fits: fits,
	}
	if hasBid {
		row.BreakEven = formatTrim(new(big.Rat).Sub(strike, bid))
	}
	return scored{row: row, yield: yield, strike: strike}
}

// status is blocked if any gate blocked, else unknown if any is unknown, else ok.
func status(gates []compute.GateResult) string {
	st := statusOK
	for _, g := range gates {
		switch g.Status {
		case compute.GateBlock:
			return "blocked"
		case compute.GateUnknown:
			st = "unknown"
		case compute.GatePass:
		}
	}
	return st
}
