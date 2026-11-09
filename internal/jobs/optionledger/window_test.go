package optionledger

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func days(ws []window) [][2]string {
	out := make([][2]string, len(ws))
	for i, w := range ws {
		out[i] = [2]string{string(w.first), string(w.last)}
	}
	return out
}

func checkWindows(t *testing.T, got []window, want [][2]string) {
	t.Helper()
	g := days(got)
	if len(g) != len(want) {
		t.Fatalf("windows = %v, want %v", g, want)
	}
	for i := range g {
		if g[i] != want[i] {
			t.Fatalf("windows = %v, want %v", g, want)
		}
	}
}

func TestWindowFromHistoryStartWithoutOkFetch(t *testing.T) {
	t.Parallel()
	ws, err := planWindows("2025-12-01", "2026-03-01", "", true)
	if err != nil {
		t.Fatal(err)
	}
	checkWindows(t, ws, [][2]string{{"2025-12-01", "2026-03-01"}})
}

func TestWindowThirtyDayOverlap(t *testing.T) {
	t.Parallel()
	ws, err := planWindows("2025-12-01", "2026-11-09", contracts.Day("2026-11-05"), true)
	if err != nil {
		t.Fatal(err)
	}
	checkWindows(t, ws, [][2]string{{"2026-10-06", "2026-11-09"}})
	// never before history_from
	ws, err = planWindows("2025-12-01", "2026-01-10", "2025-12-10", true)
	if err != nil {
		t.Fatal(err)
	}
	checkWindows(t, ws, [][2]string{{"2025-12-01", "2026-01-10"}})
}

func TestWindowsCutAt365Days(t *testing.T) {
	t.Parallel()
	ws, err := planWindows("2024-01-01", "2025-12-31", "", true)
	if err != nil {
		t.Fatal(err)
	}
	// 2024 has 366 days: the first window holds 365 of them.
	checkWindows(t, ws, [][2]string{{"2024-01-01", "2024-12-30"}, {"2024-12-31", "2025-12-30"}, {"2025-12-31", "2025-12-31"}})
	ws, err = planWindows("2025-01-01", "2025-12-31", "", true)
	if err != nil {
		t.Fatal(err)
	}
	checkWindows(t, ws, [][2]string{{"2025-01-01", "2025-12-31"}})
}

func TestWindowFromHistoryStartWhenAliasHasNoRow(t *testing.T) {
	t.Parallel()
	ws, err := planWindows("2025-12-01", "2026-11-09", "2026-11-05", false)
	if err != nil {
		t.Fatal(err)
	}
	checkWindows(t, ws, [][2]string{{"2025-12-01", "2026-11-09"}})
}
