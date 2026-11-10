// Package contracttest holds the contract suite of symbol.Translator. An implementation runs it from its
// own tests.
package contracttest

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/symbol"
)

// Run checks newT against the promises of docs/contracts/symbol.md.
func Run(t *testing.T, newT symbol.NewTranslatorFunc) {
	t.Helper()

	toVendor := []struct {
		name, in, want string
	}{
		{"space", "BRK B", "BRK-B"},
		{"dash", "BRK-B", "BRK-B"},
		{"dot", "BRK.B", "BRK-B"},
		{"lower", "brk b", "BRK-B"},
		{"padded", " BRK B ", "BRK-B"},
		{"run of separators", "BRK  . B", "BRK-B"},
		{"second class", "BF B", "BF-B"},
		{"plain", "AAPL", "AAPL"},
		{"plain lower", "aapl", "AAPL"},
		{"digits and letter", "278A", "278A"},
	}
	for _, tc := range toVendor {
		t.Run("ToVendor/"+tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := newT().ToVendor(tc.in, symbol.AlphaVantage)
			if err != nil {
				t.Fatalf("ToVendor(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ToVendor(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	t.Run("ToVendor/empty", func(t *testing.T) {
		t.Parallel()
		for _, in := range []string{"", "   "} {
			if got, err := newT().ToVendor(in, symbol.AlphaVantage); err == nil {
				t.Errorf("ToVendor(%q) = %q, want an error", in, got)
			}
		}
	})
	t.Run("ToVendor/unknown vendor", func(t *testing.T) {
		t.Parallel()
		if got, err := newT().ToVendor("AAPL", symbol.Vendor("nope")); err == nil {
			t.Errorf("ToVendor with an unknown vendor = %q, want an error", got)
		}
	})

	osi := []struct {
		name, in, want string
	}{
		{"space", "BRK B", "BRKB"},
		{"second class", "BF B", "BFB"},
		{"plain", "SPY", "SPY"},
		{"messy", " brk  b ", "BRKB"},
		{"dot", "BRK.B", "BRKB"},
		{"long root unchecked", "ABCDEFGH", "ABCDEFGH"},
	}
	for _, tc := range osi {
		t.Run("OSIRoot/"+tc.name, func(t *testing.T) {
			t.Parallel()
			if got := newT().OSIRoot(tc.in); got != tc.want {
				t.Errorf("OSIRoot(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
