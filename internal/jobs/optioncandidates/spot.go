package optioncandidates

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// The marks that say where the spot of a row came from and which session its chain is from
// (docs/design/candidates-spot.md Decisions 1, 2 and 4).
const (
	markSpotQuotePrefix = "spot_quote:"
	markSpotChain       = "spot_chain"
	markSpotOpenPrefix  = "spot_open:"
	markChainDayPrefix  = "chain_day:"

	spotLookbackDays = 7
	quoteStatusOK    = 1
)

// spotFromQuote gives the Last of the newest stored stock quote of the instrument and the session day and ReceivedAt of that quote:
// session days from seven days before day up to day, rows with status ok and a Last, not received after now; the
// greatest session day wins, then the greatest ReceivedAt. found is false when no row qualifies.
func spotFromQuote(
	ctx context.Context, rd store.QuoteSnapshotReader, id int64, day contracts.Day, now time.Time,
) (price string, sessionDay contracts.Day, receivedAt time.Time, found bool, err error) {
	rows, err := rd.QuoteSnapshots(ctx, id, addDays(day, -spotLookbackDays), day)
	if err != nil {
		return "", "", time.Time{}, false, err
	}
	var best *contracts.QuoteSnapshot
	for i := range rows {
		r := &rows[i]
		if r.ReceivedAt.After(now) || r.Last == nil || r.Status != quoteStatusOK {
			continue
		}
		if best == nil || r.SessionDay.After(best.SessionDay.Time) ||
			(r.SessionDay.Equal(best.SessionDay.Time) && r.ReceivedAt.After(best.ReceivedAt)) {
			best = r
		}
	}
	if best == nil {
		return "", "", time.Time{}, false, nil
	}
	return optText(best.Last), contracts.Day(best.SessionDay.Format(time.DateOnly)), best.ReceivedAt, true, nil
}

// spotFromBar gives the close of the bar of basis trades with the greatest session day from day from up to day to
// (both inclusive) and that session day. found is false when there is no bar with a close.
func spotFromBar(
	ctx context.Context, rd bar.PriceBarReader, id int64, from, to contracts.Day,
) (closeText string, sessionDay contracts.Day, found bool, err error) {
	rows, err := rd.PriceBars(ctx, id, "trades", from, to)
	if err != nil {
		return "", "", false, err
	}
	var best *contracts.PriceBar
	for i := range rows {
		r := &rows[i]
		if r.Close == "" {
			continue
		}
		if best == nil || r.SessionDay.After(best.SessionDay.Time) {
			best = r
		}
	}
	if best == nil {
		return "", "", false, nil
	}
	return best.Close, contracts.Day(best.SessionDay.Format(time.DateOnly)), true, nil
}

// barFrom is the first day of the bar read: the session before the chain day, so a run at the open, whose chain day
// is the run day, reaches the previous close. Without a session before it, seven days before the chain day.
func barFrom(days []contracts.Day, chainDay contracts.Day) contracts.Day {
	from := addDays(chainDay, -spotLookbackDays)
	for _, d := range days {
		if d < chainDay {
			from = d
		}
	}
	return from
}

// openSpot gives the underlying_price of the newest open row of the run day that has one, as price text, and the
// received_at of that row. found is false when no row is of the day with a price.
func openSpot(rows []openRow, day contracts.Day) (price string, receivedAt time.Time, found bool) {
	var best *openRow
	for i := range rows {
		r := &rows[i]
		if r.UnderlyingPrice == nil || r.SessionDay.Format(time.DateOnly) != string(day) {
			continue
		}
		if best == nil || r.ReceivedAt.After(best.ReceivedAt) {
			best = r
		}
	}
	if best == nil {
		return "", time.Time{}, false
	}
	return optText(best.UnderlyingPrice), best.ReceivedAt, true
}
