package grpcbroker

import (
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

func ptr(v float64) *float64 { return &v }

func TestQuoteFields(t *testing.T) {
	t.Parallel()
	statuses := []struct {
		in   pb.QuoteStatus
		want broker.Status
		bad  bool
	}{
		{pb.QuoteStatus_QUOTE_STATUS_UNSPECIFIED, 0, true},
		{pb.QuoteStatus_QUOTE_STATUS_OK, broker.StatusOK, false},
		{pb.QuoteStatus_QUOTE_STATUS_PARTIAL, broker.StatusPartial, false},
		{pb.QuoteStatus_QUOTE_STATUS_NO_DATA, broker.StatusNoData, false},
		{pb.QuoteStatus_QUOTE_STATUS_NO_SUBSCRIPTION, broker.StatusNoSubscription, false},
		{pb.QuoteStatus_QUOTE_STATUS_TIMEOUT, broker.StatusTimeout, false},
		{pb.QuoteStatus_QUOTE_STATUS_ERROR, broker.StatusError, false},
		{pb.QuoteStatus_QUOTE_STATUS_DISCONNECTED, broker.StatusDisconnected, false},
	}
	for _, c := range statuses {
		got, err := fromQuote(&pb.Quote{Status: c.in}, nil)
		if c.bad != (err != nil) || (!c.bad && got.Status != c.want) {
			t.Errorf("%v: got %v, %v", c.in, got.Status, err)
		}
	}

	t.Run("doubles", testDoubles)
	t.Run("times", testTimes)
}

func testDoubles(t *testing.T) {
	t.Parallel()
	unset, err := fromQuote(&pb.Quote{Status: pb.QuoteStatus_QUOTE_STATUS_OK}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, p := range doubles(unset) {
		if p != nil {
			t.Errorf("%s unset but %v", name, *p)
		}
	}
	zero := 0.0
	setQ := &pb.Quote{
		Status: pb.QuoteStatus_QUOTE_STATUS_OK,
		Bid:    ptr(1), Ask: ptr(2), Last: ptr(3), Close: ptr(4), Volume: ptr(5), OpenInterest: ptr(6),
		BidSize: ptr(7), AskSize: ptr(8), LastSize: &zero, ImpliedVol: ptr(9), Delta: ptr(10),
		Gamma: ptr(11), Vega: ptr(12), Theta: ptr(13), UndPrice: ptr(14),
	}
	set, err := fromQuote(setQ, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{
		"bid": 1, "ask": 2, "last": 3, "close": 4, "volume": 5, "open_interest": 6, "bid_size": 7,
		"ask_size": 8, "last_size": 0, "implied_vol": 9, "delta": 10, "gamma": 11, "vega": 12,
		"theta": 13, "und_price": 14,
	}
	for name, p := range doubles(set) {
		if p == nil || *p != want[name] {
			t.Errorf("%s: got %v, want %v", name, p, want[name])
		}
	}

}

func testTimes(t *testing.T) {
	t.Parallel()
	recv := time.Date(2026, 1, 5, 21, 0, 0, 0, time.FixedZone("x", 3600))
	ev := time.Date(2026, 1, 5, 20, 59, 0, 0, time.UTC)
	q, err := fromQuote(&pb.Quote{
		Status: pb.QuoteStatus_QUOTE_STATUS_PARTIAL, Generation: 42,
		MarketDataType: pb.MarketDataType_MARKET_DATA_TYPE_DELAYED,
		ReceivedAt:     timestamppb.New(recv), EventTs: timestamppb.New(ev),
		ErrorCode: 10090, ErrorText: "partial",
		Contract: &pb.ContractSpec{SecType: pb.SecType_SEC_TYPE_OPTION, Right: pb.Right_RIGHT_PUT, ConId: 9},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if q.Generation != 42 || q.MarketDataType != 3 || q.ErrorCode != 10090 || q.ErrorText != "partial" {
		t.Errorf("scalars: %+v", q)
	}
	if !q.ReceivedAt.Equal(recv) || q.ReceivedAt.Location() != time.UTC {
		t.Errorf("received_at %v", q.ReceivedAt)
	}
	if q.EventTs == nil || !q.EventTs.Equal(ev) {
		t.Errorf("event_ts %v", q.EventTs)
	}
	if q.Contract.SecType != "OPT" || q.Contract.Right != "P" || q.Contract.ConID != 9 {
		t.Errorf("contract %+v", q.Contract)
	}
	if none, _ := fromQuote(&pb.Quote{Status: pb.QuoteStatus_QUOTE_STATUS_OK}, nil); none.EventTs != nil || !none.ReceivedAt.IsZero() {
		t.Errorf("unset times: %+v", none)
	}
}

func doubles(q broker.Quote) map[string]*float64 {
	return map[string]*float64{
		"bid": q.Bid, "ask": q.Ask, "last": q.Last, "close": q.Close, "volume": q.Volume,
		"open_interest": q.OpenInterest, "bid_size": q.BidSize, "ask_size": q.AskSize,
		"last_size": q.LastSize, "implied_vol": q.ImpliedVol, "delta": q.Delta, "gamma": q.Gamma,
		"vega": q.Vega, "theta": q.Theta, "und_price": q.UndPrice,
	}
}

func TestFromExecutionsTimeUTC(t *testing.T) {
	t.Parallel()
	got, err := fromExecutions(&pb.ExecutionsResponse{
		Account: "a", Collected: true,
		Executions: []*pb.Execution{{ExecId: "1", Time: "2026-11-09T09:00:00-05:00"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC); !got.Executions[0].Time.Equal(want) || got.Executions[0].Time.Location() != time.UTC {
		t.Fatalf("got %v", got.Executions[0].Time)
	}
}

func TestQuoteKindMapsAllFourKinds(t *testing.T) {
	t.Parallel()
	for k, want := range map[broker.Kind]pb.QuoteKind{
		broker.KindOptionEod:    pb.QuoteKind_QUOTE_KIND_OPTION_EOD,
		broker.KindUnderlyingIV: pb.QuoteKind_QUOTE_KIND_UNDERLYING_IV,
		broker.KindSnapshot:     pb.QuoteKind_QUOTE_KIND_SNAPSHOT,
		broker.KindOptionOpen:   pb.QuoteKind_QUOTE_KIND_OPTION_OPEN,
	} {
		got, err := quoteKind(k)
		if err != nil || got != want {
			t.Fatalf("kind %d: got %v, err %v, want %v", k, got, err, want)
		}
	}
	if _, err := quoteKind(broker.Kind(99)); err == nil || kindErr(t, err) != contracts.Permanent {
		t.Fatalf("unknown kind: %v", err)
	}
}

func TestCashSecTypeRoundTrip(t *testing.T) {
	t.Parallel()
	c := broker.Contract{SecType: "CASH", Symbol: "USD", Exchange: "IDEALPRO", Currency: "JPY"}
	spec, err := toSpec(c)
	if err != nil {
		t.Fatalf("toSpec CASH: %v", err)
	}
	if spec.GetSecType() != pb.SecType_SEC_TYPE_CASH {
		t.Fatalf("sec type = %v, want SEC_TYPE_CASH", spec.GetSecType())
	}
	if got := fromSpec(spec); got != c {
		t.Fatalf("round trip = %+v, want %+v", got, c)
	}
	if got := fromSpec(&pb.ContractSpec{SecType: pb.SecType_SEC_TYPE_CASH}); got.SecType != "CASH" {
		t.Fatalf("fromSpec CASH = %q", got.SecType)
	}
}

func TestUnmappedSecTypeRefused(t *testing.T) {
	t.Parallel()
	_, err := toSpec(broker.Contract{SecType: "FUT"})
	if err == nil || !strings.Contains(err.Error(), "unknown sec type") {
		t.Fatalf("err = %v, want unknown sec type", err)
	}
}

func okQuote(missing ...*pb.MissingField) *pb.Quote {
	return &pb.Quote{Status: pb.QuoteStatus_QUOTE_STATUS_OK, Missing: missing}
}

func TestFromQuoteMissing(t *testing.T) {
	t.Parallel()
	got, err := fromQuote(okQuote(
		&pb.MissingField{Field: "open_interest", Reason: pb.MissingReason_MISSING_REASON_NOT_RECEIVED_GRACE},
		&pb.MissingField{Field: "bid", Reason: pb.MissingReason_MISSING_REASON_ERROR, ErrorCode: 200},
	), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []broker.MissingField{
		{Field: "open_interest", Reason: broker.MissingNotReceivedGrace},
		{Field: "bid", Reason: broker.MissingError, ErrorCode: 200},
	}
	if len(got.Missing) != 2 || got.Missing[0] != want[0] || got.Missing[1] != want[1] {
		t.Fatalf("got %+v", got.Missing)
	}
}

func TestFromQuoteMissingMapsEveryReason(t *testing.T) {
	t.Parallel()
	cases := map[pb.MissingReason]broker.MissingReason{
		pb.MissingReason_MISSING_REASON_NOT_RECEIVED_RELEASE: broker.MissingNotReceivedRelease,
		pb.MissingReason_MISSING_REASON_NOT_RECEIVED_GRACE:   broker.MissingNotReceivedGrace,
		pb.MissingReason_MISSING_REASON_INVALID:              broker.MissingInvalid,
		pb.MissingReason_MISSING_REASON_ERROR:                broker.MissingError,
		pb.MissingReason_MISSING_REASON_RECEIVED_ZERO:        broker.MissingReceivedZero,
		pb.MissingReason_MISSING_REASON_MARKET_DATA_TYPE:     broker.MissingMarketDataType,
	}
	for in, want := range cases {
		got, err := fromQuote(okQuote(&pb.MissingField{Field: "f", Reason: in}), nil)
		if err != nil || len(got.Missing) != 1 || got.Missing[0].Reason != want {
			t.Fatalf("%s: got %+v, %v", in, got.Missing, err)
		}
	}
}

func TestFromQuoteMissingUnspecified(t *testing.T) {
	t.Parallel()
	_, err := fromQuote(okQuote(&pb.MissingField{Field: "bid"}), nil)
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
		t.Fatalf("got %v", err)
	}
}

func TestFromQuoteNoMissingIsNil(t *testing.T) {
	t.Parallel()
	for _, q := range []*pb.Quote{okQuote(), {Status: pb.QuoteStatus_QUOTE_STATUS_OK, Missing: []*pb.MissingField{}}} {
		got, err := fromQuote(q, nil)
		if err != nil || got.Missing != nil {
			t.Fatalf("got %#v, %v", got.Missing, err)
		}
	}
}

func TestFromAccountAndExecutionsNoAccountText(t *testing.T) {
	t.Parallel()
	_, err := fromAccount(&pb.AccountResponse{Generation: 1})
	if err == nil || !strings.Contains(err.Error(), "account: answer has no account") {
		t.Fatalf("account: got %v", err)
	}
	_, err = fromExecutions(&pb.ExecutionsResponse{Generation: 1, Collected: true})
	if err == nil || !strings.Contains(err.Error(), "executions: answer has no account") {
		t.Fatalf("executions: got %v", err)
	}
}
