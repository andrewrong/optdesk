package data_health_page

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/store/finding"
)

func TestDataCheckNewRuleRows(t *testing.T) {
	t.Parallel()
	run := twoFindings
	run.Findings = append(append([]finding.Finding{}, twoFindings.Findings...),
		finding.Finding{Rule: finding.RuleCandidateRuleVersion, Dataset: "candidates.option_candidate", Day: "2026-03-09",
			Severity: finding.SeverityWarn, Evidence: []finding.Count{{Name: "rows", N: 12}}},
		finding.Finding{Rule: finding.RuleEarningsCoverage, Dataset: "obs.earnings_date", Day: "2026-03-09",
			Severity: finding.SeverityWarn, Evidence: []finding.Count{{Name: "names", N: 35}, {Name: "listed", N: 0}, {Name: "errors", N: 35}}},
		finding.Finding{Rule: finding.RuleEarningsCoverage, Dataset: "obs.earnings_date", Day: "2026-03-08",
			Severity: finding.SeverityWarn, Evidence: []finding.Count{{Name: "names", N: 35}, {Name: "listed", N: 0}, {Name: "listed_before", N: 4}}},
		finding.Finding{Rule: finding.RuleBarOHLCShape, Dataset: "obs.price_bar", Day: "2026-03-09",
			Severity: finding.SeverityError, Evidence: []finding.Count{{Name: "rows", N: 5000}, {Name: "bad", N: 3}}},
	)
	d, _ := buildChecks(t, fakeChecks{run: run})
	rows := d.DataCheck.Findings
	if len(rows) != 6 {
		t.Fatalf("%d rows", len(rows))
	}
	// Existing two rows keep their text and relative order (errors first, then by dataset).
	byKey := map[string]checkRow{}
	for _, r := range rows {
		byKey[r.Rule+"|"+r.Day] = r
	}
	if r := byKey["reading_repeat|2026-03-09"]; r.EvidenceText != "35 个读数中 35 个与前一日相同" {
		t.Errorf("old row %+v", r)
	}
	if r := byKey["capture_gap|2026-03-06"]; r.EvidenceText != "共 63 个键，缺失 60 个，过期 3 个" {
		t.Errorf("old row %+v", r)
	}
	want := map[string][3]string{
		"candidate_rule_version|2026-03-09": {"候选的规则版本落后 (candidate_rule_version)",
			"规则版本是算候选时那套规则的编号，存的编号和现在代码里的不同，说明这些候选是旧规则算的，需要重算", "共 12 行"},
		"earnings_coverage|2026-03-09": {"财报日检查失败或突然查不到 (earnings_coverage)",
			"检查出错，或昨天有日期今天一个都没有；提供方没有日期不算问题", "检查 35 个，查到 0 个，出错 35 个"},
		"earnings_coverage|2026-03-08": {"财报日检查失败或突然查不到 (earnings_coverage)",
			"检查出错，或昨天有日期今天一个都没有；提供方没有日期不算问题", "检查 35 个，昨天查到 4 个，今天 0 个"},
		"bar_ohlc_shape|2026-03-09": {"日线开收价超出最高最低价 (bar_ohlc_shape)",
			"一根日线的开盘或收盘价必须落在它自己的最高价和最低价之间，不满足就是存错了", "共 5000 根，不合格 3 根"},
	}
	for k, w := range want {
		r := byKey[k]
		if r.Label != w[0] || r.WhyText != w[1] || r.EvidenceText != w[2] || r.WhyText == zhRuleWhyOther {
			t.Errorf("%s: %+v", k, r)
		}
	}
}
