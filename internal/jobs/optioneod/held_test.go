package optioneod

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

type fakeAccount struct {
	positions map[string][]store.Position // by alias + " " + day
}

func (f *fakeAccount) Positions(_ context.Context, alias, day string) ([]store.Position, error) {
	return f.positions[alias+" "+day], nil
}
func (f *fakeAccount) Values(context.Context, string, string) ([]store.AccountValue, error) {
	return nil, nil
}
func (f *fakeAccount) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return nil, nil
}

func shortCall(expiry, strike string) store.Position {
	return store.Position{Symbol: "TEST", SecType: "OPT", Right: "call", Expiry: expiry, Strike: strike, Quantity: "-1", Multiplier: "100"}
}

func heldBroker() *fakeBroker {
	b := newBroker()
	b.chains = []broker.Chain{{Exchange: "SMART", TradingClass: "TEST", Multiplier: "100",
		Expirations: []string{"20261116", "20261204"}, Strikes: []string{"95", "100", "105", "200"}}}
	return b
}

// heldContracts runs 2026-11-06 and returns the option contracts asked for.
func heldContracts(t *testing.T, acct store.AccountReader) []broker.Contract {
	t.Helper()
	b := heldBroker()
	clk := clockwork.NewFakeClockAt(etTime(t, "2026-11-06 17:00"))
	watch := []jobs.WatchEntry{watchEntry("US", "TEST", "STK", "100", 1000001)}
	j, err := NewWithHeld(b, &fakeResolver{}, &fakeWriter{}, watch, acct, clk)
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), testDay, []contracts.ExpectedKey{testKey})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	var out []broker.Contract
	for _, s := range b.snaps {
		if s.kind == broker.KindOptionEod {
			out = append(out, s.cs...)
		}
	}
	return out
}

func count(cs []broker.Contract, expiry, strike, right string) int {
	n := 0
	for _, c := range cs {
		if c.Expiry == expiry && c.Strike == strike && c.Right == right {
			n++
		}
	}
	return n
}

func TestHeldCallAddedAtTenDTE(t *testing.T) {
	t.Parallel()
	acct := &fakeAccount{positions: map[string][]store.Position{
		"A 2026-11-06": {shortCall("2026-11-16", "100"), {Symbol: "TEST", SecType: "OPT", Right: "call", Expiry: "2026-11-16", Strike: "105", Quantity: "1"}},
	}}
	got := heldContracts(t, acct)
	if n := count(got, "20261116", "100", "C"); n != 1 {
		t.Fatalf("held short call at 10 DTE asked %d times, want 1", n)
	}
	if n := count(got, "20261116", "105", "C"); n != 0 {
		t.Fatal("a long call must not be captured")
	}
}

func TestHeldCallDeduplicated(t *testing.T) {
	t.Parallel()
	acct := &fakeAccount{positions: map[string][]store.Position{
		"A 2026-11-06": {shortCall("2026-12-04", "100.0")},
		"B 2026-11-05": {shortCall("2026-12-04", "100")},
	}}
	got := heldContracts(t, acct)
	if n := count(got, "20261204", "100", "C"); n != 1 {
		t.Fatalf("contract in the window and held twice asked %d times, want 1", n)
	}
	if n := count(got, "20261204", "100.0", "C"); n != 0 {
		t.Fatal("the held strike text duplicated a selected contract")
	}
}

func TestNilAccountReaderUnchanged(t *testing.T) {
	t.Parallel()
	got := heldContracts(t, nil)
	if n := count(got, "20261116", "100", "C"); n != 0 {
		t.Fatal("no reader must not add a held call")
	}
	if n := count(got, "20261204", "100", "C"); n != 1 {
		t.Fatalf("window contract asked %d times, want 1", n)
	}
}

func etTime(t *testing.T, at string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	v, err := time.ParseInLocation("2006-01-02 15:04", at, loc)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func heldSymbolsRun(t *testing.T, acct store.AccountReader, tr symbol.Translator) []broker.Contract {
	t.Helper()
	b := heldBroker()
	clk := clockwork.NewFakeClockAt(etTime(t, "2026-11-06 17:00"))
	watch := []jobs.WatchEntry{watchEntry("US", "TEST", "STK", "100", 1000001)}
	var ctor NewOptionEodHeldSymbolsFunc = NewWithHeldSymbols
	j, err := ctor(b, &fakeResolver{}, &fakeWriter{}, watch, acct, clk, tr)
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), testDay, []contracts.ExpectedKey{testKey}); res.Err != nil {
		t.Fatal(res.Err)
	}
	var out []broker.Contract
	for _, s := range b.snaps {
		if s.kind == broker.KindOptionEod {
			out = append(out, s.cs...)
		}
	}
	return out
}

func TestHeldSymbolsTranslatorAndReader(t *testing.T) {
	t.Parallel()
	acct := &fakeAccount{positions: map[string][]store.Position{
		"A 2026-11-06": {shortCall("2026-11-16", "100")},
	}}
	tr := &askedTranslator{}
	got := heldSymbolsRun(t, acct, tr)
	tr.mu.Lock()
	asked := slices.Clone(tr.asked)
	tr.mu.Unlock()
	if !slices.Contains(asked, "TEST") {
		t.Fatalf("translator asked for %v, want TEST", asked)
	}
	if n := count(got, "20261116", "100", "C"); n != 1 {
		t.Fatalf("held call asked %d times, want 1", n)
	}
}

func TestHeldSymbolsNilTranslatorUnchanged(t *testing.T) {
	t.Parallel()
	acct := &fakeAccount{positions: map[string][]store.Position{
		"A 2026-11-06": {shortCall("2026-11-16", "100")},
	}}
	want := heldContracts(t, acct)
	got := heldSymbolsRun(t, acct, nil)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan with nil translator %v, want %v", got, want)
	}
	b := heldBroker()
	watch := []jobs.WatchEntry{watchEntry("US", "TEST", "STK", "100", 1000001)}
	clk := clockwork.NewFakeClockAt(etTime(t, "2026-11-06 17:00"))
	a, err := NewWithHeld(b, &fakeResolver{}, &fakeWriter{}, watch, acct, clk)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewWithHeldSymbols(b, &fakeResolver{}, &fakeWriter{}, watch, acct, clk, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.(*job).m, c.(*job).m) {
		t.Fatal("manifest differs between NewWithHeld and NewWithHeldSymbols(nil)")
	}
}
