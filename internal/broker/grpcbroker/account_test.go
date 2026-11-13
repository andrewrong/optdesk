package grpcbroker

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

type fakeAccount struct {
	pb.UnimplementedAccountServiceServer
	fn func(context.Context) (*pb.AccountResponse, error)
	ex func(context.Context) (*pb.ExecutionsResponse, error)
	// calls counts the Executions calls.
	calls atomic.Int32
}

func (f *fakeAccount) Executions(ctx context.Context, _ *pb.ExecutionsRequest) (*pb.ExecutionsResponse, error) {
	f.calls.Add(1)
	return f.ex(ctx)
}

func (f *fakeAccount) Account(ctx context.Context, _ *pb.AccountRequest) (*pb.AccountResponse, error) {
	return f.fn(ctx)
}

var _ broker.AccountReader = (*AccountClient)(nil)

func serveAccount(t *testing.T, fn func(context.Context) (*pb.AccountResponse, error), o broker.Options) (*AccountClient, string) {
	t.Helper()
	c, path, _ := serveBoth(t, fn, nil, o)
	return c, path
}

func serveBoth(t *testing.T, fn func(context.Context) (*pb.AccountResponse, error),
	ex func(context.Context) (*pb.ExecutionsResponse, error), o broker.Options) (*AccountClient, string, *fakeAccount) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s")
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	fk := &fakeAccount{fn: fn, ex: ex}
	pb.RegisterAccountServiceServer(srv, fk)
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(srv.Stop)
	c, err := DialAccount(context.Background(), path, o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, path, fk
}

func respOf(r *pb.AccountResponse) func(context.Context) (*pb.AccountResponse, error) {
	return func(context.Context) (*pb.AccountResponse, error) { return r, nil }
}

func TestAccountValues(t *testing.T) {
	t.Parallel()
	c, _ := serveAccount(t, respOf(&pb.AccountResponse{
		Account: "gw-b", Generation: 7,
		Values: []*pb.AccountValue{
			{Tag: "NetLiquidation", Value: "1000.50", Currency: "USD"},
			{Tag: "Cushion", Value: "0.9", Currency: ""},
			{Tag: "TotalCashValue", Value: "12", Currency: "EUR"},
		},
	}), broker.Options{})
	a, err := c.Account(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a.Alias != "gw-b" || a.Generation != 7 || len(a.Values) != 3 || len(a.Positions) != 0 {
		t.Fatalf("got %+v", a)
	}
	want := []broker.AccountValue{
		{Tag: "NetLiquidation", Value: "1000.50", Currency: "USD"},
		{Tag: "Cushion", Value: "0.9"},
		{Tag: "TotalCashValue", Value: "12", Currency: "EUR"},
	}
	for i, w := range want {
		if a.Values[i] != w {
			t.Fatalf("row %d: got %+v want %+v", i, a.Values[i], w)
		}
	}
}

func TestAccountPositions(t *testing.T) {
	t.Parallel()
	c, _ := serveAccount(t, respOf(&pb.AccountResponse{
		Account: "gw-b", Generation: 1,
		Positions: []*pb.Position{{
			Contract: &pb.ContractSpec{
				SecType: pb.SecType_SEC_TYPE_OPTION, Symbol: "SPY", Exchange: "SMART", Currency: "USD",
				Expiry: "20261218", Strike: "500", Right: pb.Right_RIGHT_PUT, Multiplier: "100",
				TradingClass: "SPY", ConId: 42,
			},
			Quantity: "-2", AvgCost: "123.4500",
		}},
	}), broker.Options{})
	a, err := c.Account(context.Background())
	if err != nil || len(a.Positions) != 1 {
		t.Fatalf("got %+v, %v", a, err)
	}
	want := broker.Position{
		Contract: broker.Contract{
			SecType: "OPT", Symbol: "SPY", Exchange: "SMART", Currency: "USD", Expiry: "20261218",
			Strike: "500", Right: "P", Multiplier: "100", TradingClass: "SPY", ConID: 42,
		},
		Quantity: "-2", AvgCost: "123.4500",
	}
	if a.Positions[0] != want {
		t.Fatalf("got %+v want %+v", a.Positions[0], want)
	}
}

func TestAccountErrorKinds(t *testing.T) {
	t.Parallel()
	for code, want := range map[codes.Code]contracts.ErrorKind{
		codes.Unavailable:        contracts.Retryable,
		codes.DeadlineExceeded:   contracts.Retryable,
		codes.FailedPrecondition: contracts.Permanent,
		codes.InvalidArgument:    contracts.Permanent,
	} {
		c, path := serveAccount(t, func(context.Context) (*pb.AccountResponse, error) {
			return nil, status.Error(code, "no")
		}, broker.Options{})
		_, err := c.Account(context.Background())
		if err == nil || kindErr(t, err) != want {
			t.Fatalf("%v: got %v", code, err)
		}
		if strings.Contains(err.Error(), path) {
			t.Fatalf("text holds the path: %q", err.Error())
		}
	}
}

func TestAccountCallTimeout(t *testing.T) {
	t.Parallel()
	c, _ := serveAccount(t, func(ctx context.Context) (*pb.AccountResponse, error) {
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	}, broker.Options{CallTimeout: 50 * time.Millisecond})
	_, err := c.Account(context.Background())
	if err == nil || kindErr(t, err) != contracts.Retryable || !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "DeadlineExceeded") {
		t.Fatalf("got %v", err)
	}
}

func TestDialAccountRefuses(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "missing.sock")
	_, err := DialAccount(context.Background(), path, broker.Options{})
	if err == nil || kindErr(t, err) != contracts.Retryable {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), "missing.sock") {
		t.Fatalf("text holds the path: %q", err.Error())
	}
}

func TestAccountEmptyAliasRefused(t *testing.T) {
	t.Parallel()
	for name, r := range map[string]*pb.AccountResponse{
		"no account":          {Generation: 1},
		"negative generation": {Account: "gw-b", Generation: -1},
	} {
		c, _ := serveAccount(t, respOf(r), broker.Options{})
		_, err := c.Account(context.Background())
		if err == nil || kindErr(t, err) != contracts.Permanent {
			t.Fatalf("%s: got %v", name, err)
		}
	}
}

func TestAccountCollectedFlags(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ v, p bool }{{true, false}, {false, true}, {true, true}, {false, false}} {
		c, _ := serveAccount(t, respOf(&pb.AccountResponse{
			Account: "gw-b", Generation: 1, ValuesCollected: tc.v, PositionsCollected: tc.p,
		}), broker.Options{})
		a, err := c.Account(context.Background())
		if err != nil || a.ValuesCollected != tc.v || a.PositionsCollected != tc.p {
			t.Fatalf("%+v: got %+v, %v", tc, a, err)
		}
	}
}

func TestExecutionsRows(t *testing.T) {
	t.Parallel()
	c, _, fk := serveBoth(t, nil, func(context.Context) (*pb.ExecutionsResponse, error) {
		return &pb.ExecutionsResponse{
			Account: "gw-b", Generation: 3, Collected: true, Pending: 2,
			Executions: []*pb.Execution{{
				ExecId: "0001.01", PermId: 99, Time: "2026-11-09T14:30:05+02:00",
				Contract: &pb.ContractSpec{SecType: pb.SecType_SEC_TYPE_STOCK, Symbol: "SPY", Currency: "USD"},
				Side:     "SLD", Quantity: "10.50", Price: "500.1000", Commission: "1.0", CommissionCurrency: "USD",
			}},
		}, nil
	}, broker.Options{})
	e, err := c.Executions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if e.Alias != "gw-b" || e.Generation != 3 || !e.Collected || e.Pending != 2 || len(e.Executions) != 1 {
		t.Fatalf("got %+v", e)
	}
	want := broker.Execution{
		ExecID: "0001.01", PermID: 99, Time: time.Date(2026, 11, 9, 12, 30, 5, 0, time.UTC),
		Contract: broker.Contract{SecType: "STK", Symbol: "SPY", Currency: "USD"},
		Side:     "SLD", Quantity: "10.50", Price: "500.1000", Commission: "1.0", CommissionCurrency: "USD",
	}
	got := e.Executions[0]
	if !got.Time.Equal(want.Time) || got.Time.Location() != time.UTC {
		t.Fatalf("time %v", got.Time)
	}
	got.Time = want.Time
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
	if fk.calls.Load() != 1 {
		t.Fatalf("calls %d", fk.calls.Load())
	}
}

func TestExecutionsNotCollected(t *testing.T) {
	t.Parallel()
	c, _, _ := serveBoth(t, nil, func(context.Context) (*pb.ExecutionsResponse, error) {
		return &pb.ExecutionsResponse{Account: "gw-b"}, nil
	}, broker.Options{})
	e, err := c.Executions(context.Background())
	if err != nil || e.Collected || len(e.Executions) != 0 {
		t.Fatalf("got %+v, %v", e, err)
	}
}

func TestExecutionsRefused(t *testing.T) {
	t.Parallel()
	for name, r := range map[string]*pb.ExecutionsResponse{
		"no account":          {Generation: 1, Collected: true},
		"negative generation": {Account: "gw-b", Generation: -1, Collected: true},
		"bad time":            {Account: "gw-b", Collected: true, Executions: []*pb.Execution{{ExecId: "1", Time: "yesterday"}}},
	} {
		c, _, fk := serveBoth(t, nil, func(context.Context) (*pb.ExecutionsResponse, error) { return r, nil }, broker.Options{})
		_, err := c.Executions(context.Background())
		if err == nil || kindErr(t, err) != contracts.Permanent {
			t.Fatalf("%s: got %v", name, err)
		}
		if fk.calls.Load() != 1 {
			t.Fatalf("%s: calls %d", name, fk.calls.Load())
		}
	}
}

func TestExecutionsErrorNoRetry(t *testing.T) {
	t.Parallel()
	c, _, fk := serveBoth(t, nil, func(context.Context) (*pb.ExecutionsResponse, error) {
		return nil, status.Error(codes.Unavailable, "no")
	}, broker.Options{})
	_, err := c.Executions(context.Background())
	if err == nil || kindErr(t, err) != contracts.Retryable {
		t.Fatalf("got %v", err)
	}
	if fk.calls.Load() != 1 {
		t.Fatalf("calls %d", fk.calls.Load())
	}
}
