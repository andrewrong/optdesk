package grpcbroker

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	pb "github.com/andrewrong/optdesk/internal/contracts/genpb"
)

type fakeReference struct {
	pb.UnimplementedReferenceServiceServer
	answer func(*pb.ContractKey) *pb.DividendProjection
}

func (f *fakeReference) Dividends(s grpc.BidiStreamingServer[pb.ContractKey, pb.DividendProjection]) error {
	var keys []*pb.ContractKey
	for {
		k, err := s.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		keys = append(keys, k)
	}
	// Answer in reverse order: the client matches by key, not by position.
	for i := len(keys) - 1; i >= 0; i-- {
		if a := f.answer(keys[i]); a != nil {
			if err := s.Send(a); err != nil {
				return err
			}
		}
	}
	return nil
}

func serveReference(t *testing.T, answer func(*pb.ContractKey) *pb.DividendProjection) broker.ReferenceClient {
	t.Helper()
	// A short directory name: a Unix socket path is limited to about 100 bytes.
	dir, err := os.MkdirTemp("", "r")
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
	pb.RegisterReferenceServiceServer(srv, &fakeReference{answer: answer})
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(srv.Stop)
	c, err := DialReference(context.Background(), path, broker.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func f64(v float64) *float64 { return &v }

func strPtr(s string) *string { return &s }

func TestDividendsContract(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 9, 21, 0, 0, 0, time.UTC)
	c := serveReference(t, func(k *pb.ContractKey) *pb.DividendProjection {
		switch k.GetSymbol() {
		case "KO":
			return &pb.DividendProjection{
				Key: k, Status: pb.DividendStatus_DIVIDEND_STATUS_OK, ReceivedAt: timestamppb.New(at),
				Past_12M: strPtr("1.94"), Next_12M: strPtr("2.00"), NextExDate: strPtr("20261130"), NextAmount: strPtr("0.5"),
			}
		case "TSLA":
			return &pb.DividendProjection{Key: k, Status: pb.DividendStatus_DIVIDEND_STATUS_NO_VALUE}
		case "7203":
			return &pb.DividendProjection{Key: k, Status: pb.DividendStatus_DIVIDEND_STATUS_TIMEOUT}
		default:
			return &pb.DividendProjection{Key: k, Status: pb.DividendStatus_DIVIDEND_STATUS_ERROR, ErrorCode: 200}
		}
	})
	keys := []broker.Contract{
		{SecType: "STK", Symbol: "KO", Exchange: "SMART", Currency: "USD", ConID: 1},
		{SecType: "STK", Symbol: "TSLA", Exchange: "SMART", Currency: "USD", ConID: 2},
		{SecType: "STK", Symbol: "7203", Exchange: "TSEJ", Currency: "JPY", ConID: 3},
		{SecType: "STK", Symbol: "XXX", Exchange: "SMART", Currency: "USD"},
	}
	got, err := c.Dividends(context.Background(), keys)
	if err != nil || len(got) != 4 {
		t.Fatalf("got %+v, %v", got, err)
	}
	for i, k := range keys {
		if got[i].Contract != k {
			t.Fatalf("row %d is %+v, want the key %+v", i, got[i].Contract, k)
		}
	}
	want := broker.DividendProjection{
		Contract: keys[0], Status: broker.DividendOK, ReceivedAt: at,
		Past12m: f64(1.94), Next12m: f64(2), NextExDate: strPtr("20261130"), NextAmount: f64(0.5),
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Fatalf("ko: got %+v want %+v", got[0], want)
	}
	for i, st := range map[int]broker.DividendStatus{1: broker.DividendNoValue, 2: broker.DividendTimeout, 3: broker.DividendError} {
		g := got[i]
		if g.Status != st || g.Past12m != nil || g.Next12m != nil || g.NextAmount != nil || g.NextExDate != nil {
			t.Fatalf("row %d: %+v", i, g)
		}
	}
	if got[3].ErrorCode != 200 {
		t.Fatalf("error code: %+v", got[3])
	}
}

func TestDividendsUnsetStatusIsError(t *testing.T) {
	t.Parallel()
	c := serveReference(t, func(k *pb.ContractKey) *pb.DividendProjection {
		return &pb.DividendProjection{Key: k}
	})
	_, err := c.Dividends(context.Background(), []broker.Contract{{SecType: "STK", Symbol: "KO", ConID: 1}})
	if err == nil {
		t.Fatal("want an error for an unset status")
	}
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
		t.Fatalf("want Permanent, got %v", err)
	}
}
