package grpcbroker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"google.golang.org/grpc"

	"github.com/andrewrong/optdesk/internal/broker"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

var _ broker.NewReferenceFunc = DialReference

var dividendStatusOfProto = map[pb.DividendStatus]broker.DividendStatus{
	pb.DividendStatus_DIVIDEND_STATUS_OK:       broker.DividendOK,
	pb.DividendStatus_DIVIDEND_STATUS_NO_VALUE: broker.DividendNoValue,
	pb.DividendStatus_DIVIDEND_STATUS_TIMEOUT:  broker.DividendTimeout,
	pb.DividendStatus_DIVIDEND_STATUS_ERROR:    broker.DividendError,
}

type referenceClient struct {
	conn    *grpc.ClientConn
	ref     pb.ReferenceServiceClient
	options broker.Options
}

// DialReference connects to the service on the Unix socket at socketPath and returns once it is
// reachable; an unreachable socket is a Retryable error. SnapshotTimeout of o bounds one Dividends call.
func DialReference(ctx context.Context, socketPath string, o broker.Options) (broker.ReferenceClient, error) {
	o = withDefaults(o)
	conn, err := connect(ctx, socketPath)
	if err != nil {
		return nil, err
	}
	return &referenceClient{conn: conn, ref: pb.NewReferenceServiceClient(conn), options: o}, nil
}

func (c *referenceClient) Close() error { return c.conn.Close() }

// Dividends sends every key, then reads one answer per key; answers are matched to keys by con_id,
// or by symbol, exchange and currency when the con_id is 0. A key with no answer, an unspecified
// status and an unparsable number are Permanent errors.
func (c *referenceClient) Dividends(ctx context.Context, cs []broker.Contract) ([]broker.DividendProjection, error) {
	keys := make([]*pb.ContractKey, len(cs))
	for i, k := range cs {
		st, ok := secTypeOfText[k.SecType]
		if !ok {
			return nil, permanent(fmt.Sprintf("unknown sec type %q", k.SecType))
		}
		keys[i] = &pb.ContractKey{Symbol: k.Symbol, SecType: st, Exchange: k.Exchange, Currency: k.Currency, ConId: k.ConID}
	}
	out := make([]broker.DividendProjection, len(cs))
	if len(cs) == 0 {
		return out, nil
	}
	ctx, cancel := context.WithTimeout(ctx, c.options.SnapshotTimeout)
	defer cancel()
	stream, err := c.ref.Dividends(ctx)
	if err != nil {
		return nil, wrapErr(ctx, "dividends", err)
	}
	for _, k := range keys {
		if err := stream.Send(k); err != nil {
			return nil, wrapErr(ctx, "dividends", sendErr(stream, err))
		}
	}
	if err := stream.CloseSend(); err != nil {
		return nil, wrapErr(ctx, "dividends", err)
	}
	filled := make([]bool, len(cs))
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, wrapErr(ctx, "dividends", err)
		}
		p, err := fromDividend(resp)
		if err != nil {
			return nil, err
		}
		for i, k := range keys {
			if !filled[i] && sameKey(k, resp.GetKey()) {
				p.Contract = cs[i]
				filled[i], out[i] = true, p
				break
			}
		}
	}
	for i, ok := range filled {
		if !ok {
			return nil, permanent(fmt.Sprintf("dividends: no answer for %s", cs[i].Symbol))
		}
	}
	return out, nil
}

// sendErr returns the error the server ended the stream with, which Send itself reports as io.EOF.
func sendErr(s grpc.BidiStreamingClient[pb.ContractKey, pb.DividendProjection], err error) error {
	if errors.Is(err, io.EOF) {
		if _, e := s.Recv(); e != nil && !errors.Is(e, io.EOF) {
			return e
		}
	}
	return err
}

func sameKey(want, got *pb.ContractKey) bool {
	if want.GetConId() != 0 || got.GetConId() != 0 {
		return want.GetConId() == got.GetConId()
	}
	return want.GetSymbol() == got.GetSymbol() && want.GetExchange() == got.GetExchange() &&
		want.GetCurrency() == got.GetCurrency()
}

func fromDividend(d *pb.DividendProjection) (broker.DividendProjection, error) {
	st, ok := dividendStatusOfProto[d.GetStatus()]
	if !ok {
		return broker.DividendProjection{}, permanent(fmt.Sprintf("dividends: status %s", d.GetStatus()))
	}
	out := broker.DividendProjection{Status: st, ErrorCode: int(d.GetErrorCode()), NextExDate: d.NextExDate}
	if d.GetReceivedAt() != nil {
		out.ReceivedAt = d.GetReceivedAt().AsTime().UTC()
	}
	for _, f := range []struct {
		text *string
		to   **float64
	}{{d.Past_12M, &out.Past12m}, {d.Next_12M, &out.Next12m}, {d.NextAmount, &out.NextAmount}} {
		if f.text == nil {
			continue
		}
		v, err := strconv.ParseFloat(*f.text, 64)
		if err != nil {
			return broker.DividendProjection{}, permanent("dividends: unparsable number")
		}
		*f.to = &v
	}
	return out, nil
}
