package main

import (
	"bytes"
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

// planJob is a job that plans the given keys.
type planJob struct {
	contracts.Job
	id   string
	keys []contracts.ExpectedKey
}

func (j planJob) ID() string { return j.id }
func (j planJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return j.keys, "h", nil
}

// readLedger answers the day read-back of printOutcomes.
type readLedger struct {
	store.OpsLedger
	present map[contracts.ExpectedKey]store.PresentStatus
	runs    []store.Run
}

func (l readLedger) Present(context.Context, contracts.Day) (map[contracts.ExpectedKey]store.PresentStatus, error) {
	return l.present, nil
}
func (readLedger) Cursor(context.Context) (store.Cursor, error)               { return store.Cursor{}, nil }
func (readLedger) Close() error                                               { return nil }
func (l readLedger) Runs(context.Context, contracts.Day) ([]store.Run, error) { return l.runs, nil }

func key(d, s string) contracts.ExpectedKey {
	return contracts.ExpectedKey{DatasetID: d, SubjectKey: s}
}

func TestRunOncePrintsOutcomePerKey(t *testing.T) {
	t.Parallel()
	ka, kb, kc, kd, ke := key("d", "a"), key("d", "b"), key("d", "c"), key("d", "d"), key("d", "e")
	t0 := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	l := readLedger{
		present: map[contracts.ExpectedKey]store.PresentStatus{ka: store.StatusPresent, kb: store.StatusNoValue},
		runs: []store.Run{
			{Key: ka, UsableN: 1, FinishedAt: t0},
			{Key: ka, UsableN: 7, FinishedAt: t0.Add(time.Minute)},
			{Key: kc, ReasonCode: "empty_before_close", FinishedAt: t0},
			{Key: kd, ErrorCode: "retryable", FinishedAt: t0},
		},
	}
	js := []contracts.Job{planJob{id: "j1", keys: []contracts.ExpectedKey{ka, kb, kc}}, planJob{id: "j2", keys: []contracts.ExpectedKey{kd, ke}}}
	var out bytes.Buffer
	if err := printOutcomes(t.Context(), &out, l, js, "2026-03-05", ""); err != nil {
		t.Fatal(err)
	}
	want := "job=j1 day=2026-03-05 dataset=d subject=a outcome=captured rows=7\n" +
		"job=j1 day=2026-03-05 dataset=d subject=b outcome=no_value reason=no_value\n" +
		"job=j1 day=2026-03-05 dataset=d subject=c outcome=empty reason=before_close\n" +
		"job=j2 day=2026-03-05 dataset=d subject=d outcome=failed reason=retryable\n" +
		"job=j2 day=2026-03-05 dataset=d subject=e outcome=missing\n" +
		"run-once 2026-03-05 status=ok captured=1 keys=5\n"
	if out.String() != want {
		t.Fatalf("got\n%s\nwant\n%s", out.String(), want)
	}
}

func TestRunOnceNothingCapturedStatus(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, fullEnv)
	var out bytes.Buffer
	a.stdout = &out
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	k := key("fred", "DGS10")
	a.deps.openLedger = func(context.Context, string) (store.OpsLedger, error) {
		return readLedger{present: map[contracts.ExpectedKey]store.PresentStatus{k: store.StatusNoValue}}, nil
	}
	a.deps.newFredSeries = func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error) {
		return planJob{id: "fred_series", keys: []contracts.ExpectedKey{k}}, nil
	}
	if err := runOnce(a, []string{"--day", "2026-03-05"}); err != nil {
		t.Fatalf("exit must stay 0: %v", err)
	}
	if !strings.Contains(out.String(), "run-once 2026-03-05 status=nothing_captured captured=0 keys=1\n") ||
		!strings.Contains(out.String(), "job=fred_series day=2026-03-05 dataset=fred subject=DGS10 outcome=no_value reason=no_value\n") {
		t.Fatalf("out %q", out.String())
	}
}

func TestRunOnceLeftOutLine(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, strings.Replace(fullEnv, "ALPHA_VANTAGE_API_KEY="+alphaTestKey+"\n", "", 1))
	var out bytes.Buffer
	a.stdout = &out
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	if err := runOnce(a, []string{"--day", "2026-03-05"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "job=earnings_refresh day=2026-03-05 outcome=left_out reason=ALPHA_VANTAGE_API_KEY is not set\n") {
		t.Fatalf("out %q", out.String())
	}
}

// The fakes of runonce_option_test.go answer the read-back of printOutcomes with an empty day.
func (evLedger) Present(context.Context, contracts.Day) (map[contracts.ExpectedKey]store.PresentStatus, error) {
	return nil, nil
}
func (evLedger) Runs(context.Context, contracts.Day) ([]store.Run, error) { return nil, nil }
func (namedJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
