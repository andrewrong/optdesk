package series

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// moduleRoot walks up from the test directory to the directory holding go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func TestMigration0014Text(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join(moduleRoot(t), "migrations", "0014_series_unit_index_points.sql"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{
		"-- +goose Up\n",
		"-- +goose NO TRANSACTION\n",
		"-- +goose Down\n",
		"ALTER TYPE obs.series_unit ADD VALUE IF NOT EXISTS 'index_points';",
	} {
		if strings.Count(text, want) != 1 {
			t.Errorf("migration text: want exactly one %q", want)
		}
	}
	down := text[strings.Index(text, "-- +goose Down\n")+len("-- +goose Down\n"):]
	for _, line := range strings.Split(strings.TrimSpace(down), "\n") {
		if !strings.HasPrefix(line, "--") {
			t.Errorf("Down section holds a statement: %q", line)
		}
	}
	if !strings.Contains(down, "cannot be dropped") {
		t.Error("Down section does not say the value cannot be dropped")
	}
}

func TestIndexPointsAccepted(t *testing.T) {
	t.Parallel()
	f := newFakeDB()
	w := &seriesStore{db: f}
	v := 18.2
	p := point(t, &v, "present")
	p.SeriesID = "VIXCLS"
	p.Unit = contracts.SeriesUnit("index_points")
	if err := w.InsertSeriesPoints(context.Background(), []contracts.SeriesPoint{p}); err != nil {
		t.Fatal(err)
	}
	if len(f.points) != 1 {
		t.Fatalf("stored %d points, want 1", len(f.points))
	}
	for _, args := range f.points {
		if args[6] != "index_points" {
			t.Errorf("unit argument = %v, want index_points", args[6])
		}
	}
}
