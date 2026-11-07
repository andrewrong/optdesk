package instrument

import "testing"

func TestParseOSI(t *testing.T) {
	t.Parallel()
	good := []struct{ name, osi, right, strike, expiry string }{
		{"call", "SPY   261120C00450000", "C", "450", "2026-11-20"},
		{"put", "SPY   261120P00450000", "P", "450", "2026-11-20"},
		{"padded symbol", "XY    261218C00007000", "C", "7", "2026-12-18"},
		{"six character symbol", "ABCDEF261120C00001000", "C", "1", "2026-11-20"},
		{"fraction", "SPY   261120C00012500", "C", "12.5", "2026-11-20"},
		{"three decimals", "SPY   261120P00012125", "P", "12.125", "2026-11-20"},
		{"largest strike", "SPY   261120C99999999", "C", "99999.999", "2026-11-20"},
		{"zero strike", "SPY   261120C00000000", "C", "0", "2026-11-20"},
		{"leap day", "SPY   280229C00100000", "C", "100", "2028-02-29"},
	}
	for _, c := range good {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r, s, e, err := parseOSI(c.osi)
			if err != nil || r != c.right || s != c.strike || e != c.expiry {
				t.Fatalf("got %q %q %q %v, want %q %q %q", r, s, e, err, c.right, c.strike, c.expiry)
			}
		})
	}
	bad := map[string]string{
		"empty":         "",
		"too short":     "SPY   261120C0045000",
		"too long":      "SPY   261120C004500000",
		"right":         "SPY   261120X00450000",
		"lower right":   "SPY   261120c00450000",
		"month":         "SPY   261320C00450000",
		"day":           "SPY   260230C00450000",
		"date letters":  "SPY   26AB20C00450000",
		"strike letter": "SPY   261120C0045000A",
		"strike sign":   "SPY   261120C-0450000",
	}
	for name, osi := range bad {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, _, _, err := parseOSI(osi); err == nil {
				t.Fatalf("%q parsed", osi)
			}
		})
	}
}
