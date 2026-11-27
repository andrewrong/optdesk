package datacheck

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

type moreProbe struct {
	finding.Probe
	runs     []finding.CandidateRun
	earnings []finding.EarningsCheck
	bars     []finding.BarShape
	failWhat string
}

func (p *moreProbe) fail(what string) error {
	if p.failWhat == what {
		return errProbe
	}
	return nil
}
func (p *moreProbe) NewestCandidateRuns(context.Context, contracts.Day, contracts.Day) ([]finding.CandidateRun, error) {
	return p.runs, p.fail("runs")
}
func (p *moreProbe) EarningsChecks(context.Context, contracts.Day, contracts.Day) ([]finding.EarningsCheck, error) {
	return p.earnings, p.fail("earnings")
}
func (p *moreProbe) BarShapes(context.Context, contracts.Day, contracts.Day) ([]finding.BarShape, error) {
	return p.bars, p.fail("bars")
}

func (p *moreProbe) RepeatedReadings(context.Context, []string, contracts.Day, contracts.Day) ([]finding.ReadingRepeat, error) {
	return nil, nil
}
func (p *moreProbe) LateOpenQuotes(context.Context, contracts.Day, contracts.Day) ([]finding.LateQuotes, error) {
	return nil, nil
}
func (p *moreProbe) SettingsChangedAt(context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
func (p *moreProbe) OpenGaps(context.Context) ([]finding.OpenGap, error) { return nil, nil }

func moreInput(p finding.Probe) Input {
	in := wrongInput(p)
	in.RuleVersions = map[string]string{datasetCandidate: "put_v2"}
	return in
}

func TestDefaultRulesNine(t *testing.T) {
	t.Parallel()
	var ids []string
	for _, r := range defaultRules() {
		ids = append(ids, r.ID)
	}
	want := []string{"reading_repeat", "option_open_late_quote", "candidates_older_than_settings", "capture_gap", "plan_absent", "gap_unclosed",
		"candidate_rule_version", "earnings_coverage", "bar_ohlc_shape"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v", ids)
	}
}

func TestCandidateRuleVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		version string
		want    int
	}{
		{"old version", "put_v1", 1},
		{"current version", "put_v2", 0},
		{"rows disagree", "", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := &moreProbe{runs: []finding.CandidateRun{{Day: dayD, Kind: "put", Rows: 40, RuleVersion: c.version}}}
			got, err := ruleCandidateVersion(context.Background(), moreInput(p))
			if err != nil || len(got) != c.want {
				t.Fatalf("got %v, %v", got, err)
			}
			if c.want == 1 {
				w := finding.Finding{Rule: finding.RuleCandidateRuleVersion, Dataset: datasetCandidate, Day: dayD,
					Severity: finding.SeverityWarn, Evidence: []finding.Count{{Name: "rows", N: 40}}}
				if !reflect.DeepEqual(got[0], w) {
					t.Fatalf("got %+v", got[0])
				}
			}
		})
	}
	t.Run("no expected version", func(t *testing.T) {
		t.Parallel()
		in := moreInput(&moreProbe{})
		in.RuleVersions = nil
		if _, err := ruleCandidateVersion(context.Background(), in); err == nil || !strings.Contains(err.Error(), "no_expected_version") {
			t.Fatalf("err %v", err)
		}
	})
}

func TestEarningsCoverage(t *testing.T) {
	t.Parallel()
	const d1, d2 = contracts.Day("2026-11-24"), dayD
	ec := func(m string, d contracts.Day, names, listed, errs int) finding.EarningsCheck {
		return finding.EarningsCheck{Market: m, Day: d, Names: names, Listed: listed, Errors: errs}
	}
	cases := []struct {
		name string
		rows []finding.EarningsCheck
		want []finding.Finding
	}{
		{"listed", []finding.EarningsCheck{ec("US", dayD, 35, 4, 0)}, nil},
		{"all errors", []finding.EarningsCheck{ec("US", dayD, 35, 0, 35)}, []finding.Finding{{
			Rule: finding.RuleEarningsCoverage, Dataset: "obs.earnings_date", Day: dayD, Severity: finding.SeverityWarn,
			Evidence: []finding.Count{{Name: "names", N: 35}, {Name: "listed"}, {Name: "errors", N: 35}}}}},
		{"collapse", []finding.EarningsCheck{ec("US", d1, 35, 4, 0), ec("US", d2, 35, 0, 0)}, []finding.Finding{{
			Rule: finding.RuleEarningsCoverage, Dataset: "obs.earnings_date", Day: d2, Severity: finding.SeverityWarn,
			Evidence: []finding.Count{{Name: "names", N: 35}, {Name: "listed"}, {Name: "listed_before", N: 4}}}}},
		{"jp listed", []finding.EarningsCheck{ec("JP", dayD, 7, 1, 0)}, nil},
		{"never listed", []finding.EarningsCheck{ec("US", d1, 35, 0, 0), ec("US", d2, 35, 0, 0)}, nil},
		{"outside window ignored", []finding.EarningsCheck{ec("US", "2026-11-26", 35, 0, 35)}, nil},
		{"markets apart", []finding.EarningsCheck{ec("US", d1, 35, 4, 0), ec("JP", d2, 7, 0, 0)}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := ruleEarningsCoverage(context.Background(), moreInput(&moreProbe{earnings: c.rows}))
			if err != nil || !reflect.DeepEqual(got, orEmpty(c.want)) {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
	t.Run("holiday is not a previous day", func(t *testing.T) {
		t.Parallel()
		// 2026-11-26 is a US holiday: absent from the XNYS window; its check rows (if any) are not compared.
		in := moreInput(&moreProbe{earnings: []finding.EarningsCheck{ec("US", d1, 35, 4, 0), ec("US", "2026-11-26", 35, 0, 0), ec("US", "2026-11-27", 35, 4, 0)}})
		in.Windows["XNYS"] = []contracts.Day{d1, "2026-11-27"}
		got, err := ruleEarningsCoverage(context.Background(), in)
		if err != nil || len(got) != 0 {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
}

func orEmpty(f []finding.Finding) []finding.Finding {
	if f == nil {
		return []finding.Finding{}
	}
	return f
}

func TestBarShape(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		day  contracts.Day
		bad  int
		want int
	}{
		{"clean", dayD, 0, 0},
		{"bad bars", dayD, 3, 1},
		{"outside windows", "2026-01-05", 3, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := &moreProbe{bars: []finding.BarShape{{Day: c.day, Rows: 5000, Bad: c.bad}}}
			got, err := ruleBarShape(context.Background(), moreInput(p))
			if err != nil || len(got) != c.want {
				t.Fatalf("got %+v, %v", got, err)
			}
			if c.want == 1 {
				w := finding.Finding{Rule: finding.RuleBarOHLCShape, Dataset: "obs.price_bar", Day: dayD, Severity: finding.SeverityError,
					Evidence: []finding.Count{{Name: "rows", N: 5000}, {Name: "bad", N: 3}}}
				if !reflect.DeepEqual(got[0], w) {
					t.Fatalf("got %+v", got[0])
				}
			}
		})
	}
}

func TestMoreRulesReturnProbeError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		what string
		rule func(context.Context, Input) ([]finding.Finding, error)
	}{
		{"runs", ruleCandidateVersion},
		{"earnings", ruleEarningsCoverage},
		{"bars", ruleBarShape},
	}
	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			t.Parallel()
			if _, err := c.rule(context.Background(), moreInput(&moreProbe{failWhat: c.what})); err != errProbe {
				t.Fatalf("err %v", err)
			}
		})
	}
}

func TestJobRunsNineRules(t *testing.T) {
	t.Parallel()
	out := &fakeOut{}
	j, err := New(Stores{Cov: &fakeCov{}, Probe: &moreProbe{}, Out: out, RuleVersions: map[string]string{datasetCandidate: "put_v2"}},
		clockwork.NewFakeClockAt(time.Date(2026, 11, 26, 23, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), checkDay, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(out.inserted) != 1 || out.inserted[0].RulesTotal != 9 {
		t.Fatalf("inserted %+v", out.inserted)
	}
}
