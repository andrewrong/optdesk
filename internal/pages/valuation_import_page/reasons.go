package valuation_import_page

// hinted maps an importer word (a reason or a review type) to the text with its hint.
var hinted = map[string]string{
	"the site computes another buy-zone top":         ReasonOtherTop,
	"HK and CN differ: the site has one HKCN column": ReasonHKCN,
	"the row R differs from the R of its market":     ReasonRMarket,
	"older than stored":                              TextOlder,
	"unmatched name":                                 TextUnmatched,
	"first":                                          TextFirst,
	"event":                                          TextEvent,
	"the key picked is not valid":                    ReasonBadKey,
}

// Reasons lists every sentence above, for the test that each has a Chinese hint.
var Reasons = []string{
	TextR, TextBuyZoneTop, TextFirst, TextEvent, TextGeneralTick, TextOlder, TextUnmatched,
	ReasonOtherTop, ReasonHKCN, ReasonRMarket, ReasonBadKey,
}
