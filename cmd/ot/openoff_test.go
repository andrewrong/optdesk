package main

import (
	"context"
	"errors"
	"testing"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// posAccount answers the same positions for every alias and day.
type posAccount struct {
	store.AccountReader
	rows []store.Position
}

func (a posAccount) Positions(context.Context, string, string) ([]store.Position, error) {
	return a.rows, nil
}

func stockPos(sym string) store.Position {
	return store.Position{Symbol: sym, SecType: "stock", Currency: "USD", ConID: 1, Quantity: "10"}
}

func openJC(rows []store.Position, list underlying.Reader) *jobCtx {
	return &jobCtx{f: &jobFactory{accounts: posAccount{rows: rows}, list: list}}
}

func symbolsOf(ps []store.Position) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Symbol)
	}
	return out
}

func TestOpenStoredDropsOptionsOffHeldStock(t *testing.T) {
	t.Parallel()
	list := &fakeListReader{rows: []underlying.Row{
		{Key: "US:OFF", Symbol: "OFF", Market: "US", Status: underlying.StatusActive, Options: false},
		{Key: "US:ON", Symbol: "ON", Market: "US", Status: underlying.StatusActive, Options: true},
	}}
	opt := store.Position{Symbol: "OFF", SecType: "option", Currency: "USD", Quantity: "1"}
	jc := openJC([]store.Position{stockPos("OFF"), stockPos("ON"), opt}, list)
	got, err := openStored(jc, nil, nil).Accounts.Positions(context.Background(), "mine", "2026-11-24")
	if err != nil {
		t.Fatal(err)
	}
	if s := symbolsOf(got); len(s) != 1 || s[0] != "ON" {
		t.Fatalf("positions = %v, want only ON", s)
	}
}

func TestOpenStoredKeepsNameOffTheList(t *testing.T) {
	t.Parallel()
	list := &fakeListReader{rows: []underlying.Row{{Key: "US:ON", Symbol: "ON", Options: true}}}
	jc := openJC([]store.Position{stockPos("STRAY")}, list)
	got, err := openStored(jc, nil, nil).Accounts.Positions(context.Background(), "mine", "2026-11-24")
	if err != nil || len(got) != 1 || got[0].Symbol != "STRAY" {
		t.Fatalf("positions = %v, err = %v, want STRAY kept", got, err)
	}
}

func TestOpenStoredListReadError(t *testing.T) {
	t.Parallel()
	list := &fakeListReader{err: errors.New("down")}
	jc := openJC([]store.Position{stockPos("ON")}, list)
	got, err := openStored(jc, nil, nil).Accounts.Positions(context.Background(), "mine", "2026-11-24")
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Retryable || got != nil {
		t.Fatalf("positions = %v, err = %v, want a retryable error and no rows", got, err)
	}
}

func TestOpenEntriesDropOptionsOffHeldOption(t *testing.T) {
	t.Parallel()
	list := &fakeListReader{rows: []underlying.Row{
		{Key: "US:OFF", Symbol: "OFF", Market: "US", Status: underlying.StatusActive, Options: false},
	}}
	held := func(context.Context) (map[string]bool, error) { return map[string]bool{"option:US:OFF": true}, nil }
	f := jobFactory{list: list, held: held}
	entriesOf := func(f jobFactory) []jobs.WatchEntry {
		var got []jobs.WatchEntry
		build := func(es []jobs.WatchEntry) (contracts.Job, error) {
			got = es
			return &fakeInner{runs: new([]*fakeInner)}, nil
		}
		j, err := newListedJob(context.Background(), build, groupOptions, f.list, f.held, clockwork.NewFakeClock(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := j.Plan(context.Background(), "2026-11-24"); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := entriesOf(f.optionsOn()); len(got) != 0 {
		t.Fatalf("option_open entries = %v, want none", got)
	}
	if got := entriesOf(f); len(got) != 1 {
		t.Fatalf("option_eod entries = %v, want the held option name", got)
	}
}
