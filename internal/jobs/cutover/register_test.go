package cutover_test

import (
	"strings"
	"testing"
)

func TestExplainedEmptyReason(t *testing.T) {
	t.Parallel()
	expl := "[[explained]]\nid = \"demo.beta\"\nreason = \"\"\ndate = \"2026-11-09\"\n"
	_, err := run(t, "", fixture(t, "register.toml"), fixture(t, "classes.toml"), expl, &fakeReader{})
	if err == nil || !strings.Contains(err.Error(), "empty reason") {
		t.Fatalf("err = %v, want an empty reason refusal", err)
	}
}

func TestRegisterReadsThreeFigures(t *testing.T) {
	t.Parallel()
	rep, err := run(t, "", fixture(t, "register.toml"), fixture(t, "classes.toml"), "", &fakeReader{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Results) != 3 || rep.Results[0].ID != "demo.alpha" || rep.Results[2].ID != "demo.fresh" {
		t.Fatalf("results = %+v", rep.Results)
	}
}
