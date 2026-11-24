package home_page

import "testing"

func TestGapLineWording(t *testing.T) {
	t.Parallel()
	for n, want := range map[int]string{0: "今天没有缺口", 1: "1 个缺口，在各自的卡片里显示", 3: "3 个缺口，在各自的卡片里显示"} {
		if got := gapLine(n); got != want {
			t.Errorf("gapLine(%d) = %q, want %q", n, got, want)
		}
	}
}
