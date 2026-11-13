package jobs_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
	"github.com/andrewrong/optdesk/internal/jobs/optioneod"
)

func TestEveryDeclaredNeedIsInsideTheCapture(t *testing.T) {
	t.Parallel()
	needs, err := optioncandidates.Needs()
	if err != nil {
		t.Fatal(err)
	}
	if len(needs) == 0 {
		t.Fatal("no need declared")
	}
	for _, n := range needs {
		if err := optioneod.Covers(n); err != nil {
			t.Errorf("%s: %v", n.Strategy, err)
		}
	}
}

func TestANeedOutsideTheCaptureIsRefused(t *testing.T) {
	t.Parallel()
	needs, err := optioncandidates.Needs()
	if err != nil || len(needs) == 0 {
		t.Fatalf("needs %v, err %v", needs, err)
	}
	n := needs[0]
	n.MinDTE = 10
	if err := optioneod.Covers(n); err == nil {
		t.Fatal("a need with min_dte 10 is accepted")
	}
}
