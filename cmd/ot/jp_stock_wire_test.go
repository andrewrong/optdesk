package main

import (
	"context"
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

// jpStockWatch has two synthetic JP stock entries; the codes are not the owner's.
const jpStockWatch = `
[[underlying]]
issuer = "edinet:E00001"
market = "JP"
symbol = "2222"
sec_type = "STK"
exchange = "TSEJ"
currency = "JPY"
con_id = 910002

[[underlying]]
issuer = "edinet:E00002"
market = "JP"
symbol = "1111"
sec_type = "STK"
exchange = "TSEJ"
currency = "JPY"
con_id = 910001
`

func TestJPStockKeysInPlan(t *testing.T) {
	t.Parallel()
	watch, err := parseWatch([]byte(jpStockWatch))
	if err != nil {
		t.Fatal(err)
	}
	d := newDeps()
	clk := fakeClock{time.Date(2026, 11, 9, 7, 0, 0, 0, time.UTC)}
	j, err := buildOptionEodJP(d, unreachableBroker{}, &memResolver{}, &memSink{}, watch, clk)
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := j.Plan(context.Background(), "2026-11-09")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, k := range keys {
		got = append(got, k.SubjectKey)
	}
	if strings.Join(got, ",") != "N225,1111,2222" {
		t.Fatalf("plan %v, want N225,1111,2222", got)
	}
}

func TestJPFallbackNoWatchCtor(t *testing.T) {
	t.Parallel()
	d := newDeps()
	d.newOptionEodJPWatch = nil
	called := false
	d.newOptionEodJP = func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock) (contracts.Job, error) {
		called = true
		return nil, nil
	}
	watch := []jobs.WatchEntry{{Market: "JP", Contract: broker.Contract{SecType: "STK", Symbol: "1111"}}}
	if _, err := buildOptionEodJP(d, nil, nil, nil, watch, nil); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("newOptionEodJP was not used when newOptionEodJPWatch is nil")
	}
}

func TestJPDryRunHasStockKeys(t *testing.T) {
	t.Parallel()
	var ran []string
	a, out := dryApp(t, &ran)
	a.deps.readFile = dryWatchFile(t, jpStockWatch)
	a.deps.newOptionEodJPWatch = optioneod.NewJPWithWatch
	if err := runOnce(a, []string{"--dry-run", "--job", "option_eod_jp", "--day", "2026-11-10"}); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"subject=N225 ", "subject=1111 ", "subject=2222 "} {
		if !strings.Contains(out.String(), sub) {
			t.Errorf("dry run output lacks %q: %s", sub, out.String())
		}
	}
}

// TestJPDryRunResolvesConID: a JP stock of the watch without a con_id reaches the job with the con_id the broker answers.
func TestJPDryRunResolvesConID(t *testing.T) {
	t.Parallel()
	var ran []string
	a, out := dryApp(t, &ran)
	a.deps.readFile = dryWatchFile(t, `
[[underlying]]
issuer = "edinet:E00003"
market = "JP"
symbol = "3333"
sec_type = "STK"
exchange = "TSEJ"
currency = "JPY"
`)
	brk := &conidBroker{ids: map[string]int64{"3333": 910333}}
	a.deps.dialBroker = func(context.Context, string, broker.Options) (broker.Broker, error) { return brk, nil }
	var seen []jobs.WatchEntry
	a.deps.newOptionEodJPWatch = func(_ broker.Broker, _ store.InstrumentResolver, wr store.OptionEodWriter,
		w []jobs.WatchEntry, clk clock.Clock) (contracts.Job, error) {
		seen = w
		return dryJob{id: jobOptionEodJP, keys: []contracts.ExpectedKey{{DatasetID: "obs.option_eod", SubjectKey: "3333"}},
			clk: clk, w: wr, rows: map[string][]int{"3333": {1}}, ran: &ran, mu: &sync.Mutex{}}, nil
	}
	if err := runOnce(a, []string{"--dry-run", "--job", "option_eod_jp", "--day", "2026-11-10"}); err != nil {
		t.Fatal(err)
	}
	if len(brk.stockCall) != 1 || brk.stockCall[0] != "3333" {
		t.Fatalf("broker stock lookups %v", brk.stockCall)
	}
	if len(seen) != 1 || seen[0].Contract.ConID != 910333 {
		t.Fatalf("job got %+v, want con_id 910333", seen)
	}
	if strings.Contains(out.String(), "no_value") || !strings.Contains(out.String(), "subject=3333 ") {
		t.Fatalf("key line: %s", out.String())
	}
}
