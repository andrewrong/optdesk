package datacheck

import (
	"context"
	"errors"
	"slices"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

const (
	datasetEarnings = "obs.earnings_date"
	datasetBar      = "obs.price_bar"
)

// ruleCandidateVersion: the newest candidate run of a kind carries a rule version other than the expected one
// (empty when its rows disagree).
func ruleCandidateVersion(ctx context.Context, in Input) ([]finding.Finding, error) {
	want, ok := in.RuleVersions[datasetCandidate]
	if !ok {
		return nil, errors.New("no_expected_version")
	}
	out := []finding.Finding{}
	from, to, ok := span(in, calUS)
	if !ok {
		return out, nil
	}
	runs, err := in.Probe.NewestCandidateRuns(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for _, r := range runs {
		if r.RuleVersion == want {
			continue
		}
		out = append(out, finding.Finding{
			Rule: finding.RuleCandidateRuleVersion, Dataset: datasetCandidate, Day: r.Day, Severity: finding.SeverityWarn,
			Evidence: []finding.Count{{Name: "rows", N: r.Rows}},
		})
	}
	return out, nil
}

// ruleEarningsCoverage: per market and window day with check rows, any error row, or no listed name where the
// previous window day with rows had some.
func ruleEarningsCoverage(ctx context.Context, in Input) ([]finding.Finding, error) {
	out := []finding.Finding{}
	from, to, ok := span(in, calUS, calJP)
	if !ok {
		return out, nil
	}
	checks, err := in.Probe.EarningsChecks(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for _, m := range []struct{ market, cal string }{{"US", calUS}, {"JP", calJP}} {
		var rows []finding.EarningsCheck
		for _, c := range checks {
			if c.Market == m.market && slices.Contains(in.Windows[m.cal], c.Day) {
				rows = append(rows, c)
			}
		}
		slices.SortFunc(rows, func(a, b finding.EarningsCheck) int { return compareDay(a.Day, b.Day) })
		for i, c := range rows {
			switch {
			case c.Errors > 0:
				out = append(out, finding.Finding{
					Rule: finding.RuleEarningsCoverage, Dataset: datasetEarnings, Day: c.Day, Severity: finding.SeverityWarn,
					Evidence: []finding.Count{{Name: "names", N: c.Names}, {Name: "listed", N: c.Listed}, {Name: "errors", N: c.Errors}},
				})
			case c.Listed == 0 && i > 0 && rows[i-1].Listed > 0:
				out = append(out, finding.Finding{
					Rule: finding.RuleEarningsCoverage, Dataset: datasetEarnings, Day: c.Day, Severity: finding.SeverityWarn,
					Evidence: []finding.Count{{Name: "names", N: c.Names}, {Name: "listed"}, {Name: "listed_before", N: rows[i-1].Listed}},
				})
			}
		}
	}
	return out, nil
}

func compareDay(a, b contracts.Day) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// ruleBarShape: a window day with trades bars that break low <= open, close <= high.
func ruleBarShape(ctx context.Context, in Input) ([]finding.Finding, error) {
	out := []finding.Finding{}
	from, to, ok := span(in, calUS, calJP)
	if !ok {
		return out, nil
	}
	shapes, err := in.Probe.BarShapes(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for _, s := range shapes {
		if s.Bad < 1 || (!slices.Contains(in.Windows[calUS], s.Day) && !slices.Contains(in.Windows[calJP], s.Day)) {
			continue
		}
		out = append(out, finding.Finding{
			Rule: finding.RuleBarOHLCShape, Dataset: datasetBar, Day: s.Day, Severity: finding.SeverityError,
			Evidence: []finding.Count{{Name: "rows", N: s.Rows}, {Name: "bad", N: s.Bad}},
		})
	}
	return out, nil
}
