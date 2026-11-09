package optioncand

import "testing"

func TestParseDecimal(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", ".", "-", "1.2.3", "1e3", "1/2", "abc", "1 "} {
		if _, ok := parseDec(s); ok {
			t.Errorf("%q parsed", s)
		}
	}
	for s, want := range map[string]string{"1.99": "199/100", "0.1": "1/10", "-2": "-2/1", "4.20": "21/5"} {
		r, ok := parseDec(s)
		if !ok || r.String() != want {
			t.Errorf("%q = %v %v, want %s", s, r, ok, want)
		}
	}
}

func TestFormatFixed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in     string
		places int
		want   string
	}{
		{"0.2433333333", 6, "0.243333"},
		{"0.0000005", 6, "0.000000"}, // half to even: 0 is even
		{"0.0000015", 6, "0.000002"}, // 1 is odd
		{"0.0000025", 6, "0.000002"},
		{"98", 6, "98.000000"},
		{"-0.5", 0, "0"},
		{"-1.25", 1, "-1.2"},
		{"12.5", 0, "12"},
	}
	for _, tc := range cases {
		r, _ := parseDec(tc.in)
		if got := formatFixed(r, tc.places); got != tc.want {
			t.Errorf("formatFixed(%s, %d) = %s, want %s", tc.in, tc.places, got, tc.want)
		}
	}
	r, _ := parseDec("98.500000")
	if got := formatTrim(r); got != "98.5" {
		t.Errorf("trim %s", got)
	}
	r, _ = parseDec("98")
	if got := formatTrim(r); got != "98" {
		t.Errorf("trim %s", got)
	}
}

func TestSessionAge(t *testing.T) {
	t.Parallel()
	days := weekdays()
	if age, ok := sessionAge(days, "2026-11-06", runDay); !ok || age != 1 {
		t.Errorf("friday to monday = %d %v", age, ok)
	}
	if _, ok := sessionAge(days, "2020-01-01", runDay); ok {
		t.Errorf("a day before the list has an age")
	}
}
