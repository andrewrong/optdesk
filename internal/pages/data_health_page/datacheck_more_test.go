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

// The six rules of data check keep their label and reason (literals as they stand on main), in sorted order, when
// the three new rules sit among them.
func TestDataCheckSixOldRowsUnchanged(t *testing.T) {
	t.Parallel()
	mk := func(rule, ds string) finding.Finding {
		return finding.Finding{Rule: rule, Dataset: ds, Day: "2026-03-09", Severity: finding.SeverityWarn}
	}
	run := finding.Run{RunID: 1, CheckDay: "2026-03-09", RulesTotal: 9, Findings: []finding.Finding{
		mk(finding.RuleBarOHLCShape, "d3"), mk(finding.RuleGapUnclosed, "d9"), mk(finding.RuleReadingRepeat, "d1"),
		mk(finding.RuleCandidateRuleVersion, "d7"), mk(finding.RuleCaptureGap, "d4"), mk(finding.RuleOpenLateQuote, "d2"),
		mk(finding.RuleEarningsCoverage, "d8"), mk(finding.RulePlanAbsent, "d5"), mk(finding.RuleCandidatesStale, "d6"),
	}}
	d, _ := buildChecks(t, fakeChecks{run: run})
	want := []struct{ rule, label, why string }{
		{"reading_repeat", "读数重复前一日 (reading_repeat)", "一天的读数和前一天完全一样，通常说明源站没有更新，旧值被当作新值保存了。"},
		{"option_open_late_quote", "期权开盘报价晚到 (option_open_late_quote)", "期权开盘报价在交易日之后才收到，用它算出的结果可能不是那一天的真实情况。"},
		{"bar_ohlc_shape", "", ""},
		{"capture_gap", "采集缺口 (capture_gap)", "运行器记录里这一天有缺失或过期的键，这些数据没有保存。"},
		{"plan_absent", "缺少计划 (plan_absent)", "这个数据集在这个交易日没有任何计划记录，任务可能根本没有运行。"},
		{"candidates_older_than_settings", "候选早于最新设置 (candidates_older_than_settings)", "候选是在最新设置保存之前算出的，没有按现在的设置重算。"},
		{"candidate_rule_version", "", ""},
		{"earnings_coverage", "", ""},
		{"gap_unclosed", "缺口未补 (gap_unclosed)", "任务记录了这个缺口，超过宽限时间仍然没有补上。"},
	}
	rows := d.DataCheck.Findings
	if len(rows) != len(want) {
		t.Fatalf("%d rows", len(rows))
	}
	for i, w := range want {
		r := rows[i]
		if r.Rule != w.rule {
			t.Fatalf("row %d is %s, want %s", i, r.Rule, w.rule)
		}
		if w.label != "" && (r.Label != w.label || r.WhyText != w.why) {
			t.Errorf("row %d: %+v", i, r)
		}
	}
}
