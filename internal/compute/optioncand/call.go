package optioncand

import (
	"math/big"
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
)

// evaluateCall returns one covered-call row for every alias that holds shares, gates in the
// order shares, strike_floor, quote, events, guard (Decision 11).
func evaluateCall(in compute.CandidateInput, p compute.CandidateParams, q compute.OptionQuote, dte int, strike, spot *big.Rat, spotOK bool) []scored {
	accounts := append([]compute.AliasFunds(nil), in.Accounts...)
	sort.SliceStable(accounts, func(i, j int) bool { return accounts[i].Alias < accounts[j].Alias })
	bid, hasBid := parseDec(q.Bid)
	ask, hasAsk := parseDec(q.Ask)
	premium := bid
	if !hasBid {
		premium = new(big.Rat)
	}
	var yield *big.Rat
	if spotOK && spot.Sign() > 0 {
		yield = annualized(premium, spot, dte)
	} else {
		yield = new(big.Rat)
	}
	quote := quoteGate(q, p, bid, ask, hasBid, hasAsk)
	events, eventMark := eventsGate(in, p, q.Expiry)
	var guardIn *big.Rat
	if hasBid {
		guardIn = bid
	} else {
		guardIn = new(big.Rat)
	}
	guard, guardMark, extrinsic := guardGate(in, p, q.Expiry, strike, guardIn)
	var rows []scored
	for _, a := range accounts {
		if a.Shares <= 0 {
			continue
		}
		gates := []compute.GateResult{
			sharesGate(a, q.Multiplier), floorGate(in, a, strike), quote, events, guard,
		}
		var marks []string
		if spotOK && spot.Cmp(strike) > 0 {
			marks = append(marks, "itm")
		}
		for _, m := range []string{eventMark, guardMark} {
			if m != "" {
				marks = append(marks, m)
			}
		}
		if q.OpenInterest == 0 {
			marks = append(marks, "no_open_interest")
		}
		row := compute.Candidate{
			Kind: compute.KindCoveredCall, Underlying: in.Underlying, InstrumentID: q.InstrumentID,
			Alias: a.Alias, Expiry: q.Expiry, DTE: dte, Right: "C", Strike: q.Strike,
			Multiplier: q.Multiplier, Bid: q.Bid, Ask: q.Ask, Spot: in.Spot, BuyPrice: in.BuyPrice,
			SalePrice: in.SalePrice, AnnualizedYield: formatFixed(yield, 6), Delta: q.Delta, IV: q.IV,
			Status: status(gates), Gates: gates, Marks: marks,
		}
		if hasBid {
			row.BreakEven = formatTrim(new(big.Rat).Add(strike, bid))
		}
		if extrinsic != nil {
			row.Extrinsic = formatTrim(extrinsic)
		}
		rows = append(rows, scored{row: row, yield: yield, strike: strike})
	}
	return rows
}

// sharesGate: the shares minus multiplier x open short calls must cover one contract.
func sharesGate(a compute.AliasFunds, multiplier string) compute.GateResult {
	mult, ok := parseDec(multiplier)
	if !ok || mult.Sign() <= 0 {
		return gate("shares", compute.GateBlock, "no_free_shares")
	}
	free := new(big.Rat).Sub(big.NewRat(int64(a.Shares), 1), new(big.Rat).Mul(mult, big.NewRat(int64(a.OpenShortCalls), 1)))
	if free.Cmp(mult) < 0 {
		return gate("shares", compute.GateBlock, "no_free_shares")
	}
	return gate("shares", compute.GatePass, "")
}

// floorGate: the strike is at least SalePrice if set, else the alias's average cost; with
// neither the gate is unknown.
func floorGate(in compute.CandidateInput, a compute.AliasFunds, strike *big.Rat) compute.GateResult {
	floor, ok := parseDec(in.SalePrice)
	if !ok {
		floor, ok = parseDec(a.AvgCost)
	}
	if !ok {
		return gate("strike_floor", compute.GateUnknown, "no_floor")
	}
	if strike.Cmp(floor) < 0 {
		return gate("strike_floor", compute.GateBlock, "below_floor")
	}
	return gate("strike_floor", compute.GatePass, "")
}
