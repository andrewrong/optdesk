package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

var netNow = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func netPoint(id, day string) contracts.SeriesPoint {
	d, _ := time.Parse("2006-01-02", day)
	v := 1.5
	return contracts.SeriesPoint{SeriesID: id, ObservationDate: types.SerializableDate{Time: d}, Value: &v}
}

// netSource answers every series with the days it holds; it counts its calls.
type netSource struct {
	days  []string
	calls int
	err   error
}

func (s *netSource) Fetch(_ context.Context, plan sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	s.calls++
	var out []contracts.SeriesPoint
	for _, id := range plan.SeriesIDs {
		for _, d := range s.days {
			out = append(out, netPoint(id, d))
		}
	}
	return out, s.err
}

type netSeries struct{ days []string }

func (s netSeries) SeriesPoints(_ context.Context, id string, _, _ contracts.Day) ([]contracts.SeriesPoint, error) {
	var out []contracts.SeriesPoint
	for _, d := range s.days {
		out = append(out, netPoint(id, d))
	}
	return out, nil
}
func (netSeries) InsertSeriesPoints(context.Context, []contracts.SeriesPoint) error { return nil }
func (netSeries) Close() error                                                      { return nil }

func runNet(t *testing.T, net bool, key string, src sources.Source, stored []string, exps []store.Expectation) string {
	t.Helper()
	var out bytes.Buffer
	a := checkApp(t, map[string]string{"OT_HOME": "/h", store.DSNEnvPages: checkSecretDSN, "FRED_API_KEY": key}, &out, nil, nil)
	a.profile.Zone = time.UTC
	a.deps.newFRED = func(sources.FREDConfig) (sources.Source, error) { return src, nil }
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) { return netSeries{days: stored}, nil }
	e := &checkEnv{a: a, opts: checkOpts{Net: net}, c: &checker{w: &out}}
	checkFredNet(context.Background(), e, netNow, exps)
	return out.String()
}

type failSource struct{ t *testing.T }

func (f failSource) Fetch(context.Context, sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	f.t.Fatal("the source was called without --net")
	return nil, nil
}

func TestCheckNetOffline(t *testing.T) {
	t.Parallel()
	out := runNet(t, false, "k", failSource{t}, nil, nil)
	if strings.TrimSpace(out) != "EXPLAINED fred.public 2026-11-10 offline: pass --net" {
		t.Errorf("offline line: %q", out)
	}
}

func TestCheckNetStale(t *testing.T) {
	t.Parallel()
	src := &netSource{days: []string{"2026-11-06", "2026-11-09"}}
	closed := []store.Expectation{{DatasetID: "obs.series_point", ExpectedDay: "2026-11-09", PresentDay: "2026-11-06", DueBy: netNow.Add(-time.Hour)}}
	out := runNet(t, true, "k", src, []string{"2026-11-06"}, closed)
	if !strings.Contains(out, "FAIL fred.DGS10 2026-11-09 stored 2026-11-06 is older than public 2026-11-09") {
		t.Errorf("stale: %s", out)
	}
	out = runNet(t, true, "k", src, []string{"2026-11-09"}, closed)
	if strings.Contains(out, "FAIL") || !strings.Contains(out, "PASS fred.DGS10 2026-11-09") {
		t.Errorf("equal days: %s", out)
	}
	out = runNet(t, true, "", src, nil, nil)
	if !strings.Contains(out, "FAIL fred.public") || !strings.Contains(out, "FRED_API_KEY") {
		t.Errorf("no key: %s", out)
	}
	out = runNet(t, true, "k", &netSource{err: context.DeadlineExceeded}, nil, nil)
	if !strings.Contains(out, "FAIL fred.public") {
		t.Errorf("source error: %s", out)
	}
}

func TestCheckNetWindowOpen(t *testing.T) {
	t.Parallel()
	src := &netSource{days: []string{"2026-11-09"}}
	open := []store.Expectation{{DatasetID: "obs.series_point", ExpectedDay: "2026-11-09", PresentDay: "2026-11-06", DueBy: netNow.Add(time.Hour)}}
	out := runNet(t, true, "k", src, []string{"2026-11-06"}, open)
	if !strings.Contains(out, "EXPLAINED fred.DGS10 2026-11-09 window_open") || strings.Contains(out, "FAIL") {
		t.Errorf("window open: %s", out)
	}
}

func TestCheckNetOldStaleNotExplainedByLaterWindow(t *testing.T) {
	t.Parallel()
	src := &netSource{days: []string{"2026-11-03"}}
	exps := []store.Expectation{
		{DatasetID: "obs.series_point", ExpectedDay: "2026-11-03", PresentDay: "2026-10-30", DueBy: netNow.Add(-72 * time.Hour)},
		{DatasetID: "obs.series_point", ExpectedDay: "2026-11-09", PresentDay: "2026-10-30", DueBy: netNow.Add(time.Hour)},
	}
	out := runNet(t, true, "k", src, []string{"2026-10-30"}, exps)
	if !strings.Contains(out, "FAIL fred.DGS10 2026-11-03") {
		t.Errorf("a stale day whose window closed must FAIL: %s", out)
	}
}
