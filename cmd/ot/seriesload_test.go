package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/serieshistory"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeLoader struct {
	plans []sources.FetchPlan
	res   []serieshistory.SeriesLoaded
	err   error
}

func (f *fakeLoader) Load(_ context.Context, p sources.FetchPlan) ([]serieshistory.SeriesLoaded, error) {
	f.plans = append(f.plans, p)
	return f.res, f.err
}

type typedErr struct{ msg string }

func (e typedErr) Error() string             { return e.msg }
func (e typedErr) Kind() contracts.ErrorKind { return contracts.Retryable }

const loadEnv = "OT_PG_DSN_SERIES_FRED=ser-dsn\nFRED_API_KEY=k\n"

func seriesLoadApp(t *testing.T, env string, f *fakeLoader) (*app, *bytes.Buffer, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	a, _ := testApp(t, env)
	var stdout bytes.Buffer
	a.stdout = &stdout
	opened, closed := new(atomic.Int32), new(atomic.Int32)
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		opened.Add(1)
		return fakeSeries{n: closed}, nil
	}
	a.deps.newFRED = func(sources.FREDConfig) (sources.Source, error) { return emptySource{}, nil }
	a.deps.newSeriesHistory = func(sources.Source, store.SeriesWriter) (serieshistory.SeriesHistoryLoader, error) { return f, nil }
	return a, &stdout, opened, closed
}

func TestSeriesLoadPlanAndOutput(t *testing.T) {
	t.Parallel()
	f := &fakeLoader{res: []serieshistory.SeriesLoaded{{SeriesID: "SP500", Fetched: 3}, {SeriesID: "VIXCLS", Fetched: 0}}}
	a, stdout, opened, closed := seriesLoadApp(t, loadEnv, f)
	args := []string{"--series", "SP500", "--series", "VIXCLS", "--from", "2021-01-01", "--to", "2026-01-01"}
	if err := seriesLoad(a, args); err != nil {
		t.Fatal(err)
	}
	if len(f.plans) != 1 {
		t.Fatalf("Load called %d times", len(f.plans))
	}
	p := f.plans[0]
	if strings.Join(p.SeriesIDs, ",") != "SP500,VIXCLS" || p.From != "2021-01-01" || p.To != "2026-01-01" {
		t.Fatalf("plan %+v", p)
	}
	if got, want := stdout.String(), "SP500 fetched 3\nVIXCLS fetched 0\n"; got != want {
		t.Fatalf("stdout %q, want %q", got, want)
	}
	if opened.Load() != 1 || closed.Load() != 1 {
		t.Fatalf("opened %d closed %d", opened.Load(), closed.Load())
	}
}

func TestSeriesLoadRefusals(t *testing.T) {
	t.Parallel()
	ok := []string{"--series", "SP500", "--from", "2021-01-01", "--to", "2026-01-01"}
	for _, tc := range []struct {
		name string
		args []string
		env  string
		demo bool
		want string
	}{
		{"no series", ok[2:], loadEnv, false, "--series"},
		{"no from", []string{"--series", "A", "--to", "2026-01-01"}, loadEnv, false, "--from"},
		{"no to", []string{"--series", "A", "--from", "2026-01-01"}, loadEnv, false, "--to"},
		{"bad from", []string{"--series", "A", "--from", "x", "--to", "2026-01-01"}, loadEnv, false, "--from"},
		{"bad to", []string{"--series", "A", "--from", "2026-01-01", "--to", "2026-13-40"}, loadEnv, false, "--to"},
		{"demo", ok, loadEnv, true, "series-load needs a real profile; the demo profile opens no store"},
		{"no key", ok, "OT_PG_DSN_SERIES_FRED=s\n", false, "FRED_API_KEY"},
		{"no dsn", ok, "FRED_API_KEY=k\n", false, "OT_PG_DSN_SERIES_FRED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := &fakeLoader{}
			a, stdout, opened, _ := seriesLoadApp(t, tc.env, f)
			if tc.demo {
				a.profile.Name = "demo"
			}
			err := seriesLoad(a, tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err %v, want text %q", err, tc.want)
			}
			if len(f.plans) != 0 || opened.Load() != 0 || stdout.Len() != 0 {
				t.Fatalf("calls %d opened %d stdout %q", len(f.plans), opened.Load(), stdout.String())
			}
		})
	}
}

func TestSeriesLoadErrorByClassOnly(t *testing.T) {
	t.Parallel()
	args := []string{"--series", "SP500", "--from", "2021-01-01", "--to", "2026-01-01"}
	f := &fakeLoader{err: typedErr{msg: "GET https://x?api_key=sentinel-key failed"}}
	a, stdout, _, closed := seriesLoadApp(t, loadEnv, f)
	err := seriesLoad(a, args)
	if err == nil || !strings.Contains(err.Error(), "retryable") || strings.Contains(err.Error(), "sentinel") {
		t.Fatalf("err %v", err)
	}
	if stdout.Len() != 0 || closed.Load() != 1 {
		t.Fatalf("stdout %q closed %d", stdout.String(), closed.Load())
	}
	f.err = errors.New("plain sentinel-key")
	err = seriesLoad(a, args)
	if err == nil || strings.Contains(err.Error(), "sentinel") {
		t.Fatalf("err %v", err)
	}
}
