package risk_page

// Chinese texts of the page; this file is the one door for them (cmd/ot/english_test.go).
const (
	textForeignStock = "非美元的股票未计入 Delta"
	// textNoQuoteStructures is a format: the count of structures left out for lack of a quote.
	textNoQuoteStructures = "%d 个期权组合因缺报价未计入 Delta 和 Vega；Delta 和 Vega 为其余组合之和"
)
