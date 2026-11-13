package ops_today_page

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/runview"
)

const (
	dsUS    = "obs.option_eod"
	dsIndex = "obs.option_eod_index"
)

var covT = time.Date(2026, 11, 12, 21, 0, 0, 0, time.UTC)

func covRun(dataset, key, status string, usable int, at time.Time) runview.Row {
	return runview.Row{Dataset: dataset, Key: key, Status: status, Usable: usable, Returned: usable, FinishedAt: at}
}

func TestCoverageIsEmptyWithoutTheDatasets(t *testing.T) {
	t.Parallel()
	runs := []runview.Row{covRun("obs.option_open", "us", "done", 5, covT)}
	if got := coverage(runs); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := coverage(nil); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestCoverageCountsNamesAndContracts(t *testing.T) {
	t.Parallel()
	runs := []runview.Row{covRun(dsUS, "AAPL", "done", 10, covT), covRun(dsUS, "MSFT", "done", 20, covT)}
	want := "Option chains stored today: US 2 names, 30 contracts; index: no run yet; gaps: none."
	if got := coverage(runs); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestCoverageNamesTheIndexKeys(t *testing.T) {
	t.Parallel()
	runs := []runview.Row{covRun(dsUS, "AAPL", "done", 10, covT),
		covRun(dsIndex, "SPX", "done", 80, covT), covRun(dsIndex, "XSP", "done", 70, covT), covRun(dsIndex, "RUT", "done", 60, covT)}
	want := "Option chains stored today: US 1 names, 10 contracts; index RUT 60, SPX 80, XSP 70 contracts; gaps: none."
	if got := coverage(runs); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestCoverageNoIndexRunYet(t *testing.T) {
	t.Parallel()
	got := coverage([]runview.Row{covRun(dsUS, "AAPL", "done", 1, covT)})
	if !strings.Contains(got, "; index: no run yet; ") {
		t.Fatalf("got %q", got)
	}
}

func TestCoverageGapIsAFailedOrEmptyKey(t *testing.T) {
	t.Parallel()
	runs := []runview.Row{covRun(dsUS, "ZZZ", "failed", 4, covT), covRun(dsUS, "AAPL", "done", 0, covT),
		covRun(dsUS, "MSFT", "done", 3, covT), covRun(dsIndex, "SPX", "done", 80, covT)}
	want := "gaps: AAPL, ZZZ."
	if got := coverage(runs); !strings.HasSuffix(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestCoverageUsesTheNewestRunOfAKey(t *testing.T) {
	t.Parallel()
	runs := []runview.Row{covRun(dsUS, "AAPL", "done", 10, covT), covRun(dsUS, "AAPL", "failed", 0, covT.Add(time.Hour)),
		covRun(dsUS, "MSFT", "failed", 0, covT), covRun(dsUS, "MSFT", "done", 7, covT.Add(time.Hour)),
		{Dataset: dsUS, Key: "MSFT", Status: "running", Usable: 0}}
	want := "Option chains stored today: US 2 names, 7 contracts; index: no run yet; gaps: AAPL."
	if got := coverage(runs); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestCoverageCutsLongGapLists(t *testing.T) {
	t.Parallel()
	var runs []runview.Row
	for i := 0; i < 8; i++ {
		runs = append(runs, covRun(dsUS, fmt.Sprintf("K%d", i), "failed", 0, covT))
	}
	got := coverage(runs)
	if !strings.HasSuffix(got, "gaps: K0, K1, K2, K3, K4 and 3 more.") {
		t.Fatalf("got %q", got)
	}
	var many []runview.Row
	for i := 0; i < 80; i++ {
		many = append(many, covRun(dsUS, fmt.Sprintf("LONGKEY%02d", i), "failed", 0, covT))
	}
	if n := len([]rune(coverage(many))); n > maxText {
		t.Fatalf("%d runes", n)
	}
}

func TestBuildPutsTheLineInNotesAndValidates(t *testing.T) {
	t.Parallel()
	runs := []runview.Row{covRun(dsUS, "AAPL", "done", 10, covT), covRun(dsIndex, "SPX", "done", 80, covT)}
	b, err := Build(runs, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	var out struct {
		Notes []string `json:"notes"`
		Today struct {
			Rows []json.RawMessage `json:"rows"`
		} `json:"today"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	want := "Option chains stored today: US 1 names, 10 contracts; index SPX 80 contracts; gaps: none."
	if len(out.Notes) != 1 || out.Notes[0] != want {
		t.Fatalf("notes %q", out.Notes)
	}
	if len(out.Today.Rows) != 0 {
		t.Fatalf("rows %d", len(out.Today.Rows))
	}
}
