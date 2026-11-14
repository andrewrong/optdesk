package option_boards

import "testing"

func TestShiftPercent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
		bad      bool
	}{
		{in: "0.123456", want: "12.3456%"},
		{in: "0.38", want: "38%"},
		{in: "0.5", want: "50%"},
		{in: "0.05", want: "5%"},
		{in: "1", want: "100%"},
		{in: "-0.15", want: "-15%"},
		{in: "0.000", want: "0.0%"},
		{in: "-0.00", want: "0%"},
		{in: "abc", bad: true},
		{in: "1e-3", bad: true},
		{in: "", bad: true},
		{in: ".5", bad: true},
		{in: "1/2", bad: true},
	}
	for _, c := range cases {
		got, err := shiftPercent(c.in)
		if c.bad {
			if err == nil {
				t.Errorf("shiftPercent(%q) = %q, want an error", c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("shiftPercent(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestSignedText(t *testing.T) {
	t.Parallel()
	if got := signedText("-0.15"); got != "−0.15" {
		t.Errorf("got %q", got)
	}
	if got := signedText("0.15"); got != "0.15" {
		t.Errorf("got %q", got)
	}
}

func TestScaledInt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in    string
		scale int
		want  int64
		ok    bool
	}{
		{"-0.15", 2, -15, true},
		{"0.15", 2, 15, true},
		{"22", 0, 22, true},
		{"0.1", 2, 10, true},
		{"0.150", 2, 15, true},
		{"0.155", 2, 0, false},
		{"1e-3", 2, 0, false},
		{"abc", 0, 0, false},
	}
	for _, c := range cases {
		got, ok := scaledInt(c.in, c.scale)
		if got != c.want || ok != c.ok {
			t.Errorf("scaledInt(%q, %d) = %d, %v; want %d, %v", c.in, c.scale, got, ok, c.want, c.ok)
		}
	}
}

func TestCompareDecimal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b string
		want int
	}{
		{"9.5", "10", -1},
		{"10", "9.5", 1},
		{"62", "62.0", 0},
		{"62.50", "62.5", 0},
		{"-1", "1", -1},
		{"-2", "-10", 1},
		{"-0", "0", 0},
		{"007", "7", 0},
		{"0.25", "0.3", -1},
	}
	for _, c := range cases {
		got, err := compareDecimal(c.a, c.b)
		if err != nil || got != c.want {
			t.Errorf("compareDecimal(%q, %q) = %d, %v; want %d", c.a, c.b, got, err, c.want)
		}
	}
	if _, err := compareDecimal("abc", "1"); err == nil {
		t.Error("want an error for abc")
	}
}

func TestMonthDayAndWeekday(t *testing.T) {
	t.Parallel()
	md, err := monthDay("2026-11-13")
	if err != nil || md != "11-13" {
		t.Errorf("monthDay = %q, %v", md, err)
	}
	wd, err := weekdayText("2026-11-13")
	if err != nil || wd != "Fri" {
		t.Errorf("weekdayText = %q, %v", wd, err)
	}
	if wd, _ := weekdayText("2026-11-15"); wd != "Sun" {
		t.Errorf("sunday = %q", wd)
	}
	if _, err := monthDay("2026-13-01"); err == nil {
		t.Error("want an error")
	}
}

func TestDaysBack(t *testing.T) {
	t.Parallel()
	got, err := daysBack("2026-03-02", 3)
	if err != nil || got != "2026-02-27" {
		t.Errorf("daysBack = %q, %v", got, err)
	}
}

func TestRoundDecimal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
		bad      bool
	}{
		{in: "-0.31495717239425247", want: "-0.315"},
		{in: "0.12345", want: "0.1235"},
		{in: "-0.12345", want: "-0.1235"},
		{in: "-0.00001", want: "0"},
		{in: "0.99999999", want: "1"},
		{in: "-0.2446", want: "-0.2446"},
		{in: "-0.15", want: "-0.15"},
		{in: "-0.1", want: "-0.1"},
		{in: "0.12344", want: "0.1234"},
		{in: "abc", bad: true},
		{in: "1e-3", bad: true},
	}
	for _, tc := range cases {
		got, ok := roundDecimal(tc.in, 4)
		if ok == tc.bad || got != tc.want {
			t.Errorf("roundDecimal(%q, 4) = %q, %v; want %q, ok=%v", tc.in, got, ok, tc.want, !tc.bad)
		}
	}
}

func TestFixedDecimals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in     string
		places int
		want   string
		bad    bool
	}{
		{"0.5", 2, "0.50", false},
		{"-0.5594", 2, "-0.56", false},
		{"1.005", 2, "1.01", false},
		{"-0.001", 2, "0.00", false},
		{"9.996", 2, "10.00", false},
		{"65", 2, "65.00", false},
		{"144.4379", 1, "144.4", false},
		{"abc", 2, "", true},
		{"1e-3", 2, "", true},
	}
	for _, tc := range cases {
		got, ok := fixedDecimals(tc.in, tc.places)
		if ok == tc.bad || got != tc.want {
			t.Errorf("fixedDecimals(%q, %d) = %q, %v; want %q", tc.in, tc.places, got, ok, tc.want)
		}
	}
}
