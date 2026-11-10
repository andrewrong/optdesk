package barsrefresh

import (
	"math/big"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestRestatedRule(t *testing.T) {
	t.Parallel()
	tol := big.NewRat(1, 100000)
	st := []contracts.PriceBar{stored(1, "2026-11-09", "100.0000", 1)}
	cases := []struct {
		name   string
		pulled []broker.Bar
		want   bool
	}{
		{"split", []broker.Bar{mkbar("2026-11-09", "50.0000")}, true},
		{"within tolerance", []broker.Bar{mkbar("2026-11-09", "100.0005")}, false},
		{"just above tolerance", []broker.Bar{mkbar("2026-11-09", "100.0011")}, true},
		{"below by more", []broker.Bar{mkbar("2026-11-09", "99.9980")}, true},
		{"day not stored", []broker.Bar{mkbar("2026-11-10", "1.0000")}, false},
	}
	for _, c := range cases {
		got, err := restated(st, c.pulled, tol)
		if err != nil || got != c.want {
			t.Errorf("%s: got %v, %v", c.name, got, err)
		}
	}
}

func TestRestatedStoredZero(t *testing.T) {
	t.Parallel()
	tol := big.NewRat(1, 100000)
	st := []contracts.PriceBar{stored(1, "2026-11-09", "0", 1)}
	if got, _ := restated(st, []broker.Bar{mkbar("2026-11-09", "0.0000")}, tol); got {
		t.Error("equal zero restated")
	}
	if got, _ := restated(st, []broker.Bar{mkbar("2026-11-09", "1")}, tol); !got {
		t.Error("zero to one not restated")
	}
}

func TestRestatedBadCloseIsError(t *testing.T) {
	t.Parallel()
	st := []contracts.PriceBar{stored(1, "2026-11-09", "100.0000", 1)}
	if _, err := restated(st, []broker.Bar{mkbar("2026-11-09", "x")}, big.NewRat(1, 100000)); err == nil {
		t.Error("no error")
	}
}
