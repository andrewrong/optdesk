package valuation_page

// Chinese texts of the warnings, empty states and prompts of the page. The labels, the column heads and the status
// word "not entered" stay English.
const (
	chooseMarketText = "请选择这家公司的 R 市场。"
	noCompanyText    = "没有这个键对应的公司。"

	// tablesMissingText is the gap tables_missing of the table, the company view and the form.
	tablesMissingText = "没有存放 R 版本或保证金表，所以不显示价格线。"

	// closeUnavailableText is the gap close_unavailable of the company view; the table says it for several names.
	closeUnavailableText      = "读不到收盘价。"
	closeUnavailableTableText = "有些公司读不到收盘价，这些公司的价格区间缺失。"

	storeUnavailableText = "这里读不到估值存档。"
)
