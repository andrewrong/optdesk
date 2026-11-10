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
// ErrorCode and ErrorText are the error_code and error_text of the response (0 and "" when none).
type Resolved struct {
	Listed    bool
	Contract  Contract
	ErrorCode int
	ErrorText string
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
// (CallTimeout 25s, SnapshotTimeout 60s, ResolveInFlight 5, HistoryTimeout 330s).
type Options struct {
	CallTimeout     time.Duration
	SnapshotTimeout time.Duration
	ResolveInFlight int
	HistoryTimeout  time.Duration
}

// DialFunc connects to the service at socketPath.
type DialFunc func(ctx context.Context, socketPath string, o Options) (Broker, error)

// DividendStatus is the outcome of one dividend request.
type DividendStatus int

// The statuses of a DividendProjection; the unset status of the proto is an error, never a value.
const (
	DividendOK DividendStatus = iota
	DividendNoValue
	DividendTimeout
	DividendError
)

// DividendProjection is the answer for one contract (IBKR generic tick 456). An unset number or
// date is nil; NextExDate is YYYYMMDD. Nothing says whether the date is declared or projected.
type DividendProjection struct {
	Contract   Contract
	Status     DividendStatus
	Past12m    *float64
	Next12m    *float64
	NextExDate *string
	NextAmount *float64
	ErrorCode  int
	ReceivedAt time.Time
}

// ReferenceClient reads reference data. It is separate from Broker. Errors are contracts.TypedError
// as for Broker.
type ReferenceClient interface {
	// Dividends returns one DividendProjection per contract, in the order asked.
	Dividends(ctx context.Context, cs []Contract) ([]DividendProjection, error)
	// Close releases the connection.
	Close() error
}

// NewReferenceFunc connects a ReferenceClient to the service at socketPath.
type NewReferenceFunc func(ctx context.Context, socketPath string, o Options) (ReferenceClient, error)

// AccountValue is one account summary row; Value is decimal text.
type AccountValue struct {
	Tag      string
	Value    string
	Currency string
}

// Position is one open position; Quantity and AvgCost are decimal text, never floats.
type Position struct {
	Contract Contract
	Quantity string
	AvgCost  string
}

// Account is the answer of one gateway. Alias is the gateway label, never an account id.
// ValuesCollected and PositionsCollected are false when the gateway does not collect that part;
// its slice is empty then, which is not the same as collected and empty.
type Account struct {
	Alias              string
	Generation         int64
	Values             []AccountValue
	Positions          []Position
	ValuesCollected    bool
	PositionsCollected bool
}

// Execution is one complete fill with its commission. Time is the fill instant in UTC; numbers are
// decimal text. It carries no order id and no account id.
type Execution struct {
	ExecID             string
	PermID             int64
	Time               time.Time
	Contract           Contract
	Side               string
	Quantity           string
	Price              string
	Commission         string
	CommissionCurrency string
}

// Executions is the answer of one gateway. Collected is false when the gateway does not collect
// executions; Pending counts the fills left out because their commission report had not arrived.
type Executions struct {
	Alias      string
	Generation int64
	Collected  bool
	Pending    int
	Executions []Execution
}

// AccountReader reads the account of one gateway. It is separate from Broker. Errors are
// contracts.TypedError as for Broker.
type AccountReader interface {
	// Account returns the values and positions of the gateway.
	Account(ctx context.Context) (Account, error)
	// Executions returns the complete executions of the gateway.
	Executions(ctx context.Context) (Executions, error)
	// Close releases the connection.
	Close() error
}

// BarKind is the series of daily bars that is asked for. The zero value is unset and an error.
type BarKind int

// The kinds of ibkr.v1 BarKind, without the unset one.
const (
	BarTrades BarKind = iota + 1
	BarAdjustedLast
	BarMidpoint
	// BarOptionImpliedVol is IBKR's option implied volatility of the underlying; the bar's Close is the IV as decimal text (0.28 is 28 percent).
	BarOptionImpliedVol
)

// Bar is one daily bar. Day is YYYY-MM-DD; prices and Volume are decimal text, never floats;
// Volume is nil when the broker has none.
type Bar struct {
	Day    string
	Open   string
	High   string
	Low    string
	Close  string
	Volume *string
}

// BarQuery asks for the daily bars of one contract. The contract has a ConID and DurationDays is
// 1 to 730. End is the last day, YYYY-MM-DD, or empty for now; a non-empty End with BarAdjustedLast
// is refused by the service.
type BarQuery struct {
	Contract     Contract
	Kind         BarKind
	DurationDays int
	End          string
}

// HistoryClient reads daily bars. It is separate from Broker. Errors are contracts.TypedError as
// for Broker, and RESOURCE_EXHAUSTED (the service's pacing limit) is Retryable.
type HistoryClient interface {
	// Bars returns the bars of the query, oldest first.
	Bars(ctx context.Context, q BarQuery) ([]Bar, error)
	// Close releases the connection.
	Close() error
}

// NewHistoryFunc connects a HistoryClient to the service at socketPath.
type NewHistoryFunc func(ctx context.Context, socketPath string, o Options) (HistoryClient, error)
