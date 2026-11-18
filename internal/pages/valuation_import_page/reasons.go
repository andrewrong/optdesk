package valuation_import_page

// importerReasons maps the importer's English reason to the sentence with its hint.
var importerReasons = map[string]string{
	"the site computes another buy-zone top":         ReasonOtherTop,
	"HK and CN differ: the site has one HKCN column": ReasonHKCN,
	"the row R differs from the R of its market":     ReasonRMarket,
	"the key picked is not valid":                    ReasonBadKey,
}

// Reasons lists every sentence above, for the test that each has a Chinese hint.
var Reasons = []string{
	TextR, TextBuyZoneTop, TextFirst, TextEvent, TextGeneralTick, TextOlder, TextUnmatched,
	ReasonOtherTop, ReasonHKCN, ReasonRMarket, ReasonBadKey,
}
