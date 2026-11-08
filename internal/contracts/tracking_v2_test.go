package contracts_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const trackingV2 = "page.tracking.v2.json"

func pageV2Minimal(t *testing.T, name string) map[string]any {
	t.Helper()
	raw := readRaw(t, schemasDir, name)
	overrides := map[string]string{"^[A-Za-z0-9_-]{1,64}$": "id_1"}
	// The carried timestamp pattern of generated_at, which the page keeps from v1.
	stamp := raw["properties"].(map[string]any)["generated_at"].(map[string]any)["pattern"].(string)
	overrides[stamp] = "2026-11-08T09:30:00+08:00"
	if dt, ok := raw["$defs"].(map[string]any)["DateText"].(map[string]any); ok {
		day := dt["properties"].(map[string]any)["day"].(map[string]any)["pattern"].(string)
		overrides[day] = "2026-10-21"
	}
	inst, ok := minimal(t, schemasDir, name, raw, overrides).(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object schema", name)
	}
	return inst
}

func pageV2JSON(t *testing.T, m map[string]any) any {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// pageV2Leaves validates m and returns the failing leaves; it fails the test when m is valid.
func pageV2Leaves(t *testing.T, s *jsonschema.Schema, m map[string]any) []leaf {
	t.Helper()
	ve, ok := s.Validate(pageV2JSON(t, m)).(*jsonschema.ValidationError)
	if !ok {
		t.Fatal("instance is valid, want a validation error")
	}
	var ls []leaf
	leaves(ve, &ls)
	return ls
}

func pageV2Gap() map[string]any {
	return map[string]any{"reason": "no_data", "text": "尚无数据"}
}

func trackingFirstVersion(inst map[string]any) map[string]any {
	return inst["versions"].([]any)[0].(map[string]any)
}

func TestTrackingV2Valid(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, trackingV2)
	if err := s.Validate(pageV2JSON(t, pageV2Minimal(t, trackingV2))); err != nil {
		t.Fatalf("minimal instance is invalid: %v", err)
	}
}

func TestTrackingV2StaleAfter(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, trackingV2)
	inst := pageV2Minimal(t, trackingV2)
	inst["stale_after"] = "2026-11-08T09:30:00+08:00"
	if err := s.Validate(pageV2JSON(t, inst)); err != nil {
		t.Fatalf("stale_after of Timestamp form is invalid: %v", err)
	}
}

func TestTrackingV2VersionOneFails(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, trackingV2)
	inst := pageV2Minimal(t, trackingV2)
	inst["schema_version"] = 1
	want := leaf{"/schema_version", "const"}
	for _, l := range pageV2Leaves(t, s, inst) {
		if l == want {
			return
		}
	}
	t.Errorf("want %v among the failures", want)
}

func TestTrackingV2VersionsEmptyFails(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, trackingV2)
	inst := pageV2Minimal(t, trackingV2)
	inst["versions"] = []any{}
	want := leaf{"/versions", "minItems"}
	for _, l := range pageV2Leaves(t, s, inst) {
		if l == want {
			return
		}
	}
	t.Errorf("want %v among the failures", want)
}

func TestTrackingV2SummaryGap(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, trackingV2)
	inst := pageV2Minimal(t, trackingV2)
	group := trackingFirstVersion(inst)["groups"].([]any)[0].(map[string]any)
	group["summary"] = pageV2Gap()
	if err := s.Validate(pageV2JSON(t, inst)); err != nil {
		t.Fatalf("summary as a Gap is invalid: %v", err)
	}
}

func TestTrackingV2MyTradesGap(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, trackingV2)
	inst := pageV2Minimal(t, trackingV2)
	trackingFirstVersion(inst)["my_trades"] = pageV2Gap()
	if err := s.Validate(pageV2JSON(t, inst)); err != nil {
		t.Fatalf("my_trades as a Gap is invalid: %v", err)
	}
}

func TestTrackingV2CoveredCallsNull(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, trackingV2)
	inst := pageV2Minimal(t, trackingV2)
	trackingFirstVersion(inst)["covered_calls"] = nil
	if err := s.Validate(pageV2JSON(t, inst)); err != nil {
		t.Fatalf("covered_calls null is invalid: %v", err)
	}
}
