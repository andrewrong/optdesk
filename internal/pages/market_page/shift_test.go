package market_page

import "testing"

func TestShiftRound(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in             string
		shift, decimal int
		want           string
	}{
		{"0.685000", 2, 0, "68"},
		{"0.675000", 2, 0, "68"},
		{"0.125000", 2, 0, "12"},
		{"0.135000", 2, 0, "14"},
		{"0.684999", 2, 0, "68"},
		{"0.685001", 2, 0, "69"},
		{"0.000000", 2, 0, "0"},
		{"1.000000", 2, 0, "100"},
		{"0.123500", 2, 1, "12.4"},
		{"0.124500", 2, 1, "12.4"},
		{"0.031250", 2, 2, "3.12"},
		{"0.031350", 2, 2, "3.14"},
		{"-0.031250", 2, 2, "-3.12"},
		{"-0.000040", 2, 2, "0.00"},
		{"0.905000", 0, 2, "0.90"},
		{"0.915000", 0, 2, "0.92"},
		{"12", 0, 2, "12.00"},
	}
	for _, c := range cases {
		got, err := shiftRound(c.in, c.shift, c.decimal)
		if err != nil || got != c.want {
			t.Errorf("shiftRound(%q, %d, %d) = %q, %v; want %q", c.in, c.shift, c.decimal, got, err, c.want)
		}
	}
}

func TestShiftRoundRejectsNonDecimal(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"abc", "", "1e3", "0x1", " 1", "1.", ".5", "+1"} {
		if got, err := shiftRound(in, 2, 0); err == nil {
			t.Errorf("shiftRound(%q) = %q, want an error", in, got)
		}
	}
}

func TestShareRange(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]bool{"0": true, "0.000000": true, "1.000000": true, "0.5": true, "1.000001": false, "-0.1": false, "abc": false} {
		if got := inUnitRange(in); got != want {
			t.Errorf("inUnitRange(%q) = %v, want %v", in, got, want)
		}
	}
}
