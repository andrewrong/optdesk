package optionevidence

import (
	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// atmBand is the widest distance of the ATM strike from spot, as a fraction of spot. It is a trial value.
const atmBand = "0.10"

type atmPicker struct{}

var _ optionevidenceapi.ATMPicker = atmPicker{}

// NewATMPicker returns the ATMPicker of the rule in docs/contracts/compute/optionevidence.md.
func NewATMPicker() optionevidenceapi.ATMPicker { return atmPicker{} }

// ATM returns the IV and strike texts of the put of expiry nearest to spot; both empty when none.
func (atmPicker) ATM(chain []optionevidenceapi.ChainQuote, expiry contracts.Day, spot, source string) (string, string) {
	s, ok := parse(spot)
	if !ok || !s.IsPositive() {
		return "", ""
	}
	limit := decimal.RequireFromString(atmBand).Mul(s)
	var (
		bestIV, bestStrike string
		bestK, bestDist    decimal.Decimal
		found              bool
	)
	for _, q := range chain {
		if q.Expiry != expiry || q.Right != "P" || q.Source != source {
			continue
		}
		iv, ok := parse(q.IV)
		if !ok || !iv.IsPositive() {
			continue
		}
		k, ok := parse(q.Strike)
		if !ok {
			continue
		}
		dist := k.Sub(s).Abs()
		if dist.GreaterThan(limit) {
			continue
		}
		if !found || dist.LessThan(bestDist) || (dist.Equal(bestDist) && k.LessThan(bestK)) {
			bestIV, bestStrike, bestK, bestDist, found = q.IV, q.Strike, k, dist, true
		}
	}
	return bestIV, bestStrike
}
