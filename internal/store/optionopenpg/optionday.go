package optionopenpg

import "github.com/andrewrong/optdesk/internal/contracts"

// optionDayWriter is written to obs.option_day.written_by, and as its source, by this store.
const optionDayWriter = "option_open"

// optionDayUsable reports whether r writes a day row: status 1 or 2, the statuses the page shows
// (docs/design/option-captures.md Decision 6).
func optionDayUsable(r contracts.OptionOpen) bool { return r.Status == 1 || r.Status == 2 }

// optionDayArgs lists the arguments of shared.UpsertDaySQL for the usable row r, every numeric column screened, in the
// order that statement documents (shared.DayArgCount in all). An option_open capture carries no open interest, so the
// two arguments for it are NULL and the day row keeps the stored ones.
func optionDayArgs(r contracts.OptionOpen) []any {
	return []any{
		r.InstrumentID, r.SessionDay.Time, r.UnderlyingID, optionDayWriter, r.ReceivedAt, r.Status, r.Currency,
		screen(r.Bid), screen(r.Ask), screen(r.BidSize), screen(r.AskSize), screen(r.Last), screen(r.Volume),
		screen(r.UnderlyingPrice), r.UnderlyingCcy, screen(r.IvBidDecimal), screen(r.IvMidDecimal),
		screen(r.IvAskDecimal), screen(r.IvDecimal), screen(r.Delta), screen(r.Gamma),
		screen(r.ThetaPerCalendarDay), screen(r.VegaPerVolPoint), (*float64)(nil), nil, optionDayWriter,
	}
}
