package optioncand

import (
	"math/big"
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	markNoDividend = "no_dividend"
	verdictRisk    = "risk"
	verdictOK      = "ok"
	verdictUnknown = "unknown"
)

// guardGate is the early-assignment guard of Decision 12 for a call on expiry: with a next
// ex-date X from the chain day to the expiry and amount d, an extrinsic value below
// d x DivGuardFactor blocks. It returns the gate, the mark to attach (or empty) and the
// extrinsic value (nil when it was not computed).
func guardGate(in compute.CandidateInput, p compute.CandidateParams, expiry contracts.Day, strike, bid *big.Rat) (compute.GateResult, string, *big.Rat) {
	unknown := func(reason string) (compute.GateResult, string, *big.Rat) {
		return gate("guard", compute.GateUnknown, reason), "", nil
	}
	d := in.Dividend
	if d == nil {
		return unknown("no_dividend_info")
	}
	if age, ok := sessionAge(in.SessionDays, d.RowDay, in.RunDay); !ok || age > p.MaxEventAgeSessions {
		return unknown("stale_dividend")
	}
	switch d.State {
	case "no_value":
		return gate("guard", compute.GatePass, ""), markNoDividend, nil
	case "present":
	default:
		return unknown("no_dividend_info")
	}
	amount, amountOK := parseDec(d.Amount)
	spot, spotOK := parseDec(in.Spot)
	if d.NextExDate == "" || !amountOK || !spotOK {
		return unknown("no_dividend_info")
	}
	if d.NextExDate < in.ChainDay {
		return unknown("stale_dividend")
	}
	extrinsic := extrinsicValue(bid, spot, strike)
	if d.NextExDate > expiry {
		return gate("guard", compute.GatePass, ""), "", extrinsic
	}
	factor, ok := parseDec(p.DivGuardFactor)
	if !ok {
		factor = big.NewRat(1, 1)
	}
	if extrinsic.Cmp(amount.Mul(amount, factor)) < 0 {
		return gate("guard", compute.GateBlock, "early_assignment_risk"), "", extrinsic
	}
	return gate("guard", compute.GatePass, ""), "", extrinsic
}

// extrinsicValue is max(0, bid - max(0, spot - strike)).
func extrinsicValue(bid, spot, strike *big.Rat) *big.Rat {
	intrinsic := new(big.Rat).Sub(spot, strike)
	if intrinsic.Sign() < 0 {
		intrinsic.SetInt64(0)
	}
	e := new(big.Rat).Sub(bid, intrinsic)
	if e.Sign() < 0 {
		e.SetInt64(0)
	}
	return e
}

// evaluateHeldGuard judges one open short call for every alias that holds open short calls
// (one row with an empty alias when none is known). The contract is judged whatever its DTE.
func evaluateHeldGuard(in compute.CandidateInput, p compute.CandidateParams, q compute.OptionQuote, dte int, strike *big.Rat) []scored {
	var aliases []string
	for _, a := range in.Accounts {
		if a.OpenShortCalls > 0 {
			aliases = append(aliases, a.Alias)
		}
	}
	sort.Strings(aliases)
	if len(aliases) == 0 {
		aliases = []string{""}
	}
	bid, hasBid := parseDec(q.Bid)
	g, mark, extrinsic := gate("guard", compute.GateUnknown, "no_dividend_info"), "", (*big.Rat)(nil)
	if hasBid {
		g, mark, extrinsic = guardGate(in, p, q.Expiry, strike, bid)
	}
	verdict := verdictUnknown
	switch g.Status {
	case compute.GateBlock:
		verdict = verdictRisk
	case compute.GatePass:
		verdict = verdictOK
	case compute.GateUnknown:
	}
	var rows []scored
	for _, alias := range aliases {
		row := compute.Candidate{
			Kind: compute.KindHeldCallGuard, Underlying: in.Underlying, InstrumentID: q.InstrumentID,
			Alias: alias, Expiry: q.Expiry, DTE: dte, Right: "C", Strike: q.Strike,
			Multiplier: q.Multiplier, Bid: q.Bid, Ask: q.Ask, Spot: in.Spot,
			Delta: q.Delta, IV: q.IV, Status: status([]compute.GateResult{g}),
			Gates: []compute.GateResult{g}, GuardVerdict: verdict,
		}
		if mark != "" {
			row.Marks = []string{mark}
		}
		if extrinsic != nil {
			row.Extrinsic = formatTrim(extrinsic)
		}
		rows = append(rows, scored{row: row, yield: new(big.Rat), strike: strike})
	}
	return rows
}
