package valuation_import_page

// hinted maps an importer word (a reason or a review type) to the text with its hint.
var hinted = map[string]string{
	"the site computes another buy-zone top":                    ReasonOtherTop,
	"HK and CN differ: the site has one HKCN column":            ReasonHKCN,
	"R of the row differs from the R of its market in the file": ReasonRMarket,
	"no R version or margin table to compute the lines":         ReasonNoVersion,
	"the site cannot compute the lines: V or P is out of range": ReasonOutOfRange,
	"the list row has a market the import does not know":        ReasonUnknownMarket,
	"review type is required":                                   ProblemReviewType,
	"version date is required":                                  ProblemVersionDate,
	"V is required":                                             ProblemVRequired,
	"P is required":                                             ProblemPRequired,
	"V must be a number above 0":                                ProblemVRange,
	"P must be from 1 to 5 in steps of 0.5":                     ProblemPRange,
	"margin table has a value that is not a number":             ProblemMarginNumber,
	"older than stored":                                         TextOlder,
	"unmatched name":                                            TextUnmatched,
	"first":                                                     TextFirst,
	"event":                                                     TextEvent,
	"the key picked is not valid or is already on the list":     ReasonBadKey,
}

// Reasons lists every sentence above, for the test that each has a Chinese hint.
var Reasons = []string{
	TextR, TextBuyZoneTop, TextBuyZoneTopSite, TextBuyZoneTopFile, TextFirst, TextEvent, TextGeneralTick, TextOlder, TextUnmatched,
	ReasonOtherTop, ReasonHKCN, ReasonRMarket, ReasonBadKey, ReasonNoVersion, ReasonOutOfRange, ReasonUnknownMarket,
	ProblemReviewType, ProblemVersionDate, ProblemVRequired, ProblemPRequired, ProblemVRange, ProblemPRange,
	ProblemMarginNumber, ProblemNoRowHint, ProblemRowHint,
	TextTitle, TextIntro, TextFile, TextPreview, TextNotInFile, TextUnvalued, TextRemoved,
	TextCountInsert, TextCountSame, TextCountOlder, TextCountRefused, TextCountNeedsTick,
	TextStored, TextOlderThanStored, TextMarginTable, TextResult, TextSameFileAgain, TextConfirm,
}
