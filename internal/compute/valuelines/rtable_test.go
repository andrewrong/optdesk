package valuelines

import (
	"errors"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

func TestYieldOnEdgeTakesLowerBucket(t *testing.T) {
	t.Parallel()
	rows := []valuelinesapi.RRow{
		{YieldUpTo: "", US: "u3", JP: "j3", HKCN: "h3"},
		{YieldUpTo: "4.5", US: "u2", JP: "j2", HKCN: "h2"},
		{YieldUpTo: "4.0", US: "u1", JP: "j1", HKCN: "h1"},
	}
	for y, want := range map[string]string{"4.5": "u2", "4.0": "u1", "4.51": "u3", "3": "u1", "4.2": "u2"} {
		got, err := New().ProposeR(rows, y)
		if err != nil || got.US != want {
			t.Errorf("yield %s: %+v %v, want %s", y, got, err, want)
		}
	}
	got, _ := New().ProposeR(rows, "4.5")
	if got.JP != "j2" || got.HKCN != "h2" {
		t.Errorf("%+v", got)
	}
	if _, err := New().ProposeR(rows[1:], "9"); !errors.Is(err, valuelinesapi.ErrBadInput) {
		t.Errorf("no bucket: %v", err)
	}
	if _, err := New().ProposeR(rows, "x"); !errors.Is(err, valuelinesapi.ErrBadInput) {
		t.Errorf("bad yield: %v", err)
	}
}

func TestStepWarning(t *testing.T) {
	t.Parallel()
	prev := valuelinesapi.RValues{US: "0.90", JP: "0.80", HKCN: "0.70"}
	next := valuelinesapi.RValues{US: "0.96", JP: "0.85", HKCN: "0.64"}
	w := New().RStepWarnings(prev, next)
	if len(w) != 2 || !strings.Contains(w[0], "US") || !strings.Contains(w[1], "HKCN") {
		t.Errorf("%v", w)
	}
	if w := New().RStepWarnings(prev, prev); len(w) != 0 {
		t.Errorf("%v", w)
	}
}

func TestRoundTextHalfAwayFromZero(t *testing.T) {
	t.Parallel()
	in := "2.345"
	if got := New().RoundText(in, 2); got != "2.35" || in != "2.345" {
		t.Errorf("%s", got)
	}
	if got := New().RoundText("-2.345", 2); got != "-2.35" {
		t.Errorf("%s", got)
	}
	if got := New().RoundText("abc", 2); got != "abc" {
		t.Errorf("%s", got)
	}
}
