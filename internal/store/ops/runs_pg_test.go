//go:build pg

package ops

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

// newRunsDatabase is newDatabase plus the table and the views of 0035. That migration also alters
// master.option_list, which this database lacks: only the part from the reason table on applies here.
func newRunsDatabase(t *testing.T) string {
	t.Helper()
	name := newDatabase(t)
	raw, err := os.ReadFile("../../../migrations/0035_option_list_band_run_view.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	_, tail, ok := strings.Cut(up, "CREATE TABLE ops.option_run_reason")
	if !ok {
		t.Fatal("0035 has no reason table section")
	}
	up = "CREATE TABLE ops.option_run_reason" + tail
	ctx := context.Background()
	c, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	if _, err := c.Exec(ctx, up); err != nil {
		t.Fatalf("apply 0035: %v", err)
	}
	return name
}

func TestRunViewReadByBothRolesPG(t *testing.T) {
	t.Parallel()
	name := newRunsDatabase(t)
	ctx := context.Background()
	led, err := Open(ctx, connURL(t, name, "ot_runner"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = led.Close() })
	run := store.Run{ID: "2026-11-12/obs.option_open/AAPL/1", Day: "2026-11-12",
		Key:    contracts.ExpectedKey{DatasetID: "obs.option_open", SubjectKey: "AAPL"},
		Status: store.RunRunning, LeaseUntil: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)}
	if err := led.RecordRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"ot_runner", "ot_ro_pages"} {
		r, err := OpenRuns(ctx, connURL(t, name, role))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = r.Close() })
		rows, err := r.RunsOn(ctx, "2026-11-12")
		if err != nil || len(rows) != 1 || rows[0].Key != "AAPL" || rows[0].Status != "running" || !rows[0].FinishedAt.IsZero() {
			t.Fatalf("%s: rows %+v, %v", role, rows, err)
		}
		if rows, err := r.RunsOn(ctx, "2026-11-11"); err != nil || rows == nil || len(rows) != 0 {
			t.Fatalf("%s: other day %#v, %v", role, rows, err)
		}
	}
}

func TestReasonsReplaceAndNotWiredPG(t *testing.T) {
	t.Parallel()
	name := newRunsDatabase(t)
	ctx := context.Background()
	w, err := OpenRuns(ctx, connURL(t, name, "ot_runner"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	at := time.Date(2026, 11, 12, 21, 5, 0, 0, time.UTC)
	set := []runview.Reason{{Scope: "AAPL", Code: "no_chain", Detail: "a", At: at}, {Scope: "MSFT", Code: "no_close", At: at}}
	if err := w.ReplaceReasons(ctx, "2026-11-12", "obs.option_open", "k", set); err != nil {
		t.Fatal(err)
	}
	if err := w.ReplaceReasons(ctx, "2026-11-12", "obs.option_open", "k", set[:1]); err != nil {
		t.Fatal(err)
	}
	got, err := w.ReasonsOn(ctx, "2026-11-12")
	if err != nil || len(got) != 1 || got[0].Scope != "AAPL" || got[0].Code != "no_chain" || !got[0].At.Equal(at) {
		t.Fatalf("after replace: %+v, %v", got, err)
	}
	if err := w.ReplaceReasons(ctx, "2026-11-12", "obs.option_open", "k", nil); err != nil {
		t.Fatal(err)
	}
	if got, err := w.ReasonsOn(ctx, "2026-11-12"); err != nil || len(got) != 0 {
		t.Fatalf("clean run: %+v, %v", got, err)
	}
	nw := []runview.Reason{{Scope: "job_b", Code: "not_wired", At: at}, {Scope: "job_a", Code: "not_wired", At: at}}
	if err := w.ReplaceReasons(ctx, runview.NotWiredDay, runview.NotWiredDataset, "", nw); err != nil {
		t.Fatal(err)
	}
	got, err = w.NotWired(ctx)
	if err != nil || len(got) != 2 || got[0].Scope != "job_a" {
		t.Fatalf("not wired: %+v, %v", got, err)
	}
	if err := w.ReplaceReasons(ctx, "2026-11-12", "obs.option_open", "k", []runview.Reason{{Scope: "x", Code: "bogus", At: at}}); err == nil {
		t.Error("an unknown code was stored")
	}
}
