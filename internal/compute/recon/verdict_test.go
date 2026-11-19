package recon

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/compute/reconapi"
)

func TestR6NothingComparedIsUnknown(t *testing.T) {
	t.Parallel()
	o := reconapi.Output{Skipped: 5}
	if got := Verdict("R6", o, 5, 0); got != basecheckapi.Unknown {
		t.Errorf("level = %s", got)
	}
}

func TestR6MissingOursCountsAsBad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		o    reconapi.Output
		want basecheckapi.Level
	}{
		{reconapi.Output{N: 1000, Matched: 1000}, basecheckapi.Pass},
		{reconapi.Output{N: 1000, Matched: 999, MissingOurs: 1}, basecheckapi.Pass},          // 0.001 is not above
		{reconapi.Output{N: 1000, Matched: 998, MissingOurs: 2}, basecheckapi.Warn},          // 0.002
		{reconapi.Output{N: 1000, Matched: 990, Diff: 5, MissingOurs: 5}, basecheckapi.Warn}, // exactly 0.01 is not above
		{reconapi.Output{N: 1000, Matched: 989, Diff: 5, MissingOurs: 6}, basecheckapi.Fail},
		{reconapi.Output{N: 10, Matched: 9, MissingOurs: 1}, basecheckapi.Fail},
	} {
		if got := Verdict("R6", tc.o, 10, 10); got != tc.want {
			t.Errorf("%+v = %s, want %s", tc.o, got, tc.want)
		}
	}
}

func TestR6HalfAskedCapsAtWarn(t *testing.T) {
	t.Parallel()
	o := reconapi.Output{N: 100, Matched: 100}
	if got := Verdict("R6", o, 10, 5); got != basecheckapi.Pass {
		t.Errorf("half compared = %s, want pass", got)
	}
	if got := Verdict("R6", o, 10, 4); got != basecheckapi.Warn {
		t.Errorf("under half = %s, want warn", got)
	}
	bad := reconapi.Output{N: 100, Matched: 90, Diff: 10}
	if got := Verdict("R6", bad, 10, 4); got != basecheckapi.Fail {
		t.Errorf("fail stays fail = %s", got)
	}
}

func TestR1AnyDiffFails(t *testing.T) {
	t.Parallel()
	for _, o := range []reconapi.Output{
		{N: 100, Matched: 99, Diff: 1},
		{N: 100, Matched: 99, MissingOurs: 1},
		{N: 100, Matched: 99, MissingOther: 1},
	} {
		if got := Verdict("R1", o, 0, 0); got != basecheckapi.Fail {
			t.Errorf("%+v = %s, want fail", o, got)
		}
	}
	if got := Verdict("R1", reconapi.Output{N: 3, Matched: 3, Skipped: 2}, 0, 0); got != basecheckapi.Pass {
		t.Errorf("clean = %s, want pass", got)
	}
}
