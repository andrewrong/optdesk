package optiontrack

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func withEvents(p compute.LedgerPosition, evs ...trackapi.TrackEvent) trackapi.TrackedOption {
	in := input([]compute.LedgerPosition{p})
	in.Events = evs
	return New().Track(in, defParams).Open[0]
}

func TestEventsInLife(t *testing.T) {
	t.Parallel()
	ev := func(day contracts.Day, earn, ex contracts.Day) trackapi.TrackEvent {
		return trackapi.TrackEvent{Underlying: "AAPL", Kind: "csp", Day: day, NextEarnings: earn, NextExDate: ex, ExAmount: "0.25"}
	}
	// the call sees both dates; boundaries are inclusive
	r := withEvents(pos("a", call), ev("2026-11-09", "2026-11-09", "2026-12-18"))
	if !r.EarningsInLife || !r.ExInLife || r.EventStatus != "ok" || r.NextExDate != "2026-12-18" || r.ExAmount != "0.25" {
		t.Fatalf("%+v", r)
	}
	r = withEvents(pos("a", call), ev("2026-11-09", "2026-11-08", "2026-12-19"))
	if r.EarningsInLife || r.ExInLife || r.NextEarnings != "2026-11-08" {
		t.Fatalf("outside the life: %+v", r)
	}
	r = withEvents(pos("a", call), ev("2026-11-09", "", ""))
	if r.EarningsInLife || r.ExInLife || r.EventStatus != "ok" {
		t.Fatalf("no dates: %+v", r)
	}
	// the newest Day wins over an older row
	r = withEvents(pos("a", call), ev("2026-11-02", "2026-11-20", ""), ev("2026-11-09", "2026-12-01", ""))
	if r.NextEarnings != "2026-12-01" {
		t.Fatalf("newest row: %+v", r)
	}
	// rows of other names are not used
	other := ev("2026-11-09", "2026-12-01", "")
	other.Underlying = "MSFT"
	if r = withEvents(pos("a", call), other); r.EventStatus != "no_event_row" {
		t.Fatalf("%+v", r)
	}
	// a put shows no ex-date
	r = withEvents(pos("a", nil), ev("2026-11-09", "2026-12-01", "2026-11-20"))
	if r.NextExDate != "" || r.ExAmount != "" || r.ExInLife || !r.EarningsInLife {
		t.Fatalf("put: %+v", r)
	}
}

func TestNoEventRow(t *testing.T) {
	t.Parallel()
	r := withEvents(pos("a", nil))
	if r.EventStatus != "no_event_row" || r.NextEarnings != "" || r.EarningsInLife || r.ExInLife {
		t.Fatalf("%+v", r)
	}
}

func guard(status, reason string) trackapi.TrackEvent {
	return trackapi.TrackEvent{
		Underlying: "AAPL", Expiry: "2026-12-18", Strike: "100.0", Right: "C", Kind: "held_call_guard",
		Day: "2026-11-09", GuardStatus: status, GuardReason: reason,
	}
}

func TestCallGuardFromRow(t *testing.T) {
	t.Parallel()
	r := withEvents(pos("a", call), guard("block", "early_assignment_risk"))
	if r.Guard != "block" || r.GuardReason != "early_assignment_risk" || r.GuardBasis != "bid" {
		t.Fatalf("%+v", r)
	}
	// the newest guard row of the terms wins
	old := guard("ok", "")
	old.Day = "2026-11-02"
	r = withEvents(pos("a", call), old, guard("unknown", "stale_dividend"))
	if r.Guard != "unknown" || r.GuardReason != "stale_dividend" {
		t.Fatalf("%+v", r)
	}
}

func TestCallWithoutGuardRowIsUnknown(t *testing.T) {
	t.Parallel()
	r := withEvents(pos("a", call))
	if r.Guard != "unknown" || r.GuardReason != "no_guard_row" || r.GuardBasis != "" {
		t.Fatalf("%+v", r)
	}
	// rows of another strike, expiry or kind do not count
	wrongStrike := guard("ok", "")
	wrongStrike.Strike = "105"
	wrongExpiry := guard("ok", "")
	wrongExpiry.Expiry = "2026-12-11"
	wrongKind := guard("ok", "")
	wrongKind.Kind = "csp"
	r = withEvents(pos("a", call), wrongStrike, wrongExpiry, wrongKind)
	if r.Guard != "unknown" || r.GuardReason != "no_guard_row" {
		t.Fatalf("%+v", r)
	}
}

func TestPutHasNoGuard(t *testing.T) {
	t.Parallel()
	g := guard("block", "early_assignment_risk")
	g.Right = "P"
	r := withEvents(pos("a", nil), g)
	if r.Guard != "n/a" || r.GuardReason != "" || r.GuardBasis != "" {
		t.Fatalf("%+v", r)
	}
}
