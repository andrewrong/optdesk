package ivbar

import "testing"

func TestFromCloseRounds(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"0.25", "0.250000"},
		{"0.1234565", "0.123456"}, // half to even: tie, 6 is even
		{"0.1234575", "0.123458"}, // half to even: tie, 7 is odd
		{"0.1234566", "0.123457"},
		{"0.1234564", "0.123456"},
		{"10", "10.000000"},
		{"10.0000000", "10.000000"},
		{"3", "3.000000"},
		{"0.0000004", "0.000000"},
		{"00.5", "0.500000"},
	}
	for _, c := range cases {
		got, err := FromClose(c.in)
		if err != nil || got != c.want {
			t.Errorf("FromClose(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestFromCloseRejectsZeroAndAboveTen(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"0", "0.0", "0.000000", "10.000001", "10.0000001", "11", "100"} {
		if got, err := FromClose(in); err == nil {
			t.Errorf("FromClose(%q) = %q, want an error", in, got)
		}
	}
}

func TestFromCloseRejectsText(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", " ", "abc", "NaN", "Inf", "1e-1", "-0.1", "+0.1", "0.1.1", " 0.1", "0.1 ", "1/2", ".", "1.", ".5", "0x1"} {
		if got, err := FromClose(in); err == nil {
			t.Errorf("FromClose(%q) = %q, want an error", in, got)
		}
	}
}
