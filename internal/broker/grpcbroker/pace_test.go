package grpcbroker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

func TestResolveInFlight(t *testing.T) {
	t.Parallel()
	var cur, peak atomic.Int64
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	f := &fake{resolve: func(ctx context.Context, _ *pb.ResolveContractRequest) (*pb.ResolveContractResponse, error) {
		n := cur.Add(1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		entered <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		cur.Add(-1)
		return &pb.ResolveContractResponse{Status: pb.ResolveStatus_RESOLVE_STATUS_NOT_LISTED}, nil
	}}
	b := serve(t, f, broker.Options{ResolveInFlight: 2})
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = b.ResolveContract(context.Background(), opt(1))
		}()
	}
	<-entered
	<-entered
	// A third call waits on the gate until its context ends.
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		_, err := b.ResolveContract(ctx, opt(1))
		errc <- err
	}()
	cancel()
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting call: %v", err)
	}
	close(release)
	wg.Wait()
	if p := peak.Load(); p != 2 {
		t.Fatalf("peak in flight %d, want 2", p)
	}
}
