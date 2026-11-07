package grpcbroker

import (
	"context"
	"time"

	"google.golang.org/grpc"

	"github.com/andrewrong/optdesk/internal/broker"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

// Account is the answer of one gateway: the alias of its entry (never an account id), the session
// generation and the rows. Values or Positions is empty when the gateway does not collect it.
type Account struct {
	Alias      string
	Generation int64
	Values     []AccountValue
	Positions  []Position
}

// AccountValue is one summary row; Value is the received decimal text.
type AccountValue struct {
	Tag      string
	Value    string
	Currency string
}

// Position is one open position; Quantity and AvgCost are the received decimal text.
type Position struct {
	Contract broker.Contract
	Quantity string
	AvgCost  string
}

// AccountClient reads the account of one gateway over a Unix socket. It keeps no state between calls.
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

// Account makes the one call, with no retry. An answer with no alias or a negative generation is Permanent.
func (c *AccountClient) Account(ctx context.Context) (Account, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	resp, err := c.acc.Account(ctx, &pb.AccountRequest{})
	if err != nil {
		return Account{}, wrapErr(ctx, "account", err)
	}
	if resp.GetAccount() == "" {
		return Account{}, permanent("account: answer has no alias")
	}
	if resp.GetGeneration() < 0 {
		return Account{}, permanent("account: negative generation")
	}
	out := Account{
		Alias: resp.GetAccount(), Generation: resp.GetGeneration(),
		Values:    make([]AccountValue, 0, len(resp.GetValues())),
		Positions: make([]Position, 0, len(resp.GetPositions())),
	}
	for _, v := range resp.GetValues() {
		out.Values = append(out.Values, AccountValue{Tag: v.GetTag(), Value: v.GetValue(), Currency: v.GetCurrency()})
	}
	for _, p := range resp.GetPositions() {
		out.Positions = append(out.Positions, Position{
			Contract: fromSpec(p.GetContract()), Quantity: p.GetQuantity(), AvgCost: p.GetAvgCost(),
		})
	}
	return out, nil
}

// Close releases the connection.
func (c *AccountClient) Close() error { return c.conn.Close() }
