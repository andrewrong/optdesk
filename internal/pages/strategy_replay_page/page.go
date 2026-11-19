// Package strategy_replay_page builds the page.strategy_replay_page.v1 view model: the form with its values
// (view form), the form with the report under it (view report), and the route-off page (view form, form null,
// notice form_off). The input is plain strings and counts; every number and text of the document is copied from
// it or is a constant of zh_texts.go, and the builder adds, divides and rounds nothing.
// Design: docs/design/strategy-replay-page.md#2.
package strategy_replay_page

// The views of the page.
const (
	ViewForm   = "form"
	ViewReport = "report"
)

// The notice keys.
const (
	NoticeFormOff      = "form_off"
	NoticeTimedOut     = "timed_out"
	NoticeBusy         = "busy"
	NoticeReadFailed   = "read_failed"
	NoticeBadLots      = "bad_lots"
	NoticeBadAsOf      = "bad_as_of"
	NoticeBadEarlyKeep = "bad_early_keep"
	NoticeBadEarlyDTE  = "bad_early_dte"
	NoticeBadMinLots   = "bad_min_lots"
)

// The values of the lots choice.
const (
	LotsShortPuts = "short_puts"
	LotsShortLots = "short_lots"
)

// FormInput holds the texts shown in the six fields.
type FormInput struct {
	Lots, Alias, AsOf, EarlyKeep, EarlyDTE, MinLots string
}

// Variant is a variant name with its parameter hash.
type Variant struct{ Name, ParamsHash string }

// Excluded is a reason lots were left out, with its sentence and lot count.
type Excluded struct {
	Reason, Text string
	Lots         int
}

// Line is one row of the report. An empty figure becomes null.
type Line struct {
	Set, Variant, Currency                                     string
	Lots, Cycles, ClosedEarly, Held, Assigned, Rolled, InMoney int
	KeptShare, MeanGross, WorstGross                           string
	HoldKeptShare, ActualKeptShare                             string
	UnderMinimum                                               bool
}

// ReportInput is the report as plain fields. Lots is the value of the lots choice; MinLots is the minimum the
// run used. An empty FirstQuoteDay or LastQuoteDay becomes null.
type ReportInput struct {
	AsOf, Lots                         string
	FirstQuoteDay, LastQuoteDay        string
	Variants                           []Variant
	LotCount, LotsWithQuote, StillOpen int
	LotDays, QuoteDays                 int
	Excluded                           []Excluded
	Lines                              []Line
	MinLots                            int
}

// Input is everything Build reads. Form nil is the route-off page: view form, form null, notice form_off,
// whatever else is set. Report is read in the view report only. Notices are keys of the Notice constants.
// GeneratedAt is copied as it is; the builder reads no clock.
type Input struct {
	GeneratedAt string
	View        string
	Form        *FormInput
	Report      *ReportInput
	Notices     []string
}
