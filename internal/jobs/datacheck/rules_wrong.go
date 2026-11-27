package datacheck

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

// minRepeatSubjects is the fewest subjects with an ok reading on a day for reading_repeat to judge it.
const minRepeatSubjects = 5

// repeatedReadingNames are the readings computed from bars, which repeat when the bars are stale.
var repeatedReadingNames = []string{"market.hv30_own", "market.rv20", "market.rv60", "market.rv120"}

const (
	calUS = "XNYS"
	calJP = "XTKS"

	datasetOpenQuote = "obs.option_open"
	datasetCandidate = "candidates.option_candidate"
)

// span returns the first and last day of the windows named, and false when all are empty.
func span(in Input, cals ...string) (from, to contracts.Day, ok bool) {
	for _, c := range cals {
		for _, d := range in.Windows[c] {
			if !ok || d < from {
				from = d
			}
			if !ok || d > to {
				to = d
			}
			ok = true
		}
	}
	return from, to, ok
}

// ruleReadingRepeat: at least half of at least 5 subjects repeat the value of the previous day.
func ruleReadingRepeat(ctx context.Context, in Input) ([]finding.Finding, error) {
	out := []finding.Finding{}
	from, to, ok := span(in, calUS)
	if !ok {
		return out, nil
	}
	facts, err := in.Probe.RepeatedReadings(ctx, repeatedReadingNames, from, to)
	if err != nil {
		return nil, err
	}
	for _, f := range facts {
		if f.OK < minRepeatSubjects || f.Same*2 < f.OK {
			continue
		}
		out = append(out, finding.Finding{
			Rule: finding.RuleReadingRepeat, Dataset: f.Name, Day: f.Day, Severity: finding.SeverityError,
			Evidence: []finding.Count{{Name: "ok", N: f.OK}, {Name: "same", N: f.Same}},
		})
	}
	return out, nil
}

// ruleOpenLateQuote: any option_open row received on a later local day than its session day.
func ruleOpenLateQuote(ctx context.Context, in Input) ([]finding.Finding, error) {
	out := []finding.Finding{}
	from, to, ok := span(in, calUS, calJP)
	if !ok {
		return out, nil
	}
	facts, err := in.Probe.LateOpenQuotes(ctx, from, to)
	if err != nil {
		return nil, err
	}
	// One finding per day: the key has no currency, so the currencies of a day are added.
	idx := map[contracts.Day]int{}
	for _, f := range facts {
		if f.Late < 1 {
			continue
		}
		i, seen := idx[f.Day]
		if !seen {
			idx[f.Day] = len(out)
			out = append(out, finding.Finding{
				Rule: finding.RuleOpenLateQuote, Dataset: datasetOpenQuote, Day: f.Day, Severity: finding.SeverityError,
				Evidence: []finding.Count{{Name: "rows"}, {Name: "late"}},
			})
			i = len(out) - 1
		}
		out[i].Evidence[0].N += f.Rows
		out[i].Evidence[1].N += f.Late
	}
	return out, nil
}

// ruleCandidatesStale: the newest day's candidate runs were computed before the newest stored setting change.
func ruleCandidatesStale(ctx context.Context, in Input) ([]finding.Finding, error) {
	out := []finding.Finding{}
	from, to, ok := span(in, calUS)
	if !ok {
		return out, nil
	}
	runs, err := in.Probe.NewestCandidateRuns(ctx, from, to)
	if err != nil {
		return nil, err
	}
	changed, has, err := in.Probe.SettingsChangedAt(ctx)
	if err != nil {
		return nil, err
	}
	if !has || len(runs) == 0 {
		return out, nil
	}
	newest := runs[0].Day
	for _, r := range runs {
		if r.Day > newest {
			newest = r.Day
		}
	}
	// One finding for the day: the key has no kind, so the stale kinds are added and the worst lag is shown.
	rows, behind, stale := 0, 0, false
	for _, r := range runs {
		if r.Day != newest || !r.ComputedAt.Before(changed) {
			continue
		}
		stale = true
		rows += r.Rows
		if h := int(changed.Sub(r.ComputedAt).Hours()); h > behind {
			behind = h
		}
	}
	if stale {
		out = append(out, finding.Finding{
			Rule: finding.RuleCandidatesStale, Dataset: datasetCandidate, Day: newest, Severity: finding.SeverityWarn,
			Evidence: []finding.Count{{Name: "rows", N: rows}, {Name: "hours_behind", N: behind}},
		})
	}
	return out, nil
}
