package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// deadEnv names a port that refuses at once: the store cannot be opened.
var deadEnv = map[string]string{"OT_PG_DSN_OPTION_OPEN": "postgres://u@127.0.0.1:1/x?connect_timeout=1"}

var openIDs = []string{jobOptionOpenResolve, jobOptionOpenPlan, jobOptionOpen, jobOptionOpenResolveGap}

type closeBroker struct {
	broker.Broker
	mu     *sync.Mutex
	closed *int
}

func (b closeBroker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	*b.closed++
	return nil
}

type openInst struct{ store.InstrumentStore }

type openBars struct{ bar.BarStore }

// openHarness builds the families by hand over a jobCtx, as wireJobs does.
type openHarness struct {
	a      *app
	jc     *jobCtx
	log    *bytes.Buffer
	mu     sync.Mutex
	dialed []broker.Options
	closed int
}

func newOpenHarness(t *testing.T, env map[string]string, brokerUp bool, dialErr error) *openHarness {
	t.Helper()
	a, _ := testApp(t, "")
	var buf bytes.Buffer
	a.log = slog.New(slog.NewTextHandler(&buf, nil))
	a.getenv = func(k string) string { return env[k] }
	h := &openHarness{a: a, log: &buf}
	a.deps.dialBroker = func(_ context.Context, _ string, o broker.Options) (broker.Broker, error) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.dialed = append(h.dialed, o)
		if dialErr != nil {
			return nil, dialErr
		}
		return closeBroker{mu: &h.mu, closed: &h.closed}, nil
	}
	h.jc = &jobCtx{
		f:        &jobFactory{clk: fakeClock{time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)}, inst: openInst{}, bars: openBars{}},
		brk:      closeBroker{mu: &h.mu, closed: &h.closed},
		brokerUp: brokerUp, extra: &closerSet{},
		cfg: runOnceConfig{openDSN: env[openquote.DSNEnvOptionOpen]},
	}
	return h
}

// newFileHarness is the harness of a deployed scheduler: the lines live in the env file that readEnv returns
// (selected by CREW_SECRETS and CREW_SERVICE), the process environment holds no DSN, and the config comes from
// loadConfig.
func newFileHarness(t *testing.T, file map[string]string, brokerUp bool) (*openHarness, runOnceConfig) {
	t.Helper()
	h := newOpenHarness(t, nil, brokerUp, nil)
	home, err := os.MkdirTemp("", "ot")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	proc := map[string]string{"OT_HOME": home, "CREW_SECRETS": "/secrets", "CREW_SERVICE": "optdesk"}
	h.a.getenv = func(k string) string { return proc[k] }
	lines := map[string]string{"FRED_API_KEY": "k"}
	for _, k := range []string{store.DSNEnvRunner, store.DSNEnvSeriesFred, store.DSNEnvOptionEod, store.DSNEnvQuoteSnapshot,
		store.DSNEnvMaster, store.DSNEnvPages, store.DSNEnvReadings, store.DSNEnvLoader} {
		lines[k] = "postgres://u@127.0.0.1:1/x"
	}
	for k, v := range file {
		lines[k] = v
	}
	var path string
	h.a.deps.readEnv = func(p string) (map[string]string, error) { path = p; return lines, nil }
	cfg, err := loadConfig(h.a, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := "/secrets" + string(os.PathSeparator) + "optdesk.env"; path != want {
		t.Fatalf("read %q, want %q", path, want)
	}
	h.jc.cfg = cfg
	return h, cfg
}

func TestOptionOpenDSNOnlyInEnvFileWiresFourJobs(t *testing.T) {
	t.Parallel()
	h, cfg := newFileHarness(t, deadEnv, true)
	off := startDisabled(cfg, true)
	for _, id := range openIDs {
		if _, ok := off[id]; ok {
			t.Fatalf("%s is disabled: %v", id, off)
		}
		h.build(t, id)
	}
	if strings.Contains(h.log.String(), "is not set") {
		t.Fatalf("log: %s", h.log.String())
	}
}

func TestOptionOpenDSNInNeitherPlaceLeavesFourJobsOut(t *testing.T) {
	t.Parallel()
	h, cfg := newFileHarness(t, nil, true)
	if cfg.openDSN != "" {
		t.Fatalf("openDSN %q", cfg.openDSN)
	}
	off := startDisabled(cfg, true)
	for _, id := range openIDs {
		if _, ok := off[id]; ok {
			t.Fatalf("%s is disabled: %v", id, off)
		}
		if j, reason, err := jobFamilies[id].build(t.Context(), h.a, h.jc); j != nil || reason != "" || err != nil {
			t.Fatalf("%s: job %v reason %q err %v", id, j, reason, err)
		}
	}
	if n := strings.Count(h.log.String(), "OT_PG_DSN_OPTION_OPEN is not set"); n != 4 {
		t.Fatalf("%d lines, want 4: %s", n, h.log.String())
	}
}

func (h *openHarness) build(t *testing.T, id string) (contracts.Job, string) {
	t.Helper()
	j, reason, err := jobFamilies[id].build(t.Context(), h.a, h.jc)
	if err != nil {
		t.Fatal(err)
	}
	if j == nil || j.ID() != id {
		t.Fatalf("%s: job %v", id, j)
	}
	return j, reason
}

func TestOptionOpenMissingDSNLeavesFourJobsOutWithOneLineEach(t *testing.T) {
	t.Parallel()
	h := newOpenHarness(t, nil, true, nil)
	for _, id := range openIDs {
		if j, reason, err := jobFamilies[id].build(t.Context(), h.a, h.jc); j != nil || reason != "" || err != nil {
			t.Fatalf("%s: job %v reason %q err %v", id, j, reason, err)
		}
	}
	log := h.log.String()
	for _, id := range openIDs {
		if n := strings.Count(log, "job="+id+" "); n != 1 {
			t.Fatalf("%d lines name %s: %s", n, id, log)
		}
	}
	if n := strings.Count(log, "OT_PG_DSN_OPTION_OPEN is not set"); n != 4 {
		t.Fatalf("%d lines name the DSN line: %s", n, log)
	}
}

func TestOptionOpenStoreThatCannotOpenIsHeldAndListedDisabled(t *testing.T) {
	t.Parallel()
	h := newOpenHarness(t, deadEnv, true, nil)
	w := wiring{brokerUp: true, off: startDisabled(runOnceConfig{}, true), held: map[string]contracts.Job{}}
	for _, id := range openIDs {
		j, reason := h.build(t, id)
		w.held[id], w.off[id] = j, reason
	}
	dis := schedulerDisabled(runOnceConfig{}, w)
	for _, id := range openIDs {
		if dis[id] != reasonOpenStoreFail {
			t.Fatalf("disabled[%s] = %q", id, dis[id])
		}
	}
	if n := strings.Count(h.log.String(), reasonOpenStoreFail); n != 4 {
		t.Fatalf("%d lines, want 4: %s", n, h.log.String())
	}
}

func TestOptionOpenStartOffNeedsTheDSNLine(t *testing.T) {
	t.Parallel()
	set := runOnceConfig{openDSN: deadEnv[openquote.DSNEnvOptionOpen]}
	if got := ifOpenDSN(brokerDownOff)(set, false); got != reasonBrokerDown {
		t.Fatalf("wired, socket dead: %q", got)
	}
	if got := ifOpenDSN(brokerDownOff)(set, true); got != "" {
		t.Fatalf("wired, socket up: %q", got)
	}
	if got := ifOpenDSN(brokerDownOff)(runOnceConfig{}, false); got != "" {
		t.Fatalf("family absent: %q", got)
	}
}

func TestOptionOpenDeadSocketDoesNotDial(t *testing.T) {
	t.Parallel()
	h := newOpenHarness(t, deadEnv, false, nil)
	for _, id := range openIDs {
		h.build(t, id)
	}
	if len(h.dialed) != 0 {
		t.Fatalf("dialed with the broker down: %v", h.dialed)
	}
}

func TestOptionOpenResolveFamiliesDialOwnClientWithOneInFlight(t *testing.T) {
	t.Parallel()
	h := newOpenHarness(t, deadEnv, true, nil)
	for _, id := range openIDs {
		h.build(t, id)
	}
	if len(h.dialed) != 2 || h.dialed[0].ResolveInFlight != 1 || h.dialed[1].ResolveInFlight != 1 {
		t.Fatalf("dials %+v", h.dialed)
	}
	h.jc.extra.closeAll()
	if h.closed != 2 {
		t.Fatalf("closed %d clients, want 2", h.closed)
	}
}

func TestOptionOpenResolveDialFailureHolds(t *testing.T) {
	t.Parallel()
	h := newOpenHarness(t, deadEnv, true, errors.New("down"))
	if _, reason := h.build(t, jobOptionOpenResolveGap); reason == "" {
		t.Fatal("a dead dial left the job unheld")
	}
}

func TestOpenResolveMaxNewFromEnvironment(t *testing.T) {
	t.Parallel()
	for env, want := range map[string]int{"": 0, "0": 0, "1500": 1500, "-3": 0, "x": 0} {
		h := newOpenHarness(t, map[string]string{envOpenResolveMaxNew: env}, true, nil)
		if got := openResolveMaxNew(h.a); got != want {
			t.Fatalf("%q gives %d, want %d", env, got, want)
		}
	}
}

var jpOpenIDs = []string{jobOptionOpenResolveJP, jobOptionOpenPlanJP, jobOptionOpenJP}

func TestOptionOpenJPFamiliesRegisteredInOrder(t *testing.T) {
	t.Parallel()
	for id, order := range map[string]int{jobOptionOpenResolveJP: 165, jobOptionOpenPlanJP: 170, jobOptionOpenJP: 180} {
		f, ok := jobFamilies[id]
		if !ok || f.order != order {
			t.Fatalf("%s: registered %v, order %d, want %d", id, ok, f.order, order)
		}
	}
}

func TestOptionOpenJPMissingDSNLeavesFamiliesOutWithOneLineEach(t *testing.T) {
	t.Parallel()
	h := newOpenHarness(t, nil, true, nil)
	for _, id := range jpOpenIDs {
		if j, reason, err := jobFamilies[id].build(t.Context(), h.a, h.jc); j != nil || reason != "" || err != nil {
			t.Fatalf("%s: job %v reason %q err %v", id, j, reason, err)
		}
		if n := strings.Count(h.log.String(), "job="+id+" "); n != 1 {
			t.Fatalf("%d lines name %s: %s", n, id, h.log.String())
		}
	}
	if n := strings.Count(h.log.String(), "OT_PG_DSN_OPTION_OPEN is not set"); n != 3 {
		t.Fatalf("%d lines name the DSN line: %s", n, h.log.String())
	}
}

func TestOptionOpenJPDeadSocketHoldsThroughStartOff(t *testing.T) {
	t.Parallel()
	cfg := runOnceConfig{openDSN: deadEnv[openquote.DSNEnvOptionOpen]}
	off := startDisabled(cfg, false)
	for _, id := range jpOpenIDs {
		if off[id] != reasonBrokerDown {
			t.Fatalf("startOff[%s] = %q", id, off[id])
		}
	}
	if off = startDisabled(runOnceConfig{}, false); off[jobOptionOpenJP] != "" {
		t.Fatalf("family absent but listed: %v", off)
	}
}

func TestOptionOpenJPResolveDialsOwnClientWithOneInFlight(t *testing.T) {
	t.Parallel()
	h := newOpenHarness(t, deadEnv, true, nil)
	for _, id := range jpOpenIDs {
		h.build(t, id)
	}
	if len(h.dialed) != 1 || h.dialed[0].ResolveInFlight != 1 {
		t.Fatalf("dials %+v", h.dialed)
	}
	h.jc.extra.closeAll()
	if h.closed != 1 {
		t.Fatalf("closed %d clients, want 1", h.closed)
	}
}

// writerEod stands in for the writer role of option_eod: every read fails and is counted.
type writerEod struct {
	store.OptionEodStore
	mu    sync.Mutex
	reads int
}

func (w *writerEod) OptionEod(context.Context, int64, contracts.Day, contracts.Day) ([]contracts.OptionEod, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.reads++
	return nil, errors.New("permission denied for schema rd")
}

// readerEod stands in for the reader role of option_eod: it holds rows.
type readerEod struct{ rows []contracts.OptionEod }

func (r readerEod) OptionEod(context.Context, int64, contracts.Day, contracts.Day) ([]contracts.OptionEod, error) {
	return r.rows, nil
}

func TestOpenStoredReadsEodThroughTheComputeReaderNotTheWriter(t *testing.T) {
	t.Parallel()
	w := &writerEod{}
	want := []contracts.OptionEod{{InstrumentID: 7}}
	jc := &jobCtx{f: &jobFactory{eod: w}}
	jc.f.compute.OptionEod = readerEod{rows: want}
	s := openStored(jc, nil)
	if s.Eod == nil {
		t.Fatal("Eod is nil, want the compute reader")
	}
	got, err := s.Eod.OptionEod(context.Background(), 1, "2026-11-01", "2026-11-09")
	if err != nil || len(got) != 1 || got[0].InstrumentID != 7 {
		t.Fatalf("rows = %v, err = %v, want the rows of the reader", got, err)
	}
	if w.reads != 0 {
		t.Fatalf("writer read %d times, want 0", w.reads)
	}
}

func TestOpenStoredLeavesEodNilWithoutComputeReader(t *testing.T) {
	t.Parallel()
	jc := &jobCtx{f: &jobFactory{eod: &writerEod{}}}
	if s := openStored(jc, nil); s.Eod != nil {
		t.Fatalf("Eod = %v, want nil", s.Eod)
	}
}
