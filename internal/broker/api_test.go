package broker_test

import (
	"context"
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type typedErr struct{ kind contracts.ErrorKind }

func (e typedErr) Error() string             { return "unavailable" }
func (e typedErr) Kind() contracts.ErrorKind { return e.kind }

// fakeBroker answers one Quote per contract, or a Retryable error when down is set.
type fakeBroker struct{ down bool }

func (f fakeBroker) ListChains(context.Context, broker.Contract) ([]broker.Chain, error) {
	return nil, nil
}

func (f fakeBroker) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	return broker.Resolved{Listed: true, Contract: c}, nil
}

func (f fakeBroker) Snapshot(_ context.Context, _ broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	if f.down {
		return nil, typedErr{contracts.Retryable}
	}
	out := make([]broker.Quote, len(cs))
	for i, c := range cs {
		out[i] = broker.Quote{Contract: c, Status: broker.StatusOK}
	}
	return out, nil
}

func (f fakeBroker) Close() error { return nil }

var (
	_ broker.Broker   = fakeBroker{}
	_ broker.DialFunc = func(context.Context, string, broker.Options) (broker.Broker, error) { return fakeBroker{}, nil }
)

// retryable is what a caller does with an error of the port.
func retryable(err error) bool {
	var te contracts.TypedError
	return errors.As(err, &te) && te.Kind() == contracts.Retryable
}

func TestSnapshotReturnsOneQuotePerContractInOrder(t *testing.T) {
	t.Parallel()
	cs := []broker.Contract{{Symbol: "B"}, {Symbol: "A"}}
	qs, err := broker.Broker(fakeBroker{}).Snapshot(context.Background(), broker.KindSnapshot, cs)
	if err != nil || len(qs) != 2 || qs[0].Contract.Symbol != "B" || qs[1].Contract.Symbol != "A" {
		t.Fatalf("got %v, %v", qs, err)
	}
}

func TestCallerBranchesOnRetryableTypedError(t *testing.T) {
	t.Parallel()
	_, err := broker.Broker(fakeBroker{down: true}).Snapshot(context.Background(), broker.KindOptionEod, nil)
	if !retryable(err) {
		t.Fatalf("want a Retryable TypedError, got %v", err)
	}
	if retryable(errors.New("plain")) || retryable(nil) {
		t.Fatal("a plain or nil error must not be retryable")
	}
}
