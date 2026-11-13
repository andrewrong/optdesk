package main

import (
	"bytes"
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func familyIDs() []string {
	var ids []string
	for _, f := range sortedJobFamilies() {
		ids = append(ids, f.id)
	}
	return ids
}

// TestJobSetUnchanged: the eleven families of the first cut are registered in the order of Decision 3; later
// families may sit between them.
func TestJobSetUnchanged(t *testing.T) {
	t.Parallel()
	want := []string{
		"fred_series", "option_eod", "option_eod_jp", "dividend_capture", "option_ledger", "quote_snapshot",
		"compute_figures", "calendar_refresh", "earnings_refresh", "account_capture", "option_candidates",
		"underlying_resolve",
	}
	got := familyIDs()
	i := 0
	for _, id := range got {
		if i < len(want) && id == want[i] {
			i++
		}
	}
	if i != len(want) {
		t.Fatalf("registered %v lacks %v in order", got, want)
	}
}

// TestJobOrderUnique: ids and order numbers are unique and the sort is strictly increasing.
func TestJobOrderUnique(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	prev := 0
	for _, f := range sortedJobFamilies() {
		if seen[f.id] || f.order <= prev {
			t.Fatalf("family %s order %d after %d (seen %v)", f.id, f.order, prev, seen[f.id])
		}
		seen[f.id] = true
		prev = f.order
	}
}

// TestJobDupPanics: a second family with a registered id or order panics and leaves the registry as it was.
func TestJobDupPanics(t *testing.T) {
	t.Parallel()
	before := familyIDs()
	for name, f := range map[string]jobFamily{
		"id":    {id: "fred_series", order: 9999},
		"order": {id: "dup_probe_job", order: 10},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("duplicate %s did not panic", name)
				}
			}()
			registerJob(f)
		}()
	}
	if got := familyIDs(); !slices.Equal(got, before) {
		t.Fatalf("registry changed: %v", got)
	}
}

// TestJobDocsExist: every registered id has docs/contracts/ot/jobs/<id>.md with an Order line and a Version heading.
func TestJobDocsExist(t *testing.T) {
	t.Parallel()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	root := os.DirFS(filepath.Join(filepath.Dir(file), "..", ".."))
	for _, id := range familyIDs() {
		raw, err := fs.ReadFile(root, "docs/contracts/ot/jobs/"+id+".md")
		if err != nil {
			t.Errorf("job %s has no document: %v", id, err)
			continue
		}
		text := string(raw)
		if !strings.Contains(text, "\nOrder: ") && !strings.HasPrefix(text, "Order: ") || !strings.Contains(text, "\n## Version") {
			t.Errorf("document of %s lacks the Order line or the Version heading", id)
		}
	}
}

// TestOffReasonHeld: a held job is dropped from the run list with one line carrying its reason, stays in the
// scheduler list with the reason as disabled, and its probe never answers a nil job with an empty reason.
func TestOffReasonHeld(t *testing.T) {
	t.Parallel()
	earn := candJobStub{id: jobEarningsRefresh}
	cand := candJobStub{id: jobOptionCandidates}
	keep := candJobStub{id: jobFredSeries}
	w := wiring{
		jobs:     []contracts.Job{keep, earn, cand},
		held:     map[string]contracts.Job{jobEarningsRefresh: earn, jobOptionCandidates: cand},
		off:      map[string]string{jobEarningsRefresh: reasonNoAlphaKey, jobOptionCandidates: reasonNoCandidateDSN},
		brokerUp: true,
	}
	var buf bytes.Buffer
	got := w.runnable(slog.New(slog.NewTextHandler(&buf, nil)))
	if len(got) != 1 || got[0].ID() != jobFredSeries {
		t.Fatalf("runnable %v", got)
	}
	for _, want := range []string{jobEarningsRefresh, reasonNoAlphaKey, jobOptionCandidates, reasonNoCandidateDSN} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("log %q lacks %q", buf.String(), want)
		}
	}
	cfg := runOnceConfig{fredKey: "k", eventsDSN: "x"}
	dis := schedulerDisabled(cfg, w)
	if dis[jobEarningsRefresh] != reasonNoAlphaKey || dis[jobOptionCandidates] != reasonNoCandidateDSN || len(dis) != 2 {
		t.Fatalf("disabled %v", dis)
	}
	p := &prober{w: w, log: slog.New(slog.NewTextHandler(&buf, nil)), hasKey: true}
	if j, r := p.probe(context.Background(), jobDividendCapture); j != nil || r != reasonBuildJob {
		t.Fatalf("registered family without a case: %v %q", j, r)
	}
	if j, r := p.probe(context.Background(), "no_such_job"); j != nil || r != "unknown job" {
		t.Fatalf("unknown id: %v %q", j, r)
	}
}

// TestHoursFamilyOrders: the two calendar_hours families sit at the orders 85 and 86.
func TestHoursFamilyOrders(t *testing.T) {
	t.Parallel()
	got := map[string]int{}
	for _, f := range sortedJobFamilies() {
		got[f.id] = f.order
	}
	if got["calendar_hours_us"] != 85 || got["calendar_hours_jp"] != 86 {
		t.Fatalf("orders us %d jp %d, want 85 and 86", got["calendar_hours_us"], got["calendar_hours_jp"])
	}
}
