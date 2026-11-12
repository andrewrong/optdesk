package main

import (
	"context"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/earningsrefresh"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
	"github.com/andrewrong/optdesk/internal/symbol"
)

// jqTestKey is the J-Quants key of the tests; it must never reach a log line or an error.
const jqTestKey = "jq-test-secret"

// jqEnv is the rig's .env with the J-Quants line.
func jqEnv() string { return fullEnv + "JQUANTS_API_KEY=" + jqTestKey + "\n" }

const jpWatch = "[[underlying]]\nissuer = \"cik:0000320193\"\nmarket = \"US\"\nsymbol = \"AAPL\"\n" +
	"[[underlying]]\nissuer = \"edinet:E02144\"\nmarket = \"JP\"\nsymbol = \"7203\"\n" +
	"[[underlying]]\nissuer = \"edinet:E01777\"\nmarket = \"JP\"\nsymbol = \"6758\"\n"

// jqSource is a J-Quants source over a fake client: one client call per symbol and Fetch.
type jqSource struct {
	cfg     sources.EarningsConfig
	fetches atomic.Int32
	calls   atomic.Int32
	err     error
}

type jqErr struct{}

func (jqErr) Error() string { return "jquants: boom" }

func (jqErr) Kind() contracts.ErrorKind { return contracts.Retryable }

func (jqErr) ErrorCode() string { return "http_status" }

func (s *jqSource) Fetch(context.Context) ([]sources.EarningsListing, error) {
	s.fetches.Add(1)
	for range s.cfg.Symbols {
		s.calls.Add(1)
		if s.err != nil {
			return nil, s.err
		}
	}
	return nil, nil
}

// jqRig wires run-once with the real earnings_refresh job over an in-memory store and a counting J-Quants source.
func jqRig(t *testing.T, env string) (*rig, *jqSource, *memEvents) {
	t.Helper()
	r := newRig(t, env)
	r.list.rows = listRowsFromWatch(t, jpWatch)
	src, mem := &jqSource{}, &memEvents{}
	d := &r.a.deps
	d.newJQuants = func(cfg sources.EarningsConfig) (sources.EarningsSource, error) {
		src.cfg = cfg
		return src, nil
	}
	d.newAlphaVantage = func(sources.EarningsConfig) (sources.EarningsSource, error) { return &jqSource{}, nil }
	d.openEarningsDates = func(context.Context, string) (store.EarningsDateStore, error) { return mem, nil }
	d.newEarningsRefresh = func(
		srcs map[string]sources.EarningsSource, w store.EarningsDateWriter, rd store.EarningsDateReader, es []jobs.WatchEntry, c clock.Clock, _ symbol.Translator,
	) (contracts.Job, error) {
		r.earnSrcs = srcs
		return earningsrefresh.New(srcs, w, rd, es, earningsTestClock{})
	}
	return r, src, mem
}

// earningsTestClock is 2026-11-09 in New York, the day these tests run the earnings job: it answers for today only.
type earningsTestClock struct{}

func (earningsTestClock) Now() time.Time { return time.Date(2026, 11, 9, 21, 30, 0, 0, time.UTC) }

func earningsJob(t *testing.T, r *rig) contracts.Job {
	t.Helper()
	for _, j := range r.jobs {
		if j.ID() == "earnings_refresh" {
			return j
		}
	}
	t.Fatalf("earnings_refresh not wired: %v", jobIDs(r.jobs))
	return nil
}

// planDay plans the job for day, as the reconciler does before it runs a key: a listed job runs the snapshot of a Plan.
func planDay(t *testing.T, j contracts.Job, day contracts.Day) {
	t.Helper()
	if _, _, err := j.Plan(t.Context(), day); err != nil {
		t.Fatal(err)
	}
}

func rowsOf(m *memEvents, market string) []contracts.EarningsDate {
	var out []contracts.EarningsDate
	for _, x := range m.rows {
		if string(x.Market) == market {
			out = append(out, x)
		}
	}
	return out
}

func TestJPSourceWired(t *testing.T) {
	t.Parallel()
	r, src, _ := jqRig(t, jqEnv())
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	if r.earnSrcs["JP"] != sources.EarningsSource(src) || r.earnSrcs["US"] == nil {
		t.Fatalf("sources %v", r.earnSrcs)
	}
	if src.cfg.APIKey != jqTestKey || src.cfg.BaseURL != jquantsBaseURL {
		t.Fatalf("source config has the wrong key or host")
	}
	if want := []string{"7203", "6758"}; !slices.Equal(src.cfg.Symbols, want) {
		t.Fatalf("symbols %v, want %v", src.cfg.Symbols, want)
	}
	if strings.Contains(r.log.String(), jqTestKey) {
		t.Fatal("the key reached the log")
	}
}

// The J-Quants source is built at each Plan from the JP names of the store that pass the filter at that Plan: a paused
// name drops out and a new active one comes in, with no restart.
func TestJPSourceFollowsTheStoredList(t *testing.T) {
	t.Parallel()
	r, src, _ := jqRig(t, jqEnv())
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	j := earningsJob(t, r)
	rows := r.list.rows
	for i := range rows {
		if rows[i].Symbol == "7203" {
			rows[i].Status = underlying.StatusPaused
		}
	}
	rows = append(rows, listRowsOf(jobs.WatchEntry{Issuer: "edinet:E02778", Market: "JP",
		Contract: broker.Contract{SecType: "STK", Symbol: "9984"}})...)
	r.list.rows = rows
	if _, _, err := j.Plan(context.Background(), "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"6758", "9984"}; !slices.Equal(src.cfg.Symbols, want) {
		t.Fatalf("symbols %v, want %v", src.cfg.Symbols, want)
	}
}

func TestMissingJQuantsKeyNamedLine(t *testing.T) {
	t.Parallel()
	r, src, mem := jqRig(t, fullEnv)
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.earnSrcs["JP"]; ok || r.earnSrcs["US"] == nil {
		t.Fatalf("only the JP source is left out: %v", r.earnSrcs)
	}
	if !strings.Contains(r.log.String(), "JQUANTS_API_KEY is not set") {
		t.Fatalf("log %q", r.log.String())
	}
	planDay(t, earningsJob(t, r), "2026-11-09")
	res := earningsJob(t, r).Run(t.Context(), "2026-11-09", nil)
	if res.Err != nil || src.fetches.Load() != 0 {
		t.Fatalf("err %v fetches %d", res.Err, src.fetches.Load())
	}
	jp := rowsOf(mem, "JP")
	if len(jp) != 2 || jp[0].Status != "unsupported" || jp[0].Source != "none" || jp[1].Status != "unsupported" {
		t.Fatalf("JP rows %+v", jp)
	}
}

func TestJQuantsFailureOnlyJPRows(t *testing.T) {
	t.Parallel()
	r, src, mem := jqRig(t, jqEnv())
	src.err = jqErr{}
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	planDay(t, earningsJob(t, r), "2026-11-09")
	earningsJob(t, r).Run(t.Context(), "2026-11-09", nil)
	jp, us := rowsOf(mem, "JP"), rowsOf(mem, "US")
	if len(jp) != 2 || jp[0].Status != "error" || jp[1].Status != "error" || jp[0].Source != "jquants" {
		t.Fatalf("JP rows %+v", jp)
	}
	if len(us) != 1 || us[0].Status == "error" {
		t.Fatalf("US rows %+v", us)
	}
}

func TestJQuantsOneCallPerDay(t *testing.T) {
	t.Parallel()
	r, src, _ := jqRig(t, jqEnv())
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	j := earningsJob(t, r)
	planDay(t, j, "2026-11-09")
	for i := 0; i < 2; i++ {
		if res := j.Run(t.Context(), "2026-11-09", nil); res.Err != nil {
			t.Fatalf("tick %d: %v", i, res.Err)
		}
	}
	if src.fetches.Load() != 1 || src.calls.Load() != 2 {
		t.Fatalf("fetches %d calls %d, want 1 and 2 (one per JP entry)", src.fetches.Load(), src.calls.Load())
	}
}

func TestJQuantsErrorDayIsFetchedAgain(t *testing.T) {
	t.Parallel()
	r, src, _ := jqRig(t, jqEnv())
	src.err = jqErr{}
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	j := earningsJob(t, r)
	planDay(t, j, "2026-11-09")
	j.Run(t.Context(), "2026-11-09", nil)
	before := src.calls.Load()
	if res := j.Run(t.Context(), "2026-11-09", nil); res.Err == nil {
		t.Fatal("a second run that fails again must return the Permanent error")
	}
	if src.calls.Load() == before || before != 1 {
		t.Fatalf("calls %d then %d: an error row must be fetched again", before, src.calls.Load())
	}
}
