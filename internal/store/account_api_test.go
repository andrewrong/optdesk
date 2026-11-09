package store_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenAccountFunc = func(context.Context, string) (store.AccountStore, error) {
	return newMemAccount(), nil
}

// memAccount is a fake AccountStore with the insert-only rules of the contract.
type memAccount struct {
	captures   map[string]store.Capture
	executions map[string]store.Execution // key alias + "/" + exec id
	closed     bool
}

func newMemAccount() *memAccount {
	return &memAccount{captures: map[string]store.Capture{}, executions: map[string]store.Execution{}}
}

func (m *memAccount) WriteCapture(_ context.Context, c store.Capture) error {
	if _, ok := m.captures[c.ID]; ok {
		return nil
	}
	m.captures[c.ID] = c
	return nil
}

func (m *memAccount) WriteExecutions(_ context.Context, alias string, rows []store.Execution) error {
	for _, r := range rows {
		k := alias + "/" + r.ExecID
		if _, ok := m.executions[k]; ok {
			continue
		}
		r.Alias = alias
		m.executions[k] = r
	}
	return nil
}

func (m *memAccount) Positions(_ context.Context, alias, day string) ([]store.Position, error) {
	out := []store.Position{}
	for _, c := range m.captures {
		if c.Alias == alias && c.SessionDay == day {
			out = append(out, c.Positions...)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out, nil
}

func (m *memAccount) Values(_ context.Context, alias, day string) ([]store.AccountValue, error) {
	out := []store.AccountValue{}
	for _, c := range m.captures {
		if c.Alias == alias && c.SessionDay == day {
			out = append(out, c.Values...)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tag < out[j].Tag })
	return out, nil
}

func (m *memAccount) Executions(_ context.Context, alias, from, to string) ([]store.Execution, error) {
	out := []store.Execution{}
	for _, e := range m.executions {
		if e.Alias == alias && e.SessionDay >= from && e.SessionDay <= to {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExecID < out[j].ExecID })
	return out, nil
}

func (m *memAccount) Close() error { m.closed = true; return nil }

func TestAccountStoreContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var s store.AccountStore = newMemAccount()
	at := time.Date(2026, 11, 9, 21, 0, 0, 0, time.UTC)
	c := store.Capture{
		ID: "a:2026-11-09:run1", Alias: "a", At: at, SessionDay: "2026-11-09", Status: "ok",
		Values:    []store.AccountValue{{Tag: "NetLiquidation", Value: "100000.50", Currency: "USD"}, {Tag: "Cushion", Value: "0.8", Currency: "USD"}},
		Positions: []store.Position{{Symbol: "SPY", SecType: "STK", Quantity: "10", AvgCost: "500.25"}, {Symbol: "AAPL", SecType: "STK", Quantity: "5", AvgCost: "200"}},
	}
	if err := s.WriteCapture(ctx, c); err != nil {
		t.Fatal(err)
	}
	// A retry of the same capture id writes nothing more.
	retry := c
	retry.Positions = []store.Position{{Symbol: "ZZZ"}}
	if err := s.WriteCapture(ctx, retry); err != nil {
		t.Fatal(err)
	}
	pos, err := s.Positions(ctx, "a", "2026-11-09")
	if err != nil || len(pos) != 2 || pos[0].Symbol != "AAPL" || pos[1].Symbol != "SPY" {
		t.Fatalf("positions = %+v, %v", pos, err)
	}
	vals, err := s.Values(ctx, "a", "2026-11-09")
	if err != nil || len(vals) != 2 || vals[0].Tag != "Cushion" || vals[1].Value != "100000.50" {
		t.Fatalf("values = %+v, %v", vals, err)
	}
	other, err := s.Positions(ctx, "b", "2026-11-09")
	if err != nil || other == nil || len(other) != 0 {
		t.Fatalf("alias b = %#v, %v", other, err)
	}
}

func TestAccountStoreContractExecutions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newMemAccount()
	var w store.AccountWriter = s
	var r store.AccountReader = s
	rows := []store.Execution{
		{ExecID: "e2", SessionDay: "2026-11-09", Price: "1.25", Commission: "0.65"},
		{ExecID: "e1", SessionDay: "2026-11-08", Price: "2.5", Commission: "0.65"},
	}
	if err := w.WriteExecutions(ctx, "a", rows); err != nil {
		t.Fatal(err)
	}
	// The key (alias, exec_id) exists: a second call changes nothing.
	if err := w.WriteExecutions(ctx, "a", []store.Execution{{ExecID: "e1", SessionDay: "2026-11-08", Price: "9"}}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Executions(ctx, "a", "2026-11-08", "2026-11-09")
	if err != nil || len(got) != 2 || got[0].ExecID != "e1" || got[0].Price != "2.5" || got[0].Alias != "a" {
		t.Fatalf("executions = %+v, %v", got, err)
	}
	if err := s.Close(); err != nil || !s.closed {
		t.Fatalf("close: %v", err)
	}
}
