package structures_page

import "testing"

func TestPercentTextShift(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"0.123456": "12.3456%", "-0.0912": "-9.12%", "0.5": "50%", "1": "100%", "0.000100": "0.0100%",
	} {
		got, err := percentText(in)
		if err != nil || got != want {
			t.Errorf("percentText(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := percentText("abc"); err == nil {
		t.Error("abc gave no error")
	}
	if got := (cells{}).percent(figure("abc")); got != mkGap("missing_input") {
		t.Errorf("percent cell of abc = %v, want a gap", got)
	}
}

func TestShiftDayAndClip(t *testing.T) {
	t.Parallel()
	if d, err := shiftDay("2026-11-30", 1); err != nil || d != "2026-12-01" {
		t.Fatalf("shiftDay = %q, %v", d, err)
	}
	if _, err := shiftDay("nope", 1); err == nil {
		t.Fatal("bad day gave no error")
	}
	long := ""
	for range 300 {
		long += "字"
	}
	if got := []rune(clip(long)); len(got) != 200 {
		t.Fatalf("clip kept %d runes", len(got))
	}
}
