package main

import (
	"context"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// brokerDownError is the retryable answer of unreachableBroker; its text holds no path and no dial text.
type brokerDownError struct{}

func (brokerDownError) Error() string             { return "broker is not reachable" }
func (brokerDownError) Kind() contracts.ErrorKind { return contracts.Retryable }

// unreachableBroker stands in for a broker whose dial failed: every call answers retryable.
type unreachableBroker struct{}

func (unreachableBroker) ListChains(context.Context, broker.Contract) ([]broker.Chain, error) {
	return nil, brokerDownError{}
}

func (unreachableBroker) ResolveContract(context.Context, broker.Contract) (broker.Resolved, error) {
	return broker.Resolved{}, brokerDownError{}
}

func (unreachableBroker) Snapshot(context.Context, broker.Kind, []broker.Contract) ([]broker.Quote, error) {
	return nil, brokerDownError{}
}

func (unreachableBroker) Close() error { return nil }
