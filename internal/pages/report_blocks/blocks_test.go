package report_blocks_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/report_blocks"
	"github.com/andrewrong/optdesk/internal/pages/report_page"
	"github.com/andrewrong/optdesk/internal/store"
)

const day = contracts.Day("2026-11-10")

var (
	zone = time.FixedZone("owner", 8*3600)
	at   = time.Date(2026, 11, 10, 2, 0, 0, 0, time.UTC)
)

// fakeAccount holds values and positions by alias and day, and the executions by alias.
type fakeAccount struct {
	store.AccountReader
	values map[string]map[string][]store.AccountValue
	pos    map[string]map[string][]store.Position
	execs  map[string][]store.Execution
}

func (f fakeAccount) Values(_ context.Context, alias, d string) ([]store.AccountValue, error) {
	return f.values[alias][d], nil
}

func (f fakeAccount) Positions(_ context.Context, alias, d string) ([]store.Position, error) {
	return f.pos[alias][d], nil
}

func (f fakeAccount) Executions(_ context.Context, alias, from, to string) ([]store.Execution, error) {
	var out []store.Execution
	for _, e := range f.execs[alias] {
		if e.SessionDay >= from && e.SessionDay <= to {
			out = append(out, e)
		}
	}
	return out, nil
}

func vals(net, cash, excess string) []store.AccountValue {
	return []store.AccountValue{
		{Tag: "NetLiquidation", Value: net, Currency: "USD"},
		{Tag: "TotalCashValue", Value: cash, Currency: "USD"},
		{Tag: "ExcessLiquidity", Value: excess, Currency: "USD"},
	}
}

func twoAccounts() fakeAccount {
	return fakeAccount{values: map[string]map[string][]store.AccountValue{
		"main": {"2026-11-09": vals("99000.00", "40000.00", "20000.00"), "2026-11-10": vals("100000.00", "42000.00", "25000.00")},
		"wife": {"2026-11-09": vals("4900.00", "900.00", "800.00"), "2026-11-10": vals("5000.00", "1000.00", "900.00")},
	}}
}

type fakeLedger struct{}

func (fakeLedger) FlexTradeRows(context.Context, string) ([]store.FlexTradeRow, error) {
	return []store.FlexTradeRow{{TransactionID: "t1", Revision: 1, Attributes: `{"symbol":"AAA"}`}}, nil
}
func (fakeLedger) LatestOkFetch(context.Context, string, string) (store.FlexFetchRecord, bool, error) {
	return store.FlexFetchRecord{}, false, nil
}
func (fakeLedger) StoredAliases(context.Context) ([]string, error) { return nil, nil }

type fakeSessions struct{}

func (fakeSessions) SessionDays(_ context.Context, _ string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	f, _ := time.Parse("2006-01-02", string(from))
	e, _ := time.Parse("2006-01-02", string(to))
	var out []contracts.SessionDay
	for d := f; !d.After(e); d = d.AddDate(0, 0, 1) {
		out = append(out, contracts.SessionDay{CalendarID: "XNYS", SessionDay: types.SerializableDate{Time: d},
			IsSession: d.Weekday() != time.Saturday && d.Weekday() != time.Sunday})
	}
	return out, nil
}

type replayer struct{}

func (replayer) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	return compute.LedgerResult{Positions: []compute.LedgerPosition{
		{Underlying: "AAA", Status: "open", Right: "P", Quantity: "-2", Strike: "50", Multiplier: "100", Currency: "USD", PremiumIncome: "300"},
		{Underlying: "BBB", Status: "open", Right: "C", Quantity: "-1", Currency: "USD"},
	}}
}

type fakeSeries struct{ pts []contracts.SeriesPoint }

func (f fakeSeries) SeriesPoints(_ context.Context, id string, _, _ contracts.Day) ([]contracts.SeriesPoint, error) {
	if id != "SP500" {
		return nil, nil
	}
	return f.pts, nil
}

func point(d string, v float64) contracts.SeriesPoint {
	t, _ := time.Parse("2006-01-02", d)
	return contracts.SeriesPoint{SeriesID: "SP500", ObservationDate: types.SerializableDate{Time: t}, Value: &v}
}

// recorder keeps what the figures were asked, so a test can see what the builders passed.
type recorder struct {
	mu      sync.Mutex
	moves   []blockapi.AccountMoveIn
	execs   []blockapi.ExecIn
	closeIn []blockapi.CloseIn
}

type fakeFigures struct{ r *recorder }

func (f fakeFigures) AccountMove(in blockapi.AccountMoveIn) blockapi.AccountMoveOut {
	f.r.mu.Lock()
	f.r.moves = append(f.r.moves, in)
	f.r.mu.Unlock()
	out := blockapi.AccountMoveOut{
		DayAmount: compute.FigureValue{Value: "1000.00"}, DayPct: compute.FigureValue{Value: "1.01"},
		MarginBuffer: compute.FigureValue{Value: "25.00"}, CashAfterAssignment: compute.FigureValue{Value: "32000.00"},
	}
	for _, v := range in.Values {
		if v.Tag == "NetLiquidation" {
			out.Net = compute.FigureValue{Value: v.Value}
		}
	}
	if in.Alias == "wife" {
		out.CashAfterAssignment = compute.FigureValue{Value: "-5.50"}
	}
	if len(in.Prior) == 0 {
		out.DayAmount, out.DayPct = compute.FigureValue{Gap: "missing_input"}, compute.FigureValue{Gap: "missing_input"}
	}
	return out
}

// Trades gives a kind to every fill that has a prior quantity, as pagefig does.
func (f fakeFigures) Trades(_ contracts.Day, in []blockapi.ExecIn) []blockapi.TradeLine {
	f.r.mu.Lock()
	f.r.execs = append(f.r.execs, in...)
	f.r.mu.Unlock()
	out := make([]blockapi.TradeLine, 0, len(in))
	for _, e := range in {
		l := blockapi.TradeLine{Time: e.Time, Instrument: e.Symbol, Quantity: e.Quantity, Price: e.Price}
		if e.PriorQuantity == "0" {
			l.Kind = "open"
		} else if e.PriorQuantity != "" {
			l.Kind = "close"
		}
		out = append(out, l)
	}
	return out
}

func (f fakeFigures) Moves(in []blockapi.CloseIn) []blockapi.MoveLine {
	f.r.mu.Lock()
	f.r.closeIn = append(f.r.closeIn, in...)
	f.r.mu.Unlock()
	return []blockapi.MoveLine{{Name: in[0].Name, Symbol: in[0].Symbol, D1: compute.FigureValue{Value: "-0.50"},
		D5: compute.FigureValue{Gap: "no_data"}, DataDay: in[0].Days[0]}}
}
func (fakeFigures) Household(blockapi.HouseholdInput) blockapi.HouseholdResult {
	return blockapi.HouseholdResult{}
}
func (fakeFigures) Glance([]blockapi.CompanyState) []blockapi.MoneyLine   { return nil }
func (fakeFigures) Changes([]blockapi.CompanyState) []blockapi.ChangeLine { return nil }

func config(acc fakeAccount, r *recorder, pts ...contracts.SeriesPoint) report_blocks.Config {
	return report_blocks.Config{
		Zone: zone, Aliases: []string{"main", "wife"}, Account: acc, Ledger: fakeLedger{}, Sessions: fakeSessions{},
		Ledgers: func() compute.OptionLedger { return replayer{} },
		Series:  fakeSeries{pts: pts},
		Figures: func() blockapi.PageFigures { return fakeFigures{r: r} },
	}
}

func run(t *testing.T, f report_blocks.BlockFunc) map[string]any {
	t.Helper()
	b, err := f(context.Background(), day, at)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func obj(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%v is not an object", v)
	}
	return m
}

func rows(t *testing.T, block map[string]any, field string) []any {
	t.Helper()
	r, ok := obj(t, block[field])["rows"].([]any)
	if !ok {
		t.Fatalf("%s has no rows: %v", field, block[field])
	}
	return r
}

func TestOneRowPerAliasNoTotal(t *testing.T) {
	t.Parallel()
	b := run(t, report_blocks.AccountMoves(config(twoAccounts(), &recorder{})))
	rs := rows(t, b, "accounts")
	if len(rs) != 2 {
		t.Fatalf("%d rows, want one per alias", len(rs))
	}
	for i, want := range []string{"main", "wife"} {
		if got := obj(t, obj(t, rs[i])["account"])["text"]; got != want {
			t.Errorf("row %d account = %v, want %s", i, got, want)
		}
	}
	main := obj(t, rs[0])
	if net := obj(t, main["net"]); net["text"] != "100000.00 USD" || net["value"] != 100000.00 {
		t.Errorf("net = %v", net)
	}
	if day := obj(t, main["day_amount"]); day["text"] != "+1000.00 USD" || day["tone"] != "up" {
		t.Errorf("day_amount = %v", day)
	}
	if pct := obj(t, main["day_pct"]); pct["text"] != "+1.01%" {
		t.Errorf("day_pct = %v", pct)
	}
	for _, f := range []string{"biggest"} {
		if g := obj(t, main[f]); g["reason"] != "missing_quote" {
			t.Errorf("%s = %v, want missing_quote", f, g)
		}
	}
	if g := obj(t, b["movers"]); g["reason"] != "missing_quote" {
		t.Errorf("movers = %v", g)
	}
}

func TestSecondAccountMissingIsPartial(t *testing.T) {
	t.Parallel()
	acc := twoAccounts()
	delete(acc.values, "wife")
	b := run(t, report_blocks.AccountMoves(config(acc, &recorder{})))
	if b["state"] != "partial" {
		t.Errorf("state = %v, want partial", b["state"])
	}
	rs := rows(t, b, "accounts")
	if got := obj(t, obj(t, rs[0])["net"])["text"]; got != "100000.00 USD" {
		t.Errorf("main net = %v: the first account must not be touched", got)
	}
	wife := obj(t, rs[1])
	for _, f := range []string{"net", "day_amount", "day_pct"} {
		if g := obj(t, wife[f]); g["reason"] != "no_data" {
			t.Errorf("wife %s = %v, want no_data", f, g)
		}
	}
	if len(rows(t, b, "accounts")) != 2 || len(b["meters"].([]any)) != 2 {
		t.Error("the second account must keep its row and its meters")
	}
}

func TestMetersPerAccount(t *testing.T) {
	t.Parallel()
	r := &recorder{}
	b := run(t, report_blocks.AccountMoves(config(twoAccounts(), r)))
	ms := b["meters"].([]any)
	if len(ms) != 2 {
		t.Fatalf("%d meters, want 2", len(ms))
	}
	m0, m1 := obj(t, ms[0]), obj(t, ms[1])
	if m0["account"] != "main" || m1["account"] != "wife" {
		t.Errorf("accounts = %v %v", m0["account"], m1["account"])
	}
	if got := obj(t, m0["margin_buffer"])["text"]; got != "25.00%" {
		t.Errorf("margin_buffer = %v", got)
	}
	if got := obj(t, m0["cash_after_assignment"])["text"]; got != "+32000.00 USD" {
		t.Errorf("cash_after_assignment = %v", got)
	}
	if got := obj(t, m1["cash_after_assignment"]); got["text"] != "-5.50 USD" || got["tone"] != "down" {
		t.Errorf("wife cash_after_assignment = %v", got)
	}
	if len(r.moves) != 2 {
		t.Fatalf("figures asked %d times", len(r.moves))
	}
	for _, in := range r.moves {
		if len(in.Obligations) != 1 || in.Obligations[0].Underlying != "AAA" || in.Obligations[0].Contracts != "2" {
			t.Errorf("%s obligations = %+v, want the one short put of the replay", in.Alias, in.Obligations)
		}
		if len(in.Prior) == 0 {
			t.Errorf("%s has no prior capture", in.Alias)
		}
	}
}

func TestNoExecutionsIsOkEmpty(t *testing.T) {
	t.Parallel()
	b := run(t, report_blocks.Actions(config(twoAccounts(), &recorder{})))
	if b["state"] != "ok" {
		t.Errorf("state = %v, want ok", b["state"])
	}
	if gs := b["gaps"].([]any); len(gs) != 0 {
		t.Errorf("gaps = %v", gs)
	}
	if rs := rows(t, b, "trades"); len(rs) != 0 {
		t.Errorf("rows = %v", rs)
	}
	if v, ok := b["candidates"]; !ok || v != nil {
		t.Errorf("candidates = %v, want null", v)
	}
}

func TestTradesPriorFromEarlierCapture(t *testing.T) {
	t.Parallel()
	acc := twoAccounts()
	acc.pos = map[string]map[string][]store.Position{"main": {"2026-11-09": {{Symbol: "AAA", SecType: "stock", Quantity: "-100"}}}}
	t0 := time.Date(2026, 11, 10, 14, 30, 0, 0, time.UTC)
	acc.execs = map[string][]store.Execution{"main": {
		{ExecID: "3", SessionDay: "2026-11-10", ExecutedAt: t0.Add(time.Minute), Symbol: "AAA", SecType: "stock", Side: "BOT", Quantity: "10", Price: "1"},
		{ExecID: "2", SessionDay: "2026-11-10", ExecutedAt: t0, Symbol: "AAA", SecType: "stock", Side: "BOT", Quantity: "100", Price: "1"},
		{ExecID: "1", SessionDay: "2026-11-10", ExecutedAt: t0, Symbol: "BBB", SecType: "stock", Side: "SLD", Quantity: "5", Price: "2"},
	}}
	r := &recorder{}
	b := run(t, report_blocks.Actions(config(acc, r)))
	if len(r.execs) != 3 {
		t.Fatalf("%d executions passed", len(r.execs))
	}
	got := map[string]string{}
	for _, e := range r.execs {
		got[e.Symbol+e.Quantity] = e.PriorQuantity
	}
	if got["AAA100"] != "-100" || got["BBB5"] != "0" || got["AAA10"] != "" {
		t.Errorf("prior quantities = %v", got)
	}
	rs := rows(t, b, "trades")
	if len(rs) != 3 {
		t.Fatalf("%d rows", len(rs))
	}
	if first := obj(t, rs[0]); obj(t, first["time"])["text"] != "22:30" || obj(t, first["account"])["text"] != "main" {
		t.Errorf("first row = %v", first)
	}
	if g := obj(t, obj(t, rs[2])["action"]); g["reason"] != "missing_input" {
		t.Errorf("later fill of the same contract = %v, want a gap", g)
	}
	if b["state"] != "partial" {
		t.Errorf("state = %v, want partial", b["state"])
	}
}

func TestIndexRowFromSeries(t *testing.T) {
	t.Parallel()
	r := &recorder{}
	// stored out of order on purpose; the closes go to compute newest first
	c := config(twoAccounts(), r, point("2026-11-06", 5900.5), point("2026-11-09", 5950.25), point("2026-11-05", 5800))
	b := run(t, report_blocks.MarketWatch(c))
	if len(r.closeIn) != 1 || len(r.closeIn[0].Closes) != 3 || r.closeIn[0].Closes[0] != "5950.25" || r.closeIn[0].Days[2] != "2026-11-05" {
		t.Fatalf("closes passed = %+v", r.closeIn)
	}
	rs := rows(t, b, "indices")
	if len(rs) != 1 {
		t.Fatalf("%d rows", len(rs))
	}
	row := obj(t, rs[0])
	if cl := obj(t, row["close"]); cl["text"] != "5950.25" || cl["value"] != 5950.25 || cl["unit"] != "index_points" {
		t.Errorf("close = %v", cl)
	}
	if d := obj(t, row["day"]); d["text"] != "-0.50%" || d["tone"] != "down" {
		t.Errorf("day = %v", d)
	}
	if g := obj(t, row["five_day"]); g["reason"] != "no_data" {
		t.Errorf("five_day = %v", g)
	}
	if b["state"] != "partial" {
		t.Errorf("state = %v", b["state"])
	}
	empty := run(t, report_blocks.MarketWatch(config(twoAccounts(), r)))
	if g := obj(t, empty["indices"]); g["reason"] != "no_data" || empty["state"] != "gap" {
		t.Errorf("no series: indices = %v state = %v", g, empty["state"])
	}
	old := run(t, report_blocks.MarketWatch(config(twoAccounts(), r, point("2026-10-20", 5000))))
	if g := obj(t, old["indices"]); g["reason"] != "stale_input" {
		t.Errorf("old series: indices = %v", g)
	}
}

func TestCompaniesCellIsMissingQuote(t *testing.T) {
	t.Parallel()
	b := run(t, report_blocks.MarketWatch(config(twoAccounts(), &recorder{}, point("2026-11-09", 5950.25))))
	if g := obj(t, b["companies"]); g["reason"] != "missing_quote" || g["text"] == "" {
		t.Errorf("companies = %v", g)
	}
	nr := report_blocks.Config{Zone: zone}
	for name, f := range map[string]report_blocks.BlockFunc{
		"actions": report_blocks.Actions(nr), "moves": report_blocks.AccountMoves(nr), "watch": report_blocks.MarketWatch(nr),
	} {
		if m := run(t, f); m["state"] != "gap" || obj(t, m["gaps"].([]any)[0])["reason"] != "no_reader" {
			t.Errorf("%s without readers = %v", name, m)
		}
	}
}

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", filepath.Base(url)))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func validate(t *testing.T, name string, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.UseLoader(fileLoader{})
	s, err := c.Compile("https://optdesk.invalid/schemas/page.report_page.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(v); err != nil {
		t.Fatalf("%s: output is invalid: %v\n%s", name, err, b)
	}
}

func TestOutputValidatesAgainstReportSchema(t *testing.T) {
	t.Parallel()
	risks, err := os.ReadFile(filepath.Join("..", "report_page", "testdata", "risks_gap.json"))
	if err != nil {
		t.Fatal(err)
	}
	full := twoAccounts()
	full.pos = map[string]map[string][]store.Position{"main": {"2026-11-09": {{Symbol: "AAA", SecType: "stock", Quantity: "100"}}}}
	full.execs = map[string][]store.Execution{"main": {
		{ExecID: "1", SessionDay: "2026-11-10", ExecutedAt: at, Symbol: "AAA", SecType: "stock", Side: "SLD", Quantity: "100", Price: "12.5"},
		{ExecID: "2", SessionDay: "2026-11-10", ExecutedAt: at.Add(time.Minute), Symbol: "AAA", SecType: "stock", Side: "SLD", Quantity: "1", Price: "12.5"},
	}}
	noWife := twoAccounts()
	delete(noWife.values, "wife")
	cases := map[string]report_blocks.Config{
		"full":    config(full, &recorder{}, point("2026-11-09", 5950.25), point("2026-11-06", 5900)),
		"no wife": config(noWife, &recorder{}),
		"nothing": {Zone: zone},
	}
	for name, c := range cases {
		pc := report_page.Config{Market: "US", Slot: "close", Zone: zone,
			Risks:   func(context.Context, contracts.Day, time.Time) ([]byte, error) { return risks, nil },
			Actions: report_page.BlockFunc(report_blocks.Actions(c)), AccountMoves: report_page.BlockFunc(report_blocks.AccountMoves(c)),
			MarketWatch: report_page.BlockFunc(report_blocks.MarketWatch(c)),
		}
		b, err := report_page.Build(context.Background(), pc, day, at)
		if err != nil {
			t.Fatal(err)
		}
		validate(t, name, b)
	}
}
