package ruleparam

import (
	"errors"
	"testing"
)

func TestNormalizeCanonicalText(t *testing.T) {
	t.Parallel()
	p := New()
	cases := []struct {
		key, in, want string
		refused       bool
	}{
		{"min_dte", "021", "21", false},
		{"max_spread", "0.10", "0.1", false},
		{"min_dte", "1e1", "", true},
		{"min_dte", " 7 ", "7", false},
		{"div_guard_factor", "1.0", "1", false},
		{"div_guard_factor", "2.500", "2.5", false},
		{"div_guard_factor", "03.0", "3", false},
		{"capture_target", "0.50", "0.5", false},
		{"min_dte", "+21", "", true},
		{"min_dte", "-21", "", true},
		{"min_dte", "21.0", "", true},
		{"min_dte", "2 1", "", true},
		{"max_spread", ".1", "", true},
		{"max_spread", "0.1.", "", true},
		{"max_spread", "0,1", "", true},
		{"max_spread", "1E-1", "", true},
		{"min_dte", "٢١", "", true},
	}
	for _, c := range cases {
		got, err := p.Normalize(c.key, c.in)
		if c.refused {
			if !errors.Is(err, ErrRefused) {
				t.Errorf("%s %q: err = %v, want refusal", c.key, c.in, err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s %q = %q, %v; want %q", c.key, c.in, got, err, c.want)
		}
	}
}

func TestNormalizeRefusesOutOfRange(t *testing.T) {
	t.Parallel()
	p := New()
	for _, row := range table {
		key, min, max := row[0], row[2], row[3]
		for _, ok := range []string{min, max} {
			if got, err := p.Normalize(key, ok); err != nil || got != ok {
				t.Errorf("%s %q = %q, %v; want accepted", key, ok, got, err)
			}
		}
	}
	bad := [][2]string{
		{"min_dte", "6"}, {"min_dte", "121"}, {"max_dte", "13"}, {"max_dte", "181"},
		{"min_bid_size", "0"}, {"min_bid_size", "101"},
		{"max_spread", "0.009"}, {"max_spread", "0.31"},
		{"min_open_interest", "0"}, {"min_open_interest", "1001"},
		{"div_guard_factor", "0.49"}, {"div_guard_factor", "3.01"},
		{"max_rate_age_days", "0"}, {"max_rate_age_days", "31"},
		{"max_event_age_sessions", "0"}, {"max_event_age_sessions", "11"},
		{"min_report_gap_days", "29"}, {"min_report_gap_days", "101"},
		{"max_strikes", "3"}, {"max_strikes", "61"},
		{"capture_target", "0.09"}, {"capture_target", "0.96"},
		{"exit_dte", "0"}, {"exit_dte", "46"},
		{"min_dte", "99999999999999999999999999"},
	}
	for _, b := range bad {
		if _, err := p.Normalize(b[0], b[1]); !errors.Is(err, ErrRefused) {
			t.Errorf("%s %q: err = %v, want refusal", b[0], b[1], err)
		}
	}
	// an int key refuses a fraction, even inside the range
	if _, err := p.Normalize("max_strikes", "24.5"); !errors.Is(err, ErrRefused) {
		t.Errorf("fraction: %v", err)
	}
}

func TestMaxDteNotBelowMinDte(t *testing.T) {
	t.Parallel()
	p := New()
	// alone, the text of max_dte is judged against its own bound only
	if _, err := p.Normalize("max_dte", "13"); !errors.Is(err, ErrRefused) {
		t.Errorf("max_dte 13 alone: %v", err)
	}
	if got, err := p.Normalize("max_dte", "14"); err != nil || got != "14" {
		t.Errorf("max_dte 14 alone: %q, %v", got, err)
	}
	cases := []struct {
		stored  map[string]string
		refused bool
	}{
		{map[string]string{"min_dte": "60", "max_dte": "45"}, true},
		{map[string]string{"min_dte": "60"}, true}, // max_dte default 45
		{map[string]string{"max_dte": "14"}, true}, // min_dte default 21
		{map[string]string{"min_dte": "45", "max_dte": "45"}, false},
		{map[string]string{"min_dte": "021", "max_dte": "45"}, false},
		{map[string]string{"min_dte": "30", "max_dte": ""}, false},
		{nil, false},
	}
	for _, c := range cases {
		err := CheckWindow(c.stored)
		if (err != nil) != c.refused || (err != nil && !errors.Is(err, ErrRefused)) {
			t.Errorf("CheckWindow(%v) = %v, refused want %v", c.stored, err, c.refused)
		}
	}
}
