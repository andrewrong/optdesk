package positionmarks

import (
	"context"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/store"
)

// The row status words of the dataset that no broker status gives.
const (
	statusNoData = "no_data"
	statusError  = "error"
)

// target is one position to mark. A non-empty status means no quote is asked for it: its row is basis none
// with that status (a contract that is not listed or does not route is never dropped, Decision 10).
type target struct {
	pos      store.Position
	contract broker.Contract
	status   string
}

// newestCapture keeps the positions of the newest capture of the day: the capture with the latest CapturedAt
// (the larger CaptureID breaks a tie).
func newestCapture(ps []store.Position) []store.Position {
	if len(ps) == 0 {
		return nil
	}
	best := ps[0]
	for _, p := range ps[1:] {
		if p.CapturedAt.After(best.CapturedAt) || (p.CapturedAt.Equal(best.CapturedAt) && p.CaptureID > best.CaptureID) {
			best = p
		}
	}
	out := make([]store.Position, 0, len(ps))
	for _, p := range ps {
		if p.CaptureID == best.CaptureID {
			out = append(out, p)
		}
	}
	return out
}

// contractOf builds the broker contract of a position from its ConID (Decision 10): sec type STK, IND or
// OPT, the expiry without dashes, P or C, and SMART as the exchange. The exchange of a contract that is not
// in USD is left empty here: planTarget fills it from ResolveContract. ok is false for a sec type with no word.
func contractOf(p store.Position) (c broker.Contract, ok bool) {
	c = broker.Contract{
		Symbol: p.Symbol, Currency: p.Currency, Strike: p.Strike, Multiplier: p.Multiplier,
		Expiry: strings.ReplaceAll(p.Expiry, "-", ""), ConID: p.ConID,
	}
	switch p.SecType {
	case "stock":
		c.SecType = "STK"
	case "index":
		c.SecType = "IND"
	case "option":
		c.SecType = "OPT"
	default:
		return c, false
	}
	switch p.Right {
	case "put":
		c.Right = "P"
	case "call":
		c.Right = "C"
	}
	if p.Currency == "USD" {
		c.Exchange = "SMART"
	}
	return c, true
}

// planTarget maps one position; a contract outside USD (JPY in practice) is first looked up with
// ResolveContract and takes the exchange of the answer. A failed lookup gives the row status "error", an
// unlisted contract "no_data"; the run goes on with the other positions.
func planTarget(ctx context.Context, market broker.Broker, p store.Position) target {
	c, ok := contractOf(p)
	if !ok {
		return target{pos: p, status: statusNoData}
	}
	if c.Exchange != "" {
		return target{pos: p, contract: c}
	}
	r, err := market.ResolveContract(ctx, c)
	if err != nil {
		return target{pos: p, status: statusError}
	}
	if !r.Listed || r.Contract.Exchange == "" {
		return target{pos: p, status: statusNoData}
	}
	c.Exchange = r.Contract.Exchange
	return target{pos: p, contract: c}
}

// statusWord is the broker status in words, the status enum of the dataset.
func statusWord(s broker.Status) string {
	switch s {
	case broker.StatusOK:
		return "ok"
	case broker.StatusPartial:
		return "partial"
	case broker.StatusNoData:
		return statusNoData
	case broker.StatusNoSubscription:
		return "no_subscription"
	case broker.StatusTimeout:
		return "timeout"
	case broker.StatusDisconnected:
		return "disconnected"
	case broker.StatusError:
		return statusError
	}
	return statusError
}

func key(captureID string, conID int64) string { return captureID + "#" + strconv.FormatInt(conID, 10) }
