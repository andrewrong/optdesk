package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidity"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/compute/optionledger"
	"github.com/andrewrong/optdesk/internal/compute/optionresult"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/pages/market_state_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

func homeFixture(t *testing.T, demo bool) http.Handler {
	t.Helper()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	rd := siteReaders{}
	if demo {
		a.profile.Name = "demo"
		a.deps.newOptionLedger, a.deps.newOptionResults = optionledger.New, optionresult.New
		rd, _, _ = openSiteReaders(context.Background(), a)
	}
	return siteHandler(a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
}

func TestHomeRouteServesFiveMembers(t *testing.T) {
	t.Parallel()
	for name, demo := range map[string]bool{"no reader": false, "demo": true} {
		h := homeFixture(t, demo)
		rec := companiesGet(t, h, http.MethodGet, "/"+siteHomeRoute)
		if rec.Code != 200 {
			t.Fatalf("%s: %d %q", name, rec.Code, rec.Body.String())
		}
		var p map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var schema string
		_ = json.Unmarshal(p["schema"], &schema)
		if schema != "home_page" {
			t.Fatalf("%s: schema %q", name, schema)
		}
		for _, m := range []string{"household", "companies", "macro", "market", "options"} {
			if len(p[m]) == 0 {
				t.Errorf("%s: member %s missing", name, m)
			}
		}
		if string(p["gap_line"]) == `""` || len(p["gap_line"]) == 0 {
			t.Errorf("%s: no gap_line", name)
		}
	}
}

func TestHomeRouteNoReaderMembersAreGaps(t *testing.T) {
	t.Parallel()
	rec := companiesGet(t, homeFixture(t, false), http.MethodGet, "/home")
	var p struct {
		Overview struct {
			Gaps json.RawMessage `json:"gaps"`
		} `json:"overview"`
		GapLine string `json:"gap_line"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(p.Overview.Gaps), "[") {
		t.Fatalf("overview.gaps is not a JSON array: %q", p.Overview.Gaps)
	}
	if p.GapLine == "" || p.GapLine == "今天没有缺口" {
		t.Fatalf("gap_line %q: five members without a reader are gaps", p.GapLine)
	}
}

func TestRootRedirectsToHome(t *testing.T) {
	t.Parallel()
	rec := companiesGet(t, homeFixture(t, false), http.MethodGet, "/")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/home" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestHomePageNamed(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	p, closeFn, err := homePageNamed(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	if p.Meta.Name != homeName || p.Meta.Schema != "page.home_page.v3" {
		t.Fatalf("meta %+v", p.Meta)
	}
}

func TestHomeCompaniesRowsFromStore(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	rd := siteReaders{underlying: companiesStore(), account: holdsAAA{}, aliases: []string{"main"}}
	body, err := buildHome(t.Context(), a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"AAA"`) || strings.Contains(string(body), `"BBB"`) {
		t.Fatalf("only the held name belongs in the card: %s", body)
	}
	if !strings.Contains(string(body), "Held companies: 1") {
		t.Fatalf("say is not the held count: %s", body)
	}
}

func TestHomeCompaniesNoAccountNoRows(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	rd := siteReaders{underlying: companiesStore()}
	body, err := buildHome(t.Context(), a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"AAA"`) || !strings.Contains(string(body), "Held companies: 0") {
		t.Fatalf("no account reader must give no rows: %s", body)
	}
}

type fakeEarningsDates struct{ rows []contracts.EarningsDate }

func (f fakeEarningsDates) EarningsDates(context.Context, string, contracts.Day, contracts.Day) ([]contracts.EarningsDate, error) {
	return f.rows, nil
}

func homeEarningsCell(t *testing.T, rd siteReaders) (kind, reason, text string) {
	t.Helper()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	rd.underlying, rd.account, rd.aliases = companiesStore(), holdsAAA{}, []string{"main"}
	body, err := buildHome(t.Context(), a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Companies struct {
			Card struct {
				Rows []struct {
					Earnings struct {
						Kind   string `json:"kind"`
						Reason string `json:"reason"`
						Text   string `json:"text"`
					} `json:"earnings"`
				} `json:"rows"`
			} `json:"card"`
		} `json:"companies"`
	}
	if err := json.Unmarshal(body, &p); err != nil || len(p.Companies.Card.Rows) != 1 {
		t.Fatalf("rows %d, err %v: %s", len(p.Companies.Card.Rows), err, body)
	}
	e := p.Companies.Card.Rows[0].Earnings
	return e.Kind, e.Reason, e.Text
}

func TestHomeCompaniesEarningsFromStore(t *testing.T) {
	t.Parallel()
	report := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	rd := siteReaders{earnings: fakeEarningsDates{rows: []contracts.EarningsDate{{
		Status: gen.EarningsDateStatus("listed"), ReportDate: &types.SerializableDate{Time: report},
	}}}}
	kind, reason, text := homeEarningsCell(t, rd)
	if kind != "value" || text != "11-20" || reason == "no_reader" {
		t.Fatalf("earnings cell must carry the stored date and no no_reader gap: %q %q %q", kind, reason, text)
	}
}

func TestHomeCompaniesNoEarningsReaderIsGap(t *testing.T) {
	t.Parallel()
	kind, reason, _ := homeEarningsCell(t, siteReaders{})
	if kind != "gap" || reason != "no_reader" {
		t.Fatalf("a site without an earnings reader keeps the no_reader gap: %q %q", kind, reason)
	}
}

// holdsDEMOB holds shares of DEMOB on every day.
type holdsDEMOB struct{ holdsAAA }

func (holdsDEMOB) Positions(context.Context, string, string) ([]store.Position, error) {
	return []store.Position{{Symbol: "DEMOB", SecType: "stock", Quantity: "10"}}, nil
}

// holdsSymbol holds shares of one symbol on every day.
type holdsSymbol struct {
	holdsAAA
	symbol string
}

func (h holdsSymbol) Positions(context.Context, string, string) ([]store.Position, error) {
	return []store.Position{{Symbol: h.symbol, SecType: "stock", Quantity: "10"}}, nil
}

// fakeSymbolCatalog lists underlying 7 and names it DEMOB.
type fakeSymbolCatalog struct{}

func (fakeSymbolCatalog) OptionEodLatest(context.Context, contracts.Day) ([]store.OptionEodLatest, error) {
	return []store.OptionEodLatest{{UnderlyingID: 7, SessionDay: "2026-11-05"}}, nil
}

func (fakeSymbolCatalog) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, id := range ids {
		if id == 7 {
			out = append(out, store.InstrumentName{InstrumentID: 7, Type: "stock", Name: "DEMOB"})
		}
	}
	return out, nil
}

// recordingBars answers six ascending closes and records the instrument ids asked.
type recordingBars struct {
	mu  sync.Mutex
	ids []int64
}

func (f *recordingBars) PriceBars(_ context.Context, id int64, _ string, _, _ contracts.Day) ([]contracts.PriceBar, error) {
	f.mu.Lock()
	f.ids = append(f.ids, id)
	f.mu.Unlock()
	var out []contracts.PriceBar
	for i := range 6 {
		d := time.Date(2026, 10, 29+i, 0, 0, 0, 0, time.UTC)
		out = append(out, contracts.PriceBar{Close: strconv.Itoa(100 + i), SessionDay: types.SerializableDate{Time: d}})
	}
	return out, nil
}

func homeDayChangeCell(t *testing.T, bars bar.PriceBarReader) map[string]any {
	t.Helper()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	rd := siteReaders{
		underlying: &fakeListReader{rows: []underlying.Row{{
			Key: "US:DEMOB", Symbol: "DEMOB", Market: "US", Issuer: "cik:0000000001", Status: underlying.StatusActive,
		}}},
		account: holdsDEMOB{}, aliases: []string{"main"}, catalog: fakeSymbolCatalog{}, names: fakeSymbolCatalog{},
	}
	if bars != nil {
		rd.bars = bars
	}
	body, err := buildHome(t.Context(), a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Companies struct {
			Card struct {
				Rows []struct {
					DayChange map[string]any `json:"day_change"`
				} `json:"rows"`
			} `json:"card"`
		} `json:"companies"`
	}
	if err := json.Unmarshal(body, &p); err != nil || len(p.Companies.Card.Rows) != 1 {
		t.Fatalf("rows %d, err %v: %s", len(p.Companies.Card.Rows), err, body)
	}
	return p.Companies.Card.Rows[0].DayChange
}

func TestHomeCompaniesDayChangeFromBars(t *testing.T) {
	t.Parallel()
	bars := &recordingBars{}
	cell := homeDayChangeCell(t, bars)
	if cell["kind"] != "value" || cell["unit"] != "percent" {
		t.Fatalf("day change must be a percent value: %v", cell)
	}
	if len(bars.ids) == 0 || bars.ids[0] != 7 {
		t.Fatalf("reader called with ids %v, want 7", bars.ids)
	}
}

func TestHomeCompaniesNoBarsKeepsGap(t *testing.T) {
	t.Parallel()
	cell := homeDayChangeCell(t, nil)
	if cell["kind"] != "gap" || cell["reason"] != "no_reader" {
		t.Fatalf("no bar reader keeps the no_reader gap: %v", cell)
	}
}

// homeOptLedger is a reader with one alias and one stored trade row.
type homeOptLedger struct{ store.OptionLedgerReader }

func (homeOptLedger) StoredAliases(context.Context) ([]string, error) { return []string{"main"}, nil }

func (homeOptLedger) FlexTradeRows(context.Context, string) ([]store.FlexTradeRow, error) {
	return []store.FlexTradeRow{{Alias: "main", TransactionID: "t1", Revision: 1, Attributes: "{}"}}, nil
}

// homeOptReplay returns one short put opened in the month of the page day.
type homeOptReplay struct{}

func (homeOptReplay) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	return compute.LedgerResult{RuleVersion: 1, Positions: []compute.LedgerPosition{{
		PositionID: "main:t1:0", Status: "open", Quantity: "-1", Multiplier: "100", Currency: "USD",
		OpenedOn: "2026-11-03", OpenPrice: "2.00",
	}}}
}

// homeOptStructures recognises that position as a short put with a max loss of 800.
type homeOptStructures struct{}

func (homeOptStructures) Evaluate(in compute.StructureInput, _ compute.StructureParams) compute.StructureResult {
	return compute.StructureResult{Structures: []compute.Structure{{
		ID: in.Alias + ":sp", Alias: in.Alias, Currency: "USD", Kind: "short_put",
		Legs:    []compute.StructureLeg{{PositionID: "main:t1:0", Quantity: "-1", Multiplier: "100", OpenPrice: "2.00"}},
		Figures: compute.StructureFigures{MaxLoss: compute.FigureValue{Value: "-800"}},
	}}}
}

func TestHomeOptionsTilesShowFigures(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	rd := siteReaders{
		optionLedger: homeOptLedger{},
		ledgers:      func() compute.OptionLedger { return homeOptReplay{} },
		structures:   func() compute.OptionStructures { return homeOptStructures{} },
		results:      optionresult.New,
	}
	body, err := buildHome(t.Context(), a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Options struct {
			Card struct {
				Tiles map[string]struct {
					Figure map[string]any `json:"figure"`
				} `json:"tiles"`
			} `json:"card"`
		} `json:"options"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatal(err)
	}
	tiles := page.Options.Card.Tiles
	if f := tiles["sold_count_month"].Figure; f["kind"] != "value" || f["value"] != float64(1) || f["unit"] != "structures" {
		t.Fatalf("sold = %v", f)
	}
	if f := tiles["open_premium_usd"].Figure; f["kind"] != "value" || f["text"] != "$200.00" || f["unit"] != "usd" {
		t.Fatalf("open premium = %v", f)
	}
	if f := tiles["max_loss_usd"].Figure; f["kind"] != "value" || f["text"] != "$800.00" || f["value"] != float64(800) {
		t.Fatalf("max loss = %v", f)
	}
}

func TestHomeMacroSayIsReserves(t *testing.T) {
	t.Parallel()
	f := newLiquidityFake()
	want := liquidity.Compute(liquidityapi.Input{Day: "2026-11-05", WRESBAL: f["WRESBAL"], Params: liquidity.TrialParams()})
	if want.Reserves.Reason != "" || want.Reserves4w.Reason != "" {
		t.Fatalf("figures are gaps: %+v", want)
	}
	a, _ := testApp(t, "OT_UNUSED=1\n")
	b, err := homeBuild(t.Context(), a, "ot-page", siteReaders{series: f}, "2026-11-05", time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Macro struct {
			Card struct {
				Say string `json:"say"`
			} `json:"card"`
		} `json:"macro"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	level, _ := strconv.ParseFloat(want.Reserves.Value, 64)
	if got := p.Macro.Card.Say; !strings.HasPrefix(got, fmt.Sprintf("Reserves $%.1ftn，4 周 ", level/1e12)) ||
		!strings.HasSuffix(got, "（"+string(want.Reserves.Day)+"）。") {
		t.Fatalf("say %q, want the reserves fact of %+v", got, want.Reserves)
	}
}

// fakeStockReader names one stock that has no option_eod row.
type fakeStockReader struct{}

func (fakeStockReader) StockIDs(_ context.Context, names []string) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, n := range names {
		if n == "7203" {
			out = append(out, store.InstrumentName{InstrumentID: 11, Type: "stock", Name: "7203"})
		}
	}
	return out, nil
}

func TestHomeDayChangeForNameWithoutOptions(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	bars := &recordingBars{}
	rd := siteReaders{
		underlying: &fakeListReader{rows: []underlying.Row{{
			Key: "JP:7203", Symbol: "7203", Market: "JP", Issuer: "edinet:E00001", Status: underlying.StatusActive,
		}}},
		account: holdsSymbol{symbol: "7203"}, aliases: []string{"main"}, catalog: fakeSymbolCatalog{}, names: fakeSymbolCatalog{},
		bars: bars, ext: map[string]any{siteMarketStateIDsKey: market_state_page.StockIDReader(fakeStockReader{})},
	}
	body, err := buildHome(t.Context(), a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Companies struct {
			Card struct {
				Rows []struct {
					DayChange map[string]any `json:"day_change"`
				} `json:"rows"`
			} `json:"card"`
		} `json:"companies"`
	}
	if err := json.Unmarshal(body, &p); err != nil || len(p.Companies.Card.Rows) != 1 {
		t.Fatalf("rows %d, err %v: %s", len(p.Companies.Card.Rows), err, body)
	}
	if c := p.Companies.Card.Rows[0].DayChange; c["kind"] != "value" || len(bars.ids) == 0 || bars.ids[0] != 11 {
		t.Fatalf("day change %v, ids %v, want a value from id 11", c, bars.ids)
	}
}

func TestHomeRouteHouseholdCardIsFamily(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	var asked []string
	rd := householdReaders(&asked)
	rd.aliases = []string{"main", "other"}
	rd.optionLedger, rd.ledgers = nil, nil
	h := siteHandler(a, householdClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteHomeRoute)
	if rec.Code != 200 {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	var p struct {
		Household json.RawMessage `json:"household"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	m := string(p.Household)
	if !strings.Contains(m, "计入 1 / 2 个账户") || !strings.Contains(m, "other") {
		t.Fatalf("household member: %s", m)
	}
}

// fakeHomeEvents is a macroevent.Reader that returns the rows it holds.
type fakeHomeEvents struct{ rows []macroevent.Event }

func (f fakeHomeEvents) Between(context.Context, contracts.Day, contracts.Day) ([]macroevent.Event, error) {
	return f.rows, nil
}

func (fakeHomeEvents) Kinds(context.Context) ([]macroevent.Kind, error) { return nil, nil }

type homeMacroCard struct {
	Macro struct {
		Card struct {
			Events []struct {
				Date     string `json:"date"`
				NameText string `json:"name_text"`
			} `json:"events"`
			Gaps []struct {
				Reason string `json:"reason"`
				Text   string `json:"text"`
			} `json:"gaps"`
		} `json:"card"`
	} `json:"macro"`
}

func homeMacroCardOf(t *testing.T, ev macroevent.Reader) homeMacroCard {
	t.Helper()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	b, err := homeBuild(t.Context(), a, "ot-page", siteReaders{series: newLiquidityFake(), macroEvents: ev}, "2026-11-05",
		time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var p homeMacroCard
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestHomeMacroCardListsStoredEvents(t *testing.T) {
	t.Parallel()
	p := homeMacroCardOf(t, fakeHomeEvents{rows: []macroevent.Event{
		{Kind: "cpi", Source: "fred", Date: "2026-11-12", RunDay: "2026-11-05"},
		{Kind: "fomc", Source: "fred", Date: "2026-12-09", RunDay: "2026-11-05"},
	}})
	ev := p.Macro.Card.Events
	if len(ev) != 2 || ev[0].Date != "2026-11-12" || ev[0].NameText != "CPI" || ev[1].Date != "2026-12-09" || ev[1].NameText != "FOMC decision" {
		t.Fatalf("events = %+v", ev)
	}
}

func TestHomeMacroCardNoStoredEventsSaysSo(t *testing.T) {
	t.Parallel()
	p := homeMacroCardOf(t, fakeHomeEvents{})
	found := false
	for _, g := range p.Macro.Card.Gaps {
		found = found || g.Text == "未来 60 天没有存下的宏观事件"
	}
	if len(p.Macro.Card.Events) != 0 || !found {
		t.Fatalf("card = %+v", p.Macro.Card)
	}
}

func TestHomeMacroCardWithoutReaderKeepsGap(t *testing.T) {
	t.Parallel()
	p := homeMacroCardOf(t, nil)
	found := false
	for _, g := range p.Macro.Card.Gaps {
		found = found || g.Text == "事件日历：尚未接入"
	}
	if !found {
		t.Fatalf("card = %+v", p.Macro.Card)
	}
}

// trackedCloser counts the closes of the macro event reader.
type trackedCloser struct{ closed *atomic.Int32 }

func (c trackedCloser) Close() error { c.closed.Add(1); return nil }

func TestOpenSiteReadersGivesMacroEventReader(t *testing.T) {
	t.Parallel()
	var closed, evClosed atomic.Int32
	a := sessionApp(t, &closed)
	var dsns []string
	want := fakeHomeEvents{}
	a.deps.ext = map[string]any{extMacroEvents: macroEventsDeps{
		openReader: func(_ context.Context, dsn string) (macroevent.Reader, io.Closer, error) {
			dsns = append(dsns, dsn)
			return want, trackedCloser{&evClosed}, nil
		},
	}}
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	if len(dsns) != 1 || dsns[0] != "pages-dsn" || rd.macroEvents == nil {
		t.Fatalf("opens %v, macroEvents %v", dsns, rd.macroEvents)
	}
	if evClosed.Load() != 0 {
		t.Fatal("closed before the close function")
	}
	closeFn()
	if evClosed.Load() != 1 || closed.Load() != 7 {
		t.Fatalf("reader closed %d times, other stores %d (want 1 and 7)", evClosed.Load(), closed.Load())
	}
}

func TestOpenSiteReadersMacroEventOpenFailsClosesOpened(t *testing.T) {
	t.Parallel()
	var closed atomic.Int32
	a := sessionApp(t, &closed)
	a.deps.ext = map[string]any{extMacroEvents: macroEventsDeps{
		openReader: func(context.Context, string) (macroevent.Reader, io.Closer, error) {
			return nil, nil, errors.New("open sentinel-secret")
		},
	}}
	_, _, err := openSiteReaders(t.Context(), a)
	if err == nil || strings.Contains(err.Error(), "sentinel") {
		t.Fatalf("err %v", err)
	}
	if closed.Load() != 7 {
		t.Fatalf("closed %d, want the 7 stores already opened", closed.Load())
	}
}

func TestOpenSiteReadersDemoHasNoMacroEventReader(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	a.profile.Name = "demo"
	a.deps.ext = map[string]any{extMacroEvents: macroEventsDeps{
		openReader: func(context.Context, string) (macroevent.Reader, io.Closer, error) {
			t.Error("the reader was opened for the demo profile")
			return nil, nil, errors.New("opened")
		},
	}}
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	if rd.macroEvents != nil {
		t.Fatalf("macroEvents %v", rd.macroEvents)
	}
}

func TestHomeOverviewMarketLineLinksToMarketState(t *testing.T) {
	t.Parallel()
	for name, demo := range map[string]bool{"no reader": false, "demo": true} {
		rec := companiesGet(t, homeFixture(t, demo), http.MethodGet, "/"+siteHomeRoute)
		if rec.Code != 200 {
			t.Fatalf("%s: %d %q", name, rec.Code, rec.Body.String())
		}
		var p struct {
			Overview struct {
				Items []struct{ Label, Href string } `json:"items"`
			} `json:"overview"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		// Overview lines come in the order macro, market, household, companies, options.
		want := []string{"/macro", "/market-state", "/household", "/companies", "/options"}
		if len(p.Overview.Items) != len(want) {
			t.Fatalf("%s: %d items", name, len(p.Overview.Items))
		}
		for i, it := range p.Overview.Items {
			if it.Href != want[i] {
				t.Errorf("%s: item %d (%s) href %q, want %q", name, i, it.Label, it.Href, want[i])
			}
		}
	}
}

func TestHomeCompaniesRowsIncludeHeldFundWithoutIssuer(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	rd := siteReaders{
		underlying: &fakeListReader{rows: []underlying.Row{
			{Key: "US:SGOV", Symbol: "SGOV", Market: "US", Kind: "etf", Status: underlying.StatusActive, BuyPrice: "100.5"},
			{Key: "US:VGT", Symbol: "VGT", Market: "US", Kind: "etf", Status: underlying.StatusActive},
		}},
		account: holdsSymbol{symbol: "SGOV"}, aliases: []string{"main"},
	}
	rows, err := homeRows(t.Context(), a, rd, time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Symbol != "SGOV" || rows[0].Issuer != "" || rows[0].Market != "US" || rows[0].HandPrice != "100.5" {
		t.Fatalf("want one SGOV row with empty issuer, got %+v", rows)
	}
}
