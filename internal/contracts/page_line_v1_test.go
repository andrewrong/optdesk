package contracts_test

import (
	"strings"
	"testing"
)

// TestLineBoundsBothSchemas: line of 0 and 601 characters fail, 600 passes, in each schema.
func TestLineBoundsBothSchemas(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		build func(line string) (string, map[string]any)
	}{
		{"console running", func(l string) (string, map[string]any) {
			g := loadJobConsoleV1(t, "golden")
			it := runningItem()
			it["line"] = l
			g["running"] = []any{it}
			return jobConsoleV1, g
		}},
		{"console waiting_for", func(l string) (string, map[string]any) {
			g := loadJobConsoleV1(t, "golden")
			g["requests"].([]any)[0].(map[string]any)["waiting_for"] = map[string]any{
				"job": "compute_figures", "since": "2026-11-12T14:02:00Z", "line": l}
			return jobConsoleV1, g
		}},
		{"data health running", func(l string) (string, map[string]any) {
			m := dataHealthFixture(t, "ok")
			it := dhRunning()
			it["line"] = l
			m["running"] = []any{it}
			return dataHealthPageV1, m
		}},
	}
	for _, c := range cases {
		for n, want := range map[int]bool{0: false, 1: true, 600: true, 601: false} {
			schema, m := c.build(strings.Repeat("x", n))
			if got := pageV1Valid(t, schema, m); got != want {
				t.Errorf("%s: line of %d characters valid=%v, want %v", c.name, n, got, want)
			}
		}
	}
}
