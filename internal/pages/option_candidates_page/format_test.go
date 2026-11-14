package option_candidates_page

import "testing"

func TestFixedText(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in     string
		places int
		want   string
	}{
		{"0.5", 2, "0.50"}, {"-0.5594", 2, "-0.56"}, {"1.005", 2, "1.01"}, {"0.696", 3, "0.696"},
		{"-0.001", 2, "0.00"}, {"-1.005", 2, "-1.01"}, {"12", 1, "12.0"}, {"9.96", 1, "10.0"},
	} {
		got, ok := fixedText(c.in, c.places)
		if !ok || got != c.want {
			t.Errorf("fixedText(%q, %d) = %q, %v; want %q", c.in, c.places, got, ok, c.want)
		}
	}
	for _, bad := range []string{"abc", "1e-2", "", ".5", "1.", "0.1x", "1/2"} {
		if got, ok := fixedText(bad, 2); ok {
			t.Errorf("fixedText(%q) = %q, want ok false", bad, got)
		}
	}
}

func TestYieldCellUnreadable(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"abc", "1e-2"} {
		if g, ok := yieldCell(strp(bad)).(gap); !ok || g.Reason != "unreadable" {
			t.Errorf("yieldCell(%q) = %+v", bad, yieldCell(strp(bad)))
		}
	}
}
