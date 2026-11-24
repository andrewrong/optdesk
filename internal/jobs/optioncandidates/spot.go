package optioncandidates

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// The marks that say where the spot of a row came from and which session its chain is from
// (docs/design/candidates-spot.md Decisions 1, 2 and 4).
const (
	markSpotQuotePrefix = "spot_quote:"
	markSpotChain       = "spot_chain"
	markChainDayPrefix  = "chain_day:"

	spotLookbackDays = 7
	quoteStatusOK    = 1
)

// spotFromQuote gives the Last of the newest stored stock quote of the instrument and the session day of that quote:
// session days from seven days before day up to day, rows with status ok and a Last, not received after now; the
// greatest session day wins, then the greatest ReceivedAt. found is false when no row qualifies.
func spotFromQuote(
	ctx context.Context, rd store.QuoteSnapshotReader, id int64, day contracts.Day, now time.Time,
) (price string, sessionDay contracts.Day, found bool, err error) {
	rows, err := rd.QuoteSnapshots(ctx, id, addDays(day, -spotLookbackDays), day)
	if err != nil {
		return "", "", false, err
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
		return "", "", false, nil
	}
	return optText(best.Last), contracts.Day(best.SessionDay.Format(time.DateOnly)), true, nil
}
