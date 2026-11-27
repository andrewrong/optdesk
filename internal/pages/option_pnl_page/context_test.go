package option_pnl_page

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

const buildDay = contracts.Day("2026-11-17")

var buildAt = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

type fakeLedger struct {
	aliases []string
	rows    []store.FlexTradeRow
	err     error
}

func (f fakeLedger) FlexTradeRows(context.Context, string) ([]store.FlexTradeRow, error) {
	return f.rows, f.err
}
func (fakeLedger) LatestOkFetch(context.Context, string, string) (store.FlexFetchRecord, bool, error) {
	return store.FlexFetchRecord{}, false, nil
}
func (f fakeLedger) StoredAliases(context.Context) ([]string, error) { return f.aliases, nil }

// fakeReplay answers from template positions: an open lot has a mark only when the input carries one
// for its contract, as the real replay does. It records every input.
type fakeReplay struct {
	mu        sync.Mutex
	positions []compute.LedgerPosition
	ins       []compute.LedgerInput
}

func (f *fakeReplay) Replay(in compute.LedgerInput, params compute.LedgerParams) compute.LedgerResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ins = append(f.ins, in)
	marks := map[string]compute.LedgerMark{}
	at := func(d contracts.Day) int {
		for i, s := range in.SessionDays {
			if s == d {
				return i
			}
		}
		return -1
	}
	for _, m := range in.Marks {
		// a mark older than MarkMaxAgeSessions sessions is not used, as the real replay does
		if i, a := at(m.Day), at(in.AsOf); i >= 0 && a >= 0 && a-i > params.MarkMaxAgeSessions {
			continue
		}
		marks[m.Contract] = m
	}
	var out []compute.LedgerPosition
	for _, p := range f.positions {
		if p.Status == "open" {
			if m, ok := marks[p.Contract]; ok {
				p.MarkStatus, p.MarkDay = "ok", m.Day
			} else {
				p.MarkStatus, p.MarkDay, p.UnrealizedPnL = "no_mark", "", ""
			}
		}
		out = append(out, p)
	}
	return compute.LedgerResult{Positions: out}
}

type fakeDays struct {
	mu    sync.Mutex
	rows  map[int64][]optionday.Day
	sizes []int
	from  contracts.Day
	to    contracts.Day
	err   error
}

func (f *fakeDays) Days(_ context.Context, ids []int64, from, to contracts.Day) ([]optionday.Day, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sizes = append(f.sizes, len(ids))
	f.from, f.to = from, to
	var out []optionday.Day
	for _, id := range ids {
		out = append(out, f.rows[id]...)
	}
	return out, f.err
}
func (*fakeDays) ChainDay(context.Context, int64, contracts.Day) ([]optionday.Day, error) {
	return nil, nil
}
func (*fakeDays) Close() error { return nil }

type fakeSessions struct{}

func (fakeSessions) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	var out []contracts.SessionDay
	for _, s := range []string{"2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13", "2026-11-16", "2026-11-17"} {
		d, _ := time.Parse(dayLayout, s)
		out = append(out, contracts.SessionDay{SessionDay: types.SerializableDate{Time: d}, IsSession: true})
	}
	return out, nil
}

type fakeStockIDs struct{ ids map[string]int64 }

func (f fakeStockIDs) StockIDs(_ context.Context, names []string) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, n := range names {
		if id, ok := f.ids[n]; ok {
			out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: n})
		}
	}
	return out, nil
}

type fakeBars struct {
	mu    sync.Mutex
	close map[int64]string
	calls []struct {
		id       int64
		basis    string
		from, to contracts.Day
	}
}

func (f *fakeBars) PriceBars(_ context.Context, id int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, struct {
		id       int64
		basis    string
		from, to contracts.Day
	}{id, basis, from, to})
	if c, ok := f.close[id]; ok {
		return []contracts.PriceBar{{Close: c}}, nil
	}
	return nil, nil
}

type fakeSeries struct {
	mu     sync.Mutex
	series []string
}

func (f *fakeSeries) SeriesPoints(_ context.Context, id string, _, _ contracts.Day) ([]contracts.SeriesPoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.series = append(f.series, id)
	v := 5.25
	d, _ := time.Parse(dayLayout, "2026-11-14")
	return []contracts.SeriesPoint{{ObservationDate: types.SerializableDate{Time: d}, Value: &v}, {ObservationDate: types.SerializableDate{Time: d}}}, nil
}

// fakeAttr answers by strike and records inputs.
type fakeAttr struct {
	mu      sync.Mutex
	results map[string]optionattrapi.Result
	err     error
	ins     []optionattrapi.Input
}

func (f *fakeAttr) Attribute(in optionattrapi.Input) (optionattrapi.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ins = append(f.ins, in)
	if f.err != nil {
		return optionattrapi.Result{}, f.err
	}
	return f.results[in.Lot.Strike], nil
}

// sumBooker adds exactly as the compute rule does, so the identity holds in the goldens; it records
// the results it was given.
type sumBooker struct {
	mu    sync.Mutex
	calls [][]optionattrapi.Result
}

func (b *sumBooker) Combine(rs []optionattrapi.Result) []optionattrapi.Book {
	b.mu.Lock()
	b.calls = append(b.calls, rs)
	b.mu.Unlock()
	out := []optionattrapi.Book{}
	idx := map[string]int{}
	sums := map[string]map[string]decimal.Decimal{}
	for _, r := range rs {
		i, ok := idx[r.Currency]
		if !ok {
			i = len(out)
			idx[r.Currency] = i
			out = append(out, optionattrapi.Book{Currency: r.Currency})
			sums[r.Currency] = map[string]decimal.Decimal{}
		}
		s := sums[r.Currency]
		d := func(t string) decimal.Decimal { v, _ := decimal.NewFromString(t); return v }
		bk := &out[i]
		if r.NoQuotes {
			bk.NoQuoteLots++
			s["nql"] = s["nql"].Add(d(r.LedgerPnL))
			continue
		}
		if r.Difference == "" {
			bk.UnreconciledLots++
			s["ul"] = s["ul"].Add(d(r.LedgerPnL))
			continue
		}
		bk.Lots++
		for k, t := range map[string]string{"entry": r.Entry, "exit": r.Exit, "comm": r.Commission, "dir": r.Direction,
			"mc": r.MoveCost, "iv": r.IVRemark, "time": r.Time, "un": r.Unsplit, "ai": r.AssignedIntrinsic,
			"led": r.LedgerPnL, "diff": r.Difference} {
			s[k] = s[k].Add(d(t))
		}
	}
	for cur, i := range idx {
		s, bk := sums[cur], &out[i]
		bk.Entry, bk.Exit, bk.Commission = s["entry"].String(), s["exit"].String(), s["comm"].String()
		bk.Direction, bk.MoveCost, bk.IVRemark = s["dir"].String(), s["mc"].String(), s["iv"].String()
		bk.Time, bk.Unsplit, bk.AssignedIntrinsic = s["time"].String(), s["un"].String(), s["ai"].String()
		bk.LedgerPnL, bk.Difference = s["led"].String(), s["diff"].String()
		bk.RentMinusCost = s["time"].Add(s["mc"]).String()
		bk.Total = s["entry"].Add(s["exit"]).Add(s["dir"]).Add(s["mc"]).Add(s["iv"]).Add(s["time"]).Add(s["un"]).Add(s["comm"]).String()
		bk.NoQuoteLedgerPnL, bk.UnreconciledLedgerPnL = s["nql"].String(), s["ul"].String()
	}
	return out
}

func dayRow(id int64, day, bid, ask, spot string) optionday.Day {
	return optionday.Day{InstrumentID: id, SessionDay: contracts.Day(day), Bid: new(bid), Ask: new(ask),
		UnderlyingPrice: new(spot), Iv: new("0.289")}
}

func lotPos(id, und, contract, strike, status string) compute.LedgerPosition {
	p := compute.LedgerPosition{PositionID: id, Alias: "main", Underlying: und, Contract: contract, Status: status,
		Right: "P", Expiry: "2026-12-18", Strike: strike, OpenedOn: "2026-11-03", OpenPrice: "1.80", Quantity: "-2",
		Multiplier: "100", Currency: "USD", Commissions: "1.30",
		Fills: []compute.LedgerFill{{Day: "2026-11-03", Action: "open", Quantity: "-2", Price: "1.80", Commission: "1.30"}}}
	if status == "open" {
		p.UnrealizedPnL = "100"
	} else {
		p.ClosedOn, p.RealizedPnL = "2026-11-13", "50"
	}
	return p
}

type rig struct {
	cfg    Config
	replay *fakeReplay
	days   *fakeDays
	bars   *fakeBars
	attr   *fakeAttr
	book   *sumBooker
	series *fakeSeries
}

// newRig is a Config of fakes with one alias "main" and no positions.
func newRig() *rig {
	r := &rig{replay: &fakeReplay{}, days: &fakeDays{rows: map[int64][]optionday.Day{}}, bars: &fakeBars{close: map[int64]string{}},
		attr: &fakeAttr{results: map[string]optionattrapi.Result{}}, book: &sumBooker{}, series: &fakeSeries{}}
	r.cfg = Config{RunID: "r1", BuildID: "b1", Scope: "s1", Zone: time.UTC,
		Ledger:   fakeLedger{aliases: []string{"main"}, rows: []store.FlexTradeRow{{TransactionID: "t1", Revision: 1, Attributes: `{}`}}},
		Sessions: fakeSessions{}, Days: r.days, StockIDs: fakeStockIDs{ids: map[string]int64{"BBB": 900, "DDD": 901}}, Bars: r.bars,
		Replay: r.replay, Attributor: r.attr, Booker: r.book, MarkMaxAgeSessions: 3, RecentDays: 30, MaxRateAgeDays: 7, CalendarID: "XNYS"}
	return r
}

func (r *rig) build(t *testing.T) viewModel {
	t.Helper()
	var vm viewModel
	if err := json.Unmarshal(r.raw(t), &vm); err != nil {
		t.Fatal(err)
	}
	return vm
}

func (r *rig) raw(t *testing.T) []byte {
	t.Helper()
	b, err := Build(context.Background(), r.cfg, r.series, buildDay, buildAt)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestChunksOf500(t *testing.T) {
	t.Parallel()
	ids := make([]int64, 1201)
	got := chunks(ids)
	if len(got) != 3 || len(got[0]) != 500 || len(got[1]) != 500 || len(got[2]) != 201 {
		t.Fatalf("chunk sizes = %d", len(got))
	}
	r := newRig()
	var ps []compute.LedgerPosition
	for i := range 501 {
		ps = append(ps, lotPos("p"+itoa(i), "AAA", itoa(1000+i), "90", "open"))
	}
	r.replay.positions = ps
	r.build(t)
	if len(r.days.sizes) != 2 || r.days.sizes[0] != 500 || r.days.sizes[1] != 1 {
		t.Fatalf("Days chunk sizes = %v", r.days.sizes)
	}
	if r.days.from != "2026-11-03" || r.days.to != buildDay {
		t.Fatalf("Days range = %s..%s", r.days.from, r.days.to)
	}
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func TestEndCloseFromBarOfClosedOn(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "BBB", "202", "38.00", "assigned")}
	r.bars.close[900] = "36.10"
	r.attr.results["38.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "50", Commission: "-1.30"}
	r.build(t)
	if len(r.attr.ins) != 1 || r.attr.ins[0].Lot.EndClose != "36.10" {
		t.Fatalf("inputs = %+v", r.attr.ins)
	}
	c := r.bars.calls[0]
	if c.id != 900 || c.basis != "trades" || c.from != "2026-11-13" || c.to != "2026-11-13" {
		t.Fatalf("bar call = %+v", c)
	}
}

func TestTwoReplaysOnePerPass(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open")}
	r.days.rows[101] = []optionday.Day{dayRow(101, "2026-11-17", "1.20", "1.30", "95")}
	r.attr.results["90.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "100"}
	r.build(t)
	if len(r.replay.ins) != 2 {
		t.Fatalf("replays = %d", len(r.replay.ins))
	}
	if len(r.replay.ins[0].Marks) != 0 || len(r.replay.ins[1].Marks) != 1 {
		t.Fatalf("marks = %d, %d", len(r.replay.ins[0].Marks), len(r.replay.ins[1].Marks))
	}
}

func TestReaderErrorIsReturned(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open")}
	r.days.err = errors.New("boom")
	if _, err := Build(context.Background(), r.cfg, nil, buildDay, buildAt); err == nil {
		t.Fatal("want the reader error")
	}
}

func TestLongPutIsSkipped(t *testing.T) {
	t.Parallel()
	r := newRig()
	long := lotPos("p2", "AAA", "202", "85.00", "open")
	long.Quantity = "2"
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open"), long}
	r.days.rows[101] = []optionday.Day{dayRow(101, "2026-11-17", "1.20", "1.30", "95")}
	r.attr.results["90.00"] = res("1", "1", "1", "1", "1", "", "-1", "0", "100")
	vm := r.build(t)
	if len(vm.Accounts[0].Lots) != 1 || !strings.Contains(strings.Join(vm.Notes, "|"), "main: 1 个买入的看跌期权仓位已跳过：本页只显示卖出的看跌期权") {
		t.Fatalf("lots = %d, notes = %v", len(vm.Accounts[0].Lots), vm.Notes)
	}
}

// CashIfAssigned adds the puts per currency by an independent rule, so the page tests can show the line.
func (*sumBooker) CashIfAssigned(puts []optionattrapi.OpenPut) ([]optionattrapi.CashDue, error) {
	out := []optionattrapi.CashDue{}
	sums := map[string][2]decimal.Decimal{}
	idx := map[string]int{}
	for _, p := range puts {
		k, e1 := decimal.NewFromString(p.Strike)
		m, e2 := decimal.NewFromString(p.Multiplier)
		q, e3 := decimal.NewFromString(p.Quantity)
		if e1 != nil || e2 != nil || e3 != nil {
			return nil, optionattrapi.ErrBadInput
		}
		q = q.Abs()
		i, ok := idx[p.Currency]
		if !ok {
			i = len(out)
			idx[p.Currency] = i
			out = append(out, optionattrapi.CashDue{Currency: p.Currency})
		}
		s := sums[p.Currency]
		sums[p.Currency] = [2]decimal.Decimal{s[0].Add(q), s[1].Add(k.Mul(m).Mul(q))}
		out[i].Lots++
	}
	for i := range out {
		s := sums[out[i].Currency]
		out[i].Contracts, out[i].Cash = s[0].String(), s[1].String()
	}
	return out, nil
}
