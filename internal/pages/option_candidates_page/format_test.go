package option_candidates_page

import "testing"

func TestYieldTextShift(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"0.123456": "12.3456%", "0.050000": "5.0000%", "1.500000": "150.0000%", "0.000000": "0.0000%",
		"-0.012300": "-1.2300%", "0.5": "50%", "0.05": "5%", "12.34": "1234%",
	} {
		got, err := shiftPoint(in)
		if err != nil || got != want {
			t.Errorf("shiftPoint(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"abc", "1e-2", "", "12", ".5", "1.", "0.1x"} {
		if got, err := shiftPoint(bad); err == nil {
			t.Errorf("shiftPoint(%q) = %q, want an error", bad, got)
		}
	}
	// a bad text is an error cell and not a panic
	for _, bad := range []string{"abc", "1e-2"} {
		if g, ok := yieldCell(strp(bad)).(gap); !ok || g.Reason != "unreadable" {
			t.Errorf("yieldCell(%q) = %+v", bad, yieldCell(strp(bad)))
		}
	}
}
