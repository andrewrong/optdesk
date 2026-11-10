package main

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/dividend"
)

// divRef is a ReferenceClient that answers one projection per contract.
type divRef struct{}

func (*divRef) Dividends(_ context.Context, cs []broker.Contract) ([]broker.DividendProjection, error) {
	return make([]broker.DividendProjection, len(cs)), nil
}

func (*divRef) Close() error { return nil }

// divStore is the dividend store the rig opens.
type divStore struct{ dividend.Store }

func (*divStore) Close() error { return nil }

// divRig is a run-once rig whose dividend_capture constructor records what it was given.
type divRig struct {
	*rig
	ref    broker.ReferenceClient
	res    store.InstrumentResolver
	w      dividend.Writer
	watch  []jobs.WatchEntry
	store  *divStore
	dialed atomic.Int32
	events string
	socket string
}

func newDivRig(t *testing.T) *divRig {
	t.Helper()
	d := &divRig{rig: newRig(t, fullEnv), store: &divStore{}}
	dd := &d.a.deps
	dd.newReference = func(_ context.Context, socket string, _ broker.Options) (broker.ReferenceClient, error) {
		d.dialed.Add(1)
		d.socket = socket
		return &divRef{}, nil
	}
	dd.openDividend = func(_ context.Context, dsn string) (dividend.Store, error) {
		d.events = dsn
		return d.store, nil
	}
	dd.newDividendCapture = func(
		c broker.ReferenceClient, res store.InstrumentResolver, w dividend.Writer, _ dividend.Reader, entries []jobs.WatchEntry, _ clock.Clock,
	) (contracts.Job, error) {
		d.ref, d.res, d.w, d.watch = c, res, w, entries
		return namedJob{id: "dividend_capture"}, nil
	}
	body := "[[underlying]]\nissuer = \"cik:0000320193\"\nmarket = \"US\"\nsymbol = \"AAPL\"\n"
	if err := writeFile(d.a.getenv("OT_HOME")+"/watch.toml", body); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDividendCaptureRegistered(t *testing.T) {
	t.Parallel()
	d := newDivRig(t)
	if err := runOnce(d.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	got := jobIDs(d.jobs)
	i, k := slices.Index(got, "dividend_capture"), slices.Index(got, "option_eod")
	if i < 0 || k < 0 || i != k+1 {
		t.Fatalf("jobs %v: dividend_capture is not right after option_eod", got)
	}
	if d.res != store.InstrumentResolver(d.instr) {
		t.Error("the instrument store opened for option_eod is not the resolver of dividend_capture")
	}
	if d.events != "events-dsn" {
		t.Errorf("dividend store opened with %q", d.events)
	}
	if d.w != dividend.Writer(d.store) {
		t.Error("the writer is not the opened dividend store")
	}
	if len(d.watch) != 1 || d.watch[0].Issuer != "cik:0000320193" {
		t.Errorf("watch list %+v", d.watch)
	}
	if want := filepath.Join(d.a.getenv("OT_HOME"), "run", "ibkr.sock"); d.socket != want {
		t.Errorf("reference dialled %q, want %q", d.socket, want)
	}
	if d.ref == nil {
		t.Error("no reference client was handed to the job")
	}
	if err := backfill(d.a, []string{"--day", "2026-11-04"}); err != nil || !slices.Contains(jobIDs(d.jobs), "dividend_capture") {
		t.Fatalf("backfill err %v jobs %v", err, jobIDs(d.jobs))
	}
}

func TestDividendCaptureOffWhenBrokerDown(t *testing.T) {
	t.Parallel()
	d := newDivRig(t)
	d.fail["broker"] = errors.New("down")
	if err := runOnce(d.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	got := jobIDs(d.jobs)
	if slices.Contains(got, "dividend_capture") || !slices.Contains(got, "option_eod") {
		t.Fatalf("jobs %v: want option_eod and no dividend_capture", got)
	}
	if d.dialed.Load() != 0 || d.events != "" {
		t.Errorf("the reference client was dialled (%d) or the dividend store opened (%q) with the broker down", d.dialed.Load(), d.events)
	}
	log := d.log.String()
	if !strings.Contains(log, "dividend_capture") || !strings.Contains(log, reasonBrokerDown) || strings.Contains(log, d.a.getenv("OT_HOME")) {
		t.Errorf("log %q does not name the job and the reason, or names a path", log)
	}
}
