package main

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/gatewaystate"
	"github.com/jonboulle/clockwork"
)

// fakeHealth answers one fixed Health or one error.
type fakeHealth struct {
	h   broker.Health
	err error
}

func (f fakeHealth) Health(context.Context) (broker.Health, error) { return f.h, f.err }
func (fakeHealth) Close() error                                    { return nil }

// fakeDial gives the fakeHealth of a socket path; a path with no entry fails the dial.
func fakeDial(m map[string]*fakeHealth) broker.NewHealthFunc {
	return func(_ context.Context, socket string, _ broker.Options) (broker.HealthReader, error) {
		f, ok := m[socket]
		if !ok {
			return nil, errors.New("dial failed")
		}
		return f, nil
	}
}

// recordingGateways is a notify.GatewayNotifier that keeps the facts of every pass.
type recordingGateways struct {
	mu     sync.Mutex
	passes [][]notify.GatewayFact
}

func (r *recordingGateways) NotifyGateways(_ context.Context, facts []notify.GatewayFact) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.passes = append(r.passes, slices.Clone(facts))
	return 0, nil
}

const gatewayEnv = "OT_GATEWAY_SOCKETS=main=/s/m,wife=/s/w\n"

func TestGatewayHealthUnreachable(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, gatewayEnv)
	unreachable := &fakeHealth{err: &contractsRetryable{}}
	a.deps.ext = map[string]any{extGateway: gatewayDeps{newHealth: fakeDial(map[string]*fakeHealth{"/s/m": unreachable})}}
	code, body := dataHealthGet(t, a, siteReaders{})
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	validateDataHealth(t, body)
	// main's reader errors and wife's socket does not even dial: both are unreachable.
	if got := strings.Count(string(body), `"verdict":"unreachable"`); got != 2 {
		t.Fatalf("want two unreachable gateways, body %s", body)
	}
}

// contractsRetryable is the typed error an unreachable gateway gives.
type contractsRetryable struct{}

func (*contractsRetryable) Error() string             { return "unreachable" }
func (*contractsRetryable) Kind() contracts.ErrorKind { return contracts.Retryable }

func TestParseGatewaySockets(t *testing.T) {
	t.Parallel()
	got, err := parseGatewaySockets(" main=/s/m , wife=/s/w ")
	if err != nil || len(got) != 2 || got[0] != (gatewayEntry{"main", "/s/m"}) || got[1] != (gatewayEntry{"wife", "/s/w"}) {
		t.Fatalf("got %v, err %v", got, err)
	}
	for _, bad := range []string{"/s/m", "a_b=/s/m", "main=", "=/s/m", "main=/a,main=/b", "Main=/s/m"} {
		if _, err := parseGatewaySockets(bad); err == nil {
			t.Errorf("%q is accepted", bad)
		}
	}
}

func TestGatewayNotifyPass(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, gatewayEnv)
	since := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	main := &fakeHealth{h: broker.Health{State: "farm_broken", Farms: []broker.Farm{{Name: "usfarm", Up: true}, {Name: "ushmds", Up: false, Since: since}}}}
	rec := &recordingGateways{}
	a.deps.ext = map[string]any{extGateway: gatewayDeps{
		newHealth: fakeDial(map[string]*fakeHealth{"/s/m": main, "/s/w": {err: errors.New("down")}}),
		newNotifier: func(notify.Sender, store.SentStore, clockwork.Clock) (notify.GatewayNotifier, error) {
			return rec, nil
		},
	}}
	pass := newGatewayPass(a, nil, nil, nil, nil)
	if pass == nil {
		t.Fatal("no pass")
	}
	g := notifyingGaps{inner: noopSyncer{}, n: noopNotifier{}, gateways: pass, log: a.log}
	if err := g.Sync(t.Context(), "2026-11-20", nil); err != nil {
		t.Fatal(err)
	}
	main.h = broker.Health{State: "up"} // the gateway recovers before the second tick
	if err := g.Sync(t.Context(), "2026-11-20", nil); err != nil {
		t.Fatal(err)
	}
	want := [][]notify.GatewayFact{
		{{Gateway: "main", Farm: "ushmds", Since: since, Down: true}, {Gateway: "wife"}},
		{{Gateway: "main"}, {Gateway: "wife"}},
	}
	if len(rec.passes) != 2 {
		t.Fatalf("passes %v", rec.passes)
	}
	for i := range want {
		if !slices.Equal(rec.passes[i], want[i]) {
			t.Errorf("pass %d: got %+v, want %+v", i, rec.passes[i], want[i])
		}
	}
}

type noopSyncer struct{}

func (noopSyncer) Sync(context.Context, contracts.Day, map[string]string) error { return nil }

type noopNotifier struct{}

func (noopNotifier) NotifyGaps(context.Context) (int, error) { return 0, nil }

// fakeStateWriter records every Observe; err is returned by each.
type fakeStateWriter struct {
	mu    sync.Mutex
	calls []observeCall
	err   error
}

type observeCall struct {
	alias string
	down  bool
	at    time.Time
}

func (w *fakeStateWriter) Observe(_ context.Context, alias string, down bool, at time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, observeCall{alias, down, at})
	return w.err
}
func (*fakeStateWriter) Close() error { return nil }

// gatewayPassOf builds the pass over main (health h) and wife (no answer) with the recording notifier.
func gatewayPassOf(t *testing.T, h broker.Health, w gatewaystate.Writer, clk clockwork.Clock) (gatewayPasser, *recordingGateways) {
	t.Helper()
	a, _ := testApp(t, gatewayEnv)
	rec := &recordingGateways{}
	a.deps.ext = map[string]any{extGateway: gatewayDeps{
		newHealth: fakeDial(map[string]*fakeHealth{"/s/m": {h: h}, "/s/w": {err: errors.New("down")}}),
		newNotifier: func(notify.Sender, store.SentStore, clockwork.Clock) (notify.GatewayNotifier, error) {
			return rec, nil
		},
	}}
	p := newGatewayPass(a, nil, nil, clk, w)
	if p == nil {
		t.Fatal("no pass")
	}
	return p, rec
}

func TestGatewayPassObserves(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	w := &fakeStateWriter{}
	p, rec := gatewayPassOf(t, broker.Health{State: "farm_broken"}, w, clk)
	if _, err := p.pass(t.Context()); err != nil {
		t.Fatal(err)
	}
	want := []observeCall{{"main", false, clk.Now()}, {"wife", true, clk.Now()}}
	if !slices.Equal(w.calls, want) || len(rec.passes) != 1 {
		t.Fatalf("calls %+v, passes %d, want %+v", w.calls, len(rec.passes), want)
	}
}

func TestGatewayPassWriteFailureStillNotifies(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	h := broker.Health{State: "up"}
	failing, recF := gatewayPassOf(t, h, &fakeStateWriter{err: errors.New("db down")}, clk)
	plain, recP := gatewayPassOf(t, h, nil, clk)
	if _, err := failing.pass(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := plain.pass(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(recF.passes[0], recP.passes[0]) {
		t.Fatalf("facts differ: %+v vs %+v", recF.passes[0], recP.passes[0])
	}
}

func TestGatewayPassNilWriter(t *testing.T) {
	t.Parallel()
	p, rec := gatewayPassOf(t, broker.Health{State: "up"}, nil, nil)
	if _, err := p.pass(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(rec.passes) != 1 || len(rec.passes[0]) != 2 {
		t.Fatalf("passes %+v", rec.passes)
	}
}

// init is the one write of openGatewayWriter, before any test runs: the scheduler tests have no database.
func init() {
	openGatewayWriter = func(context.Context, string) (gatewaystate.Writer, error) { return &fakeStateWriter{}, nil }
}
