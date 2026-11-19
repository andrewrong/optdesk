package contracts_test

import (
	"testing"
)

const householdPageFile = "page.household_page.v1.json"

// familyInstance is the smallest valid household page with the given optional
// members merged in.
func familyInstance(t *testing.T, extra map[string]any) map[string]any {
	t.Helper()
	inst, ok := minimal(t, schemasDir, householdPageFile, readRaw(t, schemasDir, householdPageFile), familyOverrides).(map[string]any)
	if !ok {
		t.Fatal("household page: not an object schema")
	}
	for k, v := range extra {
		inst[k] = v
	}
	return inst
}

func familyViews() []any {
	return []any{
		map[string]any{"key": "household", "label": "全家", "href": "/household", "current": true},
		map[string]any{"key": "main", "label": "账户（本人）", "href": "/household?account=main", "current": false},
	}
}

func TestFamilyFieldsValidate(t *testing.T) {
	t.Parallel()
	s := compileCarried(t, householdPageFile)
	inst := familyInstance(t, map[string]any{
		"views":      familyViews(),
		"lead_notes": []any{"账户（配偶）无数据，合计不含"},
	})
	if err := s.Validate(toJSONValue(t, inst)); err != nil {
		t.Fatalf("instance with views and lead_notes: %v", err)
	}
}

func TestFamilyOldShapeValidates(t *testing.T) {
	t.Parallel()
	s := compileCarried(t, householdPageFile)
	if err := s.Validate(toJSONValue(t, familyInstance(t, nil))); err != nil {
		t.Fatalf("instance without views and lead_notes: %v", err)
	}
}

func TestViewNeedsAllFour(t *testing.T) {
	t.Parallel()
	s := compileCarried(t, householdPageFile)
	views := familyViews()
	delete(views[1].(map[string]any), "current")
	inst := familyInstance(t, map[string]any{"views": views})
	if s.Validate(toJSONValue(t, inst)) == nil {
		t.Fatal("a view without current passed")
	}
}

func TestFamilyFieldsNullFail(t *testing.T) {
	t.Parallel()
	s := compileCarried(t, householdPageFile)
	for _, key := range []string{"views", "lead_notes"} {
		inst := familyInstance(t, map[string]any{key: nil})
		if s.Validate(toJSONValue(t, inst)) == nil {
			t.Errorf("%q: null passed", key)
		}
	}
}

// familyOverrides gives the pattern-bound strings of the page a value that matches.
var familyOverrides = map[string]string{
	"^[A-Za-z0-9_-]{1,64}$": "main",
	patDateTime:             "2026-01-02T03:04:05+08:00",
	patDate:                 "2026-01-02",
}
