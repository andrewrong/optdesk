package grpcbroker

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

var _ broker.ContractLister = (*client)(nil)

// ListContracts lists the contracts of underlying at expiry with one rpc and keeps those of
// tradingClass (every class when it is empty). UNIMPLEMENTED, from a service older than the rpc,
// is a Permanent error that wraps broker.ErrNotServed. CallTimeout bounds the call.
func (c *client) ListContracts(ctx context.Context, underlying broker.Contract, expiry contracts.Day, right, tradingClass string) ([]broker.Contract, error) {
	spec, err := toSpec(underlying)
	if err != nil {
		return nil, err
	}
	r, ok := rightOfText[right]
	if !ok {
		return nil, permanent("list contracts: unknown right")
	}
	ctx, cancel := context.WithTimeout(ctx, c.opts.CallTimeout)
	defer cancel()
	resp, err := c.cons.ListContracts(ctx, &pb.ListContractsRequest{
		Underlying: spec, Expiry: string(expiry), Right: r, TradingClass: tradingClass,
	})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return nil, &brokerError{kind: contracts.Permanent, text: "broker: list contracts: " + codes.Unimplemented.String(), err: broker.ErrNotServed}
		}
		return nil, wrapErr(ctx, "list contracts", err)
	}
	out := make([]broker.Contract, 0, len(resp.GetContracts()))
	for _, s := range resp.GetContracts() {
		if tradingClass != "" && s.GetTradingClass() != tradingClass {
			continue
		}
		out = append(out, fromSpec(s))
	}
	return out, nil
}
