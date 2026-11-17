package optionevidenceapi_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
)

// fakeEvaluator answers made-up numbers; a strike or bid that is not above 0 is bad input.
type fakeEvaluator struct{}

func (fakeEvaluator) Evaluate(r optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
	if r.Strike == "0" || r.Bid == "" {
		return optionevidenceapi.Evidence{}, fmt.Errorf("evaluate: %w", optionevidenceapi.ErrBadInput)
	}
	return optionevidenceapi.Evidence{
		BreakEven: "89.06", BelowBuy: "0.94", BelowBuyPct: "1.0444",
		OneSigma: "8.7744", OneSigmaPct: "9.0926", BreakEvenInSigmas: "0.8479",
		Fair: "", EdgePerShare: "", EdgeWhy: "earnings_before_expiry",
		Skew: "0.012", ForecastErr: "0.05",
		Labels: []string{"delta_outside_band"},
	}, nil
}

// TestEvaluatorContractFake: a caller reads the figures of a made-up AAA row from a fake Evaluator.
func TestEvaluatorContractFake(t *testing.T) {
	t.Parallel()
	var e optionevidenceapi.Evaluator = fakeEvaluator{}
	got, err := e.Evaluate(optionevidenceapi.Row{
		ChainDay: "2026-11-17", Expiry: "2026-12-18", DTE: 31,
		Spot: "96.50", Strike: "90.00", Bid: "0.94", BuyPrice: "90.00", IV: "0.312",
		EarningsBeforeExpiry: true,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got.BelowBuy != "0.94" || got.OneSigma != "8.7744" || got.BreakEvenInSigmas != "0.8479" {
		t.Errorf("figures: %+v", got)
	}
	if got.EdgePerShare != "" || got.EdgeWhy != "earnings_before_expiry" {
		t.Errorf("an empty edge has its reason: %q %q", got.EdgePerShare, got.EdgeWhy)
	}
	if got.Skew != "0.012" {
		t.Errorf("Skew = %q", got.Skew)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "delta_outside_band" {
		t.Errorf("Labels = %v", got.Labels)
	}
}

// fixedValuer prices every option at one made-up fair price.
type fixedValuer struct{ fair string }

func (f fixedValuer) Value(right, spot, strike, years, rate, vol string) (string, error) {
	return f.fair, nil
}

// TestFakeValuerSatisfiesValuer: a fixed-price fake is a Valuer, whose shape is the Value method of
// stress.Pricer.
func TestFakeValuerSatisfiesValuer(t *testing.T) {
	t.Parallel()
	var v optionevidenceapi.Valuer = fixedValuer{fair: "0.74"}
	got, err := v.Value("P", "96.50", "90.00", "0.0849", "0.04", "0.25")
	if err != nil || got != "0.74" {
		t.Fatalf("Value = %q, %v", got, err)
	}
	// The same method set as stress.Pricer's Value, without importing the strategy module.
	var _ interface {
		Value(right, spot, strike, years, rate, vol string) (string, error)
	} = v
}

// TestErrBadInputIsSentinel: a wrapped ErrBadInput is found with errors.Is, and it is not another error.
func TestErrBadInputIsSentinel(t *testing.T) {
	t.Parallel()
	_, err := fakeEvaluator{}.Evaluate(optionevidenceapi.Row{Strike: "0", Bid: "0.94"})
	if !errors.Is(err, optionevidenceapi.ErrBadInput) {
		t.Fatalf("err = %v, want ErrBadInput", err)
	}
	if errors.Is(errors.New("optionevidenceapi: bad input"), optionevidenceapi.ErrBadInput) {
		t.Error("a look-alike error must not match the sentinel")
	}
}

// TestEmptyLabelsIsNonNil: an answer with no label carries an empty non-nil slice, so a caller can range
// and encode it without a nil check.
func TestEmptyLabelsIsNonNil(t *testing.T) {
	t.Parallel()
	var e optionevidenceapi.Evaluator = emptyLabels{}
	got, err := e.Evaluate(optionevidenceapi.Row{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Labels == nil || len(got.Labels) != 0 {
		t.Errorf("Labels = %#v, want empty non-nil", got.Labels)
	}
}

type emptyLabels struct{}

func (emptyLabels) Evaluate(optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
	return optionevidenceapi.Evidence{Labels: []string{}}, nil
}
