package grpcbroker

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

var _ broker.NewHistoryFunc = DialHistory

var barKindOf = map[broker.BarKind]pb.BarKind{
	broker.BarTrades:           pb.BarKind_BAR_KIND_TRADES,
	broker.BarAdjustedLast:     pb.BarKind_BAR_KIND_ADJUSTED_LAST,
	broker.BarMidpoint:         pb.BarKind_BAR_KIND_MIDPOINT,
	broker.BarOptionImpliedVol: pb.BarKind_BAR_KIND_OPTION_IMPLIED_VOL,
}

// decimalText is a plain decimal number as text: digits with an optional sign and fraction.
var decimalText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

type historyClient struct {
	conn    *grpc.ClientConn
	data    pb.MarketDataServiceClient
	options broker.Options
}

// DialHistory connects to the service on the Unix socket at socketPath and returns once it is
// reachable; an unreachable socket is a Retryable error. HistoryTimeout of o bounds one Bars call.
func DialHistory(ctx context.Context, socketPath string, o broker.Options) (broker.HistoryClient, error) {
	o = withDefaults(o)
	conn, err := connect(ctx, socketPath)
	if err != nil {
		return nil, err
	}
	return &historyClient{conn: conn, data: pb.NewMarketDataServiceClient(conn), options: o}, nil
}

func (c *historyClient) Close() error { return c.conn.Close() }

// Bars asks for the daily bars of one contract. An unset or unknown kind and an End that is neither
// YYYY-MM-DD nor empty fail before any call; RESOURCE_EXHAUSTED (pacing) is Retryable. A bar whose
// day is not YYYY-MM-DD or whose price or volume is not a decimal is a Permanent error.
func (c *historyClient) Bars(ctx context.Context, q broker.BarQuery) ([]broker.Bar, error) {
	kind, ok := barKindOf[q.Kind]
	if !ok {
		return nil, permanent("bars: unset or unknown kind")
	}
	if q.End != "" && !isDay(q.End) {
		return nil, permanent("bars: end is not YYYY-MM-DD")
	}
	spec, err := toSpec(q.Contract)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.options.HistoryTimeout)
	defer cancel()
	resp, err := c.data.Bars(ctx, &pb.BarsRequest{
		Contract: spec, Kind: kind, DurationDays: int32(q.DurationDays), EndDay: q.End, //nolint:gosec // bounded by the service
	})
	if err != nil {
		if status.Code(err) == codes.ResourceExhausted {
			return nil, &brokerError{kind: contracts.Retryable, text: "broker: bars: " + codes.ResourceExhausted.String(), err: err}
		}
		return nil, wrapErr(ctx, "bars", err)
	}
	out := make([]broker.Bar, 0, len(resp.GetBars()))
	for _, b := range resp.GetBars() {
		bar, err := fromBar(b)
		if err != nil {
			return nil, err
		}
		out = append(out, bar)
	}
	return out, nil
}

func isDay(s string) bool {
	_, err := time.Parse(time.DateOnly, s)
	return err == nil && len(s) == len(time.DateOnly)
}

func fromBar(b *pb.Bar) (broker.Bar, error) {
	if !isDay(b.GetDay()) {
		return broker.Bar{}, permanent("bars: day is not YYYY-MM-DD")
	}
	for _, p := range []string{b.GetOpen(), b.GetHigh(), b.GetLow(), b.GetClose()} {
		if !decimalText.MatchString(p) {
			return broker.Bar{}, permanent(fmt.Sprintf("bars: price of %s is not a decimal", b.GetDay()))
		}
	}
	if b.Volume != nil && !decimalText.MatchString(b.GetVolume()) {
		return broker.Bar{}, permanent(fmt.Sprintf("bars: volume of %s is not a decimal", b.GetDay()))
	}
	return broker.Bar{
		Day: b.GetDay(), Open: b.GetOpen(), High: b.GetHigh(), Low: b.GetLow(), Close: b.GetClose(), Volume: b.Volume,
	}, nil
}
