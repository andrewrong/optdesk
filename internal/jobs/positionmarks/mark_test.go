package positionmarks

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

func TestMarkOf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name            string
		q               broker.Quote
		wantPrice, want string
	}{
		{"two sided mid", broker.Quote{Bid: new(1.10), Ask: new(1.20), Last: new(float64(9)), Close: new(float64(8))}, "1.15", mark.BasisMid},
		{"mid keeps odd decimals", broker.Quote{Bid: new(0.01), Ask: new(0.02)}, "0.015", mark.BasisMid},
		{"locked quote is a mid", broker.Quote{Bid: new(float64(2)), Ask: new(float64(2))}, "2", mark.BasisMid},
		{"Crossed quote falls to last", broker.Quote{Bid: new(float64(2)), Ask: new(float64(1)), Last: new(1.5)}, "1.5", mark.BasisLast},
		{"zero bid falls to last", broker.Quote{Bid: new(float64(0)), Ask: new(float64(1)), Last: new(0.5)}, "0.5", mark.BasisLast},
		{"missing ask falls to close", broker.Quote{Bid: new(float64(1)), Close: new(float64(3))}, "3", mark.BasisClose},
		{"zero last falls to close", broker.Quote{Last: new(float64(0)), Close: new(float64(3))}, "3", mark.BasisClose},
		{"nothing is none", broker.Quote{}, "", mark.BasisNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p, b := MarkOf(c.q)
			if p != c.wantPrice || b != c.want {
				t.Fatalf("MarkOf = %q, %q; want %q, %q", p, b, c.wantPrice, c.want)
			}
		})
	}
}
