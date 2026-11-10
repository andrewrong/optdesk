package option_boards

import "testing"

func sp(s string) *string { return &s }

func TestStrikePct(t *testing.T) {
	t.Parallel()
	cases := []struct {
		strike, spot, text string
		value              int64
	}{
		{"62", "67.5", "−8.1%", -815}, // -8.148..%
		{"100", "100", "0.0%", 0},
		{"110", "100", "10.0%", 1000},
		{"60", "62", "−3.2%", -323}, // -3.2258
	}
	for _, c := range cases {
		r := strikePct(c.strike, sp(c.spot))
		if !r.OK || r.Text != c.text || r.Value != c.value {
			t.Errorf("strikePct(%s, %s) = %+v; want %s, %d", c.strike, c.spot, r, c.text, c.value)
		}
	}
}

func TestSpreadPct(t *testing.T) {
	t.Parallel()
	// bid 0.62, ask 0.66: 0.04 / 0.64 = 6.25% -> 6.3% half away.
	r := spreadPct("0.62", sp("0.66"))
	if !r.OK || r.Text != "6.3%" || r.Value != 625 {
		t.Errorf("spreadPct = %+v", r)
	}
	r = spreadPct("1.00", sp("1.00"))
	if !r.OK || r.Text != "0.0%" || r.Value != 0 {
		t.Errorf("zero spread = %+v", r)
	}
}

func TestRatioMissingInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		got  ratioResult
		want string
	}{
		{"nil spot", strikePct("62", nil), "spot"},
		{"zero spot", strikePct("62", sp("0")), "spot"},
		{"bad spot", strikePct("62", sp("1e2")), "spot"},
		{"bad strike", strikePct("x", sp("10")), "strike"},
		{"nil ask", spreadPct("0.5", nil), "ask"},
		{"zero ask", spreadPct("0.5", sp("0.00")), "ask"},
		{"zero bid", spreadPct("0", sp("0.5")), "bid"},
		{"bad bid", spreadPct("abc", sp("0.5")), "bid"},
	}
	for _, c := range cases {
		if c.got.OK || c.got.Missing != c.want {
			t.Errorf("%s: %+v, want missing %q", c.name, c.got, c.want)
		}
	}
}

func TestRatioRoundsHalfAway(t *testing.T) {
	t.Parallel()
	// 100.05 / 100 - 1 = 0.05% exactly: one place 0.1% (half away), hundredths 5.
	r := strikePct("100.05", sp("100"))
	if r.Text != "0.1%" || r.Value != 5 {
		t.Errorf("positive half: %+v", r)
	}
	// 99.95 / 100 - 1 = -0.05%: -0.1%, not -0.0%.
	r = strikePct("99.95", sp("100"))
	if r.Text != "−0.1%" || r.Value != -5 {
		t.Errorf("negative half: %+v", r)
	}
	// -0.04%: rounds to zero, no minus sign.
	r = strikePct("99.96", sp("100"))
	if r.Text != "0.0%" || r.Value != -4 {
		t.Errorf("small negative: %+v", r)
	}
}
