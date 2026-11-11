package grpcbroker

import (
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

// The text of a SecType and a Right in broker.Contract; empty is unspecified.
var (
	secTypeText = map[pb.SecType]string{
		pb.SecType_SEC_TYPE_STOCK:  "STK",
		pb.SecType_SEC_TYPE_INDEX:  "IND",
		pb.SecType_SEC_TYPE_OPTION: "OPT",
		pb.SecType_SEC_TYPE_CASH:   "CASH",
	}
	rightText = map[pb.Right]string{
		pb.Right_RIGHT_PUT:  "P",
		pb.Right_RIGHT_CALL: "C",
	}
	kindOfBroker = map[broker.Kind]pb.QuoteKind{
		broker.KindOptionEod:    pb.QuoteKind_QUOTE_KIND_OPTION_EOD,
		broker.KindUnderlyingIV: pb.QuoteKind_QUOTE_KIND_UNDERLYING_IV,
		broker.KindSnapshot:     pb.QuoteKind_QUOTE_KIND_SNAPSHOT,
		broker.KindOptionOpen:   pb.QuoteKind_QUOTE_KIND_OPTION_OPEN,
	}
	statusOfProto = map[pb.QuoteStatus]broker.Status{
		pb.QuoteStatus_QUOTE_STATUS_OK:              broker.StatusOK,
		pb.QuoteStatus_QUOTE_STATUS_PARTIAL:         broker.StatusPartial,
		pb.QuoteStatus_QUOTE_STATUS_NO_DATA:         broker.StatusNoData,
		pb.QuoteStatus_QUOTE_STATUS_NO_SUBSCRIPTION: broker.StatusNoSubscription,
		pb.QuoteStatus_QUOTE_STATUS_TIMEOUT:         broker.StatusTimeout,
		pb.QuoteStatus_QUOTE_STATUS_ERROR:           broker.StatusError,
		pb.QuoteStatus_QUOTE_STATUS_DISCONNECTED:    broker.StatusDisconnected,
	}
)

func invert[K comparable, V comparable](m map[K]V) map[V]K {
	out := make(map[V]K, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

var (
	secTypeOfText = invert(secTypeText)
	rightOfText   = invert(rightText)
)

// quoteKind maps a broker.Kind; an unknown Kind is a Permanent error.
func quoteKind(k broker.Kind) (pb.QuoteKind, error) {
	if v, ok := kindOfBroker[k]; ok {
		return v, nil
	}
	return pb.QuoteKind_QUOTE_KIND_UNSPECIFIED, permanent(fmt.Sprintf("unknown kind %d", int(k)))
}

// toSpec converts a contract; an unknown SecType or Right text is a Permanent error.
func toSpec(c broker.Contract) (*pb.ContractSpec, error) {
	st, ok := secTypeOfText[c.SecType]
	if !ok {
		return nil, permanent(fmt.Sprintf("unknown sec type %q", c.SecType))
	}
	r, ok := rightOfText[c.Right]
	if !ok {
		return nil, permanent(fmt.Sprintf("unknown right %q", c.Right))
	}
	return &pb.ContractSpec{
		SecType: st, Symbol: c.Symbol, Exchange: c.Exchange, Currency: c.Currency,
		Expiry: c.Expiry, Strike: c.Strike, Right: r, Multiplier: c.Multiplier,
		TradingClass: c.TradingClass, ConId: c.ConID,
	}, nil
}

func fromSpec(s *pb.ContractSpec) broker.Contract {
	if s == nil {
		return broker.Contract{}
	}
	return broker.Contract{
		SecType: secTypeText[s.GetSecType()], Symbol: s.GetSymbol(), Exchange: s.GetExchange(),
		Currency: s.GetCurrency(), Expiry: s.GetExpiry(), Strike: s.GetStrike(),
		Right: rightText[s.GetRight()], Multiplier: s.GetMultiplier(),
		TradingClass: s.GetTradingClass(), ConID: s.GetConId(),
	}
}

func fromChain(c *pb.Chain) broker.Chain {
	return broker.Chain{
		Exchange: c.GetExchange(), TradingClass: c.GetTradingClass(), Multiplier: c.GetMultiplier(),
		Expirations: c.GetExpirations(), Strikes: c.GetStrikes(),
	}
}

// fromQuote converts one quote; QUOTE_STATUS_UNSPECIFIED is a Permanent error. A missing
// received_at is stamped by now when it is set, and stays the zero time otherwise.
func fromQuote(q *pb.Quote, now clock.Clock) (broker.Quote, error) {
	st, ok := statusOfProto[q.GetStatus()]
	if !ok {
		return broker.Quote{}, permanent(fmt.Sprintf("quote status %s", q.GetStatus()))
	}
	out := broker.Quote{
		Contract:       fromSpec(q.GetContract()),
		Status:         st,
		MarketDataType: int(q.GetMarketDataType()),
		Generation:     q.GetGeneration(),
		Bid:            q.Bid, Ask: q.Ask, Last: q.Last, Close: q.Close,
		Volume: q.Volume, OpenInterest: q.OpenInterest,
		BidSize: q.BidSize, AskSize: q.AskSize, LastSize: q.LastSize,
		ImpliedVol: q.ImpliedVol, Delta: q.Delta, Gamma: q.Gamma, Vega: q.Vega,
		Theta: q.Theta, UndPrice: q.UndPrice,
		ErrorCode: int(q.GetErrorCode()), ErrorText: q.GetErrorText(),
	}
	switch {
	case q.GetReceivedAt() != nil:
		out.ReceivedAt = q.GetReceivedAt().AsTime().UTC()
	case now != nil:
		out.ReceivedAt = now.Now().UTC()
	}
	if q.GetEventTs() != nil {
		t := q.GetEventTs().AsTime().UTC()
		out.EventTs = &t
	}
	return out, nil
}

// fromAccount converts an account answer; no alias or a negative generation is a Permanent error.
func fromAccount(r *pb.AccountResponse) (broker.Account, error) {
	if r.GetAccount() == "" {
		return broker.Account{}, permanent("account: answer has no alias")
	}
	if r.GetGeneration() < 0 {
		return broker.Account{}, permanent("account: negative generation")
	}
	out := broker.Account{
		Alias: r.GetAccount(), Generation: r.GetGeneration(),
		Values:             make([]broker.AccountValue, 0, len(r.GetValues())),
		Positions:          make([]broker.Position, 0, len(r.GetPositions())),
		ValuesCollected:    r.GetValuesCollected(),
		PositionsCollected: r.GetPositionsCollected(),
	}
	for _, v := range r.GetValues() {
		out.Values = append(out.Values, broker.AccountValue{Tag: v.GetTag(), Value: v.GetValue(), Currency: v.GetCurrency()})
	}
	for _, p := range r.GetPositions() {
		out.Positions = append(out.Positions, broker.Position{
			Contract: fromSpec(p.GetContract()), Quantity: p.GetQuantity(), AvgCost: p.GetAvgCost(),
		})
	}
	return out, nil
}

// fromExecutions converts an executions answer; no alias, a negative generation or a time that is
// not RFC 3339 is a Permanent error. The time is returned in UTC; decimals stay text.
func fromExecutions(r *pb.ExecutionsResponse) (broker.Executions, error) {
	if r.GetAccount() == "" {
		return broker.Executions{}, permanent("executions: answer has no alias")
	}
	if r.GetGeneration() < 0 {
		return broker.Executions{}, permanent("executions: negative generation")
	}
	out := broker.Executions{
		Alias: r.GetAccount(), Generation: r.GetGeneration(),
		Collected: r.GetCollected(), Pending: int(r.GetPending()),
		Executions: make([]broker.Execution, 0, len(r.GetExecutions())),
	}
	for _, e := range r.GetExecutions() {
		t, err := time.Parse(time.RFC3339, e.GetTime())
		if err != nil {
			return broker.Executions{}, permanent("executions: time is not RFC 3339")
		}
		out.Executions = append(out.Executions, broker.Execution{
			ExecID: e.GetExecId(), PermID: e.GetPermId(), Time: t.UTC(),
			Contract: fromSpec(e.GetContract()), Side: e.GetSide(), Quantity: e.GetQuantity(),
			Price: e.GetPrice(), Commission: e.GetCommission(), CommissionCurrency: e.GetCommissionCurrency(),
		})
	}
	return out, nil
}

func init() {
	// An empty text is the unspecified value of the proto.
	secTypeOfText[""] = pb.SecType_SEC_TYPE_UNSPECIFIED
	rightOfText[""] = pb.Right_RIGHT_UNSPECIFIED
}
