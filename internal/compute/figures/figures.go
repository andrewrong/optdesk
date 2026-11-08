// Package figures computes the Figures of the compute module from typed series; it is pure.
package figures

import (
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// calc is the compute.Figures of one Stats, Rolling, SeriesOps and Rules.
type calc struct {
	stats   compute.Stats
	rolling compute.Rolling
	ops     compute.SeriesOps
	rules   compute.Rules
}

// New is a compute.NewFiguresFunc.
func New(stats compute.Stats, rolling compute.Rolling, ops compute.SeriesOps, rules compute.Rules) compute.Figures {
	return calc{stats: stats, rolling: rolling, ops: ops, rules: rules}
}

var _ compute.NewFiguresFunc = New

// begin gives the rule of id and the Figure every method starts from: ID, Version and Day
// set. ok is false when Rules has no rule of id; the Figure is then missing with bad_input.
func (c calc) begin(id string, asOf contracts.Day) (compute.Rule, compute.Figure, bool) {
	rule, ok := c.rules.Rule(id)
	f := compute.Figure{ID: id, Version: rule.Version, Day: asOf, Status: compute.StatusOK}
	if !ok {
		return rule, missing(f, compute.BadInput), false
	}
	return rule, f, true
}

// missing marks f as a Figure without a number.
func missing(f compute.Figure, why compute.Reason) compute.Figure {
	f.Status, f.Reason, f.Value = compute.StatusMissing, why, 0
	return f
}

// stale reports whether day is more than maxAge calendar days before asOf; days that do not
// parse are not stale here, the caller treats them as bad input.
func stale(day, asOf contracts.Day, maxAge int) (isStale, valid bool) {
	d, err1 := time.Parse(time.DateOnly, string(day))
	a, err2 := time.Parse(time.DateOnly, string(asOf))
	if err1 != nil || err2 != nil {
		return false, false
	}
	return int(a.Sub(d).Hours()/24) > maxAge, true
}

// checkAge marks f stale_input or bad_input when its DataDay is too old or unreadable.
func checkAge(f compute.Figure, rule compute.Rule) (compute.Figure, bool) {
	isStale, valid := stale(f.DataDay, f.Day, rule.MaxAgeDays)
	switch {
	case !valid:
		return missing(f, compute.BadInput), false
	case isStale:
		return missing(f, compute.StaleInput), false
	}
	return f, true
}
