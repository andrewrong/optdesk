package table_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/symbol"
	"github.com/andrewrong/optdesk/internal/symbol/contracttest"
	"github.com/andrewrong/optdesk/internal/symbol/table"
)

func TestTableContract(t *testing.T) {
	t.Parallel()
	contracttest.Run(t, table.New)
}

func TestToVendorAlphaVantage(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"BRK B":   "BRK-B",
		"BRK.B":   "BRK-B",
		"BRK-B":   "BRK-B",
		" bf  b ": "BF-B",
		"AAPL":    "AAPL",
		"278A":    "278A",
	}
	for in, want := range cases {
		got, err := table.New().ToVendor(in, symbol.AlphaVantage)
		if err != nil || got != want {
			t.Errorf("ToVendor(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", " ", " . - "} {
		if got, err := table.New().ToVendor(in, symbol.AlphaVantage); err == nil {
			t.Errorf("ToVendor(%q) = %q, want an error", in, got)
		}
	}
	if _, err := table.New().ToVendor("BRK B", symbol.Vendor("other")); err == nil {
		t.Error("unknown vendor: want an error")
	}
}

func TestOSIRoot(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"BRK B":   "BRKB",
		"BRK.B":   "BRKB",
		"BRK-B":   "BRKB",
		"BF B":    "BFB",
		" spy ":   "SPY",
		"ABCDEFG": "ABCDEFG",
	}
	for in, want := range cases {
		if got := table.New().OSIRoot(in); got != want {
			t.Errorf("OSIRoot(%q) = %q, want %q", in, got, want)
		}
	}
}
