package shortput

import (
	"encoding/json"
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/strategy"
)

// inputOf is the rule's input for the snapshot: the put kind only.
func inputOf(s strategy.Snapshot) compute.CandidateInput {
	return compute.CandidateInput{
		RunDay: s.RunDay, ChainDay: s.ChainDay, SessionDays: s.SessionDays, Underlying: s.Underlying,
		Spot: s.Spot, Quotes: s.Quotes, BuyPrice: s.BuyPrice, SalePrice: s.SalePrice,
		Earnings: s.Earnings, Dividend: s.Dividend, Rate: s.Rate, RateDay: s.RateDay, Accounts: s.Accounts,
		Kinds: []compute.CandidateKind{compute.KindShortPut},
	}
}

// evaluationOf maps one rule row. Rank stays 0; Rank of the strategy sets it.
func evaluationOf(c compute.Candidate, s strategy.Snapshot) strategy.Evaluation {
	checks := make([]strategy.Check, 0, len(c.Gates))
	for _, g := range c.Gates {
		checks = append(checks, strategy.Check{Name: g.Gate, Status: string(g.Status), Reason: g.Reason})
	}
	facts := map[string]string{}
	for k, v := range s.Context {
		facts[k] = v
	}
	put := func(k, v string) {
		if v != "" {
			facts[k] = v
		}
	}
	put("issuer", s.Issuer)
	if s.UnderlyingID != 0 {
		put("underlying_id", strconv.FormatInt(s.UnderlyingID, 10))
	}
	facts["dte"] = strconv.Itoa(c.DTE)
	put("bid", c.Bid)
	put("ask", c.Ask)
	put("spot", c.Spot)
	put("buy_price", c.BuyPrice)
	put("sale_price", c.SalePrice)
	put("break_even", c.BreakEven)
	put("annualized_yield", c.AnnualizedYield)
	put("delta", c.Delta)
	put("iv", c.IV)
	put("extrinsic", c.Extrinsic)
	facts["marks"] = marksText(c.Marks)
	facts["fits"] = fitsText(c.Fits)
	return strategy.Evaluation{
		Key: c.InstrumentID,
		Legs: []strategy.Leg{{
			InstrumentID: c.InstrumentID, Side: "short", Right: "P", Strike: c.Strike,
			Expiry: c.Expiry, Multiplier: c.Multiplier, Quantity: 1,
		}},
		Checks: checks, Status: c.Status, Facts: facts, RankInUnderlying: c.RankInUnderlying,
	}
}

// marksText and fitsText write the JSON the job writes today (optioncandidates/convert.go), byte for byte.
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
