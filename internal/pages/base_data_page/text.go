package base_data_page

// English labels of the page; the Chinese texts are in zh_texts.go.

const (
	pageName   = "base_data_page"
	runID      = "base_data"
	versionDoc = 1
	repairPath = "/data-health"
	dayFormat  = "2006-01-02"
)

// Calendar ids and their zones (cmd/ot has its own constants in package main, which cannot be imported).
const (
	calUS   = "XNYS"
	calJP   = "XTKS"
	zoneUS  = "America/New_York"
	zoneJP  = "Asia/Tokyo"
	gapKind = "gap"
)

// The gap reasons of the schema.
const (
	reasonFacts    = "facts_unavailable"
	reasonView     = "view_unavailable"
	reasonCoverage = "coverage_unavailable"
)

const (
	gapFactsText    = "All four fact views failed to read, so no row is shown."
	gapCoverageText = "The coverage of the ledger could not be read, so the ledger rules are not judged."
	gapReconText    = "The cross-check runs could not be read, so the bars row is judged without them."
	gapViewHead     = "These base data could not be read and are not judged: "
	gapViewTail     = "."
)

// viewWords names a view group in words.
var viewWords = map[string]string{
	"bars":    "stock bars",
	"option":  "option quotes, chains and contracts in use",
	"other":   "rates, events, calendar and company facts",
	"account": "account facts from the broker",
}

// rowLabels are the English labels of the rows.
var rowLabels = map[string]string{
	"bars": "Stock bars, US and JP", "quotes": "Quotes", "chains": "Option chains end of day",
	"open": "Option quotes at the open", "contracts": "Option contracts", "rates": "Rates (FRED)",
	"fx": "FX USDJPY", "events": "Earnings, dividends and macro events", "calendar": "Session calendar",
	"facts": "Company fundamentals", "account": "Account capture", "flex": "Flex statement",
}

// ruleLabels are the English labels of the rules, by rule name.
var ruleLabels = map[string]string{
	"rows": "Rows", "newest": "Newest value", "coverage_gap": "Coverage gap", "coverage_latest": "Latest coverage",
	"ohlc_order": "High low order", "bar_jump": "Close jump", "age": "Age", "crossed": "Crossed quotes",
	"empty": "No quote", "errors": "Errors", "non_live": "Not live", "bad_iv": "Implausible IV",
	"series_age": "Series age", "fx_band": "FX band", "fx_jump": "FX jump", "earnings_past": "Earnings date past",
	"dividend_past": "Ex-dividend date past", "macro_exhausted": "Macro calendar used up",
	"calendar_horizon": "Calendar horizon", "period_order": "Period order", "subject_stale": "Stale filings",
	"capture_age": "Capture age", "no_netliq": "No net liquidation", "position_expired": "Expired position",
	"flex_age": "Flex age", "flex_section_absent": "Flex section absent", "history_lost": "History lost", "not_read": "Not read",
}

func ruleLabel(rule string) string {
	if l, ok := ruleLabels[rule]; ok {
		return l
	}
	return rule
}

func rowLabel(id string) string {
	if l, ok := rowLabels[id]; ok {
		return l
	}
	return id
}
