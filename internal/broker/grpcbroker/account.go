package grpcbroker

import (
	"context"
	"time"

	"google.golang.org/grpc"

	"github.com/andrewrong/optdesk/internal/broker"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

// AccountClient implements broker.AccountReader: it reads the account of one gateway over a Unix socket. It keeps no state between calls.
type AccountClient struct {
	conn    *grpc.ClientConn
	acc     pb.AccountServiceClient
	timeout time.Duration
}

// DialAccount connects to the service on the Unix socket at socketPath and returns once it is
// reachable; an unreachable socket is a Retryable error. Only CallTimeout of o is used.
func DialAccount(ctx context.Context, socketPath string, o broker.Options) (*AccountClient, error) {
	o = withDefaults(o)
	conn, err := connect(ctx, socketPath)
	if err != nil {
		return nil, err
	}
	return &AccountClient{conn: conn, acc: pb.NewAccountServiceClient(conn), timeout: o.CallTimeout}, nil
}

// Account makes the one call, with no retry. The Alias of the answer is the value the gateway
// reports, replaced by the owner alias before a job sees it. An answer with no account or a negative
// generation is Permanent.
func (c *AccountClient) Account(ctx context.Context) (broker.Account, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	resp, err := c.acc.Account(ctx, &pb.AccountRequest{})
	if err != nil {
		return broker.Account{}, wrapErr(ctx, "account", err)
	}
	return fromAccount(resp)
}

// Executions makes the one call, with no retry. An answer with no account, a negative generation or
// an execution whose time is not RFC 3339 is Permanent.
func (c *AccountClient) Executions(ctx context.Context) (broker.Executions, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	resp, err := c.acc.Executions(ctx, &pb.ExecutionsRequest{})
	if err != nil {
		return broker.Executions{}, wrapErr(ctx, "executions", err)
	}
	return fromExecutions(resp)
}

// Close releases the connection.
func (c *AccountClient) Close() error { return c.conn.Close() }
