package sharpmove_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/sharpmove"
)

var _ compute.SharpMarkFunc = sharpmove.Mark
var _ compute.SharpLimitsFunc = sharpmove.Limits

func ptr(v float64) *float64 { return &v }

func TestMark(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		day, five *float64
		wantDay   bool
		wantFive  bool
	}{
		{"day 2.99", ptr(2.99), nil, false, false},
		{"day 3.0", ptr(3.0), nil, true, false},
		{"day -3.0", ptr(-3.0), nil, true, false},
		{"day -2.99", ptr(-2.99), nil, false, false},
		{"five 7.99", nil, ptr(7.99), false, false},
		{"five 8.0", nil, ptr(8.0), false, true},
		{"five -8.0", nil, ptr(-8.0), false, true},
		{"five -7.99", nil, ptr(-7.99), false, false},
		{"both nil", nil, nil, false, false},
		{"independent: day sharp, five not", ptr(5), ptr(1), true, false},
		{"independent: five sharp, day not", ptr(1), ptr(9), false, true},
		{"day limit does not sharpen five", ptr(0), ptr(3), false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d, f := sharpmove.Mark(c.day, c.five)
			if d != c.wantDay || f != c.wantFive {
				t.Fatalf("Mark = (%v, %v), want (%v, %v)", d, f, c.wantDay, c.wantFive)
			}
		})
	}
}

func TestLimits(t *testing.T) {
	t.Parallel()
	d, f := sharpmove.Limits()
	if d != 3 || f != 8 {
		t.Fatalf("Limits = (%v, %v), want (3, 8)", d, f)
	}
}
