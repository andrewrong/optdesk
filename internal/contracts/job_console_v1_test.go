package contracts_test

import (
	"errors"
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
	var ve *jsonschema.ValidationError
	ok := errors.As(err, &ve)
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

// TestJobConsoleAcceptsDesignGaps: the two gaps Decision 12 names validate; another reason does not.
func TestJobConsoleAcceptsDesignGaps(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.job_console.v1.json")
	for _, reason := range []string{"job_requests_unavailable", "no_such_request"} {
		g := loadJobConsoleV1(t, "golden")
		g["gaps"] = []any{map[string]any{"kind": "gap", "reason": reason, "text": "gap text"}}
		if err := s.Validate(g); err != nil {
			t.Errorf("gap %s refused: %v", reason, err)
		}
	}
	g := loadJobConsoleV1(t, "golden")
	g["gaps"] = []any{map[string]any{"kind": "gap", "reason": "no_data", "text": "gap text"}}
	if err := s.Validate(g); err == nil {
		t.Error("gap reason no_data accepted")
	}
}

const jobConsoleV1 = "page.job_console.v1.json"

func runningItem() map[string]any {
	return map[string]any{
		"job": "compute_figures", "started_at": "2026-11-12T14:02:00Z", "keys_done": 1, "keys_total": 4,
		"step": "US:AAPL", "elapsed_minutes": 4, "usual_minutes": 3, "slow": false, "line": "running line",
	}
}

// TestRunningFieldsValidate: an instance with running and waiting_for passes.
func TestRunningFieldsValidate(t *testing.T) {
	t.Parallel()
	g := loadJobConsoleV1(t, "golden")
	g["running"] = []any{runningItem()}
	r := g["requests"].([]any)[0].(map[string]any)
	r["waiting_for"] = map[string]any{"job": "compute_figures", "since": "2026-11-12T14:02:00Z", "line": "waiting line"}
	if !pageV1Valid(t, jobConsoleV1, g) {
		t.Fatal("instance with running and waiting_for refused")
	}
}

// TestRunningFieldsNullFail: a null for any new field, or an unknown key inside one, fails.
func TestRunningFieldsNullFail(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"job", "started_at", "keys_done", "keys_total", "step", "elapsed_minutes", "usual_minutes", "slow"} {
		g := loadJobConsoleV1(t, "golden")
		it := runningItem()
		it[k] = nil
		g["running"] = []any{it}
		if pageV1Valid(t, jobConsoleV1, g) {
			t.Errorf("running.%s null accepted", k)
		}
	}
	for _, k := range []string{"job", "since"} {
		g := loadJobConsoleV1(t, "golden")
		w := map[string]any{"job": "compute_figures", "since": "2026-11-12T14:02:00Z", "line": "waiting line"}
		w[k] = nil
		g["requests"].([]any)[0].(map[string]any)["waiting_for"] = w
		if pageV1Valid(t, jobConsoleV1, g) {
			t.Errorf("waiting_for.%s null accepted", k)
		}
	}
	g := loadJobConsoleV1(t, "golden")
	g["running"] = nil
	if pageV1Valid(t, jobConsoleV1, g) {
		t.Error("running null accepted")
	}
	g = loadJobConsoleV1(t, "golden")
	it := runningItem()
	it["extra"] = 1
	g["running"] = []any{it}
	if pageV1Valid(t, jobConsoleV1, g) {
		t.Error("running item with an extra key accepted")
	}
}

// TestOldShapeStillValidates: a page without running and waiting_for still passes.
func TestOldShapeStillValidates(t *testing.T) {
	t.Parallel()
	g := loadJobConsoleV1(t, "golden")
	delete(g, "running")
	for _, r := range g["requests"].([]any) {
		delete(r.(map[string]any), "waiting_for")
	}
	if !pageV1Valid(t, jobConsoleV1, g) {
		t.Fatal("old shape refused")
	}
}

// TestRunningWithoutLineFails: a running item without line, or with an empty one, is refused.
func TestRunningWithoutLineFails(t *testing.T) {
	t.Parallel()
	for _, drop := range []bool{true, false} {
		g := loadJobConsoleV1(t, "golden")
		it := runningItem()
		if drop {
			delete(it, "line")
		} else {
			it["line"] = ""
		}
		g["running"] = []any{it}
		if pageV1Valid(t, jobConsoleV1, g) {
			t.Errorf("running item without line accepted (dropped=%v)", drop)
		}
	}
}

// TestWaitingForWithoutLineFails: a waiting_for without line is refused.
func TestWaitingForWithoutLineFails(t *testing.T) {
	t.Parallel()
	g := loadJobConsoleV1(t, "golden")
	g["requests"].([]any)[0].(map[string]any)["waiting_for"] = map[string]any{"job": "compute_figures", "since": "2026-11-12T14:02:00Z"}
	if pageV1Valid(t, jobConsoleV1, g) {
		t.Error("waiting_for without line accepted")
	}
}
