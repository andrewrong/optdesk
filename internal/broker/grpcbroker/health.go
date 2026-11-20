package grpcbroker

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

// HealthClient implements broker.HealthReader: it reads the health of one gateway through
// SessionService.Ping over a Unix socket. It keeps no state between calls.
type HealthClient struct {
	conn    *grpc.ClientConn
	ses     pb.SessionServiceClient
	timeout time.Duration
}

var _ broker.HealthReader = (*HealthClient)(nil)

// DialHealth connects to the service on the Unix socket at socketPath and returns once it is
// reachable; an unreachable socket is a Retryable error. Only CallTimeout of o is used.
func DialHealth(ctx context.Context, socketPath string, o broker.Options) (*HealthClient, error) {
	o = withDefaults(o)
	conn, err := connect(ctx, socketPath)
	if err != nil {
		return nil, err
	}
	return &HealthClient{conn: conn, ses: pb.NewSessionServiceClient(conn), timeout: o.CallTimeout}, nil
}

// Health makes the one call, with no retry. The Name of the answer is empty. A call that fails for
// any reason of reachability, a cancelled context included, is Retryable. An answer that sets no
// state maps to up when connected and to down otherwise.
func (c *HealthClient) Health(ctx context.Context) (broker.Health, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	resp, err := c.ses.Ping(ctx, &pb.PingRequest{})
	if err != nil {
		return broker.Health{}, healthErr(ctx, err)
	}
	h := broker.Health{State: stateOf(resp)}
	for _, f := range resp.GetFarms() {
		h.Farms = append(h.Farms, broker.Farm{Name: f.GetName(), Up: f.GetUp(), Since: time.UnixMilli(f.GetSinceUnixMs()).UTC()})
	}
	return h, nil
}

// Close releases the connection.
func (c *HealthClient) Close() error { return c.conn.Close() }

func stateOf(r *pb.PingResponse) string {
	switch r.GetState() {
	case pb.SessionState_SESSION_STATE_UP:
		return "up"
	case pb.SessionState_SESSION_STATE_SOFT_LOST:
		return "soft_lost"
	case pb.SessionState_SESSION_STATE_DOWN:
		return "down"
	case pb.SessionState_SESSION_STATE_FARM_BROKEN:
		return "farm_broken"
	case pb.SessionState_SESSION_STATE_UNSPECIFIED:
	}
	if r.GetConnected() {
		return "up"
	}
	return "down"
}

// healthErr is wrapErr, except that a cancelled call is Retryable: a gateway that does not answer
// is a state to show, never a permanent failure.
func healthErr(ctx context.Context, err error) error {
	if errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled {
		return &brokerError{kind: contracts.Retryable, text: "broker: health: canceled", err: err}
	}
	return wrapErr(ctx, "health", err)
}
