// datacheck.go builds the data_check block of the page (docs/design/data-check.md, entry 9, Decisions 12, 13).
package data_health_page

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/store/finding"
)

const gapDataCheck = "data_check_unavailable"

// DataCheckVM is the data_check block: the last run, a count per severity and one row per finding.
type DataCheckVM struct {
	CheckDay     string          `json:"check_day"`
	FinishedAt   string          `json:"finished_at"`
	NoneText     string          `json:"none_text"`
	RulesTotal   int             `json:"rules_total"`
	RulesUnknown int             `json:"rules_unknown"`
	Title        string          `json:"title"`
	Note         string          `json:"note"`
	Counts       DataCheckCounts `json:"counts"`
	Findings     []DataCheckRow  `json:"findings"`
}

// DataCheckCounts is the number of findings per severity.
type DataCheckCounts struct {
	Error   int `json:"error"`
	Warn    int `json:"warn"`
	Unknown int `json:"unknown"`
}

// DataCheckRow is one finding as the page shows it.
type DataCheckRow struct {
	Rule         string `json:"rule"`
	Label        string `json:"label"`
	Dataset      string `json:"dataset"`
	Day          string `json:"day"`
	Severity     string `json:"severity"`
	SeverityText string `json:"severity_text"`
	IsNew        bool   `json:"is_new"`
	EvidenceText string `json:"evidence_text"`
	WhyText      string `json:"why_text"`
}

// dataCheckBlock reads the newest run. A nil reader gives no block and no gap; a read error gives no block and
// the gap data_check_unavailable.
func dataCheckBlock(ctx context.Context, in Input) (*DataCheckVM, *Gap) {
	if in.Checks == nil {
		return nil, nil
	}
	run, err := in.Checks.Latest(ctx)
	if err != nil {
		return nil, &Gap{Kind: "gap", Reason: gapDataCheck, Text: zhDataCheckGapText}
	}
	vm := &DataCheckVM{Title: zhDataCheckTitle, Note: zhDataCheckNote, Findings: []DataCheckRow{}}
	if run.RunID == 0 && run.CheckDay == "" {
		vm.NoneText = zhDataCheckNone
		return vm, nil
	}
	vm.CheckDay = string(run.CheckDay)
	if !run.FinishedAt.IsZero() {
		vm.FinishedAt = run.FinishedAt.UTC().Format(time.RFC3339)
	}
	vm.RulesTotal, vm.RulesUnknown = run.RulesTotal, run.RulesUnknown
	fs := slices.Clone(run.Findings)
	slices.SortStableFunc(fs, func(a, b finding.Finding) int {
		return cmp.Or(cmp.Compare(severityRank(a.Severity), severityRank(b.Severity)),
			cmp.Compare(a.Dataset, b.Dataset), cmp.Compare(a.Day, b.Day), cmp.Compare(a.Rule, b.Rule))
	})
	for _, f := range fs {
		switch f.Severity {
		case finding.SeverityError:
			vm.Counts.Error++
		case finding.SeverityWarn:
			vm.Counts.Warn++
		case finding.SeverityUnknown:
			vm.Counts.Unknown++
		default:
			vm.Counts.Unknown++
		}
		vm.Findings = append(vm.Findings, DataCheckRow{Rule: f.Rule, Label: ruleLabel(f.Rule), Dataset: f.Dataset,
			Day: string(f.Day), Severity: string(f.Severity), SeverityText: severityText(f.Severity), IsNew: f.IsNew,
			EvidenceText: evidenceText(f.Evidence), WhyText: ruleWhy(f.Rule)})
	}
	return vm, nil
}

func severityRank(s finding.Severity) int {
	switch s {
	case finding.SeverityError:
		return 0
	case finding.SeverityWarn:
		return 1
	case finding.SeverityUnknown:
		return 2
	}
	return 2
}

func severityText(s finding.Severity) string {
	switch s {
	case finding.SeverityError:
		return zhSevError
	case finding.SeverityWarn:
		return zhSevWarn
	case finding.SeverityUnknown:
		return zhSevUnknown
	}
	return zhSevUnknown
}

// ruleLabel shows the Chinese label with the rule id beside it; an id this page does not know shows alone.
func ruleLabel(rule string) string {
	if l := map[string]string{
		finding.RuleReadingRepeat: zhLabelReadingRepeat, finding.RuleOpenLateQuote: zhLabelOpenLate,
		finding.RuleCandidatesStale: zhLabelCandidates, finding.RuleCaptureGap: zhLabelCaptureGap,
		finding.RulePlanAbsent: zhLabelPlanAbsent, finding.RuleGapUnclosed: zhLabelGapUnclosed,
	}[rule]; l != "" {
		return l + " (" + rule + ")"
	}
	return rule
}

func ruleWhy(rule string) string {
	if w := map[string]string{
		finding.RuleReadingRepeat: zhWhyReadingRepeat, finding.RuleOpenLateQuote: zhWhyOpenLate,
		finding.RuleCandidatesStale: zhWhyCandidates, finding.RuleCaptureGap: zhWhyCaptureGap,
		finding.RulePlanAbsent: zhWhyPlanAbsent, finding.RuleGapUnclosed: zhWhyGapUnclosed,
	}[rule]; w != "" {
		return w
	}
	return zhRuleWhyOther
}

// evidenceText words the counts of a finding as a Chinese sentence naming each count.
func evidenceText(ev []finding.Count) string {
	n := map[string]int{}
	has := map[string]bool{}
	for _, c := range ev {
		n[c.Name], has[c.Name] = c.N, true
	}
	var parts []string
	used := map[string]bool{}
	pair := func(a, b, format string) {
		if has[a] && has[b] {
			parts = append(parts, fmt.Sprintf(format, n[a], n[b]))
			used[a], used[b] = true, true
		}
	}
	pair("ok", "same", zhEvOKSame)
	pair("rows", "late", zhEvRowsLate)
	for _, c := range ev {
		if used[c.Name] {
			continue
		}
		f, ok := map[string]string{"ok": zhEvOK, "same": zhEvSame, "rows": zhEvRows, "late": zhEvLate,
			"hours_behind": zhEvHoursBehnd, "keys": zhEvKeys, "missing": zhEvMissing, "expired": zhEvExpired,
			"cells": zhEvCells, "age_hours": zhEvAgeHours, "read_failed": zhEvReadFailed}[c.Name]
		if !ok {
			f = zhEvOther
		}
		parts = append(parts, fmt.Sprintf(f, c.N))
	}
	if len(parts) == 0 {
		return zhEvNone
	}
	return strings.Join(parts, zhEvSep)
}
