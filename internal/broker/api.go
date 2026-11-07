// Package broker declares the Go client of the IBKR service: plain Go types and one port.
package broker

import (
	"context"
	"time"
)

// Kind is the kind of a snapshot request, one of the three kinds of ibkr.v1.
type Kind int

// The kinds of ibkr.v1.
const (
	KindOptionEod Kind = iota
	KindUnderlyingIV
	KindSnapshot
)

// Contract names one instrument. Expiry is YYYYMMDD and Strike is canonical decimal text.
type Contract struct {
	SecType      string
	Symbol       string
	Exchange     string
	Currency     string
	Expiry       string
	Strike       string
	Right        string
	Multiplier   string
	TradingClass string
	ConID        int64
}

// Chain is one option chain of an underlying. Strikes are the union over all expirations.
type Chain struct {
	Exchange     string
	TradingClass string
	Multiplier   string
	Expirations  []string
	Strikes      []string
}

// Status is the outcome of one quote.
type Status int

// The statuses of a quote.
const (
	StatusOK Status = iota
	StatusPartial
	StatusNoData
	StatusNoSubscription
	StatusTimeout
	StatusError
	StatusDisconnected
)

// Quote is the answer for one contract; an unset number is nil.
type Quote struct {
	Contract       Contract
	Status         Status
	MarketDataType int
	Generation     int64
	ReceivedAt     time.Time
	EventTs        *time.Time
	Bid            *float64
	Ask            *float64
	Last           *float64
	Close          *float64
	Volume         *float64
	OpenInterest   *float64
	BidSize        *float64
	AskSize        *float64
	LastSize       *float64
	ImpliedVol     *float64
	Delta          *float64
	Gamma          *float64
	Vega           *float64
	Theta          *float64
	UndPrice       *float64
	ErrorCode      int
	ErrorText      string
}

// Resolved is the answer of ResolveContract; Listed false is the proto's NOT_LISTED.
type Resolved struct {
	Listed   bool
	Contract Contract
}

// Broker is the client of the IBKR service. Every error is a contracts.TypedError:
// UNAVAILABLE and DEADLINE_EXCEEDED are Retryable, every other code Permanent.
type Broker interface {
	// ListChains returns the option chains of an underlying.
	ListChains(ctx context.Context, underlying Contract) ([]Chain, error)
	// ResolveContract looks one contract up.
	ResolveContract(ctx context.Context, c Contract) (Resolved, error)
	// Snapshot returns one Quote per contract, in the order asked; it does not split the call.
	Snapshot(ctx context.Context, kind Kind, cs []Contract) ([]Quote, error)
	// Close releases the connection.
	Close() error
}

// Options bounds the calls of a client; a zero field takes its default
// (CallTimeout 25s, SnapshotTimeout 60s, ResolveInFlight 5).
type Options struct {
	CallTimeout     time.Duration
	SnapshotTimeout time.Duration
	ResolveInFlight int
}

// DialFunc connects to the service at socketPath.
type DialFunc func(ctx context.Context, socketPath string, o Options) (Broker, error)
