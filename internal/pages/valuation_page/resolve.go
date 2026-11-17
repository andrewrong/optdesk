package valuation_page

import (
	"context"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// closeWindowDays is how far back from the page day the newest bar is looked for.
const closeWindowDays = 30

// basisTrades is the bar basis of the close: traded prices as printed.
const basisTrades = "trades"

// ErrNoTables is returned by Resolve when the state has no R version, no R table row source or no margin table: no
// line can be written.
var ErrNoTables = errors.New("valuation_page: no R version or margin table")

// CloseReader is the two reads of a close price: a symbol to an instrument id, then the bars of that id. It is
// filled with the stock id reader and the bar reader (Decision 18).
type CloseReader interface {
	StockIDs(ctx context.Context, names []string) ([]store.InstrumentName, error)
	PriceBars(ctx context.Context, instrumentID int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error)
}

// Resolved is the chain of one company: its lines, the band, and the close that was passed.
type Resolved struct {
	Lines    valuelinesapi.Lines
	Band     valuelinesapi.Band
	Close    string // "" when no close price was found
	CloseDay contracts.Day
	// CloseErr is set when a close read failed; the lines are then written without a close.
	CloseErr error
}

// Resolve is the one place of the chain of Decision 17: profile market column, newest R version, margin table,
// the current review and the close price of the profile's symbol, then Calculator.Lines and Calculator.Band.
// It copies the table rows of the store types into the plain valuelinesapi types and does nothing else with them.
// An entry with no current review gives (nil, nil). Missing tables give ErrNoTables; the error of Lines (for
// example valuelinesapi.ErrBadInput) is returned as it is. day is the page day, the last day of the close window.
func Resolve(ctx context.Context, e valuation.Entry, st valuation.State, cr CloseReader, calc valuelinesapi.Calculator, day contracts.Day) (*Resolved, error) {
	if e.Current == nil {
		return nil, nil
	}
	if st.RVersion == nil || st.Margin == nil {
		return nil, ErrNoTables
	}
	out := &Resolved{}
	out.Close, out.CloseDay, out.CloseErr = readClose(ctx, cr, e.Profile.Symbol, day)
	margin := make([]valuelinesapi.MarginRow, 0, len(st.Margin.Rows))
	for _, r := range st.Margin.Rows {
		margin = append(margin, valuelinesapi.MarginRow{PFrom: r.PFrom, M: r.M, W: r.W, H: r.H, S: r.S})
	}
	l, err := calc.Lines(valuelinesapi.Input{
		V: e.Current.V, P: e.Current.P, Market: e.Profile.RMarket,
		R:      valuelinesapi.RValues{US: st.RVersion.RUS, JP: st.RVersion.RJP, HKCN: st.RVersion.RHKCN},
		Margin: margin, Close: out.Close,
	})
	if err != nil {
		return nil, err
	}
	out.Lines = l
	out.Band = calc.Band(l, out.Close)
	return out, nil
}

// readClose is the close of the newest stored trades bar of symbol at or before day; "" when the symbol is empty,
// no id or no bar is found. A read error is returned and also gives no close.
func readClose(ctx context.Context, cr CloseReader, symbol string, day contracts.Day) (string, contracts.Day, error) {
	if cr == nil || symbol == "" {
		return "", "", nil
	}
	ids, err := cr.StockIDs(ctx, []string{symbol})
	if err != nil || len(ids) == 0 {
		return "", "", err
	}
	to, err := time.Parse(time.DateOnly, string(day))
	if err != nil {
		return "", "", nil
	}
	from := contracts.Day(to.AddDate(0, 0, -closeWindowDays).Format(time.DateOnly))
	bars, err := cr.PriceBars(ctx, ids[0].InstrumentID, basisTrades, from, day)
	if err != nil || len(bars) == 0 {
		return "", "", err
	}
	last := bars[len(bars)-1] // ascending by day
	return last.Close, contracts.Day(last.SessionDay.Format(time.DateOnly)), nil
}
