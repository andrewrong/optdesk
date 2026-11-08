package contracts_test

import "testing"

const corePageV1 = "page.core_page.v1.json"

func coreMinimal(t *testing.T) map[string]any {
	t.Helper()
	return pageV1Minimal(t, corePageV1, nil)
}

func coreRow(returns any) map[string]any {
	return map[string]any{"symbol": "AMD", "price_text": "130", "cyclical": true, "flag": nil, "returns": returns}
}

func TestCorePageMinimalValid(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, corePageV1, coreMinimal(t)) {
		t.Fatal("minimal instance is invalid")
	}
}

func TestCorePageVersion2Fails(t *testing.T) {
	t.Parallel()
	inst := coreMinimal(t)
	inst["schema_version"] = 2
	if pageV1Valid(t, corePageV1, inst) {
		t.Fatal("version 2 validates")
	}
}

func TestCorePageReturnsAsGapValid(t *testing.T) {
	t.Parallel()
	inst := coreMinimal(t)
	inst["rows"] = []any{coreRow(pageV1Gap())}
	if !pageV1Valid(t, corePageV1, inst) {
		t.Fatal("returns as a Gap is invalid")
	}
}

func TestCorePageReturnsBounds(t *testing.T) {
	t.Parallel()
	point := map[string]any{"assumption": "a", "text": "t", "marker_percent": 50}
	for n, want := range map[int]bool{0: false, 1: true, 6: true, 7: false} {
		returns := make([]any, n)
		for i := range returns {
			returns[i] = point
		}
		inst := coreMinimal(t)
		inst["rows"] = []any{coreRow(returns)}
		if got := pageV1Valid(t, corePageV1, inst); got != want {
			t.Errorf("%d returns: valid = %v, want %v", n, got, want)
		}
	}
}

func TestCorePageMarkerPercentRange(t *testing.T) {
	t.Parallel()
	for marker, want := range map[int]bool{0: true, 100: true, 101: false} {
		inst := coreMinimal(t)
		inst["scale"].(map[string]any)["treasury"].(map[string]any)["marker_percent"] = marker
		if got := pageV1Valid(t, corePageV1, inst); got != want {
			t.Errorf("treasury marker_percent %d: valid = %v, want %v", marker, got, want)
		}
		point := map[string]any{"assumption": "a", "text": "t", "marker_percent": marker}
		inst = coreMinimal(t)
		inst["rows"] = []any{coreRow([]any{point})}
		if got := pageV1Valid(t, corePageV1, inst); got != want {
			t.Errorf("point marker_percent %d: valid = %v, want %v", marker, got, want)
		}
	}
}
