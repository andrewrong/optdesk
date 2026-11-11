package option_boards

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// The values of quote.source (docs/design/option-open-pull.md Decisions 18 and 19).
const (
	sourceOpenPull = "open_pull"
	sourceRefresh  = "refresh"
	sourceArchive  = "close_archive"
)

// staleAfter is the age of a quote past which the page greys it (Decision 18, "older than 15 minutes grey").
const staleAfter = 15 * time.Minute

// archiveLabel is the word of a stored close value, shown with its date ("close archive, 11 Nov").
const archiveLabel = "close archive"

// The stored statuses of obs.option_open a page shows; any other status has no usable quote and the row falls back
// to its close archive value.
const (
	openStatusOK      = 1
	openStatusPartial = 2
)

// quote is the quote behind the prices of one row. The fields are copied from a stored row; only Stale is derived,
// here, from the build time, so that the web compares nothing.
type quote struct {
	AsOf        string  `json:"as_of"`
	DataType    *string `json:"data_type"`
	Source      string  `json:"source"`
	Skew        bool    `json:"skew"`
	OutsideBand bool    `json:"outside_band"`
	Stale       bool    `json:"stale"` // older than staleAfter at the build time; a close archive value is always stale
}

// refreshObject is the refresh object of the two schemas, a copy of RefreshState.
type refreshObject struct {
	Enabled    bool     `json:"enabled"`
	Symbols    []string `json:"symbols"`
	Notice     *string  `json:"notice"`
	FreshCount int      `json:"fresh_count"`
	TotalCount int      `json:"total_count"`
}

// refreshOf copies the state into the refresh object. A disabled button shows its Reason as the notice (the
// schema: when enabled is false the notice says why), otherwise the Notice; an empty text is null.
func refreshOf(s RefreshState) refreshObject {
	text := s.Notice
	if !s.Enabled && s.Reason != "" {
		text = s.Reason
	}
	var notice *string
	if text != "" {
		notice = &text
	}
	symbols := s.Symbols
	if symbols == nil {
		symbols = []string{}
	}
	return refreshObject{Enabled: s.Enabled, Symbols: symbols, Notice: notice, FreshCount: s.Fresh, TotalCount: s.Total}
}

// quoteSource holds what quote per row is copied from: the stored open rows of the build day, the close rows of
// loadEod and the data day, the date of a close value that has no end-of-day row.
type quoteSource struct {
	opens map[int]contracts.OptionOpen
	eod   map[int]contracts.OptionEod
	day   contracts.Day
	zone  *time.Location
	now   time.Time // the build time (generated_at), the clock of the staleness
}

// loadOpens reads the open rows of the build day by instrument id; a nil reader gives an empty map.
func loadOpens(ctx context.Context, r openquote.OptionOpenReader, day contracts.Day) (map[int]contracts.OptionOpen, error) {
	out := map[int]contracts.OptionOpen{}
	if r == nil {
		return out, nil
	}
	rows, err := r.OptionOpen(ctx, day)
	if err != nil {
		return nil, fmt.Errorf("option_boards: option open of %s: %w", day, err)
	}
	for _, o := range rows {
		if o.Status == openStatusOK || o.Status == openStatusPartial {
			out[o.InstrumentID] = o
		}
	}
	return out, nil
}

// of is the quote of one contract: its open or refresh row, else its close archive value.
func (q quoteSource) of(instrumentID int) quote {
	o, ok := q.opens[instrumentID]
	if !ok {
		return q.archive(instrumentID)
	}
	at := o.ReceivedAt
	if o.EventTs != nil {
		at = *o.EventTs
	}
	source := sourceOpenPull
	if o.Slot == openquote.SlotRefresh {
		source = sourceRefresh
	}
	return quote{AsOf: at.In(q.zone).Format(time.TimeOnly), DataType: dataTypeOf(o.MarketDataType), Source: source, Skew: o.Skew, OutsideBand: o.OutsideBand,
		Stale: q.now.Sub(at) > staleAfter}
}

// archive is the close archive quote of a contract, dated by its end-of-day row, else by the data day.
func (q quoteSource) archive(instrumentID int) quote {
	day := string(q.day)
	if e, ok := q.eod[instrumentID]; ok {
		day = eodDay(e, q.day)
	}
	return quote{AsOf: archiveText(day), Source: sourceArchive, Stale: true}
}

// chainQuote is the quote of a chain row; its cells are stored close bids of several contracts.
func (q quoteSource) chainQuote() quote {
	return quote{AsOf: archiveText(string(q.day)), Source: sourceArchive, Stale: true}
}

// archiveText is "close archive, 11 Nov" of a day text; a text that is not a day is shown as it is.
func archiveText(day string) string {
	t, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return archiveLabel + ", " + day
	}
	return archiveLabel + ", " + t.Format("2 Jan")
}

// dataTypeOf names the broker market data type: 1 live, 2 frozen, 3 delayed, 4 delayed frozen (shown as delayed);
// anything else is null.
func dataTypeOf(t int) *string {
	var s string
	switch t {
	case 1:
		s = "live"
	case 2:
		s = "frozen"
	case 3, 4:
		s = "delayed"
	default:
		return nil
	}
	return &s
}

// livePrices is the raw decimal text of the prices of an open pull or refresh row, shown next to the quote that names it.
// A field the row does not hold is "n/a"; it is never filled from the close archive, whose label it would not carry.
type livePrices struct {
	Bid, Ask, Delta, IV, Theta, Gamma, Vega, Volume string
}

// live is the prices of the open or refresh row of an instrument, nil when the quote is a close archive value.
func (q quoteSource) live(instrumentID int) *livePrices {
	o, ok := q.opens[instrumentID]
	if !ok {
		return nil
	}
	p := &livePrices{
		Bid: floatText(o.Bid), Ask: floatText(o.Ask), Delta: floatText(o.Delta),
		IV: gapText, Theta: floatText(o.ThetaPerCalendarDay), Gamma: floatText(o.Gamma),
		Vega: floatText(o.VegaPerVolPoint), Volume: floatText(o.Volume),
	}
	if iv := floatText(o.IvDecimal); iv != gapText {
		if s, err := shiftPercent(iv); err == nil {
			p.IV = s
		}
	}
	return p
}

// textPtr is nil for the gap text, else a pointer to the text; the table cells read stored decimals as pointers.
func textPtr(s string) *string {
	if s == gapText {
		return nil
	}
	return &s
}

// DeltaText is the delta as displayed: signed with the minus sign, or "n/a".
func (p livePrices) DeltaText() string {
	if p.Delta == gapText {
		return gapText
	}
	return signedText(p.Delta)
}
