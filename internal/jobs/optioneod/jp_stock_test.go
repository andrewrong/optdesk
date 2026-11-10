package optioneod

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
)

var stockKey1 = contracts.ExpectedKey{DatasetID: "obs.option_eod_jp", SubjectKey: "1111"}

func jpWatch(market, code, secType, buy string, conID int64) jobs.WatchEntry {
	return jobs.WatchEntry{Market: market, BuyPrice: buy,
		Contract: broker.Contract{SecType: secType, Symbol: code, Exchange: "TSEJ", Currency: "JPY", ConID: conID}}
}

func stockWatch() []jobs.WatchEntry {
	return []jobs.WatchEntry{jpWatch("JP", "2222", "STK", "", 910002), jpWatch("JP", "1111", "STK", "", 910001)}
}

// stockChain is a chain of OSE.JPN, class code, multiplier 100 with the given expirations and strikes.
func stockChainOf(code string, expirations, strikes []string) broker.Chain {
	return broker.Chain{Exchange: "OSE.JPN", TradingClass: code, Multiplier: "100", Expirations: expirations, Strikes: strikes}
}

func strikeRange(from, to, step int) []string {
	var out []string
	for s := from; s <= to; s += step {
		out = append(out, strconv.Itoa(s))
	}
	return out
}

// newStockRig builds option_eod_jp over watch at 16:00 JST on jpDay (2026-11-09); the stock price is 3000.
func newStockRig(t *testing.T, watch []jobs.WatchEntry, chains ...broker.Chain) *rig {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 9, 16, 0, 0, 0, loc)
	r := &rig{b: newBroker(), res: &fakeResolver{}, w: &fakeWriter{}, clk: clockwork.NewFakeClockAt(now)}
	r.b.price = num(3000)
	r.b.chains = chains
	j, err := NewJPWithWatch(r.b, r.res, r.w, watch, r.clk)
	if err != nil {
		t.Fatal(err)
	}
	r.job = j.(*job)
	return r
}

func subjects(t *testing.T, j contracts.Job) []string {
	t.Helper()
	keys, _, err := j.Plan(context.Background(), jpDay)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, k := range keys {
		out = append(out, k.SubjectKey)
	}
	return out
}

func strikesOf(cs []broker.Contract) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Strike)
	}
	return uniq(out)
}

func TestJPStockKeysFromWatch(t *testing.T) {
	t.Parallel()
	r := newStockRig(t, stockWatch())
	if got, want := subjects(t, r.job), []string{"N225", "1111", "2222"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("plan %v, want %v", got, want)
	}
	keys, _, _ := r.job.Plan(context.Background(), jpDay)
	for _, k := range keys {
		if k.DatasetID != "obs.option_eod_jp" {
			t.Fatalf("key %+v", k)
		}
	}
}

func TestJPStockIgnoresNonStock(t *testing.T) {
	t.Parallel()
	watch := []jobs.WatchEntry{
		jpWatch("US", "3333", "STK", "", 1),
		jpWatch("JP", "4444", "IND", "", 2),
		jpWatch("JP", "1111", "STK", "", 3),
	}
	r := newStockRig(t, watch)
	if got, want := subjects(t, r.job), []string{"N225", "1111"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("plan %v, want %v", got, want)
	}
	dup := append(stockWatch(), jpWatch("JP", "1111", "STK", "", 5))
	if _, err := NewJPWithWatch(newBroker(), &fakeResolver{}, &fakeWriter{}, dup, clockwork.NewFakeClock()); err == nil {
		t.Fatal("a repeated code must be refused")
	}
	bad := []jobs.WatchEntry{jpWatch("JP", "1111", "STK", "x", 3)}
	if _, err := NewJPWithWatch(newBroker(), &fakeResolver{}, &fakeWriter{}, bad, clockwork.NewFakeClock()); err == nil {
		t.Fatal("a buy price that is no number must be refused")
	}
}

func TestJPStockEmptyWatch(t *testing.T) {
	t.Parallel()
	r := newStockRig(t, nil)
	if got := subjects(t, r.job); !reflect.DeepEqual(got, []string{"N225"}) {
		t.Fatalf("plan %v", got)
	}
	plain, err := NewJP(r.b, r.res, r.w, r.clk)
	if err != nil {
		t.Fatal(err)
	}
	_, h1, _ := plain.Plan(context.Background(), jpDay)
	_, h2, _ := r.job.Plan(context.Background(), jpDay)
	if h1 != h2 {
		t.Fatalf("hash of the N225-only job moved: %s %s", h1, h2)
	}
	if _, err := NewJPWithWatch(nil, r.res, r.w, nil, r.clk); err == nil {
		t.Fatal("a nil broker must be refused")
	}
}

func TestJPStockChainByCodeOSE(t *testing.T) {
	t.Parallel()
	exp := []string{"20261210"}
	other := stockChainOf("2222", exp, []string{"3000", "3010"})
	smart := stockChainOf("1111", exp, []string{"3000", "3010"})
	smart.Exchange = "SMART"
	r := newStockRig(t, stockWatch(), other, smart, stockChainOf("1111", exp, []string{"2990", "3000"}))
	res := r.run(jpDay, stockKey1)
	if res.Err != nil || res.Returned != 4 {
		t.Fatalf("result %+v", res)
	}
	for _, c := range r.b.resolved {
		if c.Exchange != "OSE.JPN" || c.TradingClass != "1111" || c.Multiplier != "100" || c.Symbol != "1111" || c.Currency != "JPY" {
			t.Fatalf("contract %+v", c)
		}
	}
	if got := strikesOf(r.b.resolved); !reflect.DeepEqual(got, []string{"2990", "3000"}) {
		t.Fatalf("strikes %v come from another chain", got)
	}
}

func TestJPStockWrongMultiplier(t *testing.T) {
	t.Parallel()
	ch := stockChainOf("1111", []string{"20261210"}, []string{"3000"})
	ch.Multiplier = "1000"
	r := newStockRig(t, stockWatch(), ch)
	res := r.run(jpDay, stockKey1)
	if kindOf(res) != contracts.NoValue || !strings.Contains(res.Err.Error(), "no chain of multiplier 100") {
		t.Fatalf("result %+v", res)
	}
	if !strings.Contains(res.Err.Error(), "option_eod_jp 1111") || r.b.resolves != 0 || len(r.w.all()) != 0 {
		t.Fatalf("error %q resolves %d rows %d", res.Err, r.b.resolves, len(r.w.all()))
	}
}

func TestJPStockNoChainNoValue(t *testing.T) {
	t.Parallel()
	for name, chains := range map[string][]broker.Chain{
		"none":        nil,
		"other class": {stockChainOf("2222", []string{"20261210"}, []string{"3000"})},
	} {
		r := newStockRig(t, stockWatch(), chains...)
		res := r.run(jpDay, stockKey1)
		if kindOf(res) != contracts.NoValue || !strings.Contains(res.Err.Error(), "no listed options") {
			t.Fatalf("%s: result %+v", name, res)
		}
		if r.b.resolves != 0 || len(r.w.all()) != 0 {
			t.Fatalf("%s: resolves %d", name, r.b.resolves)
		}
	}
}

func TestJPStockChainErrPermanent(t *testing.T) {
	t.Parallel()
	r := newStockRig(t, stockWatch())
	r.b.chainErr = errors.New("gateway down")
	if res := r.run(jpDay, stockKey1); kindOf(res) != contracts.Permanent {
		t.Fatalf("result %+v", res)
	}
}

func TestJPStockBuyPriceStrikes(t *testing.T) {
	t.Parallel()
	watch := []jobs.WatchEntry{jpWatch("JP", "1111", "STK", "3000", 910001)}
	r := newStockRig(t, watch, stockChainOf("1111", []string{"20261210"}, strikeRange(2000, 4000, 10)))
	res := r.run(jpDay, stockKey1)
	if res.Err != nil {
		t.Fatalf("result %+v", res)
	}
	got := strikesOf(r.b.resolved)
	if len(got) == 0 || len(got) > 12 {
		t.Fatalf("strikes %v, want 1..12", got)
	}
	for _, s := range got {
		if v, _ := strconv.Atoi(s); v < 2400 || v > 3150 {
			t.Fatalf("strike %s outside 0.80..1.05 of the buy price", s)
		}
	}
}

func TestJPStockBandStrikeCap(t *testing.T) {
	t.Parallel()
	// 40 strikes 2810..3200 around the price 3000, two expiries inside the window, no buy price.
	r := newStockRig(t, stockWatch(), stockChainOf("1111", []string{"20261210", "20261225"}, strikeRange(2810, 3200, 10)))
	res := r.run(jpDay, stockKey1)
	if res.Err != nil {
		t.Fatalf("result %+v", res)
	}
	below, above := 0, 0
	for _, s := range strikesOf(r.b.resolved) {
		if v, _ := strconv.Atoi(s); v < 3000 {
			below++
		} else {
			above++
		}
	}
	if below > 6 || above > 6 || below+above != 12 {
		t.Fatalf("below %d above %d, want at most 6 each and 12 in all", below, above)
	}
	if n := len(r.b.resolved); n > 48 || n != 48 {
		t.Fatalf("%d contracts, want 48", n)
	}
}

func TestJPStockDTEWindow(t *testing.T) {
	t.Parallel()
	// From 2026-11-09: 13 days is 11-22, 14 is 11-23, 75 is 2027-01-23, 76 is 2027-01-24.
	r := newStockRig(t, stockWatch(), stockChainOf("1111", []string{"20261122", "20261123", "20270123", "20270124", "20270311"}, []string{"3000"}))
	if res := r.run(jpDay, stockKey1); res.Err != nil {
		t.Fatalf("result %+v", res)
	}
	var exp []string
	for _, c := range r.b.resolved {
		exp = append(exp, c.Expiry)
	}
	if got := uniq(exp); !reflect.DeepEqual(got, []string{"20261123", "20270123"}) {
		t.Fatalf("expiries %v", got)
	}
}

func TestJPStockNoExpiryNoValue(t *testing.T) {
	t.Parallel()
	for name, ch := range map[string]broker.Chain{
		"no expiry in window": stockChainOf("1111", []string{"20261112", "20270311"}, []string{"3000"}),
		"no strike":           stockChainOf("1111", []string{"20261210"}, []string{"9000"}),
	} {
		r := newStockRig(t, stockWatch(), ch)
		res := r.run(jpDay, stockKey1)
		if kindOf(res) != contracts.NoValue || !strings.Contains(res.Err.Error(), "no contract in window") {
			t.Fatalf("%s: result %+v", name, res)
		}
		if r.b.resolves != 0 {
			t.Fatalf("%s: resolves %d", name, r.b.resolves)
		}
	}
}

// thinBroker answers an option snapshot with the ask only, as a thin book does.
type thinBroker struct{ *fakeBroker }

func (b thinBroker) Snapshot(ctx context.Context, kind broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	qs, err := b.fakeBroker.Snapshot(ctx, kind, cs)
	if kind == broker.KindOptionEod {
		for i := range qs {
			qs[i].Status = broker.StatusPartial
			qs[i].Bid, qs[i].Last = nil, nil
			qs[i].Ask = num(800)
		}
	}
	return qs, err
}

func TestJPStockThinQuoteRow(t *testing.T) {
	t.Parallel()
	r := newStockRig(t, stockWatch(), stockChainOf("1111", []string{"20261210"}, []string{"3000"}))
	j, err := NewJPWithWatch(thinBroker{r.b}, r.res, r.w, stockWatch(), r.clk)
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), jpDay, []contracts.ExpectedKey{stockKey1}); res.Err != nil || res.Returned != 2 {
		t.Fatalf("result %+v", res)
	}
	for _, row := range r.w.all() {
		if row.Bid != nil || row.Last != nil || row.Ask == nil || *row.Ask != 800 || row.Status != 1 {
			t.Fatalf("row %+v", row)
		}
	}
}

func TestJPStockDecimalStrike(t *testing.T) {
	t.Parallel()
	r := newStockRig(t, stockWatch(), stockChainOf("1111", []string{"20261210"}, []string{"2983", "3000.5", "3016.67", "3033.33"}))
	if res := r.run(jpDay, stockKey1); res.Err != nil {
		t.Fatalf("result %+v", res)
	}
	if got, want := strikesOf(r.b.resolved), []string{"2983", "3000.5", "3016.67", "3033.33"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("strikes %v, want the chain text %v", got, want)
	}
}

func TestJPN225Unchanged(t *testing.T) {
	t.Parallel()
	exp := []string{"20261113"}
	r := newStockRig(t, stockWatch(),
		broker.Chain{Exchange: "OSE.JPN", TradingClass: "225", Multiplier: "1000", Expirations: exp, Strikes: strikeRange(2000, 4000, 25)},
		stockChainOf("1111", exp, []string{"3000"}))
	r.b.price = num(3000)
	if res := r.run(jpDay, jpKey); res.Err != nil {
		t.Fatalf("result %+v", res)
	}
	for _, c := range r.b.resolved {
		if c.TradingClass != "225" || c.Multiplier != "1000" {
			t.Fatalf("contract %+v", c)
		}
	}
	if n := len(strikesOf(r.b.resolved)); n != 16 {
		t.Fatalf("%d strikes, N225 keeps 8 per side", n)
	}
	// A missing chain of N225 stays a failed run, not a no_value.
	r.b.chains = []broker.Chain{stockChainOf("1111", exp, []string{"3000"})}
	if res := r.run(jpDay, jpKey); kindOf(res) != contracts.Permanent {
		t.Fatalf("result %+v", res)
	}
	r.b.chains = nil
	if res := r.run(jpDay, jpKey); kindOf(res) != contracts.Permanent {
		t.Fatalf("result %+v", res)
	}
}

func TestJPStockRowsMICAndLot(t *testing.T) {
	t.Parallel()
	r := newStockRig(t, stockWatch(), stockChainOf("1111", []string{"20261210"}, []string{"3000"}))
	res := r.run(jpDay, stockKey1)
	if res.Err != nil || res.Returned != 2 {
		t.Fatalf("result %+v", res)
	}
	und := r.res.calls[0]
	if und.key.Value != "910001" || und.spec.Type != "stock" || und.spec.MIC != "XTKS" || und.spec.CalendarID != "XTKS" || und.spec.LotSize != "100" {
		t.Fatalf("underlying %+v", und)
	}
	opt := r.res.calls[1]
	if opt.spec.Type != "option" || opt.spec.MIC != "XOSE" || opt.spec.CalendarID != "XTKS" || opt.spec.LotSize != "100" || opt.spec.Currency != "JPY" {
		t.Fatalf("option %+v", opt)
	}
	if want := "1111  261210C03000000"; opt.extra[0].Value != want {
		t.Fatalf("osi %q, want %q", opt.extra[0].Value, want)
	}
	for _, row := range r.w.all() {
		if row.Currency != "JPY" || row.UnderlyingID != 910001+5000000 {
			t.Fatalf("row %+v", row)
		}
	}
}

// The hash of the plan of NewJP for 2026-11-09 as read at the merge base of this change.
func TestJPPlanHashUnchanged(t *testing.T) {
	t.Parallel()
	_, hash, err := planFrom(manifestJPBytes, "2026-11-09")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "a92e71255baa167b29a5bf726ea7ffa372f19d0260e391e15abd63b86e46f8d0" {
		t.Fatalf("hash %s", hash)
	}
	r := newStockRig(t, nil)
	if _, h, _ := r.job.Plan(context.Background(), "2026-11-09"); h != hash {
		t.Fatalf("N225-only job hash %s", h)
	}
	with := newStockRig(t, stockWatch())
	if _, h, _ := with.job.Plan(context.Background(), "2026-11-09"); h == hash {
		t.Fatal("a plan with stock keys must hash differently")
	}
}
