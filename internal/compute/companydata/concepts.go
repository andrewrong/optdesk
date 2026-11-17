package companydata

// kind says how a column is read from the facts.
type kind int

const (
	kindFlow    kind = iota // duration money: a 3M fact, or year-to-date minus year-to-date
	kindEPS                 // duration per share: a 3M fact, and for Q4 the year minus three quarters
	kindShares              // duration weighted average count: a 3M fact only
	kindInstant             // a fact dated at the quarter end
)

// column is one figure: the concepts in the order of Decision 6 of docs/design/data-collection.md
// (the first that has a value, or for a derived cell every operand, wins) and the unit read.
type column struct {
	concepts []string
	unit     string
	kind     kind
}

var (
	colRevenue = column{[]string{
		"RevenueFromContractWithCustomerExcludingAssessedTax", "Revenues", "SalesRevenueNet",
		"RevenueFromContractWithCustomerIncludingAssessedTax", "SalesRevenueGoodsNet",
	}, "USD", kindFlow}
	colOperatingIncome = column{[]string{"OperatingIncomeLoss"}, "USD", kindFlow}
	colNetIncome       = column{[]string{"NetIncomeLoss"}, "USD", kindFlow}
	colEPS             = column{[]string{"EarningsPerShareDiluted"}, "USD/shares", kindEPS}
	colShares          = column{[]string{
		"WeightedAverageNumberOfDilutedSharesOutstanding",
		"WeightedAverageNumberOfShareOutstandingBasicAndDiluted",
	}, "shares", kindShares}
	colOCF = column{[]string{
		"NetCashProvidedByUsedInOperatingActivities",
		"NetCashProvidedByUsedInOperatingActivitiesContinuingOperations",
	}, "USD", kindFlow}
	colCapex = column{[]string{
		"PaymentsToAcquirePropertyPlantAndEquipment", "PaymentsToAcquireProductiveAssets",
		"PaymentsToAcquireOtherPropertyPlantAndEquipment", "PaymentsForCapitalImprovements",
	}, "USD", kindFlow}
	colBuybacks  = column{[]string{"PaymentsForRepurchaseOfCommonStock"}, "USD", kindFlow}
	colDividends = column{[]string{"PaymentsOfDividendsCommonStock", "PaymentsOfDividends"}, "USD", kindFlow}
	colCash      = column{[]string{"CashAndCashEquivalentsAtCarryingValue"}, "USD", kindInstant}
	colLongDebt  = column{[]string{"LongTermDebtNoncurrent", "LongTermDebt"}, "USD", kindInstant}
	colShortDebt = column{[]string{"DebtCurrent", "LongTermDebtCurrent"}, "USD", kindInstant}
)

// wantedUnit maps a concept to the unit read for it.
var wantedUnit = func() map[string]string {
	m := map[string]string{}
	for _, c := range []column{colRevenue, colOperatingIncome, colNetIncome, colEPS, colShares, colOCF,
		colCapex, colBuybacks, colDividends, colCash, colLongDebt, colShortDebt} {
		for _, n := range c.concepts {
			m[n] = c.unit
		}
	}
	return m
}()
