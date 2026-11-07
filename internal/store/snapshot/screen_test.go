package snapshot

import (
	"math"
	"testing"
)

func fp(v float64) *float64 { return &v }

func TestScreenSentinel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   *float64
		want *float64
	}{
		{"minus one is no value", fp(-1), nil},
		{"NaN", fp(math.NaN()), nil},
		{"+Inf", fp(math.Inf(1)), nil},
		{"-Inf", fp(math.Inf(-1)), nil},
		{"zero is kept", fp(0), fp(0)},
		{"-0.5 is kept", fp(-0.5), fp(-0.5)},
		{"0.25 is kept", fp(0.25), fp(0.25)},
		{"nil stays nil", nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := screenSentinel(c.in)
			if (got == nil) != (c.want == nil) {
				t.Fatalf("screenSentinel = %v, want %v", got, c.want)
			}
			if got != nil && *got != *c.want {
				t.Fatalf("screenSentinel = %v, want %v", *got, *c.want)
			}
		})
	}
}
