package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
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
	}
	return h
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
	set := func(k string) string { return deadEnv[k] }
	none := func(string) string { return "" }
	if got := ifOpenDSN(set, brokerDownOff)(runOnceConfig{}, false); got != reasonBrokerDown {
		t.Fatalf("wired, socket dead: %q", got)
	}
	if got := ifOpenDSN(set, brokerDownOff)(runOnceConfig{}, true); got != "" {
		t.Fatalf("wired, socket up: %q", got)
	}
	if got := ifOpenDSN(none, brokerDownOff)(runOnceConfig{}, false); got != "" {
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
