package base_data_page

import (
	"fmt"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func sortDays(d []contracts.Day) { sort.Slice(d, func(i, j int) bool { return d[i] < d[j] }) }

// groupsOf lays out the rows of the verdicts in the order and the groups of basecheckapi.Rows. A row the verdicts
// do not carry is shown unknown, with no check.
func groupsOf(out basecheckapi.Output) []groupVM {
	byID := map[string]basecheckapi.Row{}
	for _, r := range out.Rows {
		byID[r.Dataset] = r
	}
	groups := []groupVM{}
	idx := map[string]int{}
	for _, info := range basecheckapi.Rows {
		r, ok := byID[info.ID]
		if !ok {
			r = basecheckapi.Row{Dataset: info.ID, Level: basecheckapi.Unknown}
		}
		i, seen := idx[info.Group]
		if !seen {
			i = len(groups)
			idx[info.Group] = i
			groups = append(groups, groupVM{Name: info.Group, Rows: []rowVM{}})
		}
		groups[i].Rows = append(groups[i].Rows, rowPayload(info, r))
	}
	return groups
}

// rowPayload is the row of the page; its check list is built by checkList from the same row.
func rowPayload(info basecheckapi.RowInfo, r basecheckapi.Row) rowVM {
	label, _ := Hint(info.ID)
	vm := rowVM{Dataset: info.ID, Label: rowLabel(info.ID), LabelZh: label, Level: string(r.Level),
		NewestValue: r.NewestValue, NewestSubject: r.NewestSubject}
	if r.Newest != "" {
		d := r.Newest
		vm.NewestDay = &d
		vm.AgeText = ageText(info, r)
	}
	vm.Checks = checkList(info.ID, r.Checks)
	vm.FailedSummary = failedSummary(r.Checks)
	return vm
}

// ageText is sessions (s) for a row with a calendar and calendar days (d) for one without.
func ageText(info basecheckapi.RowInfo, r basecheckapi.Row) string {
	if len(info.Calendars) > 0 {
		return fmt.Sprintf("%d s", r.AgeSessions)
	}
	return fmt.Sprintf("%d d", r.AgeDays)
}

func checkList(rowID string, checks []basecheckapi.Check) []checkVM {
	out := make([]checkVM, 0, len(checks))
	for _, c := range checks {
		hint, ok := Hint(rowID + "." + c.Rule)
		if !ok {
			hint = hintFallback
		}
		out = append(out, checkVM{Rule: c.Rule, Label: ruleLabel(c.Rule), HintZh: hint, Level: string(c.Level),
			N: c.N, Bad: c.Bad, Share: c.Share, WarnAt: c.WarnAt, FailAt: c.FailAt,
			ThresholdSource: c.ThresholdSource, Example: c.Example})
	}
	return out
}

// failedSummary lists the checks at warn or fail: "age; crossed 2.1%".
func failedSummary(checks []basecheckapi.Check) string {
	var parts []string
	for _, c := range checks {
		if c.Level != basecheckapi.Warn && c.Level != basecheckapi.Fail {
			continue
		}
		p := c.Rule
		if c.Rule == "coverage_gap" {
			p = fmt.Sprintf("%s %d days", c.Rule, c.Bad)
		} else if pct, ok := percentText(c.Share); ok {
			p = fmt.Sprintf("%s %s%%", c.Rule, pct)
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, "; ")
}

func countsOf(m map[basecheckapi.Level]int) countsVM {
	return countsVM{Pass: m[basecheckapi.Pass], Warn: m[basecheckapi.Warn], Fail: m[basecheckapi.Fail], Unknown: m[basecheckapi.Unknown]}
}

func glossary() []GlossaryRow {
	return []GlossaryRow{
		{termPass, textPass}, {termWarn, textWarn}, {termFail, textFail}, {termUnknown, textUnknown},
		{termSession, textSession}, {termLedger, textLedger}, {termTrial, textTrial}, {termShare, textShare},
		{termSplit, textSplit}, {termCrossed, textCrossed}, {termNonLive, textNonLive}, {termCoverage, textCoverage},
	}
}

// percentText moves the decimal point of a decimal text two places, as text, so that no float arithmetic is done:
// "0.021" is "2.1" and "0.07" is "7". It is false for a share that is zero or not a plain decimal.
func percentText(share string) (string, bool) {
	whole, frac, _ := strings.Cut(share, ".")
	if whole == "" || !allDigits(whole) || (frac != "" && !allDigits(frac)) || strings.Trim(whole+frac, "0") == "" {
		return "", false
	}
	for len(frac) < 2 {
		frac += "0"
	}
	whole = strings.TrimLeft(whole+frac[:2], "0")
	if whole == "" {
		whole = "0"
	}
	if rest := strings.TrimRight(frac[2:], "0"); rest != "" {
		return whole + "." + rest, true
	}
	return whole, true
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
