package quotesnapshot

import (
	"strconv"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Row statuses of obs.quote_snapshot.
const (
	statusOK      = 1
	statusNoQuote = 2
	statusError   = 5
)

// mapQuote turns a broker.Quote into the row of one instrument: OK and PARTIAL give 1, NO_DATA 2, every
// other status 5 with the IBKR error code, else the status name. A column the quote does not carry is nil.
func mapQuote(q broker.Quote, day time.Time, slot, captureID, currency string, instrumentID int64) contracts.QuoteSnapshot {
	r := contracts.QuoteSnapshot{
		SessionDay:     types.SerializableDate{Time: day},
		CaptureID:      captureID,
		Slot:           slot,
		InstrumentID:   int(instrumentID),
		ReceivedAt:     q.ReceivedAt.UTC(),
		Generation:     int(q.Generation),
		MarketDataType: q.MarketDataType,
		Currency:       currency,
		Bid:            q.Bid, Ask: q.Ask, Last: q.Last, Volume: q.Volume,
		BidSize: q.BidSize, AskSize: q.AskSize, LastSize: q.LastSize,
	}
	if q.EventTs != nil {
		t := q.EventTs.UTC()
		r.EventTs = &t
	}
	switch q.Status {
	case broker.StatusOK, broker.StatusPartial:
		r.Status = statusOK
	case broker.StatusNoData:
		r.Status = statusNoQuote
	case broker.StatusNoSubscription, broker.StatusTimeout, broker.StatusError, broker.StatusDisconnected:
		r.Status = statusError
		code := statusName(q.Status)
		if q.ErrorCode != 0 {
			code = strconv.Itoa(q.ErrorCode)
		}
		r.ErrorCode = &code
	}
	return r
}

func statusName(s broker.Status) string {
	switch s {
	case broker.StatusNoSubscription:
		return "NO_SUBSCRIPTION"
	case broker.StatusTimeout:
		return "TIMEOUT"
	case broker.StatusDisconnected:
		return "DISCONNECTED"
	case broker.StatusOK, broker.StatusPartial, broker.StatusNoData, broker.StatusError:
		return "ERROR"
	}
	return "ERROR"
}
