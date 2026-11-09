package cutover_test

import (
	"context"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs/cutover"
)

func TestImportAccountRefusesUnmappedAlias(t *testing.T) {
	t.Parallel()
	st := failingStore{newFake(), t}
	in := line("main", "2026-11-05", "1", "1") + line("stray", "2026-11-05", "1", "1")
	_, err := cutover.ImportAccount(context.Background(), strings.NewReader(in), st, map[string]string{"main": "live"}, through)
	if err == nil {
		t.Fatal("unmapped alias accepted")
	}
}

func TestImportAccountMapsAlias(t *testing.T) {
	t.Parallel()
	f := newFake()
	_, err := cutover.ImportAccount(context.Background(), strings.NewReader(line("main", "2026-11-05", "1", "1")), f, map[string]string{"main": "live"}, through)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.captures["live:2026-11-05:import"]; !ok {
		t.Fatalf("captures = %v", f.captures)
	}
}
