package positionmarks

import (
	"context"
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestContractOfOption(t *testing.T) {
	t.Parallel()
	p := store.Position{
		Symbol: "SPY", SecType: "option", Currency: "USD", Expiry: "2026-11-20", Strike: "450", Multiplier: "100",
		Right: "put", ConID: 7,
	}
	c, ok := contractOf(p)
	want := broker.Contract{
		SecType: "OPT", Symbol: "SPY", Exchange: "SMART", Currency: "USD", Expiry: "20261120", Strike: "450",
		Right: "P", Multiplier: "100", ConID: 7,
	}
	if !ok || c != want {
		t.Fatalf("contract = %+v, %v; want %+v", c, ok, want)
	}
}

func TestContractOfSecTypesAndRights(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ sec, right, wantSec, wantRight string }{
		{"stock", "", "STK", ""}, {"index", "", "IND", ""}, {"option", "call", "OPT", "C"},
	} {
		c, ok := contractOf(store.Position{SecType: tc.sec, Right: tc.right, Currency: "USD"})
		if !ok || c.SecType != tc.wantSec || c.Right != tc.wantRight {
			t.Errorf("%s/%s -> %+v %v", tc.sec, tc.right, c, ok)
		}
	}
	if _, ok := contractOf(store.Position{SecType: "future"}); ok {
		t.Error("future must have no contract")
	}
}

type resolver struct {
	broker.Broker
	res broker.Resolved
	err error
	got []broker.Contract
}

func (r *resolver) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	r.got = append(r.got, c)
	return r.res, r.err
}

func TestPlanTargetJPYUsesResolvedExchange(t *testing.T) {
	t.Parallel()
	r := &resolver{res: broker.Resolved{Listed: true, Contract: broker.Contract{Exchange: "TSEJ"}}}
	tg := planTarget(context.Background(), r, store.Position{SecType: "stock", Currency: "JPY", ConID: 9})
	if tg.status != "" || tg.contract.Exchange != "TSEJ" || tg.contract.ConID != 9 || tg.contract.SecType != "STK" {
		t.Fatalf("target = %+v", tg)
	}
	if len(r.got) != 1 || r.got[0].ConID != 9 {
		t.Fatalf("resolve calls = %+v", r.got)
	}
}

func TestPlanTargetUSDDoesNotResolve(t *testing.T) {
	t.Parallel()
	r := &resolver{err: errors.New("must not be called")}
	tg := planTarget(context.Background(), r, store.Position{SecType: "stock", Currency: "USD", ConID: 1})
	if tg.status != "" || tg.contract.Exchange != "SMART" || len(r.got) != 0 {
		t.Fatalf("target = %+v, calls %d", tg, len(r.got))
	}
}

func TestPlanTargetNotListedOrFailedIsNotDropped(t *testing.T) {
	t.Parallel()
	p := store.Position{SecType: "stock", Currency: "JPY", ConID: 9}
	un := planTarget(context.Background(), &resolver{res: broker.Resolved{Listed: false}}, p)
	if un.status != "no_data" {
		t.Errorf("unlisted status = %q", un.status)
	}
}

func TestPlanTargetJPYStockFailedLookupUsesTSEJ(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ secType, want string }{{"stock", "STK"}, {"index", "IND"}} {
		p := store.Position{SecType: tc.secType, Currency: "JPY", ConID: 9, Symbol: "4063"}
		tg := planTarget(context.Background(), &resolver{err: errors.New("ambiguous")}, p)
		if tg.status != "" || tg.contract.Exchange != "TSEJ" || tg.contract.ConID != 9 || tg.contract.SecType != tc.want {
			t.Errorf("%s target = %+v", tc.secType, tg)
		}
	}
}

func TestPlanTargetJPYOptionFailedLookupIsError(t *testing.T) {
	t.Parallel()
	p := store.Position{SecType: "option", Currency: "JPY", ConID: 9}
	tg := planTarget(context.Background(), &resolver{err: errors.New("x")}, p)
	if tg.status != "error" {
		t.Errorf("failed status = %q", tg.status)
	}
}
