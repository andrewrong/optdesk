package option_candidates_page

import "github.com/andrewrong/optdesk/internal/compute/buypriceapi"

// English texts of the short put list (candidates-page-v2.md Decisions 2, 9, 10, 11 and 14). The Chinese hints
// and glosses are in zh_texts.go.
const (
	slRunDayHead   = "Run day "
	slScanned      = "Scanned "
	slPutsSep      = " puts / Shown "
	slRows         = " rows, "
	slUnderlyings  = " underlyings"
	slFoldedHead   = "All "
	slFoldedTail   = " puts (tier view and stored tables)"
	slDroppedHead  = "Not shown "
	slRowOne       = " row"
	slRowMany      = " rows"
	slBest         = ", best "
	slRange        = "-"
	slPct          = "%"
	slDaysSuffix   = " d"
	slMarginJoin   = ", margin "
	slNoDelta      = "No delta"
	slNoYield      = "No yield"
	slNoBuyPrice   = "No buy price"
	slPriceLines   = "/plan/price-lines"
	slWideSpreadID = "wide_spread"
	slWideSpread   = "Wide spread"

	slColStrikeID, slColStrikeLabel         = "strike", "Strike"
	slColExpiryID, slColExpiryLabel         = "expiry", "Expiry"
	slColOTMID, slColOTMLabel               = "otm", "OTM %"
	slColDeltaID, slColDeltaLabel           = "delta", "Delta"
	slColPremiumID, slColPremiumLabel       = "premium", "Premium (bid)"
	slColAnnualizedID, slColAnnualizedLabel = "annualized", "Annualized"
	slColBuyID, slColBuyLabel               = "buy", "Buy price"
	slColLiquidityID, slColLiquidityLabel   = "liquidity", "Liquidity"

	slChipExpiryID, slChipExpiryLabel = "expiry", "Expiry"
	slChipStrikeID, slChipStrikeLabel = "strike", "Strike below spot"
	slChipDeltaID, slChipDeltaLabel   = "delta", "|Delta|"
	slChipCapID, slChipCapLabel       = "cap", "Max per underlying"
	slChipBidID, slChipBidLabel       = "bid", "Bid"
	slChipOIID, slChipOILabel         = "open_interest", "Open interest"
	slChipPresent                     = "present"

	slDropOverHead = "Over "
	slDropOverTail = " per underlying"
	slDropOTMLow   = "OTM below "
	slDropOTMHigh  = "OTM above "
)

// dropLabels are the labels of the codes of optionshortlist; otm_low, otm_high and over_cap are written with
// the bounds of the result by dropLabel.
var dropLabels = map[string]string{
	"bad_row": "Unreadable row", "no_bid": "No bid", "no_open_interest": "No open interest", "no_spot": "No spot",
	"no_delta": slNoDelta, "delta_out": "Delta out of band",
}

// dropHints are the Chinese hints of the codes, by code.
var dropHints = map[string]string{
	"bad_row": zhDropBadRow, "no_bid": zhDropNoBid, "no_open_interest": zhDropNoOpenInterest, "no_spot": zhDropNoSpot,
	"otm_low": zhDropOTMLow, "otm_high": zhDropOTMHigh, "no_delta": zhDropNoDelta, "delta_out": zhDropDeltaOut,
	"over_cap": zhDropOverCap,
}

// buySourceTexts are the label and Chinese gloss of each source code of buySourceCode.
var buySourceTexts = map[string]struct{ label, gloss string }{
	buypriceapi.SourceValuation: {"Valuation buy zone", zhBuySourceValuation},
	buypriceapi.SourceHand:      {"Hand price", zhBuySourceHand},
	buypriceapi.SourceNone:      {tierGreyWord, zhBuySourceNone},
}
