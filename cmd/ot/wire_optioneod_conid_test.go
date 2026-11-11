package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optioneod"
	"github.com/andrewrong/optdesk/internal/store"
)

// conidBroker answers a con_id per stock symbol (or not listed / an error) and counts the stock resolves;
// an option contract is listed with a con_id of its own.
type conidBroker struct {
	mu        sync.Mutex
	ids       map[string]int64
	errs      map[string]error
	notListed map[string]bool
	stockCall []string
}

func (b *conidBroker) ListChains(context.Context, broker.Contract) ([]broker.Chain, error) {
	return []broker.Chain{{Exchange: "OSE.JPN", TradingClass: "1111", Multiplier: "100",
		Expirations: []string{"20261210"}, Strikes: []string{"3000"}}}, nil
}

func (b *conidBroker) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if c.SecType != "STK" {
		c.ConID = 5000
		if c.Right == "P" {
			c.ConID++
		}
		return broker.Resolved{Listed: true, Contract: c}, nil
	}
	b.stockCall = append(b.stockCall, c.Symbol)
	if err := b.errs[c.Symbol]; err != nil {
		return broker.Resolved{}, err
	}
	if b.notListed[c.Symbol] {
		return broker.Resolved{}, nil
	}
	c.ConID = b.ids[c.Symbol]
	return broker.Resolved{Listed: c.ConID != 0, Contract: c}, nil
}

func (b *conidBroker) Snapshot(_ context.Context, kind broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	last := 3000.0
	out := make([]broker.Quote, len(cs))
	for i, c := range cs {
		out[i] = broker.Quote{Contract: c, Last: &last}
	}
	return out, nil
}

func (b *conidBroker) Close() error { return nil }

func (b *conidBroker) calls() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.stockCall...)
}

func jpConIDEntry(code string, conID int64) jobs.WatchEntry {
	return jobs.WatchEntry{Market: "JP", Contract: broker.Contract{SecType: "STK", Symbol: code, Exchange: "TSEJ", Currency: "JPY", ConID: conID}}
}

func conidLog() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), &buf
}

func conidWarns(buf *bytes.Buffer) []string {
	var out []string
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "level=WARN") {
			out = append(out, l)
		}
	}
	return out
}

func TestResolveJPConIDsFillsOnlyJPStockWithoutConID(t *testing.T) {
	t.Parallel()
	us := jobs.WatchEntry{Market: "US", Contract: broker.Contract{SecType: "STK", Symbol: "AAPL", ConID: 265598}}
	watch := []jobs.WatchEntry{us, jpConIDEntry("1111", 0), jpConIDEntry("7203", 7203)}
	input := append([]jobs.WatchEntry(nil), watch...)
	b := &conidBroker{ids: map[string]int64{"1111": 910001, "7203": 1}}
	log, buf := conidLog()
	got := resolveJPConIDs(context.Background(), b, watch, log)
	want := []jobs.WatchEntry{us, jpConIDEntry("1111", 910001), jpConIDEntry("7203", 7203)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if c := b.calls(); !reflect.DeepEqual(c, []string{"1111"}) {
		t.Fatalf("resolve calls %v, want one for 1111", c)
	}
	if !reflect.DeepEqual(watch, input) || watch[1].Contract.ConID != 0 {
		t.Fatalf("input written: %+v", watch)
	}
	if len(conidWarns(buf)) != 0 {
		t.Fatalf("unexpected warn: %s", buf)
	}
}

func TestResolveJPConIDsFailureLeavesEntryAndGoesOn(t *testing.T) {
	t.Parallel()
	watch := []jobs.WatchEntry{jpConIDEntry("1111", 0), jpConIDEntry("2222", 0), jpConIDEntry("3333", 0), jpConIDEntry("4444", 0)}
	b := &conidBroker{
		ids:       map[string]int64{"3333": 910003},
		errs:      map[string]error{"2222": errors.New("gateway down")},
		notListed: map[string]bool{"1111": true},
	}
	log, buf := conidLog()
	got := resolveJPConIDs(context.Background(), b, watch, log)
	for i, wantID := range []int64{0, 0, 910003, 0} {
		if got[i].Contract.ConID != wantID {
			t.Errorf("entry %s con_id %d, want %d", got[i].Contract.Symbol, got[i].Contract.ConID, wantID)
		}
	}
	if c := b.calls(); len(c) != 4 {
		t.Fatalf("resolve calls %v, want all four", c)
	}
	lines := conidWarns(buf)
	// 1111 not listed, 2222 error, 4444 listed answer with con_id 0.
	if len(lines) != 3 {
		t.Fatalf("warn lines %d: %s", len(lines), buf)
	}
	for i, sym := range []string{"1111", "2222", "4444"} {
		if !strings.Contains(lines[i], sym) {
			t.Errorf("warn line %q lacks %s", lines[i], sym)
		}
	}
}

type conidResolver struct {
	mu   sync.Mutex
	keys []store.Identifier
}

func (r *conidResolver) ResolveInstrument(_ context.Context, key store.Identifier, _ store.InstrumentSpec, _ []store.Identifier) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys = append(r.keys, key)
	return int64(len(r.keys)), nil
}

type conidInst struct {
	store.InstrumentStore
	r *conidResolver
}

func (c conidInst) ResolveInstrument(ctx context.Context, key store.Identifier, spec store.InstrumentSpec, extra []store.Identifier) (int64, error) {
	return c.r.ResolveInstrument(ctx, key, spec, extra)
}

type conidEod struct {
	store.OptionEodStore
}

func (conidEod) InsertOptionEod(context.Context, []contracts.OptionEod) error { return nil }

type openCalendar struct{ clock.Calendar }

func (openCalendar) Session(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	return clock.Session{CalendarID: id, Day: day, IsSession: true}, nil
}

func TestOptionEodJPFamilyResolvesConIDAtBuild(t *testing.T) {
	t.Parallel()
	fam, ok := jobFamilies[jobOptionEodJP]
	if !ok {
		t.Fatal("option_eod_jp family is not registered")
	}
	log, _ := conidLog()
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	clk := fakeClock{time.Date(2026, 11, 9, 16, 0, 0, 0, loc)}
	res := &conidResolver{}
	d := newDeps()
	d.newOptionEodJPWatch = optioneod.NewJPWithWatch
	b := &conidBroker{ids: map[string]int64{"1111": 910001}}
	f := &jobFactory{d: d, clk: clk, inst: conidInst{r: res}, eod: conidEod{},
		watch: []jobs.WatchEntry{jpConIDEntry("1111", 0)}}
	jc := &jobCtx{f: f, brk: b, brokerUp: true, cal: openCalendar{}}
	j, _, err := fam.build(context.Background(), &app{log: log}, jc)
	if err != nil || j == nil {
		t.Fatalf("build %v %v", j, err)
	}
	key := contracts.ExpectedKey{DatasetID: "obs.option_eod_jp", SubjectKey: "1111"}
	out := j.Run(context.Background(), "2026-11-09", []contracts.ExpectedKey{key})
	if out.Err != nil {
		t.Fatalf("result %+v: %v", out, out.Err)
	}
	want := store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(910001, 10)}
	found := false
	for _, k := range res.keys {
		found = found || k == want
	}
	if !found {
		t.Fatalf("resolver saw %v, want %v", res.keys, want)
	}
	if f.watch[0].Contract.ConID != 0 {
		t.Fatal("the factory watch list was written")
	}
}
