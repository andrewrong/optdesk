package option_tracking_page

import (
	"context"
	"fmt"
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

// markQuotes turns the stored position marks of day into quotes of the open positions whose
// Contract is the decimal con id of the row. A row without a price gives no quote. Text is
// copied; nothing is computed.
func markQuotes(ctx context.Context, c Config, alias string, day contracts.Day, positions []compute.LedgerPosition) ([]trackapi.TrackQuote, error) {
	rows, err := c.Marks.Marks(ctx, alias, string(day))
	if err != nil {
		return nil, fmt.Errorf("option_tracking_page: read position marks: %w", err)
	}
	byID := map[string]mark.PositionMark{}
	for _, r := range rows {
		if r.Basis == mark.BasisNone || r.Price == "" {
			continue
		}
		byID[strconv.FormatInt(r.ConID, 10)] = r
	}
	var out []trackapi.TrackQuote
	for _, p := range positions {
		r, ok := byID[p.Contract]
		if p.Status != "open" || !ok {
			continue
		}
		bid, ask := r.Price, r.Price
		if r.Bid != "" && r.Ask != "" {
			bid, ask = r.Bid, r.Ask
		}
		out = append(out, trackapi.TrackQuote{Underlying: p.Underlying, Expiry: contracts.Day(p.Expiry), Strike: p.Strike, Right: side(p.Right),
			Day: contracts.Day(r.SessionDay), Bid: bid, Ask: ask})
	}
	return out, nil
}
