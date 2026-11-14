package main

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
)

// shortPos is a short put position of one contract.
func shortPos(id, status, opened, expiry, closed, strike string) compute.LedgerPosition {
	return compute.LedgerPosition{
		PositionID: id, Alias: "main", Underlying: "XYZ", Contract: id, Status: status, Right: "P",
		Expiry: expiry, Strike: strike, OpenedOn: contracts.Day(opened), ClosedOn: contracts.Day(closed),
		OpenPrice: "2.00", Quantity: "-2", Multiplier: "100", Currency: "USD",
		Fills: []compute.LedgerFill{{Day: contracts.Day(opened), Action: "open", Quantity: "-2", Price: "2.00"}},
	}
}

func fateOf(lots []replayapi.Lot, contract string) replayapi.Lot {
	for _, l := range lots {
		if l.ID[11:11+len(contract)] == contract {
			return l
		}
	}
	return replayapi.Lot{}
}

func TestLotsFateHeldAssignedClosedRolled(t *testing.T) {
	t.Parallel()
	held := shortPos("held", "expired", "2026-10-01", "2026-10-16", "2026-10-16", "100")
	held.RealizedPnL = "400"
	assigned := shortPos("asg", "assigned", "2026-10-01", "2026-10-16", "2026-10-16", "100")
	early := shortPos("early", "assigned", "2026-10-01", "2026-10-30", "2026-10-12", "100")
	early.Early = true
	closed := shortPos("closed", "closed", "2026-10-01", "2026-10-30", "2026-10-06", "100")
	closed.Fills = append(closed.Fills, compute.LedgerFill{Day: "2026-10-06", Action: "close", Quantity: "2", Price: "0.50"})
	closed.RealizedPnL = "299"
	// rolled: bought back on 10-14 and a later-expiry put opens on the next session day, 10-15.
	rolled := shortPos("rolled", "closed", "2026-10-01", "2026-10-16", "2026-10-14", "100")
	rolled.Fills = append(rolled.Fills, compute.LedgerFill{Day: "2026-10-14", Action: "close", Quantity: "2", Price: "0.30"})
	newer := shortPos("newer", "open", "2026-10-15", "2026-11-20", "", "100")
	open := shortPos("open", "open", "2026-10-02", "2026-12-18", "", "90")
	sessions := []contracts.Day{"2026-10-13", "2026-10-14", "2026-10-15", "2026-10-16"}

	lots := buildLots([]compute.LedgerPosition{held, assigned, early, closed, rolled, newer, open}, sessions)
	want := map[string]struct{ fate, settle, gross string }{
		"held":   {"held", "2026-10-16", "2"},
		"asg":    {"assigned", "2026-10-16", ""},
		"early":  {"assigned_early", "2026-10-12", ""},
		"closed": {"closed_early", "2026-10-30", "1.5"},
		"rolled": {"rolled", "2026-10-16", "1.7"},
		"newer":  {"open", "2026-11-20", ""},
		"open":   {"open", "2026-12-18", ""},
	}
	for c, w := range want {
		l := fateOf(lots, c)
		if l.Fate != w.fate || string(l.SettleDay) != w.settle || l.ActualGross != w.gross {
			t.Errorf("%s: fate=%q settle=%q gross=%q, want %+v", c, l.Fate, l.SettleDay, l.ActualGross, w)
		}
	}
	if l := fateOf(lots, "held"); l.ActualNet != "2" {
		t.Errorf("net per share %q, want 2 (400 over 2 x 100)", l.ActualNet)
	}
	if l := fateOf(lots, "open"); l.ActualNet != "" {
		t.Errorf("an open lot has net %q", l.ActualNet)
	}
	for i := 1; i < len(lots); i++ {
		if lots[i-1].EntryDay > lots[i].EntryDay {
			t.Errorf("lots are not sorted by entry day: %s before %s", lots[i-1].ID, lots[i].ID)
		}
	}
}

func TestLotsRolledNeedsLaterExpiryAndTheNextSessionDay(t *testing.T) {
	t.Parallel()
	closed := shortPos("closed", "closed", "2026-10-01", "2026-10-30", "2026-10-14", "100")
	closed.Fills = append(closed.Fills, compute.LedgerFill{Action: "close", Price: "0.50"})
	sameExpiry := shortPos("same", "open", "2026-10-14", "2026-10-30", "", "100")
	late := shortPos("late", "open", "2026-10-20", "2026-12-18", "", "100")
	lots := buildLots([]compute.LedgerPosition{closed, sameExpiry, late}, []contracts.Day{"2026-10-14", "2026-10-15"})
	if l := fateOf(lots, "closed"); l.Fate != "closed_early" {
		t.Fatalf("fate %q: a put with the same expiry or a later day is no roll", l.Fate)
	}
}

func TestLotsAssignedUsesIntrinsicNotRealized(t *testing.T) {
	t.Parallel()
	p := shortPos("asg", "assigned", "2026-10-01", "2026-10-16", "2026-10-16", "100")
	p.RealizedPnL = "-250" // the position's realized figure; it must not be the basis
	lots := buildLots([]compute.LedgerPosition{p}, nil)
	if lots[0].ActualGross != "" {
		t.Fatalf("actual %q before the close is known", lots[0].ActualGross)
	}
	mkt := replayapi.Market{Closes: map[string]map[contracts.Day]string{"XYZ": {"2026-10-16": "95"}}}
	priceAssigned(lots, mkt)
	if lots[0].ActualGross != "-3" {
		t.Errorf("gross %q, want credit 2 minus intrinsic 5 = -3", lots[0].ActualGross)
	}
	if lots[0].ActualNet != "-1.25" {
		t.Errorf("net %q, want -250 over 2 x 100", lots[0].ActualNet)
	}
	// A close above the strike leaves the put worthless: the actual is the credit.
	mkt.Closes["XYZ"]["2026-10-16"] = "104"
	priceAssigned(lots, mkt)
	if lots[0].ActualGross != "2" {
		t.Errorf("gross %q, want 2", lots[0].ActualGross)
	}
	// No close: the actual stays as it was, never a guess.
	lots[0].ActualGross = ""
	priceAssigned(lots, replayapi.Market{})
	if lots[0].ActualGross != "" {
		t.Errorf("gross %q without a close", lots[0].ActualGross)
	}
}

func TestShortLotsKeepsShortPutsByDefault(t *testing.T) {
	t.Parallel()
	put := shortPos("put", "open", "2026-10-01", "2026-12-18", "", "100")
	call := shortPos("call", "open", "2026-10-01", "2026-12-18", "", "100")
	call.Right = "C"
	long := shortPos("long", "open", "2026-10-01", "2026-12-18", "", "100")
	long.Quantity = "1"
	all := []compute.LedgerPosition{put, call, long}
	if got := shortLots(all, lotsShortPuts); len(got) != 1 || got[0].Contract != "put" {
		t.Errorf("short_puts kept %v", got)
	}
	got := shortLots(all, lotsShortLots)
	if len(got) != 2 {
		t.Fatalf("short_lots kept %d, want put and call", len(got))
	}
	// A call settles by its own right.
	lots := buildLots(got[1:], nil)
	if lots[0].Right != "C" {
		t.Errorf("right %q", lots[0].Right)
	}
}
