package contracts_test

import (
	"strings"
	"testing"
)

const priceLinesPageV1 = "page.price_lines_page.v1.json"

var priceLinesPatterns = map[string]string{
	"^[0-9]{1,12}$":  "1",
	"^[0-9a-f]{64}$": strings.Repeat("a", 64),
}

func priceLinesMinimal(t *testing.T) map[string]any {
	t.Helper()
	return pageV1Minimal(t, priceLinesPageV1, priceLinesPatterns)
}

func TestPriceLinesPageMinimalValid(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, priceLinesPageV1, priceLinesMinimal(t)) {
		t.Fatal("minimal instance is invalid")
	}
}

func TestPriceLinesPageVersion2Fails(t *testing.T) {
	t.Parallel()
	inst := priceLinesMinimal(t)
	inst["schema_version"] = 2
	if pageV1Valid(t, priceLinesPageV1, inst) {
		t.Fatal("version 2 validates")
	}
}

func TestPriceLinesPagePostPathIsConstant(t *testing.T) {
	t.Parallel()
	inst := priceLinesMinimal(t)
	inst["form"].(map[string]any)["post_path"] = "/owner-settings"
	if pageV1Valid(t, priceLinesPageV1, inst) {
		t.Fatal("another post_path validates")
	}
}

func TestPriceLinesPageMacIs64LowerHex(t *testing.T) {
	t.Parallel()
	for name, mac := range map[string]string{
		"short":    strings.Repeat("a", 63),
		"upper":    strings.Repeat("A", 64),
		"not_hex":  strings.Repeat("g", 64),
		"too_long": strings.Repeat("a", 65),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			inst := priceLinesMinimal(t)
			inst["form"].(map[string]any)["mac"] = mac
			if pageV1Valid(t, priceLinesPageV1, inst) {
				t.Fatalf("mac %q validates", mac)
			}
		})
	}
}

func TestPriceLinesPageMarketEnum(t *testing.T) {
	t.Parallel()
	for market, want := range map[string]bool{"US": true, "JP": true, "EU": false} {
		t.Run(market, func(t *testing.T) {
			t.Parallel()
			inst := priceLinesMinimal(t)
			inst["rows"] = []any{map[string]any{"symbol": "AMD", "market": market, "price": "130.00", "note": nil}}
			if got := pageV1Valid(t, priceLinesPageV1, inst); got != want {
				t.Fatalf("market %s valid = %v, want %v", market, got, want)
			}
		})
	}
}
