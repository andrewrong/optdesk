package main

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/data_health_page"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// repairCoverage serves fixed sessions (the five weekdays of the week of postNow) and cells.
type repairCoverage struct{ cells []coverage.Cell }

func (f repairCoverage) Cells(context.Context, contracts.Day, contracts.Day) ([]coverage.Cell, error) {
	return append([]coverage.Cell{}, f.cells...), nil
}

func (repairCoverage) Sessions(context.Context, string, contracts.Day, contracts.Day) ([]coverage.Session, error) {
	out := []coverage.Session{}
	for _, d := range []contracts.Day{"2026-11-02", "2026-11-03", "2026-11-04", "2026-11-05", "2026-11-06"} {
		out = append(out, coverage.Session{Calendar: "XNYS", Day: d, IsSession: true})
	}
	return out, nil
}
func (repairCoverage) NewestObservations(context.Context) ([]coverage.Newest, error) {
	return []coverage.Newest{}, nil
}
func (repairCoverage) Close() error { return nil }

func repairMissing(dataset string, day contracts.Day) coverage.Cell {
	return coverage.Cell{Day: day, Dataset: dataset, Keys: 2, OK: 1, Missing: 1, MissingKeys: []string{"A"}}
}

func repairCellOf(dataset string, day contracts.Day, state coverage.State) coverage.Cell {
	c := coverage.Cell{Day: day, Dataset: dataset, Keys: 1}
	switch state {
	case coverage.StateOK:
		c.OK = 1
	case coverage.StateNoValue:
		c.NoValue = 1
	case coverage.StateExpired:
		c.Expired = 1
	case coverage.StateMissing:
		c.Missing = 1
	}
	return c
}

func repairOver(t *testing.T, dataset string, cells ...coverage.Cell) (*fakePoster, int64, bool, error) {
	t.Helper()
	fp := newFakePoster()
	first, found, err := repairGaps(t.Context(), repairCoverage{cells}, fp, dataset, fakeClock{postNow})
	return fp, first, found, err
}

func wantRequests(t *testing.T, fp *fakePoster, job string, days ...string) {
	t.Helper()
	if len(fp.enqueued) != len(days) {
		t.Fatalf("enqueued %v, want days %v", fp.enqueued, days)
	}
	for i, d := range days {
		if got := fp.enqueued[i]; got.JobID != job || got.Day != d || got.Force {
			t.Fatalf("request %d = %+v, want %s %s no force", i, got, job, d)
		}
	}
}

func TestRepairQueuesOldestFirst(t *testing.T) {
	t.Parallel()
	// the cells come newest first; the requests go oldest first.
	fp, first, found, err := repairOver(t, "obs.fx_rate",
		repairMissing("obs.fx_rate", "2026-11-05"), repairMissing("obs.fx_rate", "2026-11-03"),
		repairMissing("obs.fx_rate", "2026-11-04"))
	if err != nil || !found {
		t.Fatalf("%v %v", found, err)
	}
	wantRequests(t, fp, "fx_refresh", "2026-11-03", "2026-11-04", "2026-11-05")
	if first != 101 {
		t.Fatalf("first id %d", first)
	}
}

func TestRepairSkipsOkAndNoValue(t *testing.T) {
	t.Parallel()
	const ds = "obs.fx_rate"
	// 11-06 has no cell at all: not_run.
	fp, first, _, err := repairOver(t, ds, repairCellOf(ds, "2026-11-03", coverage.StateOK), repairCellOf(ds, "2026-11-04", coverage.StateNoValue))
	if err != nil || len(fp.enqueued) != 0 || first != 0 {
		t.Fatalf("%v %v %d", err, fp.enqueued, first)
	}
	fp, _, _, err = repairOver(t, ds, repairCellOf(ds, "2026-11-04", coverage.StateExpired))
	if err != nil {
		t.Fatal(err)
	}
	wantRequests(t, fp, "fx_refresh", "2026-11-04")
}

func TestRepairWindowJobOnePerDay(t *testing.T) {
	t.Parallel()
	for job, ds := range map[string]string{"bars_refresh": "obs.price_bar", "fred_series": "obs.series_point", "fundamentals": "obs.fundamental_fact"} {
		fp, _, _, err := repairOver(t, ds, repairMissing(ds, "2026-11-02"), repairMissing(ds, "2026-11-03"), repairMissing(ds, "2026-11-04"))
		if err != nil {
			t.Fatal(err)
		}
		wantRequests(t, fp, job, "2026-11-02", "2026-11-03", "2026-11-04")
	}
}

func TestRepairTodayOnlyPastDay(t *testing.T) {
	t.Parallel()
	const ds = "obs.option_eod"
	fp, first, _, err := repairOver(t, ds, repairMissing(ds, "2026-11-04"))
	if err != nil || len(fp.enqueued) != 0 || first != 0 {
		t.Fatalf("%v %v", err, fp.enqueued)
	}
}

func TestRepairPresentOnlyPastDay(t *testing.T) {
	t.Parallel()
	const ds = "obs.earnings_date"
	fp, _, _, err := repairOver(t, ds, repairMissing(ds, "2026-11-04"))
	if err != nil || len(fp.enqueued) != 0 {
		t.Fatalf("%v %v", err, fp.enqueued)
	}
	fp, _, _, err = repairOver(t, ds, repairMissing(ds, "2026-11-06"))
	if err != nil {
		t.Fatal(err)
	}
	wantRequests(t, fp, "earnings_refresh", "2026-11-06")
}

func TestRepairExistingRequestIsKept(t *testing.T) {
	t.Parallel()
	const ds = "obs.fx_rate"
	fp := newFakePoster()
	fp.outcome = jobrequest.EnqueueExisting
	first, _, err := repairGaps(t.Context(), repairCoverage{[]coverage.Cell{repairMissing(ds, "2026-11-04")}}, fp, ds, fakeClock{postNow})
	if err != nil || first != 5 || len(fp.enqueued) != 1 {
		t.Fatalf("%v %d %v", err, first, fp.enqueued)
	}
}

func TestRepairCountEqualsGapDays(t *testing.T) {
	t.Parallel()
	const ds = "obs.price_bar"
	cells := []coverage.Cell{repairMissing(ds, "2026-11-02"), repairCellOf(ds, "2026-11-03", coverage.StateOK),
		repairCellOf(ds, "2026-11-04", coverage.StateExpired), repairMissing(ds, "2026-11-05")}
	fp, _, _, err := repairOver(t, ds, cells...)
	if err != nil {
		t.Fatal(err)
	}
	job := repairJobOf(ds)
	page := data_health_page.DatasetCells(cells, ds, []contracts.Day{"2026-11-02", "2026-11-03", "2026-11-04", "2026-11-05", "2026-11-06"})
	data_health_page.Apply(page, job, &data_health_page.Form{}, postNow, "2026-11-06")
	if gap := len(data_health_page.GapDays(page)); gap != 3 || len(fp.enqueued) != gap {
		t.Fatalf("gap_days %d, queued %d", gap, len(fp.enqueued))
	}
}

// repairSite is a site whose coverage opener gives cells and whose poster is the fake (or absent).
func repairSite(t *testing.T, withPoster bool, cells ...coverage.Cell) (http.Handler, *fakePoster) {
	t.Helper()
	a, _ := testApp(t, underlyingsEnv+"\n"+pagesDSNName+"=postgres://pages\n")
	fp := newFakePoster()
	openers := jobRequestOpeners{}
	if withPoster {
		openers.Poster = func(context.Context, string) (jobrequest.Poster, error) { return fp, nil }
	}
	a.deps.ext = map[string]any{
		jobConsoleExtKey: openers,
		extDataHealth: dataHealthDeps{openCoverage: func(context.Context, string) (coverage.Reader, error) {
			return repairCoverage{cells}, nil
		}},
	}
	rd := siteReaders{edit: &ownerEdit{form: newDeps().newSettingsForm(nil, ownerTokenTTL, newDeps().newRuleParams()), token: postKey}}
	return siteHandler(a, fakeClock{postNow}, "", rd, echoHandler, http.NotFoundHandler()), fp
}

func TestRepairIgnoresFormDays(t *testing.T) {
	t.Parallel()
	const ds = "obs.fx_rate"
	h, fp := repairSite(t, true, repairMissing(ds, "2026-11-04"))
	rec := jcPost(t, h, postKey, url.Values{"action": {"repair"}, "dataset": {ds}, "day": {"2026-11-02"}, "days": {"2026-11-03"}, "force": {"on"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/job-console?request=101" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
	wantRequests(t, fp, "fx_refresh", "2026-11-04")
}

func TestRepairNothingQueued(t *testing.T) {
	t.Parallel()
	h, fp := repairSite(t, true)
	rec := jcPost(t, h, postKey, url.Values{"action": {"repair"}, "dataset": {"obs.fx_rate"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/data-health?repair=none" || len(fp.enqueued) != 0 {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec = jcPost(t, h, postKey, url.Values{"action": {"repair"}, "dataset": {"no.such"}}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown dataset: %d", rec.Code)
	}
}

func TestRepairNoPoster(t *testing.T) {
	t.Parallel()
	h, _ := repairSite(t, false)
	rec := jcPost(t, h, postKey, url.Values{"action": {"repair"}, "dataset": {"obs.fx_rate"}})
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), textStoreFailure) {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
}

func TestRepairAnswerNeverSaysRepaired(t *testing.T) {
	t.Parallel()
	const ds = "obs.fx_rate"
	h, _ := repairSite(t, true, repairMissing(ds, "2026-11-04"))
	for _, cells := range []string{ds, "no.such"} {
		rec := jcPost(t, h, postKey, url.Values{"action": {"repair"}, "dataset": {cells}})
		if s := strings.ToLower(rec.Body.String() + rec.Header().Get("Location")); strings.Contains(s, "repaired") {
			t.Fatalf("answer %q", s)
		}
	}
	for _, s := range []string{textUnknownDataset, repairNonePath} {
		if strings.Contains(s, "repaired") {
			t.Fatalf("text %q", s)
		}
	}
}
