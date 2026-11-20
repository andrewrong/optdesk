package shared

import "github.com/andrewrong/optdesk/internal/contracts"

const (
	// BarSourceIbkr, BarSourceYfinance and BarSourceMixed are the kinds BarSourceOf returns; they are the
	// values of the bars_source field of the pages.
	BarSourceIbkr     = "ibkr"
	BarSourceYfinance = "yfinance"
	BarSourceMixed    = "mixed"
)

// BarSourceOf says where the bars rows a page read came from: kind is ibkr (also for no rows),
// yfinance when every row is a Yahoo row, mixed when both sources are there. note is empty for ibkr and a
// Chinese sentence otherwise. A row of any source but yfinance counts as ibkr.
func BarSourceOf(rows []contracts.PriceBar) (kind, note string) {
	var ib, yf bool
	for _, r := range rows {
		if string(r.Source) == BarSourceYfinance {
			yf = true
		} else {
			ib = true
		}
	}
	switch {
	case yf && ib:
		return BarSourceMixed, noteMixed
	case yf:
		return BarSourceYfinance, noteYfinance
	}
	return BarSourceIbkr, ""
}
