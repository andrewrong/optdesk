package data_health_page

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/jobrunapi"
	"github.com/andrewrong/optdesk/internal/store/jobrun"
)

// runningRow is one element of running[] as the page writes it.
type runningRow struct {
	Line           string `json:"line"`
	Job            string `json:"job"`
	StartedAt      string `json:"started_at"`
	KeysDone       int    `json:"keys_done"`
	KeysTotal      int    `json:"keys_total"`
	Step           string `json:"step"`
	ElapsedMinutes int    `json:"elapsed_minutes"`
	UsualMinutes   int    `json:"usual_minutes"`
	Slow           bool   `json:"slow"`
}

type fakeRunReader struct{ open []jobrun.JobRun }

func (f fakeRunReader) Running(context.Context) ([]jobrun.JobRun, error) { return f.open, nil }
func (fakeRunReader) Usuals(context.Context) ([]jobrun.UsualStat, error) { return nil, nil }

// buildRunning builds the page with one open run of job whose figures are exactly fig, and returns its running[].
func buildRunning(t *testing.T, job string, fig jobrunapi.Figure) []runningRow {
	t.Helper()
	started := time.Date(2026, 11, 17, 14, 2, 0, 0, time.UTC)
	in := Input{Now: time.Date(2026, 11, 17, 15, 0, 0, 0, time.UTC), Today: "2026-11-17",
		Runs: fakeRunReader{open: []jobrun.JobRun{{ID: "r1", JobID: job, StartedAt: started}}},
		Figures: func(time.Time, []jobrunapi.Input, []jobrunapi.Usual) []jobrunapi.Figure {
			return []jobrunapi.Figure{fig}
		}}
	b, err := Build(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Running []runningRow `json:"running"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	return page.Running
}

func TestRunningLine(t *testing.T) {
	t.Parallel()
	got := buildRunning(t, "statement_capture", jobrunapi.Figure{JobID: "statement_capture", ElapsedMinutes: 10, UsualMinutes: 8,
		HasUsual: true, KeysDone: 12, KeysTotal: 40, Step: "k-0003"})
	if len(got) != 1 {
		t.Fatalf("running = %+v", got)
	}
	want := "statement_capture; 开始于 14:02 UTC (started 14:02 UTC); 已完成 12 / 40 (done 12 / 40); k-0003; 跑了 10 分钟，平常 8 分钟 (ran 10 minutes, usually 8 minutes)"
	if got[0].Line != want {
		t.Errorf("line = %q, want %q", got[0].Line, want)
	}
	if g := got[0]; g.Job != "statement_capture" || g.StartedAt != "2026-11-17T14:02:00Z" || g.KeysDone != 12 || g.KeysTotal != 40 ||
		g.Step != "k-0003" || g.ElapsedMinutes != 10 || g.UsualMinutes != 8 || g.Slow {
		t.Errorf("figures not copied: %+v", g)
	}
}

func TestRunningSlowLine(t *testing.T) {
	t.Parallel()
	got := buildRunning(t, "j", jobrunapi.Figure{JobID: "j", ElapsedMinutes: 62, UsualMinutes: 3, HasUsual: true, Slow: true, KeysDone: 1, KeysTotal: 2})
	want := "j; 开始于 14:02 UTC (started 14:02 UTC); 已完成 1 / 2 (done 1 / 2); 跑了 62 分钟，平常 3 分钟 (ran 62 minutes, usually 3 minutes); 偏慢，仅提示 (slow, a notice only)"
	if len(got) != 1 || got[0].Line != want || !got[0].Slow {
		t.Errorf("running = %+v, want line %q", got, want)
	}
}

func TestRunningNoHistoryLine(t *testing.T) {
	t.Parallel()
	got := buildRunning(t, "j", jobrunapi.Figure{JobID: "j", ElapsedMinutes: 5, KeysDone: 0, KeysTotal: 7})
	want := "j; 开始于 14:02 UTC (started 14:02 UTC); 已完成 0 / 7 (done 0 / 7); 跑了 5 分钟，没有历史 (ran 5 minutes, no history)"
	if len(got) != 1 || got[0].Line != want {
		t.Errorf("running = %+v, want line %q", got, want)
	}
}
