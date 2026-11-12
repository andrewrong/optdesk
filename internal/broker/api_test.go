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
type fakeBroker struct{ down, refuseUnknown bool }

var errUnknownKind = errors.New("unknown kind")

func (f fakeBroker) ListChains(context.Context, broker.Contract) ([]broker.Chain, error) {
	return nil, nil
}

func (f fakeBroker) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	return broker.Resolved{Listed: true, Contract: c}, nil
}

func (f fakeBroker) Snapshot(_ context.Context, kind broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	if f.down {
		return nil, typedErr{contracts.Retryable}
	}
	if f.refuseUnknown && (kind < broker.KindOptionEod || kind > broker.KindOptionOpen) {
		return nil, errUnknownKind
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

func TestKindOptionOpenIsDistinctAndUnknownKindIsRefused(t *testing.T) {
	t.Parallel()
	kinds := []broker.Kind{broker.KindOptionEod, broker.KindUnderlyingIV, broker.KindSnapshot, broker.KindOptionOpen}
	seen := map[broker.Kind]bool{}
	for _, k := range kinds {
		if seen[k] {
			t.Fatalf("kind %d appears twice", k)
		}
		seen[k] = true
	}
	if broker.KindOptionEod != 0 || broker.KindUnderlyingIV != 1 || broker.KindSnapshot != 2 {
		t.Fatal("the values of the three old kinds changed")
	}
	if seen[broker.Kind(99)] {
		t.Fatal("Kind(99) must not be a kind")
	}
	if _, err := broker.Broker(fakeBroker{refuseUnknown: true}).Snapshot(context.Background(), broker.Kind(99), nil); !errors.Is(err, errUnknownKind) {
		t.Fatalf("unknown kind: %v", err)
	}
	if _, err := broker.Broker(fakeBroker{refuseUnknown: true}).Snapshot(context.Background(), broker.KindOptionOpen, nil); err != nil {
		t.Fatalf("KindOptionOpen refused: %v", err)
	}
}

func TestFakeWithoutRefuseUnknownAcceptsAnyKind(t *testing.T) {
	t.Parallel()
	qs, err := broker.Broker(fakeBroker{}).Snapshot(context.Background(), broker.Kind(99), []broker.Contract{{Symbol: "A"}})
	if err != nil || len(qs) != 1 {
		t.Fatalf("refuseUnknown is false, want one quote, got %v, %v", qs, err)
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

type fakeReference struct{}

func (fakeReference) Dividends(_ context.Context, cs []broker.Contract) ([]broker.DividendProjection, error) {
	out := make([]broker.DividendProjection, len(cs))
	for i, c := range cs {
		out[i] = broker.DividendProjection{Contract: c, Status: broker.DividendNoValue}
	}
	return out, nil
}

func (fakeReference) Close() error { return nil }

func TestReferenceClientBuiltByNewReferenceFunc(t *testing.T) {
	t.Parallel()
	var f broker.NewReferenceFunc = func(context.Context, string, broker.Options) (broker.ReferenceClient, error) {
		return fakeReference{}, nil
	}
	c, err := f(context.Background(), "x", broker.Options{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Dividends(context.Background(), []broker.Contract{{Symbol: "KO"}})
	if err != nil || len(got) != 1 || got[0].Contract.Symbol != "KO" || got[0].Past12m != nil {
		t.Fatalf("got %+v, %v", got, err)
	}
}

// fakeAccount answers one account with an alias and the flags of what its gateway collects.
type fakeAccount struct{ down bool }

func (f fakeAccount) Account(context.Context) (broker.Account, error) {
	if f.down {
		return broker.Account{}, typedErr{contracts.Retryable}
	}
	return broker.Account{
		Alias:              "main",
		Generation:         3,
		Values:             []broker.AccountValue{{Tag: "NetLiquidation", Value: "100000.50", Currency: "USD"}},
		ValuesCollected:    true,
		PositionsCollected: false,
	}, nil
}

func (f fakeAccount) Executions(context.Context) (broker.Executions, error) {
	return broker.Executions{
		Alias:      "main",
		Generation: 3,
		Collected:  true,
		Pending:    1,
		Executions: []broker.Execution{{
			ExecID: "e1", Contract: broker.Contract{Symbol: "KO"}, Side: "BOT", Quantity: "1", Price: "60.1",
			Commission: "0.35", CommissionCurrency: "USD",
		}},
	}, nil
}

func (fakeAccount) Close() error { return nil }

var _ broker.AccountReader = fakeAccount{}

func TestAccountReaderContract(t *testing.T) {
	t.Parallel()
	var r broker.AccountReader = fakeAccount{}
	a, err := r.Account(context.Background())
	if err != nil || a.Alias != "main" || !a.ValuesCollected || a.PositionsCollected || a.Values[0].Value != "100000.50" || len(a.Positions) != 0 {
		t.Fatalf("got %+v, %v", a, err)
	}
	e, err := r.Executions(context.Background())
	if err != nil || !e.Collected || e.Pending != 1 || len(e.Executions) != 1 || e.Executions[0].Commission != "0.35" {
		t.Fatalf("got %+v, %v", e, err)
	}
	if _, err := broker.AccountReader(fakeAccount{down: true}).Account(context.Background()); !retryable(err) {
		t.Fatalf("want a Retryable TypedError, got %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
}

// fakeHistory answers the bars of the query, or a Retryable error when down is set.
type fakeHistory struct{ down bool }

func (f fakeHistory) Bars(_ context.Context, q broker.BarQuery) ([]broker.Bar, error) {
	if f.down {
		return nil, typedErr{contracts.Retryable}
	}
	return []broker.Bar{{Day: q.End, Open: "1", High: "2", Low: "0.5", Close: "1.5"}}, nil
}

func (fakeHistory) Close() error { return nil }

var _ broker.HistoryClient = fakeHistory{}

func TestHistoryClientFake(t *testing.T) {
	t.Parallel()
	var h broker.HistoryClient = fakeHistory{}
	bars, err := h.Bars(context.Background(), broker.BarQuery{Kind: broker.BarTrades, DurationDays: 5, End: "2026-11-06"})
	if err != nil || len(bars) != 1 || bars[0].Day != "2026-11-06" || bars[0].Close != "1.5" || bars[0].Volume != nil {
		t.Fatalf("got %+v, %v", bars, err)
	}
	if _, err := broker.HistoryClient(fakeHistory{down: true}).Bars(context.Background(), broker.BarQuery{}); !retryable(err) {
		t.Fatalf("want a Retryable TypedError, got %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
}

// ivHistory answers a bar whose Close is the IV text for the IV kind, and no bar for any other kind.
type ivHistory struct{}

func (ivHistory) Bars(_ context.Context, q broker.BarQuery) ([]broker.Bar, error) {
	if q.Kind != broker.BarOptionImpliedVol {
		return nil, nil
	}
	return []broker.Bar{{Day: q.End, Close: "0.2813"}}, nil
}

func (ivHistory) Close() error { return nil }

func TestBarOptionImpliedVolFake(t *testing.T) {
	t.Parallel()
	if broker.BarTrades != 1 || broker.BarAdjustedLast != 2 || broker.BarMidpoint != 3 || broker.BarOptionImpliedVol != 4 {
		t.Fatalf("kinds are %d %d %d %d, want 1 2 3 4", broker.BarTrades, broker.BarAdjustedLast, broker.BarMidpoint, broker.BarOptionImpliedVol)
	}
	var h broker.HistoryClient = ivHistory{}
	bars, err := h.Bars(context.Background(), broker.BarQuery{Kind: broker.BarOptionImpliedVol, DurationDays: 5, End: "2026-11-06"})
	if err != nil || len(bars) != 1 || bars[0].Close != "0.2813" || bars[0].Volume != nil {
		t.Fatalf("got %+v, %v", bars, err)
	}
	if bars, _ := h.Bars(context.Background(), broker.BarQuery{Kind: broker.BarMidpoint}); len(bars) != 0 {
		t.Fatalf("midpoint got %+v", bars)
	}
}

// permanentWrap is a Permanent error that wraps another, as the client of grpcbroker returns it.
type permanentWrap struct{ err error }

func (e permanentWrap) Error() string             { return "permanent: " + e.err.Error() }
func (e permanentWrap) Kind() contracts.ErrorKind { return contracts.Permanent }
func (e permanentWrap) Unwrap() error             { return e.err }

// fakeLister answers the contracts it holds, or a Retryable error when down is set, or ErrNotServed.
type fakeLister struct {
	down, old bool
	held      []broker.Contract
}

func (f fakeLister) ListContracts(_ context.Context, _ broker.Contract, _ contracts.Day, _, class string) ([]broker.Contract, error) {
	if f.down {
		return nil, typedErr{contracts.Retryable}
	}
	if f.old {
		return nil, permanentWrap{broker.ErrNotServed}
	}
	var out []broker.Contract
	for _, c := range f.held {
		if class == "" || c.TradingClass == class {
			out = append(out, c)
		}
	}
	return out, nil
}

var _ broker.ContractLister = fakeLister{}

func TestContractListerFromTheCaller(t *testing.T) {
	t.Parallel()
	und := broker.Contract{SecType: "STK", Symbol: "KO", ConID: 8894}
	held := []broker.Contract{{ConID: 1, TradingClass: "KO"}, {ConID: 2, TradingClass: "KO1"}}
	got, err := broker.ContractLister(fakeLister{held: held}).ListContracts(context.Background(), und, "20261120", "", "KO")
	if err != nil || len(got) != 1 || got[0].ConID != 1 {
		t.Fatalf("got %v, %v", got, err)
	}
	_, err = broker.ContractLister(fakeLister{down: true}).ListContracts(context.Background(), und, "20261120", "", "KO")
	if !retryable(err) {
		t.Fatalf("a down lister must be Retryable, got %v", err)
	}
	_, err = broker.ContractLister(fakeLister{old: true}).ListContracts(context.Background(), und, "20261120", "", "KO")
	if !errors.Is(err, broker.ErrNotServed) || retryable(err) {
		t.Fatalf("an old service must match ErrNotServed and not be Retryable, got %v", err)
	}
}
