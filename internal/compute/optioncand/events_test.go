package optioncand

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func withEarnings(e *compute.EarningsInfo) compute.CandidateInput {
	in := baseInput()
	in.Earnings = e
	return in
}

func TestEarningsListedInsideBlocks(t *testing.T) {
	t.Parallel()
	expiry := addDays(runDay, 30)
	for _, d := range []contracts.Day{runDay, addDays(runDay, 10), expiry} {
		c := only(t, eval(withEarnings(&compute.EarningsInfo{State: "listed", Next: d, RowDay: runDay})))
		wantGate(t, c, "events", compute.GateBlock, "earnings_before_expiry")
		if c.Status != "blocked" {
			t.Errorf("status %s", c.Status)
		}
	}
	// the owner's override replaces the vendor date
	c := only(t, eval(withEarnings(&compute.EarningsInfo{State: "listed", Next: "2027-02-01", Override: addDays(runDay, 5), RowDay: runDay})))
	wantGate(t, c, "events", compute.GateBlock, "earnings_before_expiry")
}

func TestEarningsAfterExpiryMarks(t *testing.T) {
	t.Parallel()
	c := only(t, eval(withEarnings(&compute.EarningsInfo{State: "listed", Next: addDays(runDay, 31), RowDay: runDay})))
	wantGate(t, c, "events", compute.GatePass, "")
	if len(c.Marks) != 1 || c.Marks[0] != "earnings_unconfirmed" {
		t.Errorf("marks %v", c.Marks)
	}
	// an owner date after the expiry is confirmed: no mark
	c = only(t, eval(withEarnings(&compute.EarningsInfo{State: "listed", Next: addDays(runDay, 31), Override: addDays(runDay, 40), RowDay: runDay})))
	wantGate(t, c, "events", compute.GatePass, "")
	if len(c.Marks) != 0 {
		t.Errorf("marks %v", c.Marks)
	}
}

func TestEarningsUnknownBlocks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		e      *compute.EarningsInfo
		reason string
	}{
		{"unsupported", &compute.EarningsInfo{State: "unsupported", RowDay: runDay}, "earnings_unsupported"},
		{"error", &compute.EarningsInfo{State: "error", RowDay: runDay}, "earnings_error"},
		{"no row", nil, "no_earnings"},
		{"stale row", &compute.EarningsInfo{State: "listed", Next: "2027-02-01", RowDay: "2026-11-04"}, "stale_earnings"},
		{"row before the session list", &compute.EarningsInfo{State: "listed", Next: "2027-02-01", RowDay: "2026-01-01"}, "stale_earnings"},
		{"absent, never reported", &compute.EarningsInfo{State: "absent", RowDay: runDay}, "no_earnings"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := only(t, eval(withEarnings(tc.e)))
			wantGate(t, c, "events", compute.GateUnknown, tc.reason)
			if c.Status != "unknown" || c.Rank != 0 {
				t.Errorf("status %s rank %d", c.Status, c.Rank)
			}
		})
	}
	// expiry beyond the 3-month horizon
	in := baseInput()
	in.Quotes[0].Expiry = addDays(runDay, 95)
	in.Earnings = &compute.EarningsInfo{State: "listed", Next: addDays(runDay, 200), RowDay: runDay}
	p := testParams()
	p.MaxDTE = 200
	wantGate(t, only(t, New().Evaluate(in, p)), "events", compute.GateUnknown, "beyond_horizon")
}

func TestEarningsAbsentAfterRecentReport(t *testing.T) {
	t.Parallel()
	absent := func(previous contracts.Day, expiryDays int) compute.Candidate {
		in := baseInput()
		in.Quotes[0].Expiry = addDays(runDay, expiryDays)
		in.Earnings = &compute.EarningsInfo{State: "absent", Previous: previous, RowDay: runDay}
		return only(t, eval(in))
	}
	r := addDays(runDay, -40)
	c := absent(r, 21) // the smallest DTE of the params; 21 + 40 = 61 is not below 60
	wantGate(t, c, "events", compute.GateUnknown, "no_earnings")

	p := testParams()
	p.MinDTE = 10
	in := baseInput()
	in.Quotes[0].Expiry = addDays(runDay, 15)
	in.Earnings = &compute.EarningsInfo{State: "absent", Previous: r, RowDay: runDay}
	c = only(t, New().Evaluate(in, p))
	wantGate(t, c, "events", compute.GatePass, "")
	if len(c.Marks) != 1 || c.Marks[0] != "earnings_beyond_horizon" {
		t.Errorf("marks %v", c.Marks)
	}
	in.Quotes[0].Expiry = addDays(runDay, 30)
	wantGate(t, only(t, New().Evaluate(in, p)), "events", compute.GateUnknown, "no_earnings")
	in.Quotes[0].Expiry = addDays(runDay, 15)
	in.Earnings.Previous = ""
	wantGate(t, only(t, New().Evaluate(in, p)), "events", compute.GateUnknown, "no_earnings")
	// a report 101 days back is outside the window
	in.Earnings.Previous = addDays(runDay, -101)
	wantGate(t, only(t, New().Evaluate(in, p)), "events", compute.GateUnknown, "no_earnings")
}

func TestEventAgeInSessions(t *testing.T) {
	t.Parallel()
	// the run day is a Monday: Friday 2026-11-06 is one session old, fresh
	fri := withEarnings(&compute.EarningsInfo{State: "listed", Next: "2027-02-01", RowDay: "2026-11-06"})
	wantGate(t, only(t, eval(fri)), "events", compute.GatePass, "")
	// a row of Wednesday 2026-11-04 is three sessions old on Monday: stale
	wed := withEarnings(&compute.EarningsInfo{State: "listed", Next: "2027-02-01", RowDay: "2026-11-04"})
	wantGate(t, only(t, eval(wed)), "events", compute.GateUnknown, "stale_earnings")
	// a Saturday row counts from Friday
	sat := withEarnings(&compute.EarningsInfo{State: "listed", Next: "2027-02-01", RowDay: "2026-11-07"})
	wantGate(t, only(t, eval(sat)), "events", compute.GatePass, "")
	// a session-day list with a holiday missing does not break the count: Thursday 11-05
	// is two sessions old when Friday 11-06 is a holiday
	hol := withEarnings(&compute.EarningsInfo{State: "listed", Next: "2027-02-01", RowDay: "2026-11-05"})
	var days []contracts.Day
	for _, d := range hol.SessionDays {
		if d != "2026-11-06" {
			days = append(days, d)
		}
	}
	hol.SessionDays = days
	wantGate(t, only(t, eval(hol)), "events", compute.GatePass, "")
}

func TestStaleChain(t *testing.T) {
	t.Parallel()
	in := baseInput()
	in.ChainDay = "2026-11-06" // Friday; the latest session on or before the Monday run is Monday
	in.Quotes[0].Expiry = addDays(in.ChainDay, 30)
	c := only(t, eval(in))
	wantGate(t, c, "events", compute.GateUnknown, "stale_chain")
	if c.Status != "unknown" {
		t.Errorf("status %s", c.Status)
	}
	// a Sunday run reads Friday's chain as fresh
	in.RunDay = "2026-11-08"
	if fresh := only(t, eval(in)); gateOf(t, fresh, "events").Reason == "stale_chain" {
		t.Errorf("friday chain on a sunday run is fresh")
	}
	in.SessionDays = nil
	wantGate(t, only(t, eval(in)), "events", compute.GateUnknown, "stale_chain")
}
