package optioneod

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
)

// Row statuses of obs.option_eod.
const (
	statusOK      = 1
	statusNoQuote = 2
	statusError   = 5
)

// statusOf maps the status of a quote to the row status and the error code (nil when none): OK and PARTIAL give 1,
// NO_DATA 2, the others 5 with the IBKR code, else the status name.
func statusOf(q broker.Quote) (int, *string) {
	switch q.Status {
	case broker.StatusOK, broker.StatusPartial:
		return statusOK, nil
	case broker.StatusNoData:
		return statusNoQuote, nil
	case broker.StatusNoSubscription:
		return statusError, errorCode(q, "NO_SUBSCRIPTION")
	case broker.StatusTimeout:
		return statusError, errorCode(q, "TIMEOUT")
	case broker.StatusDisconnected:
		return statusError, errorCode(q, "DISCONNECTED")
	case broker.StatusError:
		return statusError, errorCode(q, "ERROR")
	}
	return statusError, errorCode(q, "ERROR")
}

func errorCode(q broker.Quote, name string) *string {
	if q.ErrorCode != 0 {
		s := strconv.Itoa(q.ErrorCode)
		return &s
	}
	return &name
}

// captureID is the capture of one key: uuid.NewSHA1(OID, day|dataset|subject).
func captureID(day contracts.Day, k contracts.ExpectedKey) jobs.CaptureID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(day)+"|"+k.DatasetID+"|"+k.SubjectKey))
}

// mapRow turns a quote into a row; a column the quote does not carry stays nil.
func mapRow(q broker.Quote, instrumentID, underlyingID int64, day time.Time, capture jobs.CaptureID, ccy string) contracts.OptionEod {
	status, code := statusOf(q)
	var eventTs *time.Time
	if q.EventTs != nil {
		t := q.EventTs.UTC()
		eventTs = &t
	}
	return contracts.OptionEod{
		SessionDay:          types.SerializableDate{Time: day},
		CaptureID:           capture.String(),
		InstrumentID:        int(instrumentID),
		UnderlyingID:        int(underlyingID),
		Generation:          int(q.Generation),
		MarketDataType:      q.MarketDataType,
		ReceivedAt:          q.ReceivedAt.UTC(),
		EventTs:             eventTs,
		Status:              status,
		ErrorCode:           code,
		Currency:            ccy,
		UnderlyingCcy:       ccy,
		SnapshotKind:        "eod",
		Bid:                 q.Bid,
		Ask:                 q.Ask,
		BidSize:             q.BidSize,
		AskSize:             q.AskSize,
		Last:                q.Last,
		Volume:              q.Volume,
		OpenInterest:        q.OpenInterest,
		UnderlyingPrice:     q.UndPrice,
		IvDecimal:           q.ImpliedVol,
		Delta:               q.Delta,
		Gamma:               q.Gamma,
		ThetaPerCalendarDay: q.Theta,
		VegaPerVolPoint:     q.Vega,
	}
}

// osiSymbol is the 21-character option symbol: symbol padded to 6, yymmdd, C or P, strike times 1000 in 8 digits.
func osiSymbol(c broker.Contract) (string, error) { return osiSymbolWith(c, c.Symbol) }

// osiSymbolWith is osiSymbol with root, the option root of the symbol, in place of the symbol; a root over 6 is refused.
func osiSymbolWith(c broker.Contract, root string) (string, error) {
	strike, err := strconv.ParseFloat(c.Strike, 64)
	if err != nil || len(c.Expiry) != 8 || len(root) > 6 || strike < 0 || strike >= 100000 {
		return "", fmt.Errorf("optioneod: no option symbol for %s %s %s", c.Symbol, c.Expiry, c.Strike)
	}
	return fmt.Sprintf("%-6s%s%s%08d", root, c.Expiry[2:], c.Right, int64(math.Round(strike*1000))), nil
}

func optionName(c broker.Contract) string {
	return strings.Join([]string{c.Symbol, c.Expiry, c.Right, c.Strike}, " ")
}
