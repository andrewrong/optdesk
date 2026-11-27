package optionattr

import (
	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
)

// positive reads a decimal text that must be a number above zero; with abs the sign is dropped first.
func positive(field, s string, abs bool) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err == nil && abs {
		d = d.Abs()
	}
	if err != nil || !d.IsPositive() {
		return decimal.Zero, badf("%s %q is not a positive number", field, s)
	}
	return d, nil
}

// CashIfAssigned adds the open short puts per currency in order of first appearance, exactly.
func (booker) CashIfAssigned(puts []optionattrapi.OpenPut) ([]optionattrapi.CashDue, error) {
	type acc struct {
		lots            int
		contracts, cash decimal.Decimal
	}
	var order []string
	by := map[string]*acc{}
	for _, p := range puts {
		strike, err := positive("strike", p.Strike, false)
		if err != nil {
			return nil, err
		}
		mult, err := positive("multiplier", p.Multiplier, false)
		if err != nil {
			return nil, err
		}
		qty, err := positive("quantity", p.Quantity, true)
		if err != nil {
			return nil, err
		}
		a, ok := by[p.Currency]
		if !ok {
			a = &acc{}
			by[p.Currency] = a
			order = append(order, p.Currency)
		}
		a.lots++
		a.contracts = a.contracts.Add(qty)
		a.cash = a.cash.Add(strike.Mul(mult).Mul(qty))
	}
	out := make([]optionattrapi.CashDue, 0, len(order))
	for _, c := range order {
		a := by[c]
		out = append(out, optionattrapi.CashDue{Currency: c, Lots: a.lots, Contracts: a.contracts.String(), Cash: a.cash.String()})
	}
	return out, nil
}
