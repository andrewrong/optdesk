package genpb

import "testing"

// TestBarKindOptionImpliedVol pins the numbers of the four named kinds of BarKind.
func TestBarKindOptionImpliedVol(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind BarKind
		want int32
	}{
		{BarKind_BAR_KIND_TRADES, 1},
		{BarKind_BAR_KIND_ADJUSTED_LAST, 2},
		{BarKind_BAR_KIND_MIDPOINT, 3},
		{BarKind_BAR_KIND_OPTION_IMPLIED_VOL, 4},
	}
	for _, c := range cases {
		if int32(c.kind) != c.want {
			t.Errorf("%s = %d, want %d", c.kind, int32(c.kind), c.want)
		}
	}
	if got := BarKind_BAR_KIND_OPTION_IMPLIED_VOL.String(); got != "BAR_KIND_OPTION_IMPLIED_VOL" {
		t.Errorf("name = %q", got)
	}
}
