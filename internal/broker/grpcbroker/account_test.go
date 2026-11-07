package grpcbroker

import (
	"context"
	"net"
	"path/filepath"
	"strings"
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
}

func (f *fakeAccount) Account(ctx context.Context, _ *pb.AccountRequest) (*pb.AccountResponse, error) {
	return f.fn(ctx)
}

func serveAccount(t *testing.T, fn func(context.Context) (*pb.AccountResponse, error), o broker.Options) (*AccountClient, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s")
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pb.RegisterAccountServiceServer(srv, &fakeAccount{fn: fn})
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(srv.Stop)
	c, err := DialAccount(context.Background(), path, o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, path
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
	want := []AccountValue{{"NetLiquidation", "1000.50", "USD"}, {"Cushion", "0.9", ""}, {"TotalCashValue", "12", "EUR"}}
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
	want := Position{
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
		"no alias":            {Generation: 1},
		"negative generation": {Account: "gw-b", Generation: -1},
	} {
		c, _ := serveAccount(t, respOf(r), broker.Options{})
		_, err := c.Account(context.Background())
		if err == nil || kindErr(t, err) != contracts.Permanent {
			t.Fatalf("%s: got %v", name, err)
		}
	}
}
