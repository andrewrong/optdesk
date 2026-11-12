package grpcbroker

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

// listSvc is a ContractsService whose ListContracts and ResolveContract run the functions of the test.
type listSvc struct {
	pb.UnimplementedContractsServiceServer
	lists   func(*pb.ListContractsRequest) (*pb.ListContractsResponse, error)
	resolve func(context.Context, *pb.ResolveContractRequest) (*pb.ResolveContractResponse, error)
}

func (f *listSvc) ListContracts(_ context.Context, r *pb.ListContractsRequest) (*pb.ListContractsResponse, error) {
	return f.lists(r)
}

func (f *listSvc) ResolveContract(ctx context.Context, r *pb.ResolveContractRequest) (*pb.ResolveContractResponse, error) {
	return f.resolve(ctx, r)
}

func dialSvc(t *testing.T, f *listSvc) broker.Broker {
	t.Helper()
	// A short directory: a Unix socket path is limited to about 100 bytes.
	dir, err := os.MkdirTemp("", "l")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "s")
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pb.RegisterContractsServiceServer(srv, f)
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(srv.Stop)
	b, err := Dial(context.Background(), path, broker.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func serveLister(t *testing.T, f *listSvc) broker.ContractLister {
	t.Helper()
	b := dialSvc(t, f)
	lister, ok := b.(broker.ContractLister)
	if !ok {
		t.Fatal("the client of Dial does not implement ContractLister")
	}
	return lister
}

func listFake(answer func(*pb.ListContractsRequest) (*pb.ListContractsResponse, error)) *listSvc {
	return &listSvc{lists: answer}
}

func ko() broker.Contract { return broker.Contract{SecType: "STK", Symbol: "KO", ConID: 8894} }

func spec(id int64, class, strike string) *pb.ContractSpec {
	return &pb.ContractSpec{
		SecType: pb.SecType_SEC_TYPE_OPTION, Symbol: "KO", Expiry: "20261120", Right: pb.Right_RIGHT_PUT,
		Strike: strike, Multiplier: "100", TradingClass: class, ConId: id,
	}
}

func TestListContractsDropsOtherClasses(t *testing.T) {
	t.Parallel()
	var got *pb.ListContractsRequest
	c := serveLister(t, listFake(func(r *pb.ListContractsRequest) (*pb.ListContractsResponse, error) {
		got = r
		return &pb.ListContractsResponse{Contracts: []*pb.ContractSpec{spec(1, "KO", "60"), spec(2, "KO1", "61"), spec(3, "KO", "62.5")}}, nil
	}))
	out, err := c.ListContracts(context.Background(), ko(), "20261120", "", "KO")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].ConID != 1 || out[1].ConID != 3 {
		t.Fatalf("got %+v", out)
	}
	if got.GetExpiry() != "20261120" || got.GetTradingClass() != "KO" || got.GetUnderlying().GetConId() != 8894 || got.GetRight() != pb.Right_RIGHT_UNSPECIFIED {
		t.Fatalf("request %v", got)
	}
}

func TestListContractsKeepsMultiplier(t *testing.T) {
	t.Parallel()
	s := spec(1, "KO", "60")
	s.Multiplier = "10"
	c := serveLister(t, listFake(func(*pb.ListContractsRequest) (*pb.ListContractsResponse, error) {
		return &pb.ListContractsResponse{Contracts: []*pb.ContractSpec{s, spec(2, "KO", "61")}}, nil
	}))
	out, err := c.ListContracts(context.Background(), ko(), "20261120", "P", "KO")
	if err != nil || len(out) != 2 || out[0].Multiplier != "10" || out[1].Multiplier != "100" {
		t.Fatalf("got %+v, %v", out, err)
	}
}

func TestListContractsGRPCErrorIsReturned(t *testing.T) {
	t.Parallel()
	c := serveLister(t, listFake(func(*pb.ListContractsRequest) (*pb.ListContractsResponse, error) {
		return nil, status.Error(codes.Unavailable, "gateway down")
	}))
	out, err := c.ListContracts(context.Background(), ko(), "20261120", "", "KO")
	if err == nil || out != nil || errors.Is(err, broker.ErrNotServed) {
		t.Fatalf("got %v, %v", out, err)
	}
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Retryable {
		t.Fatalf("want Retryable, got %v", err)
	}
}

func TestListContractsUnimplementedIsErrNotServed(t *testing.T) {
	t.Parallel()
	c := serveLister(t, listFake(func(*pb.ListContractsRequest) (*pb.ListContractsResponse, error) {
		return nil, status.Error(codes.Unimplemented, "method ListContracts not implemented")
	}))
	_, err := c.ListContracts(context.Background(), ko(), "20261120", "", "KO")
	if !errors.Is(err, broker.ErrNotServed) {
		t.Fatalf("got %v", err)
	}
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
		t.Fatalf("want Permanent, got %v", err)
	}
}

func TestListContractsStrikeTextEqualsResolveContract(t *testing.T) {
	t.Parallel()
	s := spec(7, "KO", "12.5")
	f := &listSvc{
		lists: func(*pb.ListContractsRequest) (*pb.ListContractsResponse, error) {
			return &pb.ListContractsResponse{Contracts: []*pb.ContractSpec{s}}, nil
		},
		resolve: func(context.Context, *pb.ResolveContractRequest) (*pb.ResolveContractResponse, error) {
			return &pb.ResolveContractResponse{Status: pb.ResolveStatus_RESOLVE_STATUS_FOUND, Contract: s}, nil
		},
	}
	b := dialSvc(t, f)
	listed, err := b.(broker.ContractLister).ListContracts(context.Background(), ko(), "20261120", "", "KO")
	if err != nil || len(listed) != 1 {
		t.Fatalf("got %v, %v", listed, err)
	}
	res, err := b.ResolveContract(context.Background(), listed[0])
	if err != nil || !res.Listed {
		t.Fatalf("got %v, %v", res, err)
	}
	if listed[0].Strike != "12.5" || listed[0].Strike != res.Contract.Strike {
		t.Fatalf("strike %q vs %q", listed[0].Strike, res.Contract.Strike)
	}
}

func TestListContractsSendsExpiryAsYYYYMMDD(t *testing.T) {
	t.Parallel()
	for in, want := range map[contracts.Day]string{"2026-12-11": "20261211", "20261211": "20261211"} {
		var got *pb.ListContractsRequest
		c := serveLister(t, listFake(func(r *pb.ListContractsRequest) (*pb.ListContractsResponse, error) {
			got = r
			return &pb.ListContractsResponse{}, nil
		}))
		if _, err := c.ListContracts(context.Background(), ko(), in, "", "KO"); err != nil {
			t.Fatal(err)
		}
		if got.GetExpiry() != want {
			t.Fatalf("day %q sent as %q, want %q", in, got.GetExpiry(), want)
		}
	}
}

func TestListContractsBadDayIsPermanentWithoutRPC(t *testing.T) {
	t.Parallel()
	for _, in := range []contracts.Day{"2026-13-40", "", "2026-1-1", "20261340", "x"} {
		calls := 0
		c := serveLister(t, listFake(func(*pb.ListContractsRequest) (*pb.ListContractsResponse, error) {
			calls++
			return &pb.ListContractsResponse{}, nil
		}))
		out, err := c.ListContracts(context.Background(), ko(), in, "", "KO")
		var te contracts.TypedError
		if err == nil || out != nil || !errors.As(err, &te) || te.Kind() != contracts.Permanent || calls != 0 {
			t.Fatalf("day %q: out=%v err=%v calls=%d", in, out, err, calls)
		}
	}
}
