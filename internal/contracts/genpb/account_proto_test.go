package genpb

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type fakeAccount struct {
	UnimplementedAccountServiceServer
}

func (fakeAccount) Account(context.Context, *AccountRequest) (*AccountResponse, error) {
	return &AccountResponse{
		Account:    "B",
		Generation: 3,
		Values:     []*AccountValue{{Tag: "NetLiquidation", Value: "100.5", Currency: "USD"}},
		Positions: []*Position{
			{Contract: &ContractSpec{Symbol: "SPY"}, Quantity: "-2", AvgCost: "1.25"},
		},
	}, nil
}

func TestAccountServiceHasOneReadRpc(t *testing.T) {
	t.Parallel()
	methods := AccountService_ServiceDesc.Methods
	if len(methods) != 1 || methods[0].MethodName != "Account" {
		t.Fatalf("methods = %v, want exactly Account", methods)
	}
	if len(AccountService_ServiceDesc.Streams) != 0 {
		t.Fatalf("streams = %d, want 0", len(AccountService_ServiceDesc.Streams))
	}
}

func TestAccountSock(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "s")
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	RegisterAccountServiceServer(srv, fakeAccount{})
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("unix://"+path, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	resp, err := NewAccountServiceClient(conn).Account(context.Background(), &AccountRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetAccount() != "B" || resp.GetGeneration() != 3 || len(resp.GetValues()) != 1 ||
		resp.GetPositions()[0].GetAvgCost() != "1.25" {
		t.Fatalf("unexpected response %v", resp)
	}
}
