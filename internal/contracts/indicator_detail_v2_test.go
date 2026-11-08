package contracts_test

import (
	"testing"
)

const indicatorV2 = "page.indicator_detail.v2.json"

func indicatorMinimal(t *testing.T) map[string]any {
	t.Helper()
	return pageV2Minimal(t, indicatorV2)
}

func indicatorRow() map[string]any {
	return map[string]any{"period": "2026-09", "released": "2026-10-21", "value": "3.1", "change": "+0.1"}
}

func indicatorRecent(rows int) map[string]any {
	out := make([]any, 0, rows)
	for range rows {
		out = append(out, indicatorRow())
	}
	return map[string]any{"title": "最近 12 期", "columns": []any{"期", "公布日", "值", "较前"}, "rows": out}
}

func TestIndicatorDetailV2Valid(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, indicatorV2)
	inst := indicatorMinimal(t)
	if _, ok := inst["meta_line"]; !ok {
		t.Fatal("meta_line is not required")
	}
	if err := s.Validate(pageV2JSON(t, inst)); err != nil {
		t.Fatalf("minimal instance is invalid: %v", err)
	}
}

func TestIndicatorDetailV2StaleAfter(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, indicatorV2)
	inst := indicatorMinimal(t)
	inst["stale_after"] = "2026-11-08T09:30:00+08:00"
	if err := s.Validate(pageV2JSON(t, inst)); err != nil {
		t.Fatalf("stale_after of Timestamp form is invalid: %v", err)
	}
}

func TestIndicatorDetailV2VersionOneFails(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, indicatorV2)
	inst := indicatorMinimal(t)
	inst["schema_version"] = 1
	want := leaf{"/schema_version", "const"}
	for _, l := range pageV2Leaves(t, s, inst) {
		if l == want {
			return
		}
	}
	t.Errorf("want %v among the failures", want)
}

func TestIndicatorDetailV2Recent(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, indicatorV2)
	for name, c := range map[string]struct {
		recent any
		valid  bool
	}{
		"twelve rows":   {indicatorRecent(12), true},
		"thirteen rows": {indicatorRecent(13), false},
		"gap":           {pageV2Gap(), true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			inst := indicatorMinimal(t)
			inst["recent"] = c.recent
			err := s.Validate(pageV2JSON(t, inst))
			if c.valid && err != nil {
				t.Errorf("invalid: %v", err)
			}
			if !c.valid && err == nil {
				t.Error("valid, want a failure")
			}
		})
	}
}
