package optioncand

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	markUnconfirmed = "earnings_unconfirmed"
	markBeyond      = "earnings_beyond_horizon"
	horizonMonths   = 3
	recentReportMax = 100
)

func gate(name string, st compute.GateStatus, reason string) compute.GateResult {
	return compute.GateResult{Gate: name, Status: st, Reason: reason}
}

// chainFresh reports whether the chain day is the latest session day on or before the run day.
func chainFresh(in compute.CandidateInput) bool {
	pos := sessionPos(in.SessionDays, in.RunDay)
	return pos >= 0 && in.SessionDays[pos] == in.ChainDay
}

// eventsGate judges the earnings dates for a contract expiring on expiry; mark is the
// word to attach to the row, or empty.
func eventsGate(in compute.CandidateInput, p compute.CandidateParams, expiry contracts.Day) (compute.GateResult, string) {
	unknown := func(reason string) (compute.GateResult, string) {
		return gate("events", compute.GateUnknown, reason), ""
	}
	if !chainFresh(in) {
		return unknown("stale_chain")
	}
	e := in.Earnings
	if e == nil {
		return unknown("no_earnings")
	}
	age, ok := sessionAge(in.SessionDays, e.RowDay, in.RunDay)
	if !ok || age > p.MaxEventAgeSessions {
		return unknown("stale_earnings")
	}
	date, confirmed := e.Next, false
	switch {
	case e.Override != "":
		date, confirmed = e.Override, true
	case e.State == "unsupported":
		return unknown("earnings_unsupported")
	case e.State == "error":
		return unknown("earnings_error")
	case e.State == "absent":
		return absentGate(in, p, expiry)
	case e.State != "listed" || date == "":
		return unknown("no_earnings")
	}
	if date < in.ChainDay {
		return unknown("stale_earnings")
	}
	if date <= expiry {
		return gate("events", compute.GateBlock, "earnings_before_expiry"), ""
	}
	if horizon, ok := addMonths(in.ChainDay, horizonMonths); !ok || expiry > horizon {
		return unknown("beyond_horizon")
	}
	if confirmed {
		return gate("events", compute.GatePass, ""), ""
	}
	return gate("events", compute.GatePass, ""), markUnconfirmed
}

// absentGate handles an earnings row with no next date: a name that reported recently
// cannot report again inside a contract that expires before R + MinReportGapDays.
func absentGate(in compute.CandidateInput, p compute.CandidateParams, expiry contracts.Day) (compute.GateResult, string) {
	if horizon, ok := addMonths(in.ChainDay, horizonMonths); !ok || expiry > horizon {
		return gate("events", compute.GateUnknown, "beyond_horizon"), ""
	}
	r := in.Earnings.Previous
	back, ok1 := dayDiff(r, in.ChainDay)
	gap, ok2 := dayDiff(r, expiry)
	if ok1 && ok2 && back >= 1 && back <= recentReportMax && gap < p.MinReportGapDays {
		return gate("events", compute.GatePass, ""), markBeyond
	}
	return gate("events", compute.GateUnknown, "no_earnings"), ""
}
