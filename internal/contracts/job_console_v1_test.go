package contracts_test

import (
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func loadJobConsoleV1(t *testing.T, name string) map[string]any {
	t.Helper()
	return readRaw(t, optionPageFixturesDir, "page.job_console.v1."+name+".json")
}

// TestJobConsoleGolden validates the golden fixture against the schema.
func TestJobConsoleGolden(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.job_console.v1.json")
	if err := s.Validate(loadJobConsoleV1(t, "golden")); err != nil {
		t.Fatalf("golden: %v", err)
	}
}

// TestJobConsoleBadVersion asserts the schema refuses schema_version 2 and nothing else.
func TestJobConsoleBadVersion(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.job_console.v1.json")
	err := s.Validate(loadJobConsoleV1(t, "bad-version"))
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		t.Fatalf("err = %v, want a validation error", err)
	}
	var got []leaf
	leaves(ve, &got)
	if len(got) != 1 || got[0] != (leaf{"/schema_version", "const"}) {
		t.Errorf("failures = %v, want one /schema_version const", got)
	}
}

// TestJobConsoleGoldenForceOnlyForForceJobs: a forced request names a job whose force is true.
func TestJobConsoleGoldenForceOnlyForForceJobs(t *testing.T) {
	t.Parallel()
	g := loadJobConsoleV1(t, "golden")
	force := map[string]bool{}
	for _, j := range g["jobs"].([]any) {
		m := j.(map[string]any)
		force[m["id"].(string)] = m["force"].(bool)
	}
	for _, r := range g["requests"].([]any) {
		m := r.(map[string]any)
		if m["force"].(bool) && !force[m["job"].(string)] {
			t.Errorf("request %v: force on force-no job %v", m["id"], m["job"])
		}
	}
}
