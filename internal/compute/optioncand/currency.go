package optioncand

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optioncandapi"
)

const (
	currencyUSD = "USD"
	currencyJPY = "JPY"

	markNoJPYFunds = "no_jpy_funds"
	markNoJPYRate  = "no_jpy_rate"
)

type currencyRule struct{}

// NewCurrency returns the rule that evaluates in the currency of the input. Design: docs/design/candidates-jp.md#4.
func NewCurrency() optioncandapi.CurrencyRule { return currencyRule{} }

var _ optioncandapi.NewCurrencyRuleFunc = NewCurrency

// EvaluateIn is Evaluate for the currency of the input; an empty currency is USD, and
// any currency other than JPY follows the USD rule (the only yen logic is below).
func (currencyRule) EvaluateIn(in optioncandapi.CurrencyInput, p compute.CandidateParams) []compute.Candidate {
	cur := currencyUSD
	if in.Currency == currencyJPY {
		cur = currencyJPY
	}
	return evaluate(in.CandidateInput, p, cur)
}

// currencyGates applies the yen rule to the funding and cash-rate gates of a put and returns
// the marks of the gates that passed by mark. For USD it returns the gates unchanged.
//   - funding: with accounts but none in JPY the gate passes (mark no_jpy_funds); the per-alias
//     Fits already read unknown/no_fx for a USD alias, since fundingGate compares in the cash currency.
//   - cash rate: no yen short rate is stored, so the gate passes (mark no_jpy_rate).
func currencyGates(cur string, in compute.CandidateInput, funding, rate compute.GateResult) (compute.GateResult, compute.GateResult, []string) {
	if cur != currencyJPY {
		return funding, rate, nil
	}
	var marks []string
	if len(in.Accounts) > 0 && !hasCurrency(in.Accounts, currencyJPY) {
		funding = gate("funding", compute.GatePass, "")
		marks = append(marks, markNoJPYFunds)
	}
	marks = append(marks, markNoJPYRate)
	return funding, gate("cash_rate", compute.GatePass, ""), marks
}

func hasCurrency(accounts []compute.AliasFunds, cur string) bool {
	for _, a := range accounts {
		if a.Currency == cur {
			return true
		}
	}
	return false
}
