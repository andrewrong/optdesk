package optioncand

import (
	"math/big"
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
)

const (
	cashCurrency  = "USD"
	maxAccountAge = 1
	daysPerYear   = 365
)

// buyPriceGate: with no usable buy price it passes (owner 2026-11-12, marked by evaluatePut);
// otherwise strike minus bid (a missing bid counts as no premium) is at most the buy price.
func buyPriceGate(in compute.CandidateInput, strike, bid *big.Rat) compute.GateResult {
	buy, ok := parseDec(in.BuyPrice)
	if !ok {
		return gate("buy_price", compute.GatePass, "") // marked no_buy_price by evaluatePut
	}
	net := new(big.Rat).Sub(strike, bid)
	if net.Cmp(buy) > 0 {
		return gate("buy_price", compute.GateBlock, "above_buy_price")
	}
	return gate("buy_price", compute.GatePass, "")
}

// quoteGate: the first blocking reason wins; delayed data is unknown only when nothing blocks.
func quoteGate(q compute.OptionQuote, p compute.CandidateParams, bid, ask *big.Rat, hasBid, hasAsk bool) compute.GateResult {
	block := func(reason string) compute.GateResult { return gate("quote", compute.GateBlock, reason) }
	switch {
	case !q.QuoteOK:
		return block("no_quote")
	case !hasBid || bid.Sign() <= 0:
		return block("no_bid")
	case !hasAsk:
		return block("no_ask")
	case q.BidSize < p.MinBidSize:
		return block("small_bid_size")
	}
	maxSpread, _ := parseDec(p.MaxSpread)
	if maxSpread == nil {
		maxSpread = new(big.Rat)
	}
	spread := new(big.Rat).Sub(ask, bid)
	mid := new(big.Rat).Add(ask, bid)
	mid.Quo(mid, big.NewRat(2, 1))
	if spread.Cmp(new(big.Rat).Mul(maxSpread, mid)) > 0 {
		return block("wide_spread")
	}
	// 0 is an open interest that was not captured (marked no_open_interest), not a block
	if q.OpenInterest > 0 && q.OpenInterest < p.MinOpenInterest {
		return block("low_open_interest")
	}
	if q.MarketDataType != 1 && q.MarketDataType != 2 {
		return gate("quote", compute.GateUnknown, "delayed_data")
	}
	return gate("quote", compute.GatePass, "")
}

// fundingGate checks strike x multiplier against each alias's available funds.
func fundingGate(in compute.CandidateInput, strike *big.Rat, multiplier string) (compute.GateResult, map[string]string) {
	accounts := append([]compute.AliasFunds(nil), in.Accounts...)
	sort.SliceStable(accounts, func(i, j int) bool { return accounts[i].Alias < accounts[j].Alias })
	if len(accounts) == 0 {
		return gate("funding", compute.GateUnknown, "no_account"), nil
	}
	mult, multOK := parseDec(multiplier)
	fits := map[string]string{}
	reason, anyFit := "", false
	for _, a := range accounts {
		verdict, why := "unknown", ""
		funds, fundsOK := parseDec(a.Funds)
		age, ageOK := sessionAge(in.SessionDays, a.CaptureDay, in.RunDay)
		switch {
		case !ageOK || age > maxAccountAge:
			why = "stale_account"
		case a.Currency != cashCurrency:
			why = "no_fx"
		case !fundsOK || !multOK:
			why = "no_account"
		case new(big.Rat).Mul(strike, mult).Cmp(funds) <= 0:
			verdict, anyFit = "fit", true
		default:
			verdict = "no_fit"
		}
		fits[a.Alias] = verdict
		if verdict == "unknown" && reason == "" {
			reason = why
		}
	}
	switch {
	case anyFit:
		return gate("funding", compute.GatePass, ""), fits
	case reason != "":
		return gate("funding", compute.GateUnknown, reason), fits
	}
	return gate("funding", compute.GateBlock, "no_fit"), fits
}

// cashRateGate: the annualized yield must be above DGS3MO (percent) of a fresh day.
func cashRateGate(in compute.CandidateInput, p compute.CandidateParams, yield *big.Rat) compute.GateResult {
	rate, ok := parseDec(in.Rate)
	if !ok {
		return gate("cash_rate", compute.GateUnknown, "no_rate")
	}
	age, dok := dayDiff(in.RateDay, in.RunDay)
	if !dok || age < 0 || age > p.MaxRateAgeDays {
		return gate("cash_rate", compute.GateUnknown, "stale_rate")
	}
	rate.Quo(rate, big.NewRat(100, 1))
	if yield.Cmp(rate) <= 0 {
		return gate("cash_rate", compute.GateBlock, "yield_below_cash")
	}
	return gate("cash_rate", compute.GatePass, "")
}

// annualized is (bid / strike) x (365 / dte).
func annualized(bid, strike *big.Rat, dte int) *big.Rat {
	y := new(big.Rat).Quo(bid, strike)
	return y.Mul(y, big.NewRat(daysPerYear, int64(dte)))
}
