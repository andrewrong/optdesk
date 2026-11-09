// Package optioncand is the rule behind compute.OptionCandidates for short puts: the gates
// in order, the status rule and the ranking by annualized yield, on exact rationals. It
// performs no I/O and reads no clock; every date and every age comes from the input.
package optioncand

import (
	"math/big"

	"github.com/andrewrong/optdesk/internal/compute"
)

const statusOK = "ok"

type candidates struct{}

// New returns the rule. Only short_put is evaluated; other kinds in the input are ignored.
func New() compute.OptionCandidates { return candidates{} }

// Evaluate returns one row per put contract with DTE in MinDTE..MaxDTE, ok rows ranked first.
func (candidates) Evaluate(in compute.CandidateInput, p compute.CandidateParams) []compute.Candidate {
	if !wantsPuts(in.Kinds) {
		return nil
	}
	spot, spotOK := parseDec(in.Spot)
	var rows []scored
	for _, q := range in.Quotes {
		if q.Right != "P" {
			continue
		}
		dte, ok := dayDiff(in.ChainDay, q.Expiry)
		strike, strikeOK := parseDec(q.Strike)
		if !ok || !strikeOK || strike.Sign() <= 0 || dte < 1 || dte < p.MinDTE || dte > p.MaxDTE {
			continue
		}
		rows = append(rows, evaluatePut(in, p, q, dte, strike, spot, spotOK))
	}
	return order(rows)
}

func wantsPuts(kinds []compute.CandidateKind) bool {
	if len(kinds) == 0 {
		return true
	}
	for _, k := range kinds {
		if k == compute.KindShortPut {
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
