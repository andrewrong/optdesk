package home_page

import (
	"fmt"
	"testing"
)

func TestGapLineWording(t *testing.T) {
	t.Parallel()
	for n, want := range map[int]string{0: gapLineNone, 1: gapLineOne, 3: fmt.Sprintf(gapLineMany, 3)} {
		if got := gapLine(n); got != want {
			t.Errorf("gapLine(%d) = %q, want %q", n, got, want)
		}
	}
}
