package datacheck

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

type gapCov struct {
	coverage.Reader
	cells []coverage.Cell
	err   error
}

func (c *gapCov) Cells(context.Context, contracts.Day, contracts.Day) ([]coverage.Cell, error) {
	return c.cells, c.err
}

type gapProbe struct {
	finding.Probe
	gaps []finding.OpenGap
	err  error
}

func (p *gapProbe) OpenGaps(context.Context) ([]finding.OpenGap, error) { return p.gaps, p.err }

func gapInput(cov coverage.Reader, p finding.Probe) Input {
	return Input{
		CheckDay: checkDay,
		Now:      time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC),
		Windows: map[string][]contracts.Day{
			"XNYS": {"2026-11-23", "2026-11-24", "2026-11-25"}, // 2026-11-26 is a US holiday (Thanksgiving) and is not here
			"XTKS": {"2026-11-24", "2026-11-25"},
		},
		Cov: cov, Probe: p,
	}
}

func TestCaptureGap(t *testing.T) {
	t.Parallel()
	cells := []coverage.Cell{
		{Dataset: "obs.price_bar", Day: "2026-11-25", Keys: 63, Missing: 60, Expired: 3},
		{Dataset: "obs.price_bar", Day: "2026-11-24", Keys: 63, OK: 63},
		{Dataset: "obs.price_bar", Day: checkDay, Keys: 63, Missing: 63},
		{Dataset: "datacheck.run", Day: "2026-11-25", Keys: 1, Missing: 1},
	}
	got, err := ruleCaptureGap(context.Background(), gapInput(&gapCov{cells: cells}, nil))
	if err != nil {
		t.Fatal(err)
	}
	want := []finding.Finding{{
		Rule: finding.RuleCaptureGap, Dataset: "obs.price_bar", Day: "2026-11-25", Severity: finding.SeverityWarn,
		Evidence: []finding.Count{{Name: "keys", N: 63}, {Name: "missing", N: 60}, {Name: "expired", N: 3}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestPlanAbsent(t *testing.T) {
	t.Parallel()
	in := gapInput(&gapCov{cells: []coverage.Cell{
		{Dataset: "obs.option_eod", Day: "2026-11-23", Keys: 1, OK: 1},
		{Dataset: "obs.option_eod", Day: "2026-11-25", Keys: 1, OK: 1},
		{Dataset: "jp.bar", Day: "2026-11-24", Keys: 1, OK: 1},
		{Dataset: "jp.bar", Day: "2026-11-25", Keys: 1, OK: 1},
		{Dataset: "us.full", Day: "2026-11-23", Keys: 1, OK: 1},
		{Dataset: "us.full", Day: "2026-11-24", Keys: 1, OK: 1},
		{Dataset: "us.full", Day: "2026-11-25", Keys: 1, OK: 1},
	}}, nil)
	in.Expected = []ExpectedDataset{{"obs.option_eod", "XNYS"}, {"jp.bar", "XTKS"}, {"us.full", "XNYS"}}
	got, err := rulePlanAbsent(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	// jp.bar has no cell on 2026-11-23, a day outside the XTKS window; the holiday 2026-11-26 is not asked.
	want := []finding.Finding{{
		Rule: finding.RulePlanAbsent, Dataset: "obs.option_eod", Day: "2026-11-24", Severity: finding.SeverityWarn,
		Evidence: []finding.Count{{Name: "cells", N: 0}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestGapUnclosed(t *testing.T) {
	t.Parallel()
	now := gapInput(nil, nil).Now
	gaps := []finding.OpenGap{
		{JobID: "bars_refresh", Dataset: "obs.price_bar", Day: "2026-11-25", FirstSeenAt: now.Add(-30 * time.Hour)},
		{JobID: "calendar_refresh", Day: "2026-11-24", FirstSeenAt: now.Add(-30 * time.Hour)},
		{JobID: "bars_refresh", Dataset: "obs.price_bar", Day: "2026-11-24", FirstSeenAt: now.Add(-20 * time.Hour)},
		{JobID: "bars_refresh", Dataset: "obs.price_bar", Day: "2026-10-01", FirstSeenAt: now.Add(-900 * time.Hour)},
	}
	got, err := ruleGapUnclosed(context.Background(), gapInput(nil, &gapProbe{gaps: gaps}))
	if err != nil {
		t.Fatal(err)
	}
	age := []finding.Count{{Name: "age_hours", N: 30}}
	want := []finding.Finding{
		{Rule: finding.RuleGapUnclosed, Dataset: "obs.price_bar", Day: "2026-11-25", Severity: finding.SeverityWarn, Evidence: age},
		{Rule: finding.RuleGapUnclosed, Dataset: "calendar_refresh", Day: "2026-11-24", Severity: finding.SeverityWarn, Evidence: age},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestRuleGapReturnsProbeError(t *testing.T) {
	t.Parallel()
	boom := errors.New("down")
	in := func() Input {
		i := gapInput(&gapCov{err: boom}, &gapProbe{err: boom})
		i.Expected = []ExpectedDataset{{"obs.option_eod", "XNYS"}}
		return i
	}
	cases := map[string]func(context.Context, Input) ([]finding.Finding, error){
		"capture_gap": ruleCaptureGap, "plan_absent": rulePlanAbsent, "gap_unclosed": ruleGapUnclosed,
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := run(context.Background(), in()); !errors.Is(err, boom) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
