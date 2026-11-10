package optioneod

import (
	"context"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
)

func watchEntry(market, symbol, secType, buy string, conID int64) jobs.WatchEntry {
	return jobs.WatchEntry{Issuer: "cik:0000000001", Market: market, BuyPrice: buy,
		Contract: broker.Contract{SecType: secType, Symbol: symbol, Exchange: "SMART", Currency: "USD", ConID: conID}}
}

func TestWatchUnderlyingsUSOnly(t *testing.T) {
	t.Parallel()
	watch := []jobs.WatchEntry{
		watchEntry("US", "ZZZ", "STK", "50", 2),
		watchEntry("JP", "7203", "STK", "", 3),
		watchEntry("US", "IDX", "IND", "", 4),
		watchEntry("US", "AAA", "STK", "", 1),
	}
	j, err := NewWithWatch(newBroker(), &fakeResolver{}, &fakeWriter{}, watch, clockwork.NewFakeClock())
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := j.Plan(context.Background(), "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, k := range keys {
		got = append(got, k.SubjectKey)
	}
	if !reflect.DeepEqual(got, []string{"AAA", "ZZZ"}) {
		t.Fatalf("keys %v, want the US stock entries AAA and ZZZ", got)
	}
	if _, err := NewWithWatch(newBroker(), &fakeResolver{}, &fakeWriter{}, watch[1:3], clockwork.NewFakeClock()); err == nil {
		t.Fatal("a list with no US stock entry must be refused")
	}
}

func TestDTEWindowSelectsMonthlies(t *testing.T) {
	t.Parallel()
	chain := broker.Chain{Expirations: []string{"20261113", "20261120", "20261127", "20261204", "20261218", "20261219", "20270219"},
		Strikes: []string{"100"}}
	m := manifest{MaxExpiries: 3, StrikeBand: 0.1, MaxStrikesPerSide: 8, MinDTE: 21, MaxDTE: 45}
	got := uniq(expiriesOf(selectContracts(chain, underlying{Symbol: "T", Currency: "USD"}, "2026-11-06", 100, m)))
	// DTE 21 is 2026-11-27 and DTE 45 is 2026-12-21.
	if want := []string{"20261127", "20261204", "20261218", "20261219"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expiries %v, want %v", got, want)
	}
}

func expiriesOf(cs []broker.Contract) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Expiry)
	}
	return out
}

func TestNoWindowKeepsNearestExpiries(t *testing.T) {
	t.Parallel()
	chain := broker.Chain{Expirations: []string{"20261113", "20261120", "20261127", "20261204"}, Strikes: []string{"100"}}
	m := manifest{MaxExpiries: 3, StrikeBand: 0.1, MaxStrikesPerSide: 8}
	got := uniq(expiriesOf(selectContracts(chain, underlying{Symbol: "T", Currency: "USD"}, "2026-11-06", 100, m)))
	if want := []string{"20261113", "20261120", "20261127"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expiries %v, want %v", got, want)
	}
	// New drops the window of job.toml: its job keeps the nearest max_expiries.
	jj, err := New(newBroker(), &fakeResolver{}, &fakeWriter{}, clockwork.NewFakeClock())
	if err != nil {
		t.Fatal(err)
	}
	if mm := jj.(*job).m; mm.MinDTE != 0 || mm.MaxDTE != 0 || mm.MaxStrikes != 0 {
		t.Fatalf("New must not apply the window: %+v", mm)
	}
}

func strikesFor(t *testing.T, step, buy string) []string {
	t.Helper()
	var all []string
	st, _ := strconv.Atoi(step)
	for s := 50; s <= 150; s += st {
		all = append(all, strconv.Itoa(s))
	}
	chain := broker.Chain{Expirations: []string{"20261127"}, Strikes: all}
	m := manifest{MaxExpiries: 3, StrikeBand: 0.1, MaxStrikesPerSide: 8, MinDTE: 21, MaxDTE: 45, MaxStrikes: 24,
		buy: map[string]string{"T": buy}}
	var out []string
	for _, c := range selectContracts(chain, underlying{Symbol: "T", Currency: "USD"}, "2026-11-06", 130, m) {
		if c.Right == "C" {
			out = append(out, c.Strike)
		}
	}
	return out
}

func seq(from, to, step int) []string {
	var out []string
	for s := from; s <= to; s += step {
		out = append(out, strconv.Itoa(s))
	}
	return out
}

func TestStrikeRangeFromBuyPrice(t *testing.T) {
	t.Parallel()
	got := strikesFor(t, "5", "100")
	if want := seq(80, 130, 5); !reflect.DeepEqual(got, want) {
		t.Fatalf("strikes %v, want the 11 strikes %v", got, want)
	}
}

func TestMaxStrikesCap(t *testing.T) {
	t.Parallel()
	got := strikesFor(t, "1", "100")
	if want := seq(88, 111, 1); len(got) != 24 || !reflect.DeepEqual(got, want) {
		t.Fatalf("strikes %v, want the 24 strikes 88 to 111", got)
	}
}

func TestNoBuyPriceKeepsBand(t *testing.T) {
	t.Parallel()
	chain := broker.Chain{Expirations: []string{"20261127"}, Strikes: seq(80, 120, 5)}
	m := manifest{MaxExpiries: 1, StrikeBand: 0.1, MaxStrikesPerSide: 8, MinDTE: 21, MaxDTE: 45, MaxStrikes: 24,
		buy: map[string]string{"OTHER": "100"}}
	var got []string
	for _, c := range selectContracts(chain, underlying{Symbol: "T", Currency: "USD"}, "2026-11-06", 100, m) {
		if c.Right == "C" {
			got = append(got, c.Strike)
		}
	}
	if want := []string{"90", "95", "100", "105", "110"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("strikes %v, want the 10 percent band %v", got, want)
	}
}

func windowJob(t *testing.T, win jobs.OptionEodWindow) (*job, error) {
	t.Helper()
	c, err := NewWithWatchWindow(newBroker(), &fakeResolver{}, &fakeWriter{},
		[]jobs.WatchEntry{watchEntry("US", "AAA", "STK", "50", 1)}, win, clockwork.NewFakeClock())
	if err != nil {
		return nil, err
	}
	return c.(*job), nil
}

func TestWindowOverridesManifest(t *testing.T) {
	t.Parallel()
	j, err := windowJob(t, jobs.OptionEodWindow{MinDTE: 7, MaxDTE: 30, MaxStrikes: 10})
	if err != nil {
		t.Fatal(err)
	}
	if j.m.MinDTE != 7 || j.m.MaxDTE != 30 || j.m.MaxStrikes != 10 {
		t.Fatalf("window %d %d %d, want 7 30 10", j.m.MinDTE, j.m.MaxDTE, j.m.MaxStrikes)
	}
	j, err = windowJob(t, jobs.OptionEodWindow{MaxDTE: 60})
	if err != nil {
		t.Fatal(err)
	}
	if j.m.MinDTE != 21 || j.m.MaxDTE != 60 || j.m.MaxStrikes != 24 {
		t.Fatalf("partial window %d %d %d, want 21 60 24", j.m.MinDTE, j.m.MaxDTE, j.m.MaxStrikes)
	}
}

func TestZeroWindowKeepsManifest(t *testing.T) {
	t.Parallel()
	j, err := windowJob(t, jobs.OptionEodWindow{})
	if err != nil {
		t.Fatal(err)
	}
	if j.m.MinDTE != 21 || j.m.MaxDTE != 45 || j.m.MaxStrikes != 24 {
		t.Fatalf("window %d %d %d, want the manifest 21 45 24", j.m.MinDTE, j.m.MaxDTE, j.m.MaxStrikes)
	}
}

func TestWindowMinAboveMaxFails(t *testing.T) {
	t.Parallel()
	_, err := windowJob(t, jobs.OptionEodWindow{MinDTE: 50, MaxDTE: 40})
	if err == nil {
		t.Fatal("min_dte above max_dte must be refused")
	}
	if want := (manifest{MinDTE: 50, MaxDTE: 40}).validate(); want == nil || err.Error() != want.Error() {
		t.Fatalf("error %v, want the validate error %v", err, want)
	}
}

// askedTranslator removes spaces from a root, as the real translator does for a share class, and records the
// symbols it was asked about.
type askedTranslator struct {
	noSpace
	mu    sync.Mutex
	asked []string
}

func (a *askedTranslator) OSIRoot(s string) string {
	a.mu.Lock()
	a.asked = append(a.asked, s)
	a.mu.Unlock()
	return a.noSpace.OSIRoot(s)
}

func TestWatchSymbolsShareClass(t *testing.T) {
	t.Parallel()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 6, 17, 0, 0, 0, loc)
	b := newBroker()
	b.chains = []broker.Chain{{Exchange: "SMART", TradingClass: "BRKB", Multiplier: "100",
		Expirations: []string{"20261106"}, Strikes: []string{"95", "100", "105"}}}
	tr := &askedTranslator{}
	watch := []jobs.WatchEntry{watchEntry("US", "BRK B", "STK", "", 1)}
	var ctor jobs.NewOptionEodWatchSymbolsFunc = NewWithWatchSymbols
	j, err := ctor(b, &fakeResolver{}, &fakeWriter{}, watch, clockwork.NewFakeClockAt(now), tr)
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), "2026-11-06", []contracts.ExpectedKey{{DatasetID: "obs.option_eod", SubjectKey: "BRK B"}})
	_ = res
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if !slices.Contains(tr.asked, "BRK B") {
		t.Fatalf("translator asked for %v, want BRK B", tr.asked)
	}
}

func TestWatchSymbolsNilTranslator(t *testing.T) {
	t.Parallel()
	watch := []jobs.WatchEntry{
		watchEntry("US", "ZZZ", "STK", "50", 2),
		watchEntry("US", "BRK B", "STK", "", 1),
		watchEntry("JP", "7203", "STK", "", 3),
	}
	a, err := NewWithWatch(newBroker(), &fakeResolver{}, &fakeWriter{}, watch, clockwork.NewFakeClock())
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewWithWatchSymbols(newBroker(), &fakeResolver{}, &fakeWriter{}, watch, clockwork.NewFakeClock(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.(*job).m.Underlying, b.(*job).m.Underlying) {
		t.Fatalf("underlyings differ: %v vs %v", a.(*job).m.Underlying, b.(*job).m.Underlying)
	}
	ka, _, err := a.Plan(context.Background(), "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	kb, _, err := b.Plan(context.Background(), "2026-11-06")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ka, kb) || len(ka) != 2 {
		t.Fatalf("keys differ: %v vs %v", ka, kb)
	}
}
