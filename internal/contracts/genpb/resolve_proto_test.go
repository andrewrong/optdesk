package genpb

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type fakeContracts struct {
	UnimplementedContractsServiceServer
}

func (fakeContracts) ListContracts(_ context.Context, r *ListContractsRequest) (*ListContractsResponse, error) {
	return &ListContractsResponse{Contracts: []*ContractSpec{
		{Symbol: r.GetUnderlying().GetSymbol(), Expiry: r.GetExpiry(), Right: r.GetRight(), Strike: "12.5", ConId: 9},
	}}, nil
}

func TestListContractsRoundTrip(t *testing.T) {
	t.Parallel()
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	RegisterContractsServiceServer(srv, fakeContracts{})
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(l.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := NewContractsServiceClient(conn)
	resp, err := client.ListContracts(context.Background(), &ListContractsRequest{
		Underlying:   &ContractSpec{Symbol: "SPY"},
		Expiry:       "20260116",
		Right:        Right_RIGHT_PUT,
		TradingClass: "SPY",
	})
	if err != nil {
		t.Fatal(err)
	}
	c := resp.GetContracts()[0]
	if c.GetSymbol() != "SPY" || c.GetRight() != Right_RIGHT_PUT || c.GetStrike() != "12.5" || c.GetConId() != 9 {
		t.Fatalf("unexpected contract %v", c)
	}
	if _, err := client.ListChains(context.Background(), &ListChainsRequest{}); err == nil {
		t.Fatal("ListChains of the embedded Unimplemented server should fail")
	}
}

func TestResolveContractResponseHoursFields(t *testing.T) {
	t.Parallel()
	set := &ResolveContractResponse{
		Status:       ResolveStatus_RESOLVE_STATUS_FOUND,
		TradingHours: "20261116:0930-20261116:1600",
		LiquidHours:  "20261116:0930-20261116:1600",
		TimeZoneId:   "US/Eastern",
	}
	if set.GetTradingHours() == "" || set.GetLiquidHours() == "" || set.GetTimeZoneId() != "US/Eastern" {
		t.Fatalf("hours fields not read back: %v", set)
	}
	if set.ProtoReflect().Descriptor().Fields().ByName("time_zone_id").Number() != 7 {
		t.Fatal("time_zone_id is not field 7")
	}
	unset := &ResolveContractResponse{Status: ResolveStatus_RESOLVE_STATUS_NOT_LISTED}
	if unset.GetTradingHours() != "" || unset.GetLiquidHours() != "" || unset.GetTimeZoneId() != "" {
		t.Fatalf("unset hours fields not empty: %v", unset)
	}
}
