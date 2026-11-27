package datacheck

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

type wrongProbe struct {
	finding.Probe
	repeats  []finding.ReadingRepeat
	late     []finding.LateQuotes
	runs     []finding.CandidateRun
	changed  time.Time
	hasSet   bool
	failWhat string
}

var errProbe = errors.New("probe down")

func (p *wrongProbe) fail(what string) error {
	if p.failWhat == what {
		return errProbe
	}
	return nil
}
func (p *wrongProbe) RepeatedReadings(context.Context, []string, contracts.Day, contracts.Day) ([]finding.ReadingRepeat, error) {
	return p.repeats, p.fail("repeat")
}
func (p *wrongProbe) LateOpenQuotes(context.Context, contracts.Day, contracts.Day) ([]finding.LateQuotes, error) {
	return p.late, p.fail("late")
}
func (p *wrongProbe) NewestCandidateRuns(context.Context, contracts.Day, contracts.Day) ([]finding.CandidateRun, error) {
	return p.runs, p.fail("runs")
}
func (p *wrongProbe) SettingsChangedAt(context.Context) (time.Time, bool, error) {
	return p.changed, p.hasSet, p.fail("settings")
}

const dayD = contracts.Day("2026-11-25")

func wrongInput(p finding.Probe) Input {
	w := []contracts.Day{"2026-11-24", dayD}
	return Input{CheckDay: checkDay, Windows: map[string][]contracts.Day{"XNYS": w, "XTKS": w}, Probe: p}
}

func TestDefaultRulesOrder(t *testing.T) {
	t.Parallel()
	var ids []string
	for _, r := range defaultRules() {
		ids = append(ids, r.ID)
	}
	want := []string{"reading_repeat", "option_open_late_quote", "candidates_older_than_settings"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v", ids)
	}
}

func TestReadingRepeat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		ok, same int
		want     bool
	}{
		{"all same", 35, 35, true},
		{"below five subjects", 4, 4, false},
		{"few repeat", 40, 5, false},
		{"half repeat", 10, 5, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := &wrongProbe{repeats: []finding.ReadingRepeat{{Name: "market.rv20", Day: dayD, OK: c.ok, Same: c.same}}}
			got, err := ruleReadingRepeat(context.Background(), wrongInput(p))
			if err != nil {
				t.Fatal(err)
			}
			if !c.want {
				if len(got) != 0 {
					t.Fatalf("got %v", got)
				}
				return
			}
			want := finding.Finding{
				Rule: "reading_repeat", Dataset: "market.rv20", Day: dayD, Severity: finding.SeverityError,
				Evidence: []finding.Count{{Name: "ok", N: c.ok}, {Name: "same", N: c.same}},
			}
			if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestOpenLateQuote(t *testing.T) {
	t.Parallel()
	in := wrongInput(&wrongProbe{late: []finding.LateQuotes{{Day: dayD, Currency: "USD", Rows: 5548, Late: 0}}})
	got, err := ruleOpenLateQuote(context.Background(), in)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
	in = wrongInput(&wrongProbe{late: []finding.LateQuotes{
		{Day: dayD, Currency: "USD", Rows: 5548, Late: 0},
		{Day: dayD, Currency: "JPY", Rows: 183, Late: 183},
	}})
	got, err = ruleOpenLateQuote(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	want := finding.Finding{
		Rule: "option_open_late_quote", Dataset: "obs.option_open", Day: dayD, Severity: finding.SeverityError,
		Evidence: []finding.Count{{Name: "rows", N: 183}, {Name: "late", N: 183}},
	}
	if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Fatalf("got %+v", got)
	}
	in = wrongInput(&wrongProbe{late: []finding.LateQuotes{{Day: dayD, Currency: "JPY", Rows: 183, Late: 183}}})
	got, _ = ruleOpenLateQuote(context.Background(), in)
	if len(got) != 1 || got[0].Evidence[0].N != 183 || got[0].Evidence[1].N != 183 {
		t.Fatalf("got %+v", got)
	}
}

func TestCandidatesOlderThanSettings(t *testing.T) {
	t.Parallel()
	at := func(h, m int) time.Time { return time.Date(2026, 11, 25, h, m, 0, 0, time.UTC) }
	run := func(day contracts.Day, h int) finding.CandidateRun {
		return finding.CandidateRun{Day: day, Kind: "put", Rows: 12, ComputedAt: at(h, 0)}
	}
	cases := []struct {
		name    string
		runs    []finding.CandidateRun
		changed time.Time
		has     bool
		want    bool
	}{
		{"computed before change", []finding.CandidateRun{run(dayD, 9)}, at(11, 30), true, true},
		{"computed after change", []finding.CandidateRun{run(dayD, 12)}, at(11, 30), true, false},
		{"no setting", []finding.CandidateRun{run(dayD, 9)}, at(11, 30), false, false},
		{"older day only stale", []finding.CandidateRun{run(dayD, 12), run("2026-11-24", 9)}, at(11, 30), true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := &wrongProbe{runs: c.runs, changed: c.changed, hasSet: c.has}
			got, err := ruleCandidatesStale(context.Background(), wrongInput(p))
			if err != nil {
				t.Fatal(err)
			}
			if !c.want {
				if len(got) != 0 {
					t.Fatalf("got %v", got)
				}
				return
			}
			want := finding.Finding{
				Rule: "candidates_older_than_settings", Dataset: "candidates.option_candidate", Day: dayD,
				Severity: finding.SeverityWarn,
				Evidence: []finding.Count{{Name: "rows", N: 12}, {Name: "hours_behind", N: 2}},
			}
			if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestRuleReturnsProbeError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fail string
		run  func(context.Context, Input) ([]finding.Finding, error)
	}{
		{"reading_repeat", "repeat", ruleReadingRepeat},
		{"option_open_late_quote", "late", ruleOpenLateQuote},
		{"candidates runs", "runs", ruleCandidatesStale},
		{"candidates settings", "settings", ruleCandidatesStale},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := c.run(context.Background(), wrongInput(&wrongProbe{failWhat: c.fail}))
			if !errors.Is(err, errProbe) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
