package data_health_page

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

type fakeChecks struct {
	run finding.Run
	err error
}

func (f fakeChecks) Latest(context.Context) (finding.Run, error) { return f.run, f.err }
func (fakeChecks) Close() error                                  { return nil }

type checkRow struct {
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

type checkDoc struct {
	DataCheck *struct {
		CheckDay     string         `json:"check_day"`
		FinishedAt   string         `json:"finished_at"`
		NoneText     string         `json:"none_text"`
		RulesTotal   int            `json:"rules_total"`
		RulesUnknown int            `json:"rules_unknown"`
		Title        string         `json:"title"`
		Note         string         `json:"note"`
		Counts       map[string]int `json:"counts"`
		Findings     []checkRow     `json:"findings"`
	} `json:"data_check"`
	Gaps []struct {
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
		Text   string `json:"text"`
	} `json:"gaps"`
	Datasets []Dataset `json:"datasets"`
}

func buildChecks(t *testing.T, c finding.Reader) (checkDoc, map[string]json.RawMessage) {
	t.Helper()
	in := input(&fakeReader{sessions: weekdays("XNYS")}, nil, nil)
	in.Checks = c
	b, err := Build(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	var d checkDoc
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	return d, raw
}

var twoFindings = finding.Run{
	RunID: 7, CheckDay: "2026-03-09", FinishedAt: time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC),
	RulesTotal: 6, RulesUnknown: 1,
	Findings: []finding.Finding{
		{Rule: finding.RuleCaptureGap, Dataset: "obs.price_bar", Day: "2026-03-06", Severity: finding.SeverityWarn,
			Evidence: []finding.Count{{Name: "keys", N: 63}, {Name: "missing", N: 60}, {Name: "expired", N: 3}}},
		{Rule: finding.RuleReadingRepeat, Dataset: "vix", Day: "2026-03-09", Severity: finding.SeverityError, IsNew: true,
			Evidence: []finding.Count{{Name: "ok", N: 35}, {Name: "same", N: 35}}},
	},
}

// checkGaps counts the gaps of the data check; the fake coverage reader has no failures, which is another gap.
func checkGaps(d checkDoc) int {
	n := 0
	for _, g := range d.Gaps {
		if g.Reason == "data_check_unavailable" {
			n++
		}
	}
	return n
}

func TestDataCheckKeyAlwaysPresent(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]finding.Reader{"nil": nil, "empty": fakeChecks{}, "run": fakeChecks{run: twoFindings}} {
		_, raw := buildChecks(t, c)
		if _, ok := raw["data_check"]; !ok {
			t.Errorf("%s: no key data_check", name)
		}
	}
	d, raw := buildChecks(t, nil)
	if string(raw["data_check"]) != "null" || d.DataCheck != nil || checkGaps(d) != 0 {
		t.Errorf("nil reader: %s, gaps %v", raw["data_check"], d.Gaps)
	}
	d, _ = buildChecks(t, fakeChecks{})
	if d.DataCheck == nil || d.DataCheck.NoneText == "" || len(d.DataCheck.Findings) != 0 ||
		d.DataCheck.Counts["error"]+d.DataCheck.Counts["warn"]+d.DataCheck.Counts["unknown"] != 0 {
		t.Errorf("empty reader: %+v", d.DataCheck)
	}
}

func TestDataCheckRows(t *testing.T) {
	t.Parallel()
	d, _ := buildChecks(t, fakeChecks{run: twoFindings})
	dc := d.DataCheck
	if dc == nil || dc.RulesTotal != 6 || dc.RulesUnknown != 1 || dc.CheckDay != "2026-03-09" || dc.NoneText != "" ||
		dc.Counts["error"] != 1 || dc.Counts["warn"] != 1 || dc.Counts["unknown"] != 0 || len(dc.Findings) != 2 {
		t.Fatalf("%+v", dc)
	}
	e := dc.Findings[0]
	if e.Rule != "reading_repeat" || e.SeverityText != "错误" || !e.IsNew || e.EvidenceText != "35 个读数中 35 个与前一日相同" ||
		e.WhyText == "" || e.Label == "" {
		t.Errorf("error row %+v", e)
	}
	w := dc.Findings[1]
	if w.SeverityText != "缺口" || w.IsNew || w.EvidenceText != "共 63 个键，缺失 60 个，过期 3 个" {
		t.Errorf("warn row %+v", w)
	}
}

func TestDataCheckFixtureDecodes(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("../../../contracts/fixtures/page.data_health_page.v1.datacheck.json")
	if err != nil {
		t.Fatal(err)
	}
	var vm viewModel
	if err := json.Unmarshal(b, &vm); err != nil {
		t.Fatal(err)
	}
	if vm.DataCheck == nil || len(vm.DataCheck.Findings) != 3 || vm.DataCheck.Findings[0].Rule != "reading_repeat" {
		t.Errorf("%+v", vm.DataCheck)
	}
}

func TestDataCheckOrder(t *testing.T) {
	t.Parallel()
	mk := func(rule, ds, day string, s finding.Severity) finding.Finding {
		return finding.Finding{Rule: rule, Dataset: ds, Day: contracts.Day(day), Severity: s,
			Evidence: []finding.Count{{Name: "cells", N: 0}}}
	}
	run := finding.Run{RunID: 1, CheckDay: "2026-03-09", FinishedAt: testNow, Findings: []finding.Finding{
		mk("gap_unclosed", "a", "2026-03-01", finding.SeverityUnknown),
		mk("plan_absent", "b", "2026-03-02", finding.SeverityWarn),
		mk("plan_absent", "a", "2026-03-03", finding.SeverityWarn),
		mk("plan_absent", "a", "2026-03-02", finding.SeverityWarn),
		mk("capture_gap", "a", "2026-03-02", finding.SeverityWarn),
		mk("reading_repeat", "z", "2026-03-09", finding.SeverityError),
	}}
	d, _ := buildChecks(t, fakeChecks{run: run})
	want := [][3]string{
		{"reading_repeat", "z", "2026-03-09"},
		{"capture_gap", "a", "2026-03-02"},
		{"plan_absent", "a", "2026-03-02"},
		{"plan_absent", "a", "2026-03-03"},
		{"plan_absent", "b", "2026-03-02"},
		{"gap_unclosed", "a", "2026-03-01"},
	}
	for i, w := range want {
		g := d.DataCheck.Findings[i]
		if g.Rule != w[0] || g.Dataset != w[1] || g.Day != w[2] {
			t.Errorf("row %d: %s %s %s, want %v", i, g.Rule, g.Dataset, g.Day, w)
		}
	}
}

func TestDataCheckUnavailable(t *testing.T) {
	t.Parallel()
	d, raw := buildChecks(t, fakeChecks{err: errors.New("down")})
	if string(raw["data_check"]) != "null" {
		t.Errorf("data_check %s", raw["data_check"])
	}
	if checkGaps(d) != 1 {
		t.Errorf("gaps %+v", d.Gaps)
	}
	for _, g := range d.Gaps {
		if g.Reason == "data_check_unavailable" && (g.Kind != "gap" || g.Text == "") {
			t.Errorf("gap %+v", g)
		}
	}
}

func TestSeveritiesHaveGlossary(t *testing.T) {
	t.Parallel()
	b, err := Build(context.Background(), input(&fakeReader{}, nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	var d doc
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, g := range d.Glossary {
		have[g.Term] = true
	}
	for term, sev := range map[string]finding.Severity{"错误": finding.SeverityError, "缺口": finding.SeverityWarn,
		"无法判断": finding.SeverityUnknown} {
		if !have[term] || severityText(sev) != term {
			t.Errorf("no glossary row for %s", sev)
		}
	}
	if !have["新"] {
		t.Error("no glossary row for 新")
	}
}
