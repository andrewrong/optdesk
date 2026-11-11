package home_page

import "testing"

func TestGapLineWording(t *testing.T) {
	t.Parallel()
	for n, want := range map[int]string{0: "No gaps today", 1: "1 gap, see below", 3: "3 gaps, see below"} {
		if got := gapLine(n); got != want {
			t.Errorf("gapLine(%d) = %q, want %q", n, got, want)
		}
	}
}
