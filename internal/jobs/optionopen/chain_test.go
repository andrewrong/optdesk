package optionopen

import (
	"os"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// chainManifest holds the fields of a job manifest the chain check reads.
type chainManifest struct {
	BudgetMinutes int `toml:"budget_minutes"`
	Schedule      struct {
		Offset   string `toml:"offset"`
		RunBound string `toml:"run_bound"`
	} `toml:"schedule"`
}

func readChainManifest(t *testing.T, path string) chainManifest {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m chainManifest
	if err := toml.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return m
}

func chainDuration(t *testing.T, path, s string) time.Duration {
	t.Helper()
	d, err := time.ParseDuration(s)
	if err != nil {
		t.Fatalf("%s: duration %q: %v", path, s, err)
	}
	return d
}

// TestWorstChainEndsBeforeThePull: from 08:00 ET the one worker runs gap, fundamentals, plan and
// calendar_refresh; if each uses its whole bound the chain must end 30 minutes before the pull.
// All offsets are measured from the session open (09:30 ET, ordinary day).
func TestWorstChainEndsBeforeThePull(t *testing.T) {
	t.Parallel()

	const (
		gapPath  = "job_gap.toml"
		planPath = "job_plan.toml"
		pullPath = "job_pull.toml"
		fundPath = "../fundamentals/job.toml"
		calPath  = "../calendarrefresh/job.toml"
		margin   = 30 * time.Minute
		open     = 9*time.Hour + 30*time.Minute
	)
	gap := readChainManifest(t, gapPath)
	plan := readChainManifest(t, planPath)
	pull := readChainManifest(t, pullPath)
	fund := readChainManifest(t, fundPath)
	cal := readChainManifest(t, calPath)

	gapOff := chainDuration(t, gapPath, gap.Schedule.Offset)
	planOff := chainDuration(t, planPath, plan.Schedule.Offset)
	pullOff := chainDuration(t, pullPath, pull.Schedule.Offset)
	// fundamentals is anchored on midnight; the others on the session open.
	fundOff := chainDuration(t, fundPath, fund.Schedule.Offset) - open
	calOff := chainDuration(t, calPath, cal.Schedule.Offset)

	for name, off := range map[string]time.Duration{
		"option_open_resolve_gap": gapOff,
		"fundamentals":            fundOff,
		"option_open_plan":        planOff,
		"calendar_refresh":        calOff,
	} {
		if off >= pullOff {
			t.Errorf("%s is due at %v from the open, not before the pull at %v", name, off, pullOff)
		}
	}

	sum := time.Duration(gap.BudgetMinutes)*time.Minute +
		chainDuration(t, fundPath, fund.Schedule.RunBound) +
		time.Duration(plan.BudgetMinutes)*time.Minute +
		chainDuration(t, calPath, cal.Schedule.RunBound)
	end := gapOff + sum
	if end > pullOff-margin {
		t.Errorf("worst chain of %v from %v ends at %v from the open; the pull's slot %s is at %v and needs %v of margin",
			sum, gapOff, end, "0945", pullOff, margin)
	}
}
