package grpcbroker

import (
	"context"
	"errors"
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

// fake is a service on a Unix socket; each rpc runs the function the test set.
type fake struct {
	pb.UnimplementedMarketDataServiceServer
	pb.UnimplementedContractsServiceServer
	snapshot func(*pb.SnapshotRequest, grpc.ServerStreamingServer[pb.SnapshotResponse]) error
	resolve  func(context.Context, *pb.ResolveContractRequest) (*pb.ResolveContractResponse, error)
	chains   func(context.Context, *pb.ListChainsRequest) (*pb.ListChainsResponse, error)
	calls    atomic.Int64
}

func (f *fake) Snapshot(r *pb.SnapshotRequest, s grpc.ServerStreamingServer[pb.SnapshotResponse]) error {
	f.calls.Add(1)
	return f.snapshot(r, s)
}

func (f *fake) ResolveContract(ctx context.Context, r *pb.ResolveContractRequest) (*pb.ResolveContractResponse, error) {
	f.calls.Add(1)
	return f.resolve(ctx, r)
}

func (f *fake) ListChains(ctx context.Context, r *pb.ListChainsRequest) (*pb.ListChainsResponse, error) {
	f.calls.Add(1)
	return f.chains(ctx, r)
}

// serve starts f on a socket in t.TempDir and dials it with o.
func serve(t *testing.T, f *fake, o broker.Options) broker.Broker {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s")
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pb.RegisterMarketDataServiceServer(srv, f)
	pb.RegisterContractsServiceServer(srv, f)
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(srv.Stop)
	b, err := Dial(context.Background(), path, o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func kindErr(t *testing.T, err error) contracts.ErrorKind {
	t.Helper()
	var te contracts.TypedError
	if !errors.As(err, &te) {
		t.Fatalf("not a TypedError: %v", err)
	}
	return te.Kind()
}

func opt(id int64) broker.Contract {
	return broker.Contract{SecType: "OPT", Symbol: "SPY", Right: "C", ConID: id}
}

func TestSnapshotOrder(t *testing.T) {
	t.Parallel()
	f := &fake{snapshot: func(r *pb.SnapshotRequest, s grpc.ServerStreamingServer[pb.SnapshotResponse]) error {
		cs := r.GetContracts()
		for i := len(cs) - 1; i >= 0; i-- {
			if cs[i].GetConId() == 3 {
				continue // no answer for contract 3
			}
			q := &pb.Quote{Contract: cs[i], Status: pb.QuoteStatus_QUOTE_STATUS_OK, Generation: cs[i].GetConId()}
			if err := s.Send(&pb.SnapshotResponse{Quote: q}); err != nil {
				return err
			}
		}
		return nil
	}}
	b := serve(t, f, broker.Options{})
	qs, err := b.Snapshot(context.Background(), broker.KindSnapshot, []broker.Contract{opt(1), opt(2), opt(3), opt(4)})
	if err != nil || len(qs) != 4 {
		t.Fatalf("got %v, %v", qs, err)
	}
	for i, want := range []int64{1, 2, 3, 4} {
		if qs[i].Contract.ConID != want {
			t.Fatalf("slot %d holds contract %d, want %d", i, qs[i].Contract.ConID, want)
		}
	}
	if qs[0].Status != broker.StatusOK || qs[0].Generation != 1 || qs[2].Status != broker.StatusDisconnected {
		t.Fatalf("statuses: %+v", qs)
	}
}

func TestKinds(t *testing.T) {
	t.Parallel()
	for k, want := range map[broker.Kind]pb.QuoteKind{
		broker.KindOptionEod:    pb.QuoteKind_QUOTE_KIND_OPTION_EOD,
		broker.KindUnderlyingIV: pb.QuoteKind_QUOTE_KIND_UNDERLYING_IV,
		broker.KindSnapshot:     pb.QuoteKind_QUOTE_KIND_SNAPSHOT,
	} {
		var got pb.QuoteKind
		f := &fake{snapshot: func(r *pb.SnapshotRequest, _ grpc.ServerStreamingServer[pb.SnapshotResponse]) error {
			got = r.GetKind()
			return nil
		}}
		b := serve(t, f, broker.Options{})
		if _, err := b.Snapshot(context.Background(), k, []broker.Contract{opt(1)}); err != nil || got != want {
			t.Fatalf("kind %d: sent %v, err %v, want %v", k, got, err, want)
		}
	}
	f := &fake{}
	b := serve(t, f, broker.Options{})
	_, err := b.Snapshot(context.Background(), broker.Kind(99), []broker.Contract{opt(1)})
	if err == nil || kindErr(t, err) != contracts.Permanent || f.calls.Load() != 0 {
		t.Fatalf("unknown kind: %v, calls %d", err, f.calls.Load())
	}
}

func TestResolveNotListed(t *testing.T) {
	t.Parallel()
	status1 := pb.ResolveStatus_RESOLVE_STATUS_NOT_LISTED
	var rpcErr error
	f := &fake{resolve: func(context.Context, *pb.ResolveContractRequest) (*pb.ResolveContractResponse, error) {
		return &pb.ResolveContractResponse{Status: status1}, rpcErr
	}}
	b := serve(t, f, broker.Options{})
	r, err := b.ResolveContract(context.Background(), opt(0))
	if err != nil || r.Listed {
		t.Fatalf("not listed: %+v, %v", r, err)
	}
	rpcErr = status.Error(codes.FailedPrecondition, "several results")
	_, err = b.ResolveContract(context.Background(), opt(0))
	if err == nil || kindErr(t, err) != contracts.Permanent {
		t.Fatalf("several results: %v", err)
	}
	rpcErr, status1 = nil, pb.ResolveStatus_RESOLVE_STATUS_FOUND
	f.resolve = func(context.Context, *pb.ResolveContractRequest) (*pb.ResolveContractResponse, error) {
		return &pb.ResolveContractResponse{Status: status1, Contract: &pb.ContractSpec{Symbol: "SPY", ConId: 7}}, nil
	}
	r, err = b.ResolveContract(context.Background(), opt(0))
	if err != nil || !r.Listed || r.Contract.ConID != 7 {
		t.Fatalf("found: %+v, %v", r, err)
	}
}

func TestListChains(t *testing.T) {
	t.Parallel()
	f := &fake{chains: func(context.Context, *pb.ListChainsRequest) (*pb.ListChainsResponse, error) {
		return &pb.ListChainsResponse{Chains: []*pb.Chain{
			{Exchange: "SMART", TradingClass: "SPY", Multiplier: "100", Expirations: []string{"20260116", "20260123"}, Strikes: []string{"500", "500.5", "10"}},
		}}, nil
	}}
	b := serve(t, f, broker.Options{})
	cs, err := b.ListChains(context.Background(), broker.Contract{SecType: "STK", Symbol: "SPY"})
	if err != nil || len(cs) != 1 {
		t.Fatalf("got %v, %v", cs, err)
	}
	c := cs[0]
	if c.Exchange != "SMART" || c.Multiplier != "100" || strings.Join(c.Strikes, ",") != "500,500.5,10" || len(c.Expirations) != 2 {
		t.Fatalf("chain: %+v", c)
	}
}

func TestErrorKinds(t *testing.T) {
	t.Parallel()
	for code := codes.Canceled; code <= codes.Unauthenticated; code++ {
		want := contracts.Permanent
		if code == codes.Unavailable || code == codes.DeadlineExceeded {
			want = contracts.Retryable
		}
		err := wrapErr(context.Background(), "x", status.Error(code, "m"))
		if kindErr(t, err) != want {
			t.Fatalf("code %v gave %v, want %v", code, kindErr(t, err), want)
		}
	}
}

func TestTimeouts(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	block := func(ctx context.Context) { <-ctx.Done() }
	f := &fake{
		snapshot: func(_ *pb.SnapshotRequest, s grpc.ServerStreamingServer[pb.SnapshotResponse]) error {
			block(s.Context())
			close(done)
			return s.Context().Err()
		},
		resolve: func(ctx context.Context, _ *pb.ResolveContractRequest) (*pb.ResolveContractResponse, error) {
			block(ctx)
			return nil, ctx.Err()
		},
		chains: func(ctx context.Context, _ *pb.ListChainsRequest) (*pb.ListChainsResponse, error) {
			block(ctx)
			return nil, ctx.Err()
		},
	}
	b := serve(t, f, broker.Options{CallTimeout: time.Nanosecond, SnapshotTimeout: time.Nanosecond})
	ctx := context.Background()
	_, e1 := b.ListChains(ctx, opt(1))
	_, e2 := b.ResolveContract(ctx, opt(1))
	_, e3 := b.Snapshot(ctx, broker.KindSnapshot, []broker.Contract{opt(1)})
	for i, e := range []error{e1, e2, e3} {
		if e == nil || kindErr(t, e) != contracts.Retryable {
			t.Fatalf("call %d: %v", i, e)
		}
	}
}

func TestStreamCancel(t *testing.T) {
	t.Parallel()
	started, done := make(chan struct{}), make(chan struct{})
	f := &fake{snapshot: func(_ *pb.SnapshotRequest, s grpc.ServerStreamingServer[pb.SnapshotResponse]) error {
		close(started)
		<-s.Context().Done()
		close(done)
		return nil
	}}
	b := serve(t, f, broker.Options{})
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		_, err := b.Snapshot(ctx, broker.KindSnapshot, []broker.Contract{opt(1)})
		errc <- err
	}()
	<-started
	cancel()
	if err := <-errc; err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	<-done
}

func TestDialRefuses(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "missing.sock")
	_, err := Dial(context.Background(), path, broker.Options{})
	if err == nil || kindErr(t, err) != contracts.Retryable {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), path) || strings.Contains(err.Error(), "missing.sock") {
		t.Fatalf("text holds the path: %q", err.Error())
	}
}

func TestResolveCarriesErrorCodeAndText(t *testing.T) {
	t.Parallel()
	f := &fake{resolve: func(context.Context, *pb.ResolveContractRequest) (*pb.ResolveContractResponse, error) {
		return &pb.ResolveContractResponse{
			Status:    pb.ResolveStatus_RESOLVE_STATUS_NOT_LISTED,
			ErrorCode: 200, ErrorText: "no security definition",
		}, nil
	}}
	b := serve(t, f, broker.Options{})
	r, err := b.ResolveContract(context.Background(), opt(0))
	if err != nil || r.Listed || r.ErrorCode != 200 || r.ErrorText != "no security definition" {
		t.Fatalf("not listed with error: %+v, %v", r, err)
	}
	f.resolve = func(context.Context, *pb.ResolveContractRequest) (*pb.ResolveContractResponse, error) {
		return &pb.ResolveContractResponse{
			Status:   pb.ResolveStatus_RESOLVE_STATUS_FOUND,
			Contract: &pb.ContractSpec{Symbol: "SPY", ConId: 7}, ErrorCode: 2104, ErrorText: "farm ok",
		}, nil
	}
	r, err = b.ResolveContract(context.Background(), opt(0))
	if err != nil || !r.Listed || r.ErrorCode != 2104 || r.ErrorText != "farm ok" {
		t.Fatalf("found with error: %+v, %v", r, err)
	}
}
