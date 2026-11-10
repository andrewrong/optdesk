package computefigures_test

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
)

var _ computefigures.NewComputeFiguresValuationFunc = computefigures.NewWithValuation

type apiFacts struct{ called bool }

func (a *apiFacts) FactsFiledBy(context.Context, string, string, contracts.Day) ([]contracts.FundamentalFact, error) {
	a.called = true
	return nil, nil
}

// TestValuationDepsContract: a caller fills every field of ValuationDeps, a constructor of the function type
// reads each of them back.
func TestValuationDepsContract(t *testing.T) {
	t.Parallel()
	facts := &apiFacts{}
	deps := computefigures.ValuationDeps{
		Facts:      facts,
		Entries:    []jobs.WatchEntry{{Issuer: "cik:0000320193", Contract: broker.Contract{Symbol: "AAPL"}}},
		StandardOf: func([]contracts.FundamentalFact) valuationapi.Standard { return valuationapi.USGAAP },
		BuildFilings: func([]contracts.FundamentalFact, contracts.Day, valuationapi.Standard) valuationapi.FilingsResult {
			return valuationapi.FilingsResult{State: valuationapi.StateOK}
		},
		Band: func(string, valuationapi.Filings, contracts.Day) valuationapi.BandFigure {
			return valuationapi.BandFigure{}
		},
		Questions: func(valuationapi.Filings, contracts.Day) valuationapi.QuestionsResult {
			return valuationapi.QuestionsResult{}
		},
		Diagnosis: func(valuationapi.Filings, contracts.Day) valuationapi.DiagnosisResult {
			return valuationapi.DiagnosisResult{}
		},
	}
	var newJob computefigures.NewComputeFiguresValuationFunc = func(
		_ computefigures.ComputeStores, _ compute.Figures, _ compute.Rules, _ compute.Units, _ compute.SeriesOps,
		d computefigures.ValuationDeps, _ clock.Clock,
	) (contracts.Job, error) {
		_, _ = d.Facts.FactsFiledBy(context.Background(), "sec", d.Entries[0].Issuer, "2026-11-06")
		if d.StandardOf(nil) != valuationapi.USGAAP || d.BuildFilings(nil, "", "").State != valuationapi.StateOK {
			t.Error("functions of the deps are not the ones passed")
		}
		d.Band("", valuationapi.Filings{}, "")
		d.Questions(valuationapi.Filings{}, "")
		d.Diagnosis(valuationapi.Filings{}, "")
		return nil, nil
	}
	if _, err := newJob(computefigures.ComputeStores{}, nil, nil, nil, nil, deps, nil); err != nil || !facts.called {
		t.Fatalf("err %v, facts called %v", err, facts.called)
	}
}
