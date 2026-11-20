package job_console

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/jobrunapi"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// refreshOf builds the page over rows and runs and returns its refresh_seconds.
func refreshOf(t *testing.T, rows []jobrequest.Row, runs Runs) int {
	t.Helper()
	out, err := BuildWithRuns(context.Background(), &fakeReader{recent: rows}, fixedNow, nil, 0, nil, runs)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		RefreshSeconds int `json:"refresh_seconds"`
	}
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	return p.RefreshSeconds
}

func TestRefreshSecondsWhileRunLineOpen(t *testing.T) {
	t.Parallel()
	done := []jobrequest.Row{row(1, jobrequest.StateDone)}
	runs := Runs{Reader: oneRun("compute_figures"), Figures: fixedFigures(jobrunapi.Figure{JobID: "compute_figures"})}
	if got := refreshOf(t, done, runs); got != 5 {
		t.Errorf("open run: refresh %d, want 5", got)
	}
	if got := refreshOf(t, done, Runs{}); got != 0 {
		t.Errorf("no open run: refresh %d, want 0", got)
	}
}
