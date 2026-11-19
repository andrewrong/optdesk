package option_candidates_page

// Texts of the tier view (candidates-tiers.md Decision 11): labels and every number are English, the
// explanations and glosses are Chinese. A value in braces is filled by the page from the answer.
const (
	tierFitsWord  = zhWordFits
	tierCheckWord = zhWordCheck
	tierGreyWord  = "Not judged"
	tierGreyView  = zhWordGrey

	tierFitsID  = "fits"
	tierCheckID = "check"
	greyIDHead  = "grey_"

	glyphFits  = "✓"
	glyphCheck = "!"
	glyphGrey  = "–"

	iconCheck = "check"
	iconAlert = "alert"
	iconMinus = "minus"

	titleFitsTail  = zhTitleFitsHead
	titleCheckTail = zhTitleCheckHead

	keptEllipsis = "…"
	keptMaxChars = 200

	legendFitsTitle  = zhLegendFits
	legendCheckTitle = zhLegendCheck
	legendGreyTitle  = zhLegendGrey
	legendCashTitle  = zhLegendCash
	legendCashTier   = "cash"
	legendGreyTier   = "grey"

	notJudged          = "not judged"
	earningsUnknown    = "unknown"
	noSpotText         = "no spot"
	countFit           = " fit, "
	countCheck         = " check, "
	countCollapsed     = " collapsed"
	unreadableGates    = "unreadable_gates"
	dteSuffix          = "d"
	dteTierSuffix      = zhDteSuffix
	marginHead         = zhMarginHead
	marginVs           = zhMarginVs
	noValue            = "not formed"
	movesSuffix        = zhMovesSuffix
	yieldSuffix        = "%/yr"
	yieldSubSuffix     = zhYieldSuffix
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
	colArrowID, colArrowLabel       = "arrow", zhColArrow
	colTierID, colTierLabel         = "tier", zhColTier
	colContractID, colContractLabel = "contract", zhColContract
	colMarginID, colMarginLabel     = "margin", zhColMargin
	colBelowSpotID, colBelowLabel   = "below_spot", zhColBelow
	colPremiumID, colPremiumLabel   = "premium", zhColPremium
	colBadgesID, colBadgesLabel     = "badges", zhColBadges

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
	"no_buy_price":    {zhTitleNoBuyPrice, zhGreyLabel_no_buy_price, zhGreyGloss_no_buy_price},
	"no_spot":         {zhTitleNoSpot, zhGreyLabel_no_spot, zhGreyGloss_no_spot},
	"in_the_money":    {zhTitleInTheMoney, zhGreyLabel_in_the_money, zhGreyGloss_in_the_money},
	"no_bid":          {zhTitleNoBid, zhGreyLabel_no_bid, zhGreyGloss_no_bid},
	"above_buy_price": {zhTitleAboveBuy, zhGreyLabel_above_buy_price, zhGreyGloss_above_buy_price},
	"bad_input":       {zhTitleBadInput, zhGreyLabel_bad_input, zhGreyGloss_bad_input},
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
	"no_jpy_rate":            badgeLabelNoJPYRate,
	"no_jpy_funds":           badgeLabelNoJPYFunds,
}

// badgeTexts are the English label and the Chinese gloss of each badge, note and info mark.
var badgeTexts = map[string]struct{ label, gloss string }{
	"no_open_interest":        {zhBadgeNoOI, zhBadgeGloss_no_open_interest},
	"low_open_interest":       {zhBadgeLowOI, zhBadgeGloss_low_open_interest},
	"one_sided_quote":         {zhBadgeOneSided, zhBadgeGloss_one_sided_quote},
	"wide_quote":              {zhBadgeWideQuote, zhBadgeGloss_wide_quote},
	"stale_quote":             {zhBadgeStaleQuote, zhBadgeGloss_stale_quote},
	"earnings_before_expiry":  {zhBadgeEarnExpiry, zhBadgeGloss_earnings_before_expiry},
	"cash_short":              {zhBadgeCashShort, zhBadgeGloss_cash_short},
	"delta_above_bound":       {zhBadgeDeltaAbove, zhBadgeGloss_delta_above_bound},
	"delta_unknown":           {zhBadgeDeltaUnknown, zhBadgeGloss_delta_unknown},
	"thin_premium":            {zhBadgeThinPremium, zhBadgeGloss_thin_premium},
	"earnings_unconfirmed":    {zhBadgeEarnUnconf, zhBadgeGloss_earnings_unconfirmed},
	"earnings_beyond_horizon": {zhBadgeEarnBeyond, zhBadgeGloss_earnings_beyond_horizon},
	"earnings_unknown":        {zhBadgeEarnUnknown, zhBadgeGloss_earnings_unknown},
	"cash_unknown":            {zhBadgeCashUnknown, zhBadgeGloss_cash_unknown},
	"no_jpy_rate":             {zhBadgeNoJPYRate, markHintNoJPYRateZh},
	"no_jpy_funds":            {zhBadgeNoJPYFunds, markHintNoJPYFundsZh},
}

// cashWhyTexts are the words of the reason an alias has no cash figure.
var cashWhyTexts = map[string]string{
	"stale_cash": reasonStaleCash, "no_cash": reasonNoCash, "not_usd": reasonNotUSD, "other_currency": reasonOtherCurrency,
}

// whyTexts are the gap texts of a figure the answer could not form, by its reason code.
var whyTexts = map[string]string{
	"no_buy_price": gapNoBuy, "no_iv": gapNoIV, "no_spot": noSpotText, "no_bid": gapNoBid, "in_the_money": "in the money",
}
