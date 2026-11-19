package option_candidates_page

// Texts of the page. Option terms stay English, other text is Chinese (option-pages-v22 Decision 6).
const (
	pageTitle    = "Option candidates"
	estimateNote = "Bid (EOD estimate) is the closing bid, sold on the next trading day, not a traded price"
	jpNote       = "Japanese option candidates are a separate group in yen when a yen reader is set"
	bidLabel     = "Bid (EOD estimate)"
	bidHint      = "Closing bid, sold on the next trading day, not a traded price"
	itmHint      = "Model estimate of the chance the put ends in the money; not an assignment probability"
	wordAbove    = "above"
	wordBelow    = "below"
	staleBadge   = "Stale"
	dash         = "—"
	unreadable   = "Unreadable"
	passText     = "Pass"

	gapNoReader   = "No candidate reader: cannot show"
	gapNoRun      = "No candidate run in the last 7 days"
	gapNoRows     = "No candidates of this kind in the last 7 days"
	staleTextHead = "Candidate rows are from "
	staleTextTail = ", not the current day"
	fitSeparator  = " "
	listSeparator = ";"
)

var groupTitles = map[string]string{
	kindShortPut:      "Short put",
	kindCoveredCall:   "Covered call",
	kindHeldCallGuard: "Held call guard",
}

// markHints are the short Chinese hints of the marks; a mark not listed is shown as it is stored.
var markHints = map[string]string{
	"itm":                     "in the money",
	"earnings_unconfirmed":    "earnings date not confirmed",
	"earnings_beyond_horizon": "earnings after the contract",
	"no_dividend":             "no dividend",
	"eod_fallback":            "Price is from the last close, the open pull had no usable quote for this stock",
	"oi_prior_session":        "Open interest is from an earlier session",
	"no_jpy_rate":             markHintNoJPYRateZh,
	"no_jpy_funds":            markHintNoJPYFundsZh,
}

// fitHints are the hints of the stored fit words.
var fitHints = map[string]string{
	"fit":     "Enough cash",
	"no_fit":  "Not enough cash",
	"unknown": "Unknown",
}

// reasonTexts holds the sentence of each known code: the codes of option-candidates.md Decisions 7, 11 and 12.
var reasonTexts = map[string]string{
	"no_buy_price":           "No buy price set",
	"earnings_before_expiry": "Earnings date is before expiry",
	"yield_below_cash":       "Annualised yield is not above the short-term Treasury rate",
	"stale_chain":            "Option chain is not from the latest trading day",
	"no_fx":                  "Different currency, not converted",
	"no_free_shares":         "No free shares to cover",
	"strike_floor":           "Strike is below the sell price floor",
	"early_assignment_risk":  "Early assignment risk before the ex-dividend date",
	"no_quote":               "No quote for this contract",
	"no_bid":                 "No bid price",
	"no_ask":                 "No ask price",
	"small_bid_size":         "Too few contracts at the bid",
	"wide_spread":            "Gap between bid and ask is too wide",
	"low_open_interest":      "Too few open contracts",
	"delayed_data":           "Quote is delayed, not live",
	"above_buy_price":        "Strike is above the buy price",
	"earnings_unsupported":   "Earnings dates are not available for this company",
	"earnings_error":         "Earnings date could not be read",
	"no_earnings":            "No earnings date known",
	"stale_earnings":         "Earnings date is out of date",
	"beyond_horizon":         "Earnings date is beyond the checked period",
	"no_fit":                 "Not enough cash to cover the put",
	"stale_account":          "Account data is out of date",
	"no_account":             "No account data",
	"no_rate":                "No short-term Treasury rate",
	"stale_rate":             "Short-term Treasury rate is out of date",
	"below_floor":            "Strike is below the price floor for selling",
	"no_floor":               "No price floor set for selling",
	"no_dividend_info":       "No dividend information",
	"stale_dividend":         "Dividend information is out of date",
}

// Texts of the evidence columns (candidates-evidence.md Decisions 6, 9, 10, 14 and 15).
const (
	evBelowBuyLabel   = "Below buy"
	evOneSigmaLabel   = "One-sigma move"
	evBidFcstLabel    = "Bid vs forecast price"
	evSkewLabel       = "Skew"
	evForecastLabel   = "Forecast vol"
	evIVPLabel        = "IVP"
	evVolEdgeLabel    = "Name IV vs forecast"
	evLabelsLabel     = "Labels"
	evDayHead         = "; data day "
	evBadgeDay        = "data day in the badge"
	evBelowBuyHint    = "The break-even is this far below your buy price, in dollars and as a share of it"
	evOneSigmaHint    = "one-sigma = a one standard deviation move by expiry, from the contract's IV"
	evSkewHint        = "skew = this contract's IV minus the IV of the at-the-money put, in percentage points"
	evLabelsHint      = "Soft labels; they never change which row passes or where it is shown"
	evForecastHint    = "the mean of the 20-, 60- and 120-session realised volatility"
	evIVPHint         = "IV percentile: today's IV is at or above this share of the last year's days"
	evVolEdgeHint     = "the name's implied volatility minus the forecast, in percentage points"
	evBidFcstHintHead = "Bid minus a model price from the forecast vol (a proxy, sign often wrong; European, no dividends). "
	trialRateSentence = "DGS3MO is the cash rate for every currency (a trial)."
	evBidFcstDays     = " Data days: chain "
	evBidFcstRate     = ", rate "
	evNoRateDay       = "none"
	trialRateNote     = "The cash rate used for the bid vs forecast price is DGS3MO for every currency; this is a trial."

	evPerShare    = " per share ("
	evPerContract = " per contract)"
	evModelPrice  = "model price "
	evSigmaBadge  = " sigma to break-even"
	evATMBadge    = "ATM IV "
	evATMAt       = " at "
	evLastError   = "last error "
	evAsOf        = "as of "
	evPts         = " pts"
	evDeltaNone   = "delta not stored"

	gapNoSpot      = "no spot"
	gapNotStored   = "not stored"
	gapNoATM       = "no ATM quote"
	gapNoBuy       = "No buy price set"
	gapNoForecast  = "No forecast volatility stored"
	gapNoRate      = "No cash rate within 7 days (DGS3MO)"
	gapNoFair      = "The model price could not be made"
	gapNoReadings  = "No reading reader"
	gapNoTicker    = "No ticker: readings are by ticker"
	readNoReader   = "no_reader"
	readNoInput    = "no_input"
	forecastBrOpen = " ("
)

// labelTexts are the badge words of Evidence.Labels (Decision 14); a label not listed is shown as it is.
var labelTexts = map[string]string{
	"no_edge_at_bid":     "bid not above the forecast price",
	"delta_outside_band": "delta outside the trial band",
	"ex_div_inside_life": "ex-dividend inside the life",
}

// readingReasons are the stored reason codes of a missing reading, as the single-name page names them.
var readingReasons = map[int]string{1: "no_input", 2: "short_history", 3: "stale_input", 4: "zero_range", 5: "bad_input"}
