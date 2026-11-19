package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/strategy_replay_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

var replayTestToken = strings.Repeat("t", minOwnerTokenBytes)

// replayPostRig is a rig of the post: the replay fakes, an owner token and the handler that answers the document.
type replayPostRig struct {
	a   *app
	rd  siteReaders
	clk fakeClock
}

func newReplayPostRig(t *testing.T) replayPostRig {
	t.Helper()
	a, _ := replayRig(t)
	return replayPostRig{a: a, rd: siteReaders{edit: &ownerEdit{token: replayTestToken}},
		clk: fakeClock{time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)}}
}

// replayDoc is the part of the document the tests read.
type replayDoc struct {
	View    string `json:"view"`
	Notices []struct {
		Key string `json:"key"`
	} `json:"notices"`
	Form *struct {
		Fields []struct {
			Name, Value string
		} `json:"fields"`
	} `json:"form"`
	Report *struct {
		AsOf  string `json:"as_of"`
		Lines []struct {
			Set string `json:"set"`
		} `json:"lines"`
	} `json:"report"`
}

func (d replayDoc) noticeKeys() []string {
	var out []string
	for _, n := range d.Notices {
		out = append(out, n.Key)
	}
	return out
}

func (rg replayPostRig) post(t *testing.T, p replayPost, ctx context.Context, body url.Values, cookie bool) (int, replayDoc) {
	t.Helper()
	r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/"+strategyReplayPostRoute, strings.NewReader(body.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie {
		r.AddCookie(&http.Cookie{Name: ownerCookie, Value: replayTestToken})
	}
	w := httptest.NewRecorder()
	newHandler := func(vm []byte) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(vm) })
	}
	p.serve(rg.a, rg.clk, rg.rd, newHandler, w, r)
	var d replayDoc
	if w.Code != http.StatusForbidden && w.Code != http.StatusMethodNotAllowed {
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
			t.Fatalf("status %d body %q: %v", w.Code, w.Body.String(), err)
		}
	}
	return w.Code, d
}

// defaultBody is the post a browser sends for the form view as Build returns it.
func defaultBody(t *testing.T) url.Values {
	t.Helper()
	vm, err := strategy_replay_page.Build(t.Context(), strategy_replay_page.Input{View: strategy_replay_page.ViewForm,
		Form: ptrTo(strategy_replay_page.DefaultForm())})
	if err != nil {
		t.Fatal(err)
	}
	var d replayDoc
	if err := json.Unmarshal(vm, &d); err != nil {
		t.Fatal(err)
	}
	body := url.Values{}
	for _, f := range d.Form.Fields {
		body.Set(f.Name, f.Value)
	}
	return body
}

func ptrTo[T any](v T) *T { return &v }

var realPost = replayPost{run: replayRun, timeout: replayPostTimeout}

func TestStrategyReplayPostAnswersReportView(t *testing.T) {
	t.Parallel()
	rg := newReplayPostRig(t)
	body := defaultBody(t)
	body.Set("as_of", "2026-11-20")
	code, d := rg.post(t, realPost, t.Context(), body, true)
	if code != http.StatusOK || d.View != "report" || d.Report == nil || d.Report.AsOf != "2026-11-20" || len(d.Report.Lines) == 0 {
		t.Fatalf("status %d doc %+v", code, d)
	}
	if len(d.Form.Fields) != 6 {
		t.Errorf("the form is not answered with its fields: %+v", d.Form)
	}
}

func TestStrategyReplayPostWithoutCookieIsForbidden(t *testing.T) {
	t.Parallel()
	rg := newReplayPostRig(t)
	var calls atomic.Int32
	p := replayPost{timeout: time.Minute, run: func(context.Context, *app, replayOpts) (replayResult, error) {
		calls.Add(1)
		return replayResult{}, nil
	}}
	code, _ := rg.post(t, p, t.Context(), defaultBody(t), false)
	if code != http.StatusForbidden || calls.Load() != 0 {
		t.Fatalf("status %d, run called %d times", code, calls.Load())
	}
}

func TestStrategyReplayPostBadFields(t *testing.T) {
	t.Parallel()
	cases := []struct{ field, value, key string }{
		{"as_of", "20-11-2026", "bad_as_of"},
		{"lots", "calls", "bad_lots"},
		{"early_keep", "half", "bad_early_keep"},
		{"early_dte", "3.5", "bad_early_dte"},
		{"min_lots", "-1", "bad_min_lots"},
	}
	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			t.Parallel()
			rg := newReplayPostRig(t)
			var calls atomic.Int32
			p := replayPost{timeout: time.Minute, run: func(context.Context, *app, replayOpts) (replayResult, error) {
				calls.Add(1)
				return replayResult{}, nil
			}}
			body := defaultBody(t)
			body.Set(c.field, c.value)
			code, d := rg.post(t, p, t.Context(), body, true)
			if code != http.StatusUnprocessableEntity || strings.Join(d.noticeKeys(), ",") != c.key || d.View != "form" || calls.Load() != 0 {
				t.Fatalf("status %d keys %v view %q calls %d", code, d.noticeKeys(), d.View, calls.Load())
			}
		})
	}
}

func TestStrategyReplayPostTimedOutKeepsLockUntilRunReturns(t *testing.T) {
	t.Parallel()
	rg := newReplayPostRig(t)
	release := make(chan struct{})
	finished := make(chan struct{})
	p := replayPost{timeout: time.Minute, run: func(ctx context.Context, _ *app, _ replayOpts) (replayResult, error) {
		<-release
		defer close(finished)
		return replayResult{}, ctx.Err()
	}}
	expired, cancel := context.WithDeadline(t.Context(), rg.clk.Now().Add(-time.Second))
	defer cancel()
	code, d := rg.post(t, p, expired, defaultBody(t), true)
	if code != http.StatusOK || strings.Join(d.noticeKeys(), ",") != "timed_out" {
		t.Fatalf("status %d keys %v", code, d.noticeKeys())
	}
	if !replayGateOf(rg.a).Load() {
		t.Fatal("the lock was released before the run returned")
	}
	code, d = rg.post(t, p, t.Context(), defaultBody(t), true)
	if code != http.StatusOK || strings.Join(d.noticeKeys(), ",") != "busy" {
		t.Fatalf("second post: status %d keys %v", code, d.noticeKeys())
	}
	close(release)
	<-finished
	if !waitGate(rg.a) {
		t.Fatal("the lock was not released after the run returned")
	}
}

// waitGate yields until the gate is free; the run has returned, only its deferred release is pending.
func waitGate(a *app) bool {
	for range 1_000_000 {
		if !replayGateOf(a).Load() {
			return true
		}
		runtime.Gosched()
	}
	return false
}

func TestStrategyReplayPostMissingLineIsReadFailed(t *testing.T) {
	t.Parallel()
	rg := newReplayPostRig(t)
	rg.a.deps.openBar = nil
	code, d := rg.post(t, realPost, t.Context(), defaultBody(t), true)
	if code != http.StatusOK || strings.Join(d.noticeKeys(), ",") != "read_failed" {
		t.Fatalf("unwired opener: status %d keys %v", code, d.noticeKeys())
	}
	a, _ := testApp(t, "OT_PG_DSN_OPTION_LEDGER=x\nOT_PG_DSN_PAGES=x\nOT_PG_DSN_RUNNER=x\n")
	rg2 := replayPostRig{a: a, rd: rg.rd, clk: rg.clk}
	code, d = rg2.post(t, realPost, t.Context(), defaultBody(t), true)
	if code != http.StatusOK || strings.Join(d.noticeKeys(), ",") != "read_failed" {
		t.Fatalf("missing DSN line: status %d keys %v", code, d.noticeKeys())
	}
}

func TestStrategyReplayCLIAndPostGiveTheSameReport(t *testing.T) {
	t.Parallel()
	rg := newReplayPostRig(t)
	o := replayOpts{lots: lotsShortPuts, asOf: "2026-11-20", earlyKeep: "0.50", earlyDTE: "21", minLots: 20}
	want, err := replayRun(t.Context(), rg.a, o)
	if err != nil {
		t.Fatal(err)
	}
	var got replayResult
	p := replayPost{timeout: time.Minute, run: func(ctx context.Context, a *app, o replayOpts) (replayResult, error) {
		var err error
		got, err = replayRun(ctx, a, o)
		return got, err
	}}
	body := defaultBody(t)
	body.Set("as_of", "2026-11-20")
	if code, d := rg.post(t, p, t.Context(), body, true); code != http.StatusOK || d.View != "report" {
		t.Fatalf("status %d view %q", code, d.View)
	}
	wj, _ := json.Marshal(want.Report)
	gj, _ := json.Marshal(got.Report)
	if !bytes.Equal(wj, gj) || len(want.Report.Lines) == 0 {
		t.Fatalf("CLI report %s, post report %s", wj, gj)
	}
	// the CLI path prints that report
	if err := strategyReplayCmd(rg.a, []string{"--as-of", "2026-11-20"}); err != nil {
		t.Fatal(err)
	}
}

type noWriteLedger struct {
	*ledgerFakeStore
	t *testing.T
}

func (s noWriteLedger) RecordFlexFetch(context.Context, store.FlexFetchRecord, []store.FlexTradeRow) (store.FlexInsert, error) {
	s.t.Error("the post wrote a flex fetch")
	return store.FlexInsert{}, nil
}

func (s noWriteLedger) ImportFlexTradeRows(context.Context, []store.FlexTradeRow) (store.FlexImportResult, error) {
	s.t.Error("the post imported flex rows")
	return store.FlexImportResult{}, nil
}

type noWriteSessions struct {
	store.SessionDayStore
	t *testing.T
}

func (s noWriteSessions) UpsertSessionDays(context.Context, []contracts.SessionDay) error {
	s.t.Error("the post wrote session days")
	return nil
}

type noWriteEod struct {
	store.OptionEodStore
	t *testing.T
}

func (s noWriteEod) InsertOptionEod(context.Context, []contracts.OptionEod) error {
	s.t.Error("the post wrote option eod")
	return nil
}

type noWriteBars struct {
	bar.BarStore
	t *testing.T
}

func (s noWriteBars) InsertPriceBars(context.Context, []contracts.PriceBar) (int, error) {
	s.t.Error("the post wrote price bars")
	return 0, nil
}

func (s noWriteBars) InsertFxRates(context.Context, []contracts.FxRate) (int, error) {
	s.t.Error("the post wrote fx rates")
	return 0, nil
}

func TestStrategyReplayPostWritesNothing(t *testing.T) {
	t.Parallel()
	rg := newReplayPostRig(t)
	d := &rg.a.deps
	openLedger, openSess, openEod, openBar := d.openOptionLedgerStore, d.openSessionDay, d.openOptionEod, d.openBar
	d.openOptionLedgerStore = func(ctx context.Context, dsn string) (store.OptionLedgerStore, error) {
		s, err := openLedger(ctx, dsn)
		return noWriteLedger{ledgerFakeStore: s.(*ledgerFakeStore), t: t}, err
	}
	d.openSessionDay = func(ctx context.Context, dsn string) (store.SessionDayStore, error) {
		s, err := openSess(ctx, dsn)
		return noWriteSessions{SessionDayStore: s, t: t}, err
	}
	d.openOptionEod = func(ctx context.Context, dsn string) (store.OptionEodStore, error) {
		s, err := openEod(ctx, dsn)
		return noWriteEod{OptionEodStore: s, t: t}, err
	}
	d.openBar = func(ctx context.Context, dsn string) (bar.BarStore, error) {
		s, err := openBar(ctx, dsn)
		return noWriteBars{BarStore: s, t: t}, err
	}
	body := defaultBody(t)
	body.Set("as_of", "2026-11-20")
	if code, doc := rg.post(t, realPost, t.Context(), body, true); code != http.StatusOK || doc.View != "report" {
		t.Fatalf("status %d view %q keys %v", code, doc.View, doc.noticeKeys())
	}
}
