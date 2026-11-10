package grpcbroker

import (
	"context"
	"errors"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

var _ broker.DialFunc = Dial

// The defaults of broker.Options.
const (
	defaultCallTimeout     = 25 * time.Second
	defaultSnapshotTimeout = 60 * time.Second
	defaultResolveInFlight = 5
	defaultHistoryTimeout  = 330 * time.Second
)

type client struct {
	conn  *grpc.ClientConn
	data  pb.MarketDataServiceClient
	cons  pb.ContractsServiceClient
	opts  broker.Options
	gate  gate
	stamp clock.Clock // nil in production: a missing received_at stays the zero time
}

// Dial connects to the service on the Unix socket at socketPath and returns once it is reachable.
// The path is the caller's; the client keeps no state between calls.
func Dial(ctx context.Context, socketPath string, o broker.Options) (broker.Broker, error) {
	o = withDefaults(o)
	conn, err := connect(ctx, socketPath)
	if err != nil {
		return nil, err
	}
	return &client{
		conn: conn, data: pb.NewMarketDataServiceClient(conn), cons: pb.NewContractsServiceClient(conn),
		opts: o, gate: newGate(o.ResolveInFlight),
	}, nil
}

// withDefaults fills the fields of o that are zero.
func withDefaults(o broker.Options) broker.Options {
	if o.CallTimeout <= 0 {
		o.CallTimeout = defaultCallTimeout
	}
	if o.SnapshotTimeout <= 0 {
		o.SnapshotTimeout = defaultSnapshotTimeout
	}
	if o.ResolveInFlight <= 0 {
		o.ResolveInFlight = defaultResolveInFlight
	}
	if o.HistoryTimeout <= 0 {
		o.HistoryTimeout = defaultHistoryTimeout
	}
	return o
}

// connect opens the connection to the Unix socket and returns once it is ready. Dial and DialAccount
// both use it; a failure is a TypedError whose text never holds the path.
func connect(ctx context.Context, socketPath string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient("unix:"+socketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, permanent("dial: bad socket address")
	}
	conn.Connect()
	for s := conn.GetState(); s != connectivity.Ready; s = conn.GetState() {
		if s == connectivity.TransientFailure || s == connectivity.Shutdown || !conn.WaitForStateChange(ctx, s) {
			_ = conn.Close()
			return nil, &brokerError{kind: contracts.Retryable, text: "broker: dial: service unavailable"}
		}
	}
	return conn, nil
}

func (c *client) Close() error { return c.conn.Close() }

func (c *client) ListChains(ctx context.Context, underlying broker.Contract) ([]broker.Chain, error) {
	spec, err := toSpec(underlying)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.opts.CallTimeout)
	defer cancel()
	resp, err := c.cons.ListChains(ctx, &pb.ListChainsRequest{Underlying: spec})
	if err != nil {
		return nil, wrapErr(ctx, "list chains", err)
	}
	out := make([]broker.Chain, 0, len(resp.GetChains()))
	for _, ch := range resp.GetChains() {
		out = append(out, fromChain(ch))
	}
	return out, nil
}

func (c *client) ResolveContract(ctx context.Context, k broker.Contract) (broker.Resolved, error) {
	spec, err := toSpec(k)
	if err != nil {
		return broker.Resolved{}, err
	}
	if err := c.gate.acquire(ctx); err != nil {
		return broker.Resolved{}, wrapErr(ctx, "resolve contract", err)
	}
	defer c.gate.release()
	ctx, cancel := context.WithTimeout(ctx, c.opts.CallTimeout)
	defer cancel()
	resp, err := c.cons.ResolveContract(ctx, &pb.ResolveContractRequest{Contract: spec})
	if err != nil {
		return broker.Resolved{}, wrapErr(ctx, "resolve contract", err)
	}
	switch resp.GetStatus() {
	case pb.ResolveStatus_RESOLVE_STATUS_FOUND:
		return broker.Resolved{Listed: true, Contract: fromSpec(resp.GetContract()), ErrorCode: int(resp.GetErrorCode()), ErrorText: resp.GetErrorText()}, nil
	case pb.ResolveStatus_RESOLVE_STATUS_NOT_LISTED:
		return broker.Resolved{Listed: false, Contract: k, ErrorCode: int(resp.GetErrorCode()), ErrorText: resp.GetErrorText()}, nil
	case pb.ResolveStatus_RESOLVE_STATUS_UNSPECIFIED:
		return broker.Resolved{}, permanent("resolve contract: unspecified status")
	default:
		return broker.Resolved{}, permanent("resolve contract: unknown status")
	}
}

func (c *client) Snapshot(ctx context.Context, kind broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	qk, err := quoteKind(kind)
	if err != nil {
		return nil, err
	}
	specs := make([]*pb.ContractSpec, len(cs))
	for i, k := range cs {
		if specs[i], err = toSpec(k); err != nil {
			return nil, err
		}
	}
	out := make([]broker.Quote, len(cs))
	for i, k := range cs {
		out[i] = broker.Quote{Contract: k, Status: broker.StatusDisconnected}
	}
	if len(cs) == 0 {
		return out, nil
	}
	ctx, cancel := context.WithTimeout(ctx, c.opts.SnapshotTimeout)
	defer cancel()
	stream, err := c.data.Snapshot(ctx, &pb.SnapshotRequest{Kind: qk, Contracts: specs})
	if err != nil {
		return nil, wrapErr(ctx, "snapshot", err)
	}
	filled := make([]bool, len(cs))
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, wrapErr(ctx, "snapshot", err)
		}
		q, err := fromQuote(resp.GetQuote(), c.stamp)
		if err != nil {
			return nil, err
		}
		for i, k := range cs {
			if !filled[i] && k.ConID == q.Contract.ConID {
				filled[i], out[i] = true, q
				break
			}
		}
	}
}
