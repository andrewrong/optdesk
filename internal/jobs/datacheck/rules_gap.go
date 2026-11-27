package datacheck

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

// allCalendars lists the calendars a dataset outside the expected list may belong to.
var allCalendars = []string{calUS, calJP}

// windowDays returns the set of days a dataset is judged on: the window of its calendar when the dataset
// is in the expected list, else the days of every window.
func windowDays(in Input, dataset string) map[contracts.Day]bool {
	cals := allCalendars
	for _, e := range in.Expected {
		if e.Dataset == dataset {
			cals = []string{e.Calendar}
			break
		}
	}
	days := map[contracts.Day]bool{}
	for _, c := range cals {
		for _, d := range in.Windows[c] {
			days[d] = true
		}
	}
	return days
}

// ruleCaptureGap: a ledger cell of a window day with missing or expired keys.
func ruleCaptureGap(ctx context.Context, in Input) ([]finding.Finding, error) {
	out := []finding.Finding{}
	from, to, ok := span(in, allCalendars...)
	if !ok {
		return out, nil
	}
	cells, err := in.Cov.Cells(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for _, c := range cells {
		if c.Dataset == datasetID || c.Missing+c.Expired == 0 || !windowDays(in, c.Dataset)[c.Day] {
			continue
		}
		out = append(out, finding.Finding{
			Rule: finding.RuleCaptureGap, Dataset: c.Dataset, Day: c.Day, Severity: finding.SeverityWarn,
			Evidence: []finding.Count{{Name: "keys", N: c.Keys}, {Name: "missing", N: c.Missing}, {Name: "expired", N: c.Expired}},
		})
	}
	return out, nil
}

// rulePlanAbsent: an expected dataset with no cell on a session day of its calendar's window.
func rulePlanAbsent(ctx context.Context, in Input) ([]finding.Finding, error) {
	out := []finding.Finding{}
	from, to, ok := span(in, allCalendars...)
	if !ok || len(in.Expected) == 0 {
		return out, nil
	}
	cells, err := in.Cov.Cells(ctx, from, to)
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, c := range cells {
		have[c.Dataset+"\x00"+string(c.Day)] = true
	}
	for _, e := range in.Expected {
		if e.Dataset == datasetID {
			continue
		}
		for _, d := range in.Windows[e.Calendar] {
			if have[e.Dataset+"\x00"+string(d)] {
				continue
			}
			out = append(out, finding.Finding{
				Rule: finding.RulePlanAbsent, Dataset: e.Dataset, Day: d, Severity: finding.SeverityWarn,
				Evidence: []finding.Count{{Name: "cells", N: 0}},
			})
		}
	}
	return out, nil
}

// ruleGapUnclosed: an open job gap of a window day first seen more than GapGrace before Now.
func ruleGapUnclosed(ctx context.Context, in Input) ([]finding.Finding, error) {
	out := []finding.Finding{}
	gaps, err := in.Probe.OpenGaps(ctx)
	if err != nil {
		return nil, err
	}
	for _, g := range gaps {
		ds := g.Dataset
		if ds == "" {
			ds = g.JobID
		}
		age := in.Now.Sub(g.FirstSeenAt)
		if age <= GapGrace || !windowDays(in, ds)[g.Day] {
			continue
		}
		out = append(out, finding.Finding{
			Rule: finding.RuleGapUnclosed, Dataset: ds, Day: g.Day, Severity: finding.SeverityWarn,
			Evidence: []finding.Count{{Name: "age_hours", N: int(age.Hours())}},
		})
	}
	return out, nil
}
