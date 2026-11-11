package optionopen

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type fakeBars struct{ closes map[string]string } // day -> close

func (f fakeBars) PriceBars(_ context.Context, _ int64, _ string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	var days []string
	for d := range f.closes {
		if d >= string(from) && d <= string(to) {
			days = append(days, d)
		}
	}
	sort.Strings(days)
	var out []contracts.PriceBar
	for _, d := range days {
		t, _ := time.Parse(time.DateOnly, d)
		out = append(out, contracts.PriceBar{Close: f.closes[d], SessionDay: types.SerializableDate{Time: t}})
	}
	return out, nil
}

type fakeInstruments struct {
	mu  sync.Mutex
	ids map[string]int64
	adv func()
}

func (f *fakeInstruments) ResolveInstrument(_ context.Context, key store.Identifier, _ store.InstrumentSpec, _ []store.Identifier) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.adv != nil {
		f.adv()
	}
	if f.ids == nil {
		f.ids = map[string]int64{}
	}
	k := key.Scheme + ":" + key.Value
	if _, ok := f.ids[k]; !ok {
		f.ids[k] = int64(len(f.ids) + 1)
	}
	return f.ids[k], nil
}

type fakeOptions struct {
	mu   sync.Mutex
	refs map[int64][]openquote.OptionRef
	puts int
}

func (f *fakeOptions) PutOption(_ context.Context, r openquote.OptionRef) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refs == nil {
		f.refs = map[int64][]openquote.OptionRef{}
	}
	f.refs[r.UnderlyingID] = append(f.refs[r.UnderlyingID], r)
	f.puts++
	return nil
}

func (f *fakeOptions) ListOptions(_ context.Context, id int64) ([]openquote.OptionRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]openquote.OptionRef(nil), f.refs[id]...), nil
}

type fakeMarks struct {
	mu      sync.Mutex
	marks   []contracts.OptionNotListed
	removed int
}

func (f *fakeMarks) AddNotListed(_ context.Context, ms []contracts.OptionNotListed) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.marks = append(f.marks, ms...)
	return nil
}

func (f *fakeMarks) NotListed(_ context.Context, now time.Time) ([]contracts.OptionNotListed, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []contracts.OptionNotListed
	for _, m := range f.marks {
		if m.RetryAt.After(now) {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeMarks) RemoveNotListed(_ context.Context, k openquote.NotListedKey) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var keep []contracts.OptionNotListed
	removed := false
	for _, m := range f.marks {
		if int64(m.UnderlyingID) == k.UnderlyingID && markKey(m) == refKey(k.Expiry, k.Strike, k.Right, k.TradingClass) {
			removed = true
			continue
		}
		keep = append(keep, m)
	}
	f.marks = keep
	return removed, nil
}

func (f *fakeMarks) RemoveNotListedByUnderlying(_ context.Context, id int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var keep []contracts.OptionNotListed
	n := 0
	for _, m := range f.marks {
		if int64(m.UnderlyingID) == id {
			n++
			continue
		}
		keep = append(keep, m)
	}
	f.marks = keep
	f.removed += n
	return n, nil
}

func (f *fakeMarks) RemoveNotListedExpired(context.Context, contracts.Day) (int, error) {
	return 0, nil
}

type fakePull struct{ rows []contracts.OptionOpen }

func (f fakePull) OptionOpen(context.Context, contracts.Day) ([]contracts.OptionOpen, error) {
	return f.rows, nil
}

type fakePlans struct {
	mu    sync.Mutex
	items []contracts.OptionOpenItem
}

func (f *fakePlans) PutPlan(_ context.Context, items []contracts.OptionOpenItem) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items = append(f.items, items...)
	return nil
}

type fakeAccounts struct{ pos []store.Position }

func (f fakeAccounts) Positions(_ context.Context, alias, _ string) ([]store.Position, error) {
	if alias != "A" {
		return nil, nil
	}
	return f.pos, nil
}
func (fakeAccounts) Values(context.Context, string, string) ([]store.AccountValue, error) {
	return nil, nil
}
func (fakeAccounts) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return nil, nil
}

// fakeResolver answers ResolveContract by a script per call; the default is a listed contract with a new con_id.
type fakeResolver struct {
	mu     sync.Mutex
	chains []broker.Chain
	answer func(n int, c broker.Contract) (broker.Resolved, error)
	asked  []broker.Contract
	conID  int64
	listed int
}

func (f *fakeResolver) ListChains(context.Context, broker.Contract) ([]broker.Chain, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listed++
	return f.chains, nil
}

func (f *fakeResolver) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := len(f.asked)
	f.asked = append(f.asked, c)
	if f.answer != nil {
		return f.answer(n, c)
	}
	f.conID++
	c.ConID = 1000 + f.conID
	return broker.Resolved{Listed: true, Contract: c}, nil
}

func (*fakeResolver) Snapshot(context.Context, broker.Kind, []broker.Contract) ([]broker.Quote, error) {
	return nil, fmt.Errorf("the plan and resolve jobs never ask a quote")
}
func (*fakeResolver) Close() error { return nil }

func watchOf(symbols ...string) []jobs.WatchEntry {
	var out []jobs.WatchEntry
	for i, s := range symbols {
		out = append(out, jobs.WatchEntry{Market: "US", Contract: broker.Contract{SecType: "STK", Symbol: s, Exchange: "SMART",
			Currency: "USD", ConID: int64(100 + i)}})
	}
	return out
}

// chainOf is one chain of the class with strikes lo..hi in steps of 0.1 and the expiries given.
func chainOf(class string, lo, hi int, expiries ...string) broker.Chain {
	ch := broker.Chain{Exchange: "SMART", TradingClass: class, Multiplier: "100", Expirations: expiries}
	for k := lo; k <= hi; k++ {
		ch.Strikes = append(ch.Strikes, strconv.FormatFloat(float64(k)/10, 'f', 1, 64))
	}
	return ch
}
