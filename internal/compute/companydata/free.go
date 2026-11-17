package companydata

import (
	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
)

// freeCash is operating cash flow minus capex as exact decimal text; Missing when either is.
func freeCash(ocf, capex companydataapi.Cell) companydataapi.Cell {
	if ocf.Missing || capex.Missing {
		for _, w := range []companydataapi.Why{companydataapi.WhyPeriodMismatch, companydataapi.WhyCannotDerive} {
			if ocf.Why == w || capex.Why == w {
				return missing(w)
			}
		}
		return missing(companydataapi.WhyNoFact)
	}
	a, errA := decimal.NewFromString(ocf.Value)
	b, errB := decimal.NewFromString(capex.Value)
	if errA != nil || errB != nil {
		return missing(companydataapi.WhyCannotDerive)
	}
	c := companydataapi.Cell{
		Value: fmtDec(a.Sub(b)), Derived: true, Restated: ocf.Restated || capex.Restated,
		Concept: ocf.Concept + " - " + capex.Concept,
	}
	return c
}
