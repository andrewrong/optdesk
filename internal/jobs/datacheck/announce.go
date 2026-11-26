package datacheck

import (
	"fmt"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/store/finding"
)

// maxLines is the most findings the message lists.
const maxLines = 10

var ruleLabels = map[string]string{
	finding.RuleReadingRepeat:   "读数重复前一日",
	finding.RuleOpenLateQuote:   "开盘报价迟到",
	finding.RuleCandidatesStale: "候选早于设置",
	finding.RuleCaptureGap:      "采集缺口",
	finding.RulePlanAbsent:      "没有计划",
	finding.RuleGapUnclosed:     "缺口未关闭",
}

var severityLabels = map[finding.Severity]string{
	finding.SeverityError:   "错误",
	finding.SeverityWarn:    "缺口",
	finding.SeverityUnknown: "无法判断",
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
	return 3
}

// announceText builds the one message of the findings with IsNew: at most maxLines lines, error before warn
// before unknown, each with the rule label, the dataset, the day and the counts, then the number left out. It
// returns false when no finding is new. The text holds fixed words, datasets, days and digits only.
func announceText(fs []finding.Finding) (string, bool) {
	var fresh []finding.Finding
	for _, f := range fs {
		if f.IsNew {
			fresh = append(fresh, f)
		}
	}
	if len(fresh) == 0 {
		return "", false
	}
	sort.SliceStable(fresh, func(i, j int) bool {
		a, b := fresh[i], fresh[j]
		if ra, rb := severityRank(a.Severity), severityRank(b.Severity); ra != rb {
			return ra < rb
		}
		if a.Dataset != b.Dataset {
			return a.Dataset < b.Dataset
		}
		if a.Day != b.Day {
			return a.Day < b.Day
		}
		return a.Rule < b.Rule
	})
	var sb strings.Builder
	fmt.Fprintf(&sb, "数据检查：%d 条新发现", len(fresh))
	for i, f := range fresh {
		if i == maxLines {
			break
		}
		label, ok := ruleLabels[f.Rule]
		if !ok {
			label = f.Rule
		}
		fmt.Fprintf(&sb, "\n[%s] %s %s %s", severityLabels[f.Severity], label, f.Dataset, f.Day)
		for _, c := range f.Evidence {
			fmt.Fprintf(&sb, " %s=%d", c.Name, c.N)
		}
	}
	if len(fresh) > maxLines {
		fmt.Fprintf(&sb, "\n还有 %d 条见页面", len(fresh)-maxLines)
	}
	return sb.String(), true
}
