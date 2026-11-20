package grpcbroker

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

type fakeSession struct {
	pb.UnimplementedSessionServiceServer
	fn func(context.Context) (*pb.PingResponse, error)
}

func (f *fakeSession) Ping(ctx context.Context, _ *pb.PingRequest) (*pb.PingResponse, error) {
	return f.fn(ctx)
}

func serveHealth(t *testing.T, fn func(context.Context) (*pb.PingResponse, error)) *HealthClient {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s")
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pb.RegisterSessionServiceServer(srv, &fakeSession{fn: fn})
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(srv.Stop)
	c, err := DialHealth(context.Background(), path, broker.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestHealthMapsPing(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		in    *pb.PingResponse
		state string
	}{
		"up":          {&pb.PingResponse{Connected: true, State: pb.SessionState_SESSION_STATE_UP}, "up"},
		"soft_lost":   {&pb.PingResponse{State: pb.SessionState_SESSION_STATE_SOFT_LOST}, "soft_lost"},
		"down":        {&pb.PingResponse{State: pb.SessionState_SESSION_STATE_DOWN}, "down"},
		"farm_broken": {&pb.PingResponse{Connected: true, State: pb.SessionState_SESSION_STATE_FARM_BROKEN}, "farm_broken"},
		"older":       {&pb.PingResponse{Connected: true}, "up"},
		"older_down":  {&pb.PingResponse{}, "down"},
	} {
		c := serveHealth(t, func(context.Context) (*pb.PingResponse, error) { return tc.in, nil })
		h, err := c.Health(context.Background())
		if err != nil || h.State != tc.state || len(h.Farms) != 0 {
			t.Fatalf("%s: got %+v, %v", name, h, err)
		}
	}
	c := serveHealth(t, func(context.Context) (*pb.PingResponse, error) {
		return &pb.PingResponse{State: pb.SessionState_SESSION_STATE_FARM_BROKEN, Farms: []*pb.FarmState{
			{Name: "ushmds", Up: false, SinceUnixMs: 1700000000123},
			{Name: "hkhmds", Up: true, SinceUnixMs: 5},
		}}, nil
	})
	h, err := c.Health(context.Background())
	if err != nil || len(h.Farms) != 2 {
		t.Fatalf("got %+v, %v", h, err)
	}
	want := broker.Farm{Name: "ushmds", Up: false, Since: time.UnixMilli(1700000000123).UTC()}
	if h.Farms[0] != want || h.Farms[1].Name != "hkhmds" || !h.Farms[1].Up || h.Farms[1].Since.UnixMilli() != 5 {
		t.Fatalf("got %+v", h.Farms)
	}
}

func TestHealthUnreachable(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{})
	c := serveHealth(t, func(ctx context.Context) (*pb.PingResponse, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-entered; cancel() }()
	_, err := c.Health(ctx)
	if err == nil || kindErr(t, err) != contracts.Retryable {
		t.Fatalf("got %v", err)
	}
}
