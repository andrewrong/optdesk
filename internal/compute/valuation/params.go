package valuation

import (
	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
)

// Role names of the figures a fiscal year carries; FilingYear.Values is keyed by them.
const (
	roleOperatingCashFlow = "operating_cash_flow"
	roleCapitalSpending   = "capital_spending"
	roleStockCompensation = "stock_compensation"
	roleDilutedShares     = "diluted_shares"
	roleLongTermDebt      = "long_term_debt"
	roleRevenue           = "revenue"
	roleNetIncome         = "net_income"
	roleEquity            = "equity"
)

// Role is one figure of a fiscal year: the chain of concepts (the first that has a fact
// for the year gives the figure), its unit, and whether it is an instant (a balance at the
// end of the year) or a duration (the whole year).
type Role struct {
	Name     string
	Concepts []string
	Unit     string
	Instant  bool
}

// Source names one concept read outside the fiscal years.
type Source struct {
	Taxonomy string
	Concept  string
	Unit     string
}

// Params holds the concepts and thresholds of one accounting standard. Code that reads
// facts takes them from here and names no concept itself.
type Params struct {
	Standard      valuationapi.Standard
	Taxonomy      string
	Currency      string
	Years         int // fiscal years returned
	MinYears      int // eligible years needed, and the run that must be consecutive
	AnnualDaysMin int // length of a fiscal year in days, both ends allowed
	AnnualDaysMax int
	AnnualForms   []string
	MaxAgeDays    int // the latest year may end at most this long before the cutoff
	ShareJump     decimal.Decimal
	YearRole      string // the role whose annual-form facts define the fiscal years
	Roles         []Role
	CoverShares   Source
	SIC           Source
	FinancialSIC  [][2]int // inclusive ranges: a company in one is unvalued
	CyclicalSIC   [][2]int
}

// ParamsFor returns the parameters of a standard. Only US_GAAP has a table; any other
// standard reports false.
func ParamsFor(std valuationapi.Standard) (Params, bool) {
	if std != valuationapi.USGAAP {
		return Params{}, false
	}
	return usGAAP(), true
}

func usGAAP() Params {
	return Params{
		Standard:      valuationapi.USGAAP,
		Taxonomy:      "us-gaap",
		Currency:      "USD",
		Years:         10,
		MinYears:      5,
		AnnualDaysMin: 350,
		AnnualDaysMax: 380,
		AnnualForms:   []string{"10-K", "10-K/A", "10-KT", "10-KT/A", "20-F", "20-F/A", "40-F", "40-F/A"},
		MaxAgeDays:    457,
		ShareJump:     decimal.RequireFromString("0.40"),
		YearRole:      roleOperatingCashFlow,
		Roles: []Role{
			{Name: roleOperatingCashFlow, Unit: "USD", Concepts: []string{
				"NetCashProvidedByUsedInOperatingActivities",
				"NetCashProvidedByUsedInOperatingActivitiesContinuingOperations",
			}},
			{Name: roleCapitalSpending, Unit: "USD", Concepts: []string{
				"PaymentsToAcquirePropertyPlantAndEquipment",
				"PaymentsToAcquireProductiveAssets",
				"PaymentsToAcquireOtherPropertyPlantAndEquipment",
				"PaymentsForCapitalImprovements",
			}},
			{Name: roleStockCompensation, Unit: "USD", Concepts: []string{
				"ShareBasedCompensation",
				"AllocatedShareBasedCompensationExpense",
			}},
			{Name: roleDilutedShares, Unit: "shares", Concepts: []string{
				"WeightedAverageNumberOfDilutedSharesOutstanding",
				"WeightedAverageNumberOfShareOutstandingBasicAndDiluted",
			}},
			{Name: roleLongTermDebt, Unit: "USD", Instant: true, Concepts: []string{
				"LongTermDebt",
				"LongTermDebtNoncurrent",
			}},
			{Name: roleRevenue, Unit: "USD", Concepts: []string{
				"Revenues",
				"RevenueFromContractWithCustomerExcludingAssessedTax",
				"RevenueFromContractWithCustomerIncludingAssessedTax",
				"SalesRevenueNet",
				"SalesRevenueGoodsNet",
			}},
			{Name: roleNetIncome, Unit: "USD", Concepts: []string{"NetIncomeLoss"}},
			{Name: roleEquity, Unit: "USD", Instant: true, Concepts: []string{"StockholdersEquity"}},
		},
		CoverShares:  Source{Taxonomy: "dei", Concept: "EntityCommonStockSharesOutstanding", Unit: "shares"},
		SIC:          Source{Taxonomy: "sec", Concept: "StandardIndustrialClassification", Unit: "sic"},
		FinancialSIC: [][2]int{{6000, 6799}},
		CyclicalSIC:  [][2]int{{1000, 1499}, {2900, 2999}, {3300, 3399}, {3674, 3674}, {3700, 3799}},
	}
}

func (p Params) role(name string) Role {
	for _, r := range p.Roles {
		if r.Name == name {
			return r
		}
	}
	return Role{}
}
