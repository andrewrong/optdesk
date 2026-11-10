package accountcapture

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const testDay = contracts.Day("2026-11-05")

type fakeReader struct {
	acc     broker.Account
	accErr  error
	ex      broker.Executions
	exErr   error
	calls   int
	exCalls int
}

func (f *fakeReader) Account(context.Context) (broker.Account, error) {
	f.calls++
	return f.acc, f.accErr
}

func (f *fakeReader) Executions(context.Context) (broker.Executions, error) {
	f.exCalls++
	return f.ex, f.exErr
}
func (f *fakeReader) Close() error { return nil }

type fakeWriter struct {
	mu       sync.Mutex
	captures map[string]store.Capture
	execs    map[string][]store.Execution
	writes   int
	err      error
}

func newWriter() *fakeWriter {
	return &fakeWriter{captures: map[string]store.Capture{}, execs: map[string][]store.Execution{}}
}

func (w *fakeWriter) WriteCapture(_ context.Context, c store.Capture) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	if _, ok := w.captures[c.ID]; !ok {
		w.captures[c.ID] = c
		w.writes++
	}
	return nil
}

func (w *fakeWriter) WriteExecutions(_ context.Context, alias string, rows []store.Execution) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.execs[alias] = append(w.execs[alias], rows...)
	return nil
}

func okAccount(alias string) broker.Account {
	return broker.Account{
		Alias:  alias,
		Values: []broker.AccountValue{{Tag: "NetLiquidation", Value: "100.5", Currency: "USD"}},
		Positions: []broker.Position{{
			Contract: broker.Contract{SecType: "OPT", Symbol: "SPY", Expiry: "20261120", Strike: "450", Right: "P", Multiplier: "100", ConID: 7},
			Quantity: "-1", AvgCost: "2.5",
		}},
		ValuesCollected: true, PositionsCollected: true,
	}
}

func okExec(alias string) broker.Executions {
	e := broker.Execution{
		ExecID: "e1-" + alias, Side: "SLD", Quantity: "1",
		Contract: broker.Contract{SecType: "OPT", Symbol: "SPY", Right: "C"},
	}
	return broker.Executions{Alias: alias, Collected: true, Executions: []broker.Execution{e}}
}

func goodReader(alias string) *fakeReader {
	return &fakeReader{acc: okAccount(alias), ex: okExec(alias)}
}

type env struct {
	w   *fakeWriter
	log *bytes.Buffer
	j   *job
}

func setup(t *testing.T, main string, readers map[string]*fakeReader) env {
	t.Helper()
	rs := map[string]broker.AccountReader{}
	for a, r := range readers {
		rs[a] = r
	}
	w := newWriter()
	buf := &bytes.Buffer{}
	j, err := newJob(Config{Main: main, Readers: rs, Writer: w, Clock: clockwork.NewFakeClockAt(time.Date(2026, 11, 5, 21, 0, 0, 0, time.UTC))},
		slog.New(slog.NewTextHandler(buf, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return env{w: w, log: buf, j: j}
}

func run(e env) contracts.JobResult {
	keys, _, _ := e.j.Plan(context.Background(), testDay)
	return e.j.Run(context.Background(), testDay, keys)
}

func TestNewRefusesMainNotInReaders(t *testing.T) {
	t.Parallel()
	_, err := New(Config{Main: "x", Readers: map[string]broker.AccountReader{"a": goodReader("a")}, Writer: newWriter(), Clock: clockwork.NewFakeClock()})
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestCaptureWritesIDAndRows(t *testing.T) {
	t.Parallel()
	e := setup(t, "a", map[string]*fakeReader{"a": goodReader("a"), "b": goodReader("b")})
	res := run(e)
	if res.Err != nil || res.Usable != 2 {
		t.Fatalf("res = %+v", res)
	}
	c, ok := e.w.captures["a:2026-11-05:daily"]
	if !ok || c.Alias != "a" || c.SessionDay != "2026-11-05" || len(c.Values) != 1 || len(c.Positions) != 1 {
		t.Fatalf("capture = %+v (%v)", c, ok)
	}
	if c.Positions[0].Expiry != "2026-11-20" {
		t.Fatalf("expiry %q", c.Positions[0].Expiry)
	}
	if len(e.w.execs["a"]) != 1 || len(e.w.execs["b"]) != 1 || len(e.w.captures) != 2 {
		t.Fatalf("execs %v captures %d", e.w.execs, len(e.w.captures))
	}
}

func TestRetryOfSameRunWritesNoSecondCapture(t *testing.T) {
	t.Parallel()
	e := setup(t, "a", map[string]*fakeReader{"a": goodReader("a")})
	run(e)
	run(e)
	if len(e.w.captures) != 1 || e.w.writes != 1 {
		t.Fatalf("captures %d writes %d", len(e.w.captures), e.w.writes)
	}
}

func TestKindNotCollectedWritesNoRow(t *testing.T) {
	t.Parallel()
	r := goodReader("a")
	r.acc.ValuesCollected, r.acc.PositionsCollected = false, false
	r.ex.Collected = false
	e := setup(t, "a", map[string]*fakeReader{"a": r})
	if res := run(e); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(e.w.captures) != 0 || len(e.w.execs["a"]) != 0 {
		t.Fatalf("written: %v %v", e.w.captures, e.w.execs)
	}
}

func TestPartialKindsAndEmptyPositionsWritten(t *testing.T) {
	t.Parallel()
	r := goodReader("a")
	r.acc.ValuesCollected = false
	r.acc.Positions = nil
	e := setup(t, "a", map[string]*fakeReader{"a": r})
	run(e)
	c, ok := e.w.captures["a:2026-11-05:daily"]
	if !ok || c.Values != nil || c.Positions == nil || len(c.Positions) != 0 {
		t.Fatalf("capture = %+v (%v)", c, ok)
	}
}

func TestFailingAliasAmongTwoDoesNotStopOther(t *testing.T) {
	t.Parallel()
	bad := goodReader("a")
	bad.accErr = errors.New("logged out")
	e := setup(t, "b", map[string]*fakeReader{"a": bad, "b": goodReader("b")})
	res := run(e)
	if res.Err != nil {
		t.Fatalf("run failed: %v", res.Err)
	}
	if _, ok := e.w.captures["b:2026-11-05:daily"]; !ok {
		t.Fatal("main not written")
	}
}

func TestMainFailsRun(t *testing.T) {
	t.Parallel()
	bad := goodReader("a")
	bad.exErr = errors.New("secret 12345 detail")
	e := setup(t, "a", map[string]*fakeReader{"a": bad, "b": goodReader("b")})
	res := run(e)
	if res.Err == nil {
		t.Fatal("want failure")
	}
	if strings.Contains(res.Err.Error(), "12345") || !strings.Contains(res.Err.Error(), "alias a status failed") {
		t.Fatalf("err = %q", res.Err)
	}
	if _, ok := e.w.captures["b:2026-11-05:daily"]; !ok {
		t.Fatal("other alias stopped")
	}
}

func TestNonMainFailureDegraded(t *testing.T) {
	t.Parallel()
	bad := goodReader("b")
	bad.accErr = errors.New("logged out 99999")
	e := setup(t, "a", map[string]*fakeReader{"a": goodReader("a"), "b": bad})
	res := run(e)
	if res.Err != nil {
		t.Fatalf("run failed: %v", res.Err)
	}
	if n := strings.Count(e.log.String(), "degraded"); n != 1 {
		t.Fatalf("degraded records = %d in %q", n, e.log.String())
	}
	if l := e.log.String(); !strings.Contains(l, "alias=b") || !strings.Contains(l, "status=failed") || strings.Contains(l, "99999") {
		t.Fatalf("log = %q", l)
	}
}

func TestMainPendingFailsAfterRowsWritten(t *testing.T) {
	t.Parallel()
	r := goodReader("a")
	r.ex.Pending = 2
	e := setup(t, "a", map[string]*fakeReader{"a": r})
	res := run(e)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "status pending") {
		t.Fatalf("err = %v", res.Err)
	}
	if len(e.w.execs["a"]) != 1 || len(e.w.captures) != 1 {
		t.Fatal("complete rows not written")
	}
}

func TestNonMainPendingDegraded(t *testing.T) {
	t.Parallel()
	r := goodReader("b")
	r.ex.Pending = 1
	e := setup(t, "a", map[string]*fakeReader{"a": goodReader("a"), "b": r})
	res := run(e)
	if res.Err != nil {
		t.Fatalf("run failed: %v", res.Err)
	}
	if len(e.w.execs["b"]) != 1 || !strings.Contains(e.log.String(), "status=pending") || strings.Count(e.log.String(), "degraded") != 1 {
		t.Fatalf("execs %v log %q", e.w.execs, e.log.String())
	}
}

func TestNoGapForNonMain(t *testing.T) {
	t.Parallel()
	bad := goodReader("b")
	bad.accErr = errors.New("down")
	bad.exErr = errors.New("down")
	e := setup(t, "a", map[string]*fakeReader{"a": goodReader("a"), "b": bad})
	res := run(e)
	// no gap: the result carries no error, so the runner marks the key present.
	if res.Err != nil || res.Returned != 2 || res.Usable != 1 {
		t.Fatalf("res = %+v", res)
	}
	if len(e.w.captures) != 1 {
		t.Fatal("a failed alias wrote a row")
	}
}

// recordHandler is a slog.Handler that keeps every record it receives.
type recordHandler struct {
	mu   *sync.Mutex
	recs *[]slog.Record
}

func (h recordHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.recs = append(*h.recs, r)
	return nil
}
func (h recordHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h recordHandler) WithGroup(string) slog.Handler      { return h }

func TestDegradedLogCarriesAliasAndStatusOnly(t *testing.T) {
	t.Parallel()
	bad := goodReader("b")
	bad.accErr = errors.New("x")
	var recs []slog.Record
	h := recordHandler{mu: &sync.Mutex{}, recs: &recs}
	rs := map[string]broker.AccountReader{"a": goodReader("a"), "b": bad}
	j, err := newJob(Config{Main: "a", Readers: rs, Writer: newWriter(), Clock: clockwork.NewFakeClock()}, slog.New(h))
	if err != nil {
		t.Fatal(err)
	}
	keys, _, _ := j.Plan(context.Background(), testDay)
	j.Run(context.Background(), testDay, keys)
	var degraded []slog.Record
	for _, r := range recs {
		if r.Message == "degraded" {
			degraded = append(degraded, r)
		}
	}
	if len(degraded) != 1 {
		t.Fatalf("got %d degraded records, want 1", len(degraded))
	}
	var names []string
	degraded[0].Attrs(func(a slog.Attr) bool {
		names = append(names, a.Key)
		if a.Value.Kind() != slog.KindString {
			t.Errorf("attribute %s is %s, not a string", a.Key, a.Value.Kind())
		}
		return true
	})
	if !reflect.DeepEqual(names, []string{"alias", "status"}) {
		t.Fatalf("attributes %v", names)
	}
}

func TestResultCarriesAliasAndStatusOnly(t *testing.T) {
	t.Parallel()
	// The error of the run is a string-only type; JobResult's only numbers are the two counts of aliases.
	typ := reflect.TypeOf(typedError{})
	for i := 0; i < typ.NumField(); i++ {
		if k := typ.Field(i).Type.Kind(); k != reflect.String && k != reflect.Int {
			t.Errorf("%s.%s is %s", typ.Name(), typ.Field(i).Name, typ.Field(i).Type)
		}
	}
	rt := reflect.TypeOf(contracts.JobResult{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.Type.Kind() == reflect.Int {
			if f.Name != "Returned" && f.Name != "Usable" {
				t.Errorf("JobResult.%s is numeric", f.Name)
			}
		} else if f.Type.Kind() != reflect.Interface {
			t.Errorf("JobResult.%s is %s", f.Name, f.Type)
		}
	}
	// The text of the error names the alias and the status only.
	bad := goodReader("a")
	bad.accErr = errors.New("balance 123456")
	e := setup(t, "a", map[string]*fakeReader{"a": bad})
	res := run(e)
	if res.Err == nil || strings.Contains(res.Err.Error(), "123456") || !strings.Contains(res.Err.Error(), "alias a status failed") {
		t.Fatalf("err = %v", res.Err)
	}
}

func TestPlanIsOneKeyPerDay(t *testing.T) {
	t.Parallel()
	e := setup(t, "a", map[string]*fakeReader{"a": goodReader("a")})
	keys, h1, err := e.j.Plan(context.Background(), testDay)
	_, h2, _ := e.j.Plan(context.Background(), "2026-11-06")
	if err != nil || len(keys) != 1 || h1 == h2 || e.j.ID() != "account_capture" {
		t.Fatalf("keys %v err %v", keys, err)
	}
}

func TestCaptureFailureLogsAliasStepKind(t *testing.T) {
	t.Parallel()
	typed := &typedError{kind: contracts.Retryable, text: "error 322 account 99999"}
	cases := []struct {
		name string
		mod  func(r *fakeReader)
		step string
		kind string
	}{
		{"account typed", func(r *fakeReader) { r.accErr = typed }, "account", "retryable"},
		{"executions typed", func(r *fakeReader) { r.exErr = typed }, "executions", "retryable"},
		{"account untyped", func(r *fakeReader) { r.accErr = errors.New("boom 99999") }, "account", "unknown"},
		{"executions untyped", func(r *fakeReader) { r.exErr = errors.New("boom 99999") }, "executions", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bad := goodReader("a")
			tc.mod(bad)
			var recs []slog.Record
			h := recordHandler{mu: &sync.Mutex{}, recs: &recs}
			rs := map[string]broker.AccountReader{"a": bad}
			j, err := newJob(Config{Main: "a", Readers: rs, Writer: newWriter(), Clock: clockwork.NewFakeClock()}, slog.New(h))
			if err != nil {
				t.Fatal(err)
			}
			keys, _, _ := j.Plan(context.Background(), testDay)
			j.Run(context.Background(), testDay, keys)
			var got []map[string]string
			for _, r := range recs {
				if r.Message != "capture failed" {
					continue
				}
				m := map[string]string{}
				r.Attrs(func(a slog.Attr) bool { m[a.Key] = a.Value.String(); return true })
				got = append(got, m)
				if strings.Contains(r.Message, "99999") {
					t.Fatal("message holds account id")
				}
			}
			if len(got) != 1 {
				t.Fatalf("capture failed records = %d", len(got))
			}
			m := got[0]
			if len(m) != 3 || m["alias"] != "a" || m["step"] != tc.step || m["kind"] != tc.kind {
				t.Fatalf("attrs = %v", m)
			}
			for _, v := range m {
				if strings.Contains(v, "99999") {
					t.Fatalf("attrs hold account id: %v", m)
				}
			}
		})
	}
}

func TestCaptureFailureWriteStepLogged(t *testing.T) {
	t.Parallel()
	e := setup(t, "a", map[string]*fakeReader{"a": goodReader("a")})
	e.w.err = errors.New("db 99999")
	run(e)
	l := e.log.String()
	if strings.Count(l, "capture failed") != 1 || !strings.Contains(l, "step=write") || !strings.Contains(l, "kind=unknown") || strings.Contains(l, "99999") {
		t.Fatalf("log = %q", l)
	}
}

func TestStoreWordsOfBrokerContract(t *testing.T) {
	t.Parallel()
	acc := broker.Account{
		Alias: "a", PositionsCollected: true,
		Positions: []broker.Position{
			{Contract: broker.Contract{SecType: "OPT", Symbol: "SPY", Right: "P", ConID: 1}},
			{Contract: broker.Contract{SecType: "STK", Symbol: "AAPL", ConID: 2}},
			{Contract: broker.Contract{SecType: "IND", Symbol: "SPX", ConID: 3}},
			{Contract: broker.Contract{SecType: "FUT", Symbol: "ESZ6", ConID: 4}},
		},
	}
	ex := broker.Executions{Alias: "a", Collected: true, Executions: []broker.Execution{
		{ExecID: "e1", Contract: broker.Contract{SecType: "OPT", Symbol: "SPY", Right: "C"}},
	}}
	rd := &fakeReader{acc: acc, ex: ex}
	w := newWriter()
	var recs []slog.Record
	h := recordHandler{mu: &sync.Mutex{}, recs: &recs}
	j, err := newJob(Config{Main: "a", Readers: map[string]broker.AccountReader{"a": rd}, Writer: w, Clock: clockwork.NewFakeClock()}, slog.New(h))
	if err != nil {
		t.Fatal(err)
	}
	keys, _, _ := j.Plan(context.Background(), testDay)
	res := j.Run(context.Background(), testDay, keys)
	if res.Err != nil || res.Returned != 1 || res.Usable != 1 {
		t.Fatalf("res = %+v", res)
	}
	type word struct{ sec, right string }
	var got []word
	for _, c := range w.captures {
		for _, p := range c.Positions {
			got = append(got, word{p.SecType, p.Right})
		}
	}
	want := []word{{"option", "put"}, {"stock", ""}, {"index", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("positions = %v, want %v", got, want)
	}
	exs := w.execs["a"]
	if len(exs) != 1 || exs[0].SecType != "option" || exs[0].Right != "call" {
		t.Fatalf("executions = %+v", exs)
	}
	var skipped []map[string]string
	for _, r := range recs {
		if r.Message != "skipped rows of an unsupported sec type" {
			continue
		}
		m := map[string]string{}
		r.Attrs(func(a slog.Attr) bool { m[a.Key] = a.Value.String(); return true })
		skipped = append(skipped, m)
	}
	if len(skipped) != 1 || len(skipped[0]) != 2 || skipped[0]["alias"] != "a" || skipped[0]["count"] != "1" {
		t.Fatalf("skipped records = %v", skipped)
	}
}

func TestStoreSecTypeRejectsEmptyWord(t *testing.T) {
	t.Parallel()
	if s, ok := storeSecType(""); ok || s != "" {
		t.Fatalf("storeSecType(\"\") = %q, %v; want \"\", false", s, ok)
	}
}
