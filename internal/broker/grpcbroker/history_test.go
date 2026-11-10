package grpcbroker

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"reflect"
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

type fakeBars struct {
	pb.UnimplementedMarketDataServiceServer
	answer func(*pb.BarsRequest) (*pb.BarsResponse, error)
	calls  atomic.Int64
}

func (f *fakeBars) Bars(_ context.Context, r *pb.BarsRequest) (*pb.BarsResponse, error) {
	f.calls.Add(1)
	return f.answer(r)
}

func serveBars(t *testing.T, f *fakeBars) (broker.HistoryClient, *fakeBars) {
	t.Helper()
	dir, err := os.MkdirTemp("", "h")
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
	pb.RegisterMarketDataServiceServer(srv, f)
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(srv.Stop)
	c, err := DialHistory(context.Background(), path, broker.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, f
}

func okBars(bars ...*pb.Bar) *fakeBars {
	return &fakeBars{answer: func(*pb.BarsRequest) (*pb.BarsResponse, error) { return &pb.BarsResponse{Bars: bars}, nil }}
}

func query() broker.BarQuery {
	return broker.BarQuery{
		Contract: broker.Contract{SecType: "STK", Symbol: "KO", ConID: 8894}, Kind: broker.BarTrades, DurationDays: 5,
	}
}

func TestBarsContract(t *testing.T) {
	t.Parallel()
	vol := "1200"
	var got *pb.BarsRequest
	f := &fakeBars{answer: func(r *pb.BarsRequest) (*pb.BarsResponse, error) {
		got = r
		return &pb.BarsResponse{Bars: []*pb.Bar{
			{Day: "2026-11-05", Open: "60.1", High: "61", Low: "59.9", Close: "60.5", Volume: &vol},
			{Day: "2026-11-06", Open: "60.5", High: "61.2", Low: "60", Close: "61"},
		}}, nil
	}}
	c, _ := serveBars(t, f)
	q := query()
	q.Kind, q.End = broker.BarAdjustedLast, "2026-11-06"
	bars, err := c.Bars(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	want := []broker.Bar{
		{Day: "2026-11-05", Open: "60.1", High: "61", Low: "59.9", Close: "60.5", Volume: &vol},
		{Day: "2026-11-06", Open: "60.5", High: "61.2", Low: "60", Close: "61"},
	}
	if !reflect.DeepEqual(bars, want) {
		t.Fatalf("got %+v", bars)
	}
	if got.GetContract().GetConId() != 8894 || got.GetKind() != pb.BarKind_BAR_KIND_ADJUSTED_LAST ||
		got.GetDurationDays() != 5 || got.GetEndDay() != "2026-11-06" {
		t.Fatalf("request %v", got)
	}
}

func TestBarsBadDayIsError(t *testing.T) {
	t.Parallel()
	c, _ := serveBars(t, okBars(&pb.Bar{Day: "20261105", Open: "1", High: "1", Low: "1", Close: "1"}))
	_, err := c.Bars(context.Background(), query())
	if err == nil || kindErr(t, err) != contracts.Permanent {
		t.Fatalf("want a Permanent error, got %v", err)
	}
}

func TestBarsBadPriceIsError(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"", "abc", "1e5", "NaN", "1.2.3"} {
		c, _ := serveBars(t, okBars(&pb.Bar{Day: "2026-11-05", Open: "1", High: "1", Low: "1", Close: p}))
		_, err := c.Bars(context.Background(), query())
		if err == nil || kindErr(t, err) != contracts.Permanent {
			t.Fatalf("close %q: want a Permanent error, got %v", p, err)
		}
	}
	bad := "x"
	c, _ := serveBars(t, okBars(&pb.Bar{Day: "2026-11-05", Open: "1", High: "1", Low: "1", Close: "1", Volume: &bad}))
	if _, err := c.Bars(context.Background(), query()); err == nil {
		t.Fatal("a bad volume must be an error")
	}
}

func TestBarsPacedIsRetryable(t *testing.T) {
	t.Parallel()
	c, _ := serveBars(t, &fakeBars{answer: func(*pb.BarsRequest) (*pb.BarsResponse, error) {
		return nil, status.Error(codes.ResourceExhausted, "paced")
	}})
	_, err := c.Bars(context.Background(), query())
	if err == nil || kindErr(t, err) != contracts.Retryable {
		t.Fatalf("want a Retryable error, got %v", err)
	}
}

func TestBarsUnsetKindIsError(t *testing.T) {
	t.Parallel()
	c, f := serveBars(t, okBars())
	q := query()
	q.Kind = 0
	_, err := c.Bars(context.Background(), q)
	if err == nil || kindErr(t, err) != contracts.Permanent {
		t.Fatalf("want a Permanent error, got %v", err)
	}
	if f.calls.Load() != 0 {
		t.Fatal("an unset kind must fail before any call")
	}
}

func TestBarsBadEndDayIsError(t *testing.T) {
	t.Parallel()
	c, f := serveBars(t, okBars())
	for _, e := range []string{"20261106", "2026-13-01", "tomorrow"} {
		q := query()
		q.End = e
		_, err := c.Bars(context.Background(), q)
		if err == nil || kindErr(t, err) != contracts.Permanent {
			t.Fatalf("end %q: want a Permanent error, got %v", e, err)
		}
	}
	if f.calls.Load() != 0 {
		t.Fatal("a bad end day must fail before any call")
	}
}

func TestHistoryTimeoutDefault(t *testing.T) {
	t.Parallel()
	if got := withDefaults(broker.Options{}).HistoryTimeout; got != 330*time.Second {
		t.Fatalf("default %v", got)
	}
	if got := withDefaults(broker.Options{HistoryTimeout: time.Second}).HistoryTimeout; got != time.Second {
		t.Fatalf("set %v", got)
	}
}

func TestBarsOptionImpliedVolKind(t *testing.T) {
	t.Parallel()
	var got *pb.BarsRequest
	f := &fakeBars{answer: func(r *pb.BarsRequest) (*pb.BarsResponse, error) {
		got = r
		return &pb.BarsResponse{Bars: []*pb.Bar{
			{Day: "2026-11-05", Open: "0.28", High: "0.29", Low: "0.27", Close: "0.2813"},
		}}, nil
	}}
	c, _ := serveBars(t, f)
	q := query()
	q.Kind = broker.BarOptionImpliedVol
	bars, err := c.Bars(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if got.GetKind() != pb.BarKind_BAR_KIND_OPTION_IMPLIED_VOL {
		t.Fatalf("request kind %v", got.GetKind())
	}
	if len(bars) != 1 || bars[0].Close != "0.2813" {
		t.Fatalf("got %+v", bars)
	}
}
