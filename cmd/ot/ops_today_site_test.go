package main

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/ops"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

// fakeRunStore is the run store of a test: it answers fixed rows and counts its writes and closes.
type fakeRunStore struct {
	runs      []runview.Row
	reasons   []runview.Reason
	notWired  []runview.Reason
	writes    atomic.Int32
	closed    atomic.Int32
	openedDSN string
}

func (f *fakeRunStore) RunsOn(context.Context, contracts.Day) ([]runview.Row, error) {
	return f.runs, nil
}

func (f *fakeRunStore) ReasonsOn(context.Context, contracts.Day) ([]runview.Reason, error) {
	return f.reasons, nil
}

func (f *fakeRunStore) NotWired(context.Context) ([]runview.Reason, error) { return f.notWired, nil }

func (f *fakeRunStore) ReplaceReasons(context.Context, contracts.Day, string, string, []runview.Reason) error {
	f.writes.Add(1)
	return nil
}

func (f *fakeRunStore) Close() error { f.closed.Add(1); return nil }

var _ ops.RunStore = (*fakeRunStore)(nil)

func opsTodayApp(t *testing.T, st *fakeRunStore) *app {
	t.Helper()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=postgres://pages\n")
	a.deps.openRuns = func(_ context.Context, dsn string) (ops.RunStore, error) {
		st.openedDSN = dsn
		return st, nil
	}
	return a
}

func opsTodayFixture() *fakeRunStore {
	fin := time.Date(2026, 11, 13, 11, 15, 0, 0, time.UTC)
	return &fakeRunStore{
		runs: []runview.Row{{SessionDay: "2026-11-13", Dataset: "master.option_list", Key: "preopen", Status: "ok",
			FinishedAt: fin, Returned: 4, Usable: 3}},
		notWired: []runview.Reason{{Day: runview.NotWiredDay, Dataset: runview.NotWiredDataset, Subject: "scheduler",
			Scope: "option_open_plan", Code: "not_wired", Detail: "OT_PG_DSN_OPTION_OPEN"}},
	}
}

func TestOpsTodaySiteServesPayload(t *testing.T) {
	t.Parallel()
	st := opsTodayFixture()
	a := opsTodayApp(t, st)
	f, ok := pageFamilyOfRoute(siteOpsTodayRoute)
	if !ok || f.name != opsTodayName || f.build == nil {
		t.Fatalf("route %q: %+v", siteOpsTodayRoute, f)
	}
	var closers []io.Closer
	track := func(c io.Closer, err error) bool {
		if err != nil {
			return false
		}
		closers = append(closers, c)
		return true
	}
	v, ok := siteOpeners[siteOpsTodayKey](t.Context(), a, track)
	if !ok || v == nil || st.openedDSN != "postgres://pages" || len(closers) != 1 {
		t.Fatalf("opener: %v %v dsn %q closers %d", v, ok, st.openedDSN, len(closers))
	}
	b, err := f.build(t.Context(), a, fakeClock{time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)}, "2026-11-13",
		siteReaders{ext: map[string]any{siteOpsTodayKey: v}})
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Schema string
		Today  struct{ Rows []map[string]any }
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.Schema != opsTodayName || len(m.Today.Rows) != 2 {
		t.Fatalf("payload %s", b)
	}
	if r := m.Today.Rows[0]; r["dataset"] != "master.option_list" || r["key"] != "preopen" || r["usable_n"] != float64(3) {
		t.Errorf("run row %v", r)
	}
	if r := m.Today.Rows[1]; r["status"] != "not_wired" || r["key"] != "option_open_plan" || r["detail"] != "OT_PG_DSN_OPTION_OPEN" {
		t.Errorf("not_wired row %v", r)
	}
}

func TestOpsTodaySiteIsReadOnly(t *testing.T) {
	t.Parallel()
	st := opsTodayFixture()
	a := opsTodayApp(t, st)
	v, ok := siteOpeners[siteOpsTodayKey](t.Context(), a, func(io.Closer, error) bool { return true })
	if !ok {
		t.Fatal("opener failed")
	}
	if _, writes := v.(runview.ReasonWriter); writes {
		t.Fatalf("the page reader %T can write reasons", v)
	}
	if _, err := buildOpsToday(t.Context(), a, fakeClock{time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)}, "",
		siteReaders{ext: map[string]any{siteOpsTodayKey: v}}); err != nil {
		t.Fatal(err)
	}
	// ot page gets the same reader and closes the store it opened.
	p, closeFn, err := opsTodayPageNamed(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Build(t.Context(), nil, "2026-11-13", time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	closeFn()
	if st.writes.Load() != 0 || st.closed.Load() != 1 {
		t.Fatalf("writes %d closes %d", st.writes.Load(), st.closed.Load())
	}
	if !strings.Contains(p.Meta.Schema, "ops_today_page") {
		t.Fatalf("schema %q", p.Meta.Schema)
	}
}
