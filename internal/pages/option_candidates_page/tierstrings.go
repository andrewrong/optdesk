package option_candidates_page

// Texts of the tier view (candidates-tiers.md Decision 11): labels and every number are English, the
// explanations and glosses are Chinese. A value in braces is filled by the page from the answer.
const (
	tierFitsWord  = "Fits"
	tierCheckWord = "Check"
	tierGreyWord  = "Not judged"

	tierFitsID  = "fits"
	tierCheckID = "check"
	greyIDHead  = "grey_"

	iconCheck = "check"
	iconAlert = "alert"
	iconMinus = "minus"

	titleFitsTail  = " fit your conditions"
	titleCheckTail = " to check"

	keptEllipsis = "…"
	keptMaxChars = 200

	legendFitsTitle  = "✓ Fits"
	legendCheckTitle = "! Check"
	legendGreyTitle  = "– Not judged"
	legendCashTitle  = "Account cash"
	legendCashTier   = "cash"
	legendGreyTier   = "grey"

	notSet             = "not set"
	earningsUnknown    = "unknown"
	noSpotText         = "no spot"
	countFit           = " fit, "
	countCheck         = " check, "
	countCollapsed     = " collapsed"
	unreadableGates    = "unreadable_gates"
	dteSuffix          = "d"
	noValue            = "not formed"
	movesSuffix        = " moves*"
	yieldSuffix        = "%/yr"
	beHead             = "BE "
	beVs               = " vs "
	lineSep            = "; "
	lineBid            = "Bid "
	lineYield          = "yield "
	lineIntrinsic      = "intrinsic "
	lineTimeValue      = ", time value "
	lineBEGap          = "BE: "
	gapNoIV            = "no IV"
	gapNoBid           = "no bid"
	gapNoYield         = "no yield"
	gapBelowIntr       = "bid below intrinsic"
	whyBelowIntr       = "bid_below_intrinsic"
	belowIntrinsicText = gapBelowIntr + hintBelowIntrinsic
	fitsPrefix         = "fits: "
	fitsNone           = "fits: none"
	aliasSep           = ": "
	cashEnough         = "enough, "
	cashShort          = "short, "
	cashFree           = " free after "
	cashCommitted      = " committed ("
	cashNoFig          = "no figure: "
	gapNoAccount       = "No account cash figure"
	earningsLabelSp    = " "
	badgeToneWarn      = "warn"
	badgeToneInfo      = "info"
	badgeToneNone      = "none"
	evBadgeCode        = "evidence"
	cashBadgeHead      = "cash_"
)

// Column and field labels of the tier view.
const (
	colArrowID, colArrowLabel       = "arrow", "Details"
	colTierID, colTierLabel         = "tier", "Tier"
	colContractID, colContractLabel = "contract", "Contract"
	colMarginID, colMarginLabel     = "margin", "Margin of safety"
	colBelowSpotID, colBelowLabel   = "below_spot", "Below spot"
	colPremiumID, colPremiumLabel   = "premium", "Premium / yield"
	colBadgesID, colBadgesLabel     = "badges", "Badges"

	fieldIVID, fieldIVLabel           = "iv", "IV (this contract)"
	fieldDeltaID, fieldDeltaLabel     = "delta", "Delta (rough chance of assignment)"
	fieldITMID, fieldITMLabel         = "itm_probability", "ITM probability"
	fieldVsCashID, fieldVsCashLabel   = "vs_cash", "Vs cash"
	fieldCashID, fieldCashLabel       = "account_cash", "Account cash"
	fieldBidID, fieldBidLabel         = "bid_per_share", "Bid per share"
	deltaOutsideBand                  = "delta_outside_band"
	reasonStaleCash, reasonNoCash     = "stale cash", "no cash"
	reasonNotUSD, reasonOtherCurrency = "not USD", "other currency"
)

// greySpec is the title tail, the word of the kept line and the explanation of one grey reason.
type greySpec struct{ titleTail, keptLabel, gloss string }

var greySpecs = map[string]greySpec{
	"no_buy_price":    {" not judged: no buy price", zhGreyLabel_no_buy_price, zhGreyGloss_no_buy_price},
	"no_spot":         {" not judged: no spot", zhGreyLabel_no_spot, zhGreyGloss_no_spot},
	"in_the_money":    {" in the money", zhGreyLabel_in_the_money, zhGreyGloss_in_the_money},
	"no_bid":          {" not judged: no bid", zhGreyLabel_no_bid, zhGreyGloss_no_bid},
	"above_buy_price": {" break-even above buy price", zhGreyLabel_above_buy_price, zhGreyGloss_above_buy_price},
	"bad_input":       {" not judged: unreadable numbers", zhGreyLabel_bad_input, zhGreyGloss_bad_input},
}

const fallbackGrey = "bad_input"

// keptLabels are the Chinese words of the look-at badges in the kept line.
var keptLabels = map[string]string{
	"no_open_interest":       zhKeptLabel_no_open_interest,
	"low_open_interest":      zhKeptLabel_low_open_interest,
	"one_sided_quote":        zhKeptLabel_one_sided_quote,
	"wide_quote":             zhKeptLabel_wide_quote,
	"stale_quote":            zhKeptLabel_stale_quote,
	"earnings_before_expiry": zhKeptLabel_earnings_before_expiry,
	"cash_short":             zhKeptLabel_cash_short,
	"delta_above_bound":      zhKeptLabel_delta_above_bound,
	"delta_unknown":          zhKeptLabel_delta_unknown,
}

// badgeTexts are the English label and the Chinese gloss of each badge, note and info mark.
var badgeTexts = map[string]struct{ label, gloss string }{
	"no_open_interest":        {"No open interest", zhBadgeGloss_no_open_interest},
	"low_open_interest":       {"Low open interest", zhBadgeGloss_low_open_interest},
	"one_sided_quote":         {"One-sided quote", zhBadgeGloss_one_sided_quote},
	"wide_quote":              {"Wide quote", zhBadgeGloss_wide_quote},
	"stale_quote":             {"Stale quote", zhBadgeGloss_stale_quote},
	"earnings_before_expiry":  {"Earnings before expiry", zhBadgeGloss_earnings_before_expiry},
	"cash_short":              {"Cash short", zhBadgeGloss_cash_short},
	"delta_above_bound":       {"Delta above bound", zhBadgeGloss_delta_above_bound},
	"delta_unknown":           {"Delta unknown", zhBadgeGloss_delta_unknown},
	"thin_premium":            {"Thin premium", zhBadgeGloss_thin_premium},
	"earnings_unconfirmed":    {"Earnings unconfirmed", zhBadgeGloss_earnings_unconfirmed},
	"earnings_beyond_horizon": {"Earnings beyond horizon", zhBadgeGloss_earnings_beyond_horizon},
	"earnings_unknown":        {"Earnings unknown", zhBadgeGloss_earnings_unknown},
	"cash_unknown":            {"Cash unknown", zhBadgeGloss_cash_unknown},
}

// cashWhyTexts are the words of the reason an alias has no cash figure.
var cashWhyTexts = map[string]string{
	"stale_cash": reasonStaleCash, "no_cash": reasonNoCash, "not_usd": reasonNotUSD, "other_currency": reasonOtherCurrency,
}

// whyTexts are the gap texts of a figure the answer could not form, by its reason code.
var whyTexts = map[string]string{
	"no_buy_price": gapNoBuy, "no_iv": gapNoIV, "no_spot": noSpotText, "no_bid": gapNoBid, "in_the_money": "in the money",
}
