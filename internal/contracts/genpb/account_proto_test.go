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
		Account:            "B",
		Generation:         3,
		ValuesCollected:    true,
		PositionsCollected: true,
		Values:             []*AccountValue{{Tag: "NetLiquidation", Value: "100.5", Currency: "USD"}},
		Positions: []*Position{
			{Contract: &ContractSpec{Symbol: "SPY"}, Quantity: "-2", AvgCost: "1.25"},
		},
	}, nil
}

func (fakeAccount) Executions(context.Context, *ExecutionsRequest) (*ExecutionsResponse, error) {
	return &ExecutionsResponse{
		Account:    "B",
		Generation: 3,
		Collected:  true,
		Pending:    1,
		Executions: []*Execution{{
			ExecId: "0001", PermId: 77, Time: "2026-11-09T15:30:00Z",
			Contract: &ContractSpec{Symbol: "SPY"}, Side: "BOT", Quantity: "2", Price: "1.25",
			Commission: "0", CommissionCurrency: "USD",
		}},
	}, nil
}

func TestAccountServiceHasTwoReadRpcs(t *testing.T) {
	t.Parallel()
	var names []string
	for _, m := range AccountService_ServiceDesc.Methods {
		names = append(names, m.MethodName)
	}
	if len(names) != 2 || names[0] != "Account" || names[1] != "Executions" {
		t.Fatalf("methods = %v, want Account and Executions", names)
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
		resp.GetPositions()[0].GetAvgCost() != "1.25" ||
		!resp.GetValuesCollected() || !resp.GetPositionsCollected() {
		t.Fatalf("unexpected response %v", resp)
	}
	ex, err := NewAccountServiceClient(conn).Executions(context.Background(), &ExecutionsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	e := ex.GetExecutions()[0]
	if ex.GetAccount() != "B" || !ex.GetCollected() || ex.GetPending() != 1 ||
		e.GetPermId() != 77 || e.GetTime() != "2026-11-09T15:30:00Z" || e.GetCommission() != "0" ||
		e.GetCommissionCurrency() != "USD" || e.GetContract().GetSymbol() != "SPY" {
		t.Fatalf("unexpected executions %v", ex)
	}
}
