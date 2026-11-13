package main

import (
	"bytes"
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
)

// dayCandidates answers the rows of the day asked for only, as the store does.
type dayCandidates struct {
	fakeCandidateRW
	rows []store.Candidate
}

func (d *dayCandidates) Candidates(_ context.Context, day contracts.Day, _ string) ([]store.Candidate, error) {
	var out []store.Candidate
	for _, r := range d.rows {
		if r.SessionDay == day {
			out = append(out, r)
		}
	}
	return out, nil
}

// fakeSessionDays holds the XNYS session days 2026-11-05 and 2026-11-06 and the weekend after them.
type fakeSessionDays struct {
	store.SessionDayStore
	asked *atomic.Int32
}

func (f fakeSessionDays) SessionDays(_ context.Context, cal string, _, _ contracts.Day) ([]contracts.SessionDay, error) {
	f.asked.Add(1)
	var out []contracts.SessionDay
	for _, d := range []struct {
		day     string
		session bool
	}{{"2026-11-05", true}, {"2026-11-06", true}, {"2026-11-07", false}, {"2026-11-08", false}} {
		t, _ := time.Parse(time.DateOnly, d.day)
		out = append(out, contracts.SessionDay{CalendarID: cal, SessionDay: types.SerializableDate{Time: t}, IsSession: d.session})
	}
	return out, nil
}

func (fakeSessionDays) Close() error { return nil }

// compareRig is an app whose two readers hold the given rows for 2026-11-05.
func compareRig(t *testing.T, old []store.Candidate, res *fakeResultStore) (*app, *bytes.Buffer, *atomic.Int32) {
	t.Helper()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
	var out bytes.Buffer
	a.stdout = &out
	var closed atomic.Int32
	res.closed = &closed
	a.deps.openCandidate = func(_ context.Context, dsn string) (candidateStore, error) {
		if dsn != "pages-dsn" {
			t.Errorf("dsn %q", dsn)
		}
		return &dayCandidates{fakeCandidateRW: fakeCandidateRW{closed: &closed}, rows: old}, nil
	}
	a.deps.openStrategyResults = func(_ context.Context, dsn string) (resultStore, error) {
		if dsn != "pages-dsn" {
			t.Errorf("dsn %q", dsn)
		}
		return res, nil
	}
	a.deps.openSessionDay = func(_ context.Context, dsn string) (store.SessionDayStore, error) {
		if dsn != "pages-dsn" {
			t.Errorf("dsn %q", dsn)
		}
		return fakeSessionDays{asked: new(atomic.Int32)}, nil
	}
	return a, &out, &closed
}

// oldRowAndResult is one short put row and the result row that rebuilds it.
func oldRowAndResult() (store.Candidate, strategyresult.Row) {
	yield := "0.12"
	old := store.Candidate{SessionDay: "2026-11-05", RunID: "r1", Kind: "short_put", Issuer: "cik:0000320193", InstrumentID: 11,
		Expiry: "2026-12-18", Strike: "95", Status: "ok", Marks: "[]", Gates: "[]", AnnualizedYield: &yield}
	row := strategyresult.Row{SessionDay: "2026-11-05", RunID: "r1", Strategy: "short_put", Key: "11", UnderlyingID: 1, Status: "ok",
		Legs: "[]", Checks: "[]", Facts: "{}"}
	return old, row
}

func TestStrategyCompareEqualPrintsEqual(t *testing.T) {
	t.Parallel()
	a, out, closed := compareRig(t, nil, &fakeResultStore{})
	if err := strategyCompareCmd(a, []string{"--day", "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "equal rows=0" {
		t.Fatalf("output %q", got)
	}
	if closed.Load() != 2 {
		t.Fatalf("closed %d, want both stores", closed.Load())
	}
}

func TestStrategyCompareDifferenceExitsNonZero(t *testing.T) {
	t.Parallel()
	old, _ := oldRowAndResult()
	a, out, _ := compareRig(t, []store.Candidate{old}, &fakeResultStore{})
	err := strategyCompareCmd(a, []string{"--day", "2026-11-05"})
	if err == nil {
		t.Fatal("a difference must be an error")
	}
	if !strings.Contains(out.String(), "only in the old rows") || strings.Contains(out.String(), "equal") {
		t.Fatalf("output %q", out.String())
	}
}

func TestStrategyCompareDemoRefused(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	err := strategyCompareCmd(a, nil)
	if err == nil || !strings.Contains(err.Error(), "demo profile") {
		t.Fatalf("error %v", err)
	}
}

func TestStrategyCompareOpenErrorWithheld(t *testing.T) {
	t.Parallel()
	a, _, _ := compareRig(t, nil, &fakeResultStore{})
	a.deps.openStrategyResults = func(context.Context, string) (resultStore, error) {
		return nil, context.DeadlineExceeded
	}
	err := strategyCompareCmd(a, nil)
	if err == nil || strings.Contains(err.Error(), "pages-dsn") || !strings.Contains(err.Error(), pagesDSNName) {
		t.Fatalf("error %v", err)
	}
}

func TestStrategyCompareDefaultsToNewestSessionDay(t *testing.T) {
	t.Parallel()
	old, _ := oldRowAndResult()
	old.SessionDay = "2026-11-06" // the newest session day; no result row for it -> a difference
	a, out, _ := compareRig(t, []store.Candidate{old}, &fakeResultStore{})
	a.deps.newClock = func() clock.Clock { return fakeClock{time.Date(2026, 11, 8, 15, 0, 0, 0, time.UTC)} } // Sunday
	if err := strategyCompareCmd(a, nil); err == nil {
		t.Fatalf("default day skipped the newest session day: %q", out.String())
	}
	if !strings.Contains(out.String(), "only in the old rows") {
		t.Fatalf("output %q", out.String())
	}
}

func TestStrategyCompareNoSessionDayIsAnError(t *testing.T) {
	t.Parallel()
	a, out, _ := compareRig(t, nil, &fakeResultStore{})
	a.deps.newClock = func() clock.Clock { return fakeClock{time.Date(2026, 11, 8, 15, 0, 0, 0, time.UTC)} }
	a.deps.openSessionDay = func(context.Context, string) (store.SessionDayStore, error) {
		return emptySessionDays{}, nil
	}
	if err := strategyCompareCmd(a, nil); err == nil || strings.Contains(out.String(), "equal") {
		t.Fatalf("error %v output %q", err, out.String())
	}
}

type emptySessionDays struct{ store.SessionDayStore }

func (emptySessionDays) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	return nil, nil
}
func (emptySessionDays) Close() error { return nil }
