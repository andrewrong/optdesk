package contracts_test

import "testing"

// heldRow builds the smallest valid Row of held v3.
func heldRow(t *testing.T) map[string]any {
	t.Helper()
	def, _ := follow(t, schemasDir, heldV3, "#/$defs/Row")
	row, ok := minimal(t, schemasDir, heldV3, def, v3Overrides).(map[string]any)
	if !ok {
		t.Fatal("row is not an object")
	}
	return row
}

func heldWithRow(t *testing.T, row map[string]any) map[string]any {
	t.Helper()
	inst := minimalV3(t, heldV3)
	card := inst["card"].(map[string]any)
	card["rows"] = []any{row}
	return inst
}

func TestHeldV3Valid(t *testing.T) {
	t.Parallel()
	s := compileURL(t, schemasDir, schemaHost+heldV3)
	if err := s.Validate(toJSONValue(t, minimalV3(t, heldV3))); err != nil {
		t.Fatalf("minimal instance is invalid: %v", err)
	}
	row := heldRow(t)
	for _, k := range []string{"price", "day_sharp", "five_day_sharp"} {
		if _, ok := row[k]; !ok {
			t.Fatalf("minimal row holds no %s", k)
		}
	}
	if err := s.Validate(toJSONValue(t, heldWithRow(t, row))); err != nil {
		t.Fatalf("instance with a row is invalid: %v", err)
	}
}

func TestHeldV3Rejects(t *testing.T) {
	t.Parallel()
	s := compileURL(t, schemasDir, schemaHost+heldV3)
	cases := []struct {
		name, location, keyword string
		build                   func(t *testing.T) map[string]any
	}{
		{"row without price", "/card/rows/0", "required", func(t *testing.T) map[string]any {
			r := heldRow(t)
			delete(r, "price")
			return heldWithRow(t, r)
		}},
		{"row without day_sharp", "/card/rows/0", "required", func(t *testing.T) map[string]any {
			r := heldRow(t)
			delete(r, "day_sharp")
			return heldWithRow(t, r)
		}},
		{"row without five_day_sharp", "/card/rows/0", "required", func(t *testing.T) map[string]any {
			r := heldRow(t)
			delete(r, "five_day_sharp")
			return heldWithRow(t, r)
		}},
		{"card without sharp_note", "/card", "required", func(t *testing.T) map[string]any {
			inst := minimalV3(t, heldV3)
			delete(inst["card"].(map[string]any), "sharp_note")
			return inst
		}},
		{"schema_version 2", "/schema_version", "const", func(t *testing.T) map[string]any {
			inst := minimalV3(t, heldV3)
			inst["schema_version"] = 2
			return inst
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if !hasV3Leaf(t, s, c.build(t), c.location, c.keyword) {
				t.Errorf("want a failure at %q (%s)", c.location, c.keyword)
			}
		})
	}
}

func TestHeldV3PriceAndMarks(t *testing.T) {
	t.Parallel()
	s := compileURL(t, schemasDir, schemaHost+heldV3)
	price := map[string]any{"kind": "value", "value": 187.5, "text": "$187.50", "unit": "price"}
	gap := map[string]any{"kind": "gap", "reason": "missing_quote", "text": "no quote"}
	sharp := map[string]any{"kind": "sharp", "text": "sharp rise"}
	none := map[string]any{"kind": "none"}
	ok := func(k string, v any) bool {
		r := heldRow(t)
		r[k] = v
		return s.Validate(toJSONValue(t, heldWithRow(t, r))) == nil
	}
	for name, c := range map[string]struct {
		key  string
		val  any
		want bool
	}{
		"price value":    {"price", price, true},
		"price gap":      {"price", gap, true},
		"price negative": {"price", map[string]any{"kind": "value", "value": -1.0, "text": "x", "unit": "price"}, false},
		"price mixed":    {"price", map[string]any{"kind": "value", "reason": "no_data", "value": 1.0, "text": "x", "unit": "price"}, false},
		"day sharp":      {"day_sharp", sharp, true},
		"day none":       {"day_sharp", none, true},
		"five sharp":     {"five_day_sharp", sharp, true},
		"mark null":      {"day_sharp", nil, false},
		"mark as gap":    {"day_sharp", gap, false},
		"none with text": {"day_sharp", map[string]any{"kind": "none", "text": "x"}, false},
	} {
		if got := ok(c.key, c.val); got != c.want {
			t.Errorf("%s: valid = %v, want %v", name, got, c.want)
		}
	}
}

func TestOptionsV4(t *testing.T) {
	t.Parallel()
	s := compileURL(t, schemasDir, schemaHost+optionsV4)
	inst := minimalV3(t, optionsV4)
	if err := s.Validate(toJSONValue(t, inst)); err != nil {
		t.Fatalf("minimal instance is invalid: %v", err)
	}
	if _, ok := inst["card"].(map[string]any)["closed_series_text"]; !ok {
		t.Error("minimal card holds no closed_series_text")
	}
	noText := minimalV3(t, optionsV4)
	delete(noText["card"].(map[string]any), "closed_series_text")
	if !hasV3Leaf(t, s, noText, "/card", "required") {
		t.Error("a card without closed_series_text is accepted")
	}
	bad := minimalV3(t, optionsV4)
	bad["schema_version"] = 3
	if !hasV3Leaf(t, s, bad, "/schema_version", "const") {
		t.Error("schema_version 3 is accepted")
	}
}
