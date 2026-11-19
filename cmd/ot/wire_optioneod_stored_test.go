package main

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optioneod"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// storedBroker answers one chain and one quote per contract; it counts the contracts it is asked to resolve.
type storedBroker struct {
	mu       sync.Mutex
	resolves int
	pairs    []broker.Contract // the contracts of the option snapshots, in order
}

func storedConID(c broker.Contract) int64 {
	s, _ := strconv.ParseFloat(c.Strike, 64)
	id := int64(s) * 10
	if c.Right == "P" {
		id++
	}
	return id
}

func (b *storedBroker) ListChains(context.Context, broker.Contract) ([]broker.Chain, error) {
	return []broker.Chain{{Exchange: "SMART", TradingClass: "AAA", Multiplier: "100",
		Expirations: []string{"20261120", "20261127", "20261204"}, Strikes: []string{"95", "100", "105"}}}, nil
}

func (b *storedBroker) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	b.mu.Lock()
	b.resolves++
	b.mu.Unlock()
	c.ConID = storedConID(c)
	return broker.Resolved{Listed: true, Contract: c}, nil
}

func (b *storedBroker) Snapshot(_ context.Context, kind broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	px := 100.0
	out := make([]broker.Quote, len(cs))
	for i, c := range cs {
		out[i] = broker.Quote{Contract: c, Last: &px, UndPrice: &px, Bid: &px, Ask: &px,
			Generation: 1, MarketDataType: 1, ReceivedAt: time.Date(2026, 11, 19, 22, 0, 0, 0, time.UTC)}
	}
	if kind == broker.KindOptionEod {
		b.mu.Lock()
		b.pairs = append(b.pairs, cs...)
		b.mu.Unlock()
	}
	return out, nil
}

func (b *storedBroker) Close() error { return nil }

type storedInst struct{ conIDs map[int64]int64 }

func (storedInst) ResolveInstrument(_ context.Context, key store.Identifier, _ store.InstrumentSpec, _ []store.Identifier) (int64, error) {
	n, _ := strconv.ParseInt(key.Value, 10, 64)
	return n + 5000000, nil
}

func (s storedInst) Instruments(_ context.Context, ids []int64) ([]store.Instrument, error) {
	var out []store.Instrument
	for _, id := range ids {
		if c, ok := s.conIDs[id]; ok {
			out = append(out, store.Instrument{ID: id, Identifiers: []store.Identifier{
				{Scheme: "ibkr_conid", Value: strconv.FormatInt(c, 10)}}})
		}
	}
	return out, nil
}

func (storedInst) Close() error { return nil }

type storedEodWriter struct{ store.OptionEodStore }

func (storedEodWriter) InsertOptionEod(context.Context, []contracts.OptionEod) error { return nil }

// storedOpenStore is the fake option open store: it lists refs for every underlying and counts its closes.
type storedOpenStore struct {
	openquote.Stores
	refs   []openquote.OptionRef
	mu     sync.Mutex
	closed int
}

func (s *storedOpenStore) ListOptions(context.Context, int64) ([]openquote.OptionRef, error) {
	return s.refs, nil
}

func (s *storedOpenStore) Close() error {
	s.mu.Lock()
	s.closed++
	s.mu.Unlock()
	return nil
}

// eodRun builds option_eod through the family of the scheduler with dsn and the opener, runs AAA for the day and
// returns the broker, the closers of the build and the log.
func eodRun(t *testing.T, dsn string, opener func(context.Context, string) (openquote.Stores, error), in storedInst) (*storedBroker, *closerSet, string) {
	t.Helper()
	a, logs := testApp(t, "")
	b := &storedBroker{}
	a.deps.newOptionEodWatch = optioneod.NewWithWatchSymbols
	a.deps.openOptionOpen = opener
	f := jobFactory{d: a.deps, inst: in, eod: storedEodWriter{}}.
		withList(listRowsOf(jobs.WatchEntry{Market: "US", Contract: broker.Contract{Symbol: "AAA", SecType: "STK", Exchange: "SMART", Currency: "USD", ConID: 1}})...)
	f.clk = fakeClock{time.Date(2026, 11, 19, 22, 0, 0, 0, time.UTC)}
	extra := &closerSet{}
	jc := newJobCtx(runOnceConfig{openDSN: dsn}, &f, b, true, nil, extra, nil)
	j, _, err := jobFamilies[jobOptionEod].build(context.Background(), a, jc)
	if err != nil || j == nil {
		t.Fatalf("build: %v %v\n%s", j, err, logs)
	}
	day := contracts.Day("2026-11-19")
	keys, _, err := j.Plan(context.Background(), day)
	if err != nil || len(keys) == 0 {
		t.Fatalf("plan: %v %v", keys, err)
	}
	if res := j.Run(context.Background(), day, keys[:1]); res.Err != nil {
		t.Fatalf("run: %+v\n%s", res.Err, logs)
	}
	return b, extra, logs.String()
}

func TestOptionEodReadsStoredContracts(t *testing.T) {
	t.Parallel()
	// A first run without the store gives the pairs the job asks for.
	base, _, _ := eodRun(t, "", nil, storedInst{})
	if base.resolves == 0 || len(base.pairs) == 0 {
		t.Fatalf("base run resolved %d, asked %d pairs", base.resolves, len(base.pairs))
	}
	in := storedInst{conIDs: map[int64]int64{}}
	st := &storedOpenStore{}
	for i, p := range base.pairs {
		id := int64(7000 + i)
		in.conIDs[id] = storedConID(p)
		st.refs = append(st.refs, openquote.OptionRef{UnderlyingID: 5000001, TradingClass: p.TradingClass, Multiplier: p.Multiplier,
			Expiry: contracts.Day(p.Expiry[:4] + "-" + p.Expiry[4:6] + "-" + p.Expiry[6:]), Strike: p.Strike + ".0", Right: p.Right, InstrumentID: id})
	}
	opener := func(context.Context, string) (openquote.Stores, error) { return st, nil }
	got, extra, _ := eodRun(t, "dsn", opener, in)
	if got.resolves != 0 {
		t.Fatalf("resolved %d contracts although all are stored", got.resolves)
	}
	if len(got.pairs) != len(base.pairs) {
		t.Fatalf("asked %d pairs, want %d", len(got.pairs), len(base.pairs))
	}
	extra.closeAll()
	if st.closed != 1 {
		t.Fatalf("store closed %d times, want 1", st.closed)
	}
}

func TestOptionEodWithoutOpenDSN(t *testing.T) {
	t.Parallel()
	const dsn = "postgres://user:secret-pw@host/db"
	called := false
	opener := func(context.Context, string) (openquote.Stores, error) { called = true; return nil, nil }
	b, _, logs := eodRun(t, "", opener, storedInst{})
	if called || b.resolves == 0 {
		t.Fatalf("opener called %v, resolves %d: job must run as before", called, b.resolves)
	}
	if n := strings.Count(logs, "level=WARN"); n != 1 || !strings.Contains(logs, jobOptionEod) || !strings.Contains(logs, reasonNoOpenDSN) {
		t.Fatalf("want one Warn naming the job and the reason:\n%s", logs)
	}
	// A store that does not open: one Warn with reasonOpenStoreFail and no DSN text.
	failing := func(_ context.Context, d string) (openquote.Stores, error) {
		return nil, &dsnErr{d}
	}
	b, _, logs = eodRun(t, dsn, failing, storedInst{})
	if b.resolves == 0 {
		t.Fatalf("job did not run as before")
	}
	leak := strings.Contains(logs, "secret-pw") || strings.Contains(logs, "postgres://")
	if n := strings.Count(logs, "level=WARN"); n != 1 || !strings.Contains(logs, reasonOpenStoreFail) || leak {
		t.Fatalf("want one Warn with the reason and no DSN:\n%s", logs)
	}
}

type dsnErr struct{ dsn string }

func (e *dsnErr) Error() string { return "cannot open " + e.dsn }
