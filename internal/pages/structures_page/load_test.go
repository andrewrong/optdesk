package structures_page

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var (
	testZone = time.FixedZone("CST", 8*3600)
	testAt   = time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC)
)

const testDay = contracts.Day("2026-11-09")

type stubLedger struct {
	store.OptionLedgerReader
	rows []store.FlexTradeRow
}

func (s stubLedger) FlexTradeRows(context.Context, string) ([]store.FlexTradeRow, error) {
	return s.rows, nil
}

type stubSessions struct {
	store.SessionDayReader
	days []string
}

func (s stubSessions) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	var out []contracts.SessionDay
	for _, d := range s.days {
		t, _ := time.Parse(dayLayout, d)
		out = append(out, contracts.SessionDay{CalendarID: "XNYS", IsSession: true, SessionDay: types.SerializableDate{Time: t}})
	}
	return out, nil
}

type stubAccount struct {
	store.AccountReader
	byDay map[string][]store.Position
}

func (s stubAccount) Positions(_ context.Context, _, day string) ([]store.Position, error) {
	return s.byDay[day], nil
}

type stubEod struct {
	store.OptionEodReader
	byUnderlying map[int64][]contracts.OptionEod
}

func (s stubEod) OptionEod(_ context.Context, id int64, _, _ contracts.Day) ([]contracts.OptionEod, error) {
	return s.byUnderlying[id], nil
}

type stubTerms struct {
	store.OptionTermsReader
	terms []store.OptionTerms
}

func (s stubTerms) OptionTerms(context.Context, []int64) ([]store.OptionTerms, error) {
	return s.terms, nil
}

type stubReplay struct {
	got *compute.LedgerInput
	res compute.LedgerResult
}

func (s stubReplay) Replay(in compute.LedgerInput, _ compute.LedgerParams) compute.LedgerResult {
	*s.got = in
	return s.res
}

type stubRule struct {
	res    compute.StructureResult
	in     *compute.StructureInput
	params *compute.StructureParams
}

func (s stubRule) Evaluate(in compute.StructureInput, p compute.StructureParams) compute.StructureResult {
	*s.in, *s.params = in, p
	return s.res
}

// rig is a Config over fakes and the inputs the rules received.
type rig struct {
	cfg    Config
	replay compute.LedgerInput
	in     compute.StructureInput
	params compute.StructureParams
}

func newRig(res compute.StructureResult) *rig {
	r := &rig{}
	r.cfg = Config{
		RunID: "run1", BuildID: "build1", Zone: testZone, Aliases: []string{"main", "second"},
		Ledger:   stubLedger{},
		Sessions: stubSessions{days: []string{"2026-11-04", "2026-11-05", "2026-11-06", "2026-11-09"}},
		Account:  stubAccount{}, Eod: stubEod{}, Terms: stubTerms{},
		Ledgers:    func() compute.OptionLedger { return stubReplay{got: &r.replay} },
		Structures: func() compute.OptionStructures { return stubRule{res: res, in: &r.in, params: &r.params} },
	}
	return r
}

func (r *rig) build(t *testing.T, alias string) []byte {
	t.Helper()
	b, err := buildList(context.Background(), r.cfg, alias, testDay, testAt)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func readResult(t *testing.T, name string) compute.StructureResult {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var res compute.StructureResult
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	return res
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestNoLedgerRowsIsNothingHeld(t *testing.T) {
	t.Parallel()
	r := newRig(compute.StructureResult{})
	m := decode(t, r.build(t, "main"))
	for _, g := range m["groups"].([]any) {
		if rows := g.(map[string]any)["rows"].([]any); len(rows) != 0 {
			t.Fatalf("rows = %v", rows)
		}
	}
	if m["sentence"] != emptySentenceHead+"main"+emptySentenceTail || m["run_status"] == "failed" {
		t.Fatalf("sentence = %v, status = %v", m["sentence"], m["run_status"])
	}
}

func TestLoadFeedsTheRules(t *testing.T) {
	t.Parallel()
	r := newRig(compute.StructureResult{})
	r.cfg.Ledger = stubLedger{rows: []store.FlexTradeRow{{TransactionID: "T1", Revision: 2, Attributes: `{"symbol":"AAPL","quantity":"-1"}`}}}
	r.cfg.Ledgers = func() compute.OptionLedger {
		open := compute.LedgerPosition{PositionID: "main:T1:1", Underlying: "AAPL", Status: "open"}
		return stubReplay{got: &r.replay, res: compute.LedgerResult{Positions: []compute.LedgerPosition{open}}}
	}
	r.cfg.Account = stubAccount{byDay: map[string][]store.Position{"2026-11-06": {
		{SecType: "stock", Symbol: "AAPL", ConID: 77},
		{SecType: "option", Symbol: "AAPL", Right: "put", Strike: "150", Expiry: "2026-12-18", Quantity: "-1"},
	}}}
	bid, ask, delta := 1.1, 1.3, -0.25
	r.cfg.Eod = stubEod{byUnderlying: map[int64][]contracts.OptionEod{77: {
		{InstrumentID: 5, SessionDay: types.SerializableDate{Time: time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)}, Bid: &bid, Ask: &ask},
		{InstrumentID: 5, SessionDay: types.SerializableDate{Time: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)}, Bid: &bid, Ask: &ask, Delta: &delta},
	}}}
	r.cfg.Terms = stubTerms{terms: []store.OptionTerms{{InstrumentID: 5, Right: "P", Strike: "150", ExpirationDate: "2026-12-18"}}}
	r.build(t, "main")
	if len(r.replay.Trades) != 1 || r.replay.Trades[0].Attributes["symbol"] != "AAPL" || r.replay.Trades[0].Revision != 2 {
		t.Fatalf("trades = %+v", r.replay.Trades)
	}
	if len(r.replay.SessionDays) != 4 || r.replay.AsOf != testDay {
		t.Fatalf("replay = %+v", r.replay)
	}
	if r.in.HeldDay != "2026-11-06" || len(r.in.Held) != 1 || r.in.Held[0].Right != "P" || r.in.Held[0].Underlying != "AAPL" {
		t.Fatalf("held = %v on %q", r.in.Held, r.in.HeldDay)
	}
	want := compute.StructureQuote{Underlying: "AAPL", Right: "P", Strike: "150", Expiry: "2026-12-18", Day: "2026-11-06",
		Bid: "1.1", Ask: "1.3", Delta: "-0.25"}
	if len(r.in.Quotes) != 1 || r.in.Quotes[0] != want {
		t.Fatalf("quotes = %+v, want %+v", r.in.Quotes, want)
	}
	if r.params.CaptureTarget != "0.50" || r.params.ExitDTE != 21 {
		t.Fatalf("params = %+v", r.params)
	}
}

type stubCatalog struct {
	store.OptionEodCatalog
	latest []store.OptionEodLatest
}

func (s stubCatalog) OptionEodLatest(context.Context, contracts.Day) ([]store.OptionEodLatest, error) {
	return s.latest, nil
}

type stubNames struct {
	store.InstrumentNameReader
	names []store.InstrumentName
}

func (s stubNames) InstrumentNames(context.Context, []int64) ([]store.InstrumentName, error) {
	return s.names, nil
}

// heldPutRig is a rig holding one short AAPL put, with a capture that has the option row and, when withStock
// is set, the stock row of AAPL (instrument 77); option_eod holds the put under underlying 77.
func heldPutRig(withStock bool) *rig {
	r := newRig(compute.StructureResult{})
	r.cfg.Ledgers = func() compute.OptionLedger {
		open := compute.LedgerPosition{PositionID: "main:T1:1", Underlying: "AAPL", Status: "open"}
		return stubReplay{got: &r.replay, res: compute.LedgerResult{Positions: []compute.LedgerPosition{open}}}
	}
	held := []store.Position{{SecType: "option", Symbol: "AAPL", Right: "put", Strike: "150", Expiry: "2026-12-18", Quantity: "-1"}}
	if withStock {
		held = append(held, store.Position{SecType: "stock", Symbol: "AAPL", ConID: 77})
	}
	r.cfg.Account = stubAccount{byDay: map[string][]store.Position{"2026-11-06": held}}
	bid := 1.1
	r.cfg.Eod = stubEod{byUnderlying: map[int64][]contracts.OptionEod{77: {
		{InstrumentID: 5, SessionDay: types.SerializableDate{Time: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)}, Bid: &bid},
	}}}
	r.cfg.Terms = stubTerms{terms: []store.OptionTerms{{InstrumentID: 5, Right: "P", Strike: "150", ExpirationDate: "2026-12-18"}}}
	return r
}

var wantHeldPutQuote = compute.StructureQuote{Underlying: "AAPL", Right: "P", Strike: "150", Expiry: "2026-12-18", Day: "2026-11-06", Bid: "1.1"}

func TestQuotesFoundWithoutStockRow(t *testing.T) {
	t.Parallel()
	r := heldPutRig(false)
	r.cfg.Catalog = stubCatalog{latest: []store.OptionEodLatest{{UnderlyingID: 76}, {UnderlyingID: 77}}}
	r.cfg.Names = stubNames{names: []store.InstrumentName{{InstrumentID: 76, Name: "MSFT"}, {InstrumentID: 77, Name: "AAPL"}}}
	r.build(t, "main")
	if len(r.in.Quotes) != 1 || r.in.Quotes[0] != wantHeldPutQuote {
		t.Fatalf("quotes = %+v, want %+v", r.in.Quotes, wantHeldPutQuote)
	}
}

func TestQuotesFromStockRowWhenNoCatalog(t *testing.T) {
	t.Parallel()
	r := heldPutRig(true)
	r.build(t, "main")
	if len(r.in.Quotes) != 1 || r.in.Quotes[0] != wantHeldPutQuote {
		t.Fatalf("quotes = %+v, want %+v", r.in.Quotes, wantHeldPutQuote)
	}
	r = heldPutRig(false)
	r.build(t, "main")
	if len(r.in.Quotes) != 0 {
		t.Fatalf("quotes without stock row or catalog = %+v", r.in.Quotes)
	}
}

type stubBars struct {
	rows []contracts.PriceBar
	err  error
	id   *int64
}

func (s stubBars) PriceBars(_ context.Context, id int64, basis string, _, _ contracts.Day) ([]contracts.PriceBar, error) {
	if s.id != nil {
		*s.id = id
	}
	if basis != "trades" {
		return nil, nil
	}
	return s.rows, s.err
}

func barOn(d int, closeText string) contracts.PriceBar {
	return contracts.PriceBar{SessionDay: types.SerializableDate{Time: time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC)}, Close: closeText}
}

// printsRig is heldPutRig whose option_eod holds rows of two contracts, only one of them held, each with an
// underlying price: 91.5 on 2026-11-05 (held contract), 93.25 on 2026-11-06 (another contract).
func printsRig() *rig {
	r := heldPutRig(false)
	r.cfg.Catalog = stubCatalog{latest: []store.OptionEodLatest{{UnderlyingID: 77}}}
	r.cfg.Names = stubNames{names: []store.InstrumentName{{InstrumentID: 77, Name: "AAPL"}}}
	bid, p1, p2 := 1.1, 91.5, 93.25
	day := func(d int) types.SerializableDate {
		return types.SerializableDate{Time: time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC)}
	}
	r.cfg.Eod = stubEod{byUnderlying: map[int64][]contracts.OptionEod{77: {
		{InstrumentID: 5, SessionDay: day(5), Bid: &bid, UnderlyingPrice: &p1},
		{InstrumentID: 6, SessionDay: day(6), UnderlyingPrice: &p2},
	}}}
	return r
}

func TestSourcesCarryBarsAndPrints(t *testing.T) {
	t.Parallel()
	r := printsRig()
	var id int64
	r.cfg.Bars = stubBars{rows: []contracts.PriceBar{barOn(6, "94.1")}, id: &id}
	r.build(t, "main")
	if len(r.in.Quotes) != 1 || r.in.Quotes[0].UnderlyingClose != "94.1" {
		t.Fatalf("quotes = %+v, want the bar of the same day as the newest print", r.in.Quotes)
	}
	if id != 77 {
		t.Errorf("bars read for instrument %d, want 77", id)
	}
	// a bar older than a print loses to the print, which also comes from a row of a contract that is not held
	r.cfg.Bars = stubBars{rows: []contracts.PriceBar{barOn(5, "90")}}
	r.build(t, "main")
	if r.in.Quotes[0].UnderlyingClose != "93.25" {
		t.Fatalf("close = %q, want the print of the other contract 93.25", r.in.Quotes[0].UnderlyingClose)
	}
}

func TestNilBarsGivesPrintsOnly(t *testing.T) {
	t.Parallel()
	r := printsRig()
	r.build(t, "main")
	if len(r.in.Quotes) != 1 || r.in.Quotes[0].UnderlyingClose != "93.25" {
		t.Fatalf("quotes = %+v, want the newest print 93.25", r.in.Quotes)
	}
}

func TestBarsErrorNamesThePage(t *testing.T) {
	t.Parallel()
	r := printsRig()
	r.cfg.Bars = stubBars{err: context.DeadlineExceeded}
	if _, err := buildList(context.Background(), r.cfg, "main", testDay, testAt); err == nil || !strings.Contains(err.Error(), "structures_page") {
		t.Fatalf("err = %v", err)
	}
}
