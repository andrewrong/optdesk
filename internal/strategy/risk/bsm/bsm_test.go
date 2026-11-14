package bsm_test

import (
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/andrewrong/optdesk/internal/strategy/risk/bsm"
)

func mustValue(t *testing.T, right, spot, strike, years, rate, vol string) float64 {
	t.Helper()
	s, err := bsm.New().Value(right, spot, strike, years, rate, vol)
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestBsmNewPutCallParity(t *testing.T) {
	t.Parallel()
	// C - P = S - K e^{-rT}; literal: 100 - 95*0.9753099120 = 7.3455584
	c := mustValue(t, "C", "100", "95", "0.5", "0.05", "0.3")
	p := mustValue(t, "P", "100", "95", "0.5", "0.05", "0.3")
	if got := c - p; math.Abs(got-7.3455584) > 1e-6 {
		t.Fatalf("C-P = %v, want 7.3455584", got)
	}
}

func TestBsmNewImpliedRoundTripsToFourDecimals(t *testing.T) {
	t.Parallel()
	for _, right := range []string{"P", "C"} {
		for _, vol := range []string{"0.1500", "0.2000", "0.6543"} {
			price, err := bsm.New().Value(right, "100", "105", "0.25", "0.04", vol)
			if err != nil {
				t.Fatal(err)
			}
			got, err := bsm.New().Implied(right, "100", "105", "0.25", "0.04", price)
			if err != nil {
				t.Fatalf("%s %s: %v", right, vol, err)
			}
			g, _ := strconv.ParseFloat(got, 64)
			w, _ := strconv.ParseFloat(vol, 64)
			if math.Abs(g-w) > 5e-5 {
				t.Fatalf("%s: implied %s, want %s to four decimals", right, got, vol)
			}
		}
	}
}

func TestBsmNewPriceBelowIntrinsicAndExpiredAreErrors(t *testing.T) {
	t.Parallel()
	if _, err := bsm.New().Implied("P", "80", "100", "0.5", "0.02", "19.5"); !errors.Is(err, bsm.ErrBelowPayoff) {
		t.Fatalf("below intrinsic: %v", err)
	}
	if _, err := bsm.New().Implied("P", "80", "100", "0", "0.02", "25"); !errors.Is(err, bsm.ErrExpired) {
		t.Fatalf("expired: %v", err)
	}
	if _, err := bsm.New().Value("P", "80", "100", "-1", "0", "0.2"); !errors.Is(err, bsm.ErrExpired) {
		t.Fatalf("expired value: %v", err)
	}
}
