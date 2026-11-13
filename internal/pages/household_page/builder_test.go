package household_page_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/household_page"
	"github.com/andrewrong/optdesk/internal/store"
)

var zone = time.FixedZone("owner", 8*3600)

var at = time.Date(2026, 11, 10, 2, 0, 0, 0, time.UTC)

const day = contracts.Day("2026-11-10")

// fakeAccount holds values by alias and day; it records the aliases it was asked about.
type fakeAccount struct {
	store.AccountReader
	mu     sync.Mutex
	values map[string]map[string][]store.AccountValue
	failOn string
	asked  []string
	stored []store.Position
}

func (f *fakeAccount) Positions(context.Context, string, string) ([]store.Position, error) {
	return f.stored, nil
}

func (f *fakeAccount) Values(_ context.Context, alias, d string) ([]store.AccountValue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, alias)
	if alias == f.failOn {
		return nil, errors.New("boom")
	}
	return f.values[alias][d], nil
}

func vals(d, net, cash string) []store.AccountValue {
	cap := time.Date(2026, 11, 10, 1, 0, 0, 0, time.UTC)
	return []store.AccountValue{
		{Alias: "main", SessionDay: d, Tag: "NetLiquidation", Value: net, Currency: "USD", CapturedAt: cap},
		{Alias: "main", SessionDay: d, Tag: "TotalCashValue", Value: cash, Currency: "USD", CapturedAt: cap},
	}
}

func account() *fakeAccount {
	return &fakeAccount{values: map[string]map[string][]store.AccountValue{
		"main": {
			"2026-10-30": vals("2026-10-30", "98000.00", "40000.00"),
			"2026-11-09": vals("2026-11-09", "99000.50", "41000.00"),
			"2026-11-10": vals("2026-11-10", "100000.00", "42000.00"),
		},
		"wife": {"2026-11-10": vals("2026-11-10", "5000.00", "1000.00")},
	}}
}

type fakeLedger struct{}

func (fakeLedger) FlexTradeRows(context.Context, string) ([]store.FlexTradeRow, error) {
	return []store.FlexTradeRow{{TransactionID: "t1", Revision: 1, Attributes: `{"symbol":"AAA"}`, RecordedAt: at}}, nil
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

// fakeFigures answers the way pagefig does: the values it is given, gap lines where marks are needed.
type fakeFigures struct{ got *blockapi.HouseholdInput }

func ok(name, amount string) blockapi.MoneyLine {
	return blockapi.MoneyLine{Name: name, Currency: "USD", Amount: compute.FigureValue{Value: amount}}
}

func gap(name, reason string) blockapi.MoneyLine {
	return blockapi.MoneyLine{Name: name, Currency: "USD", Amount: compute.FigureValue{Gap: reason}}
}

func (f fakeFigures) Household(in blockapi.HouseholdInput) blockapi.HouseholdResult {
	*f.got = in
	return blockapi.HouseholdResult{
		Overview: []blockapi.MoneyLine{ok("Net value", "100000.00")},
		Accounts: []blockapi.MoneyLine{
			ok("Net liquidation", "100000.00"),
			ok("Cash", "42000.00"),
			gap("Stock value", "missing_quote"),
			gap("Option value", "missing_quote"),
		},
		Obligations: []blockapi.MoneyLine{ok("AAA short put", "10000.00")},
		Income:      []blockapi.MoneyLine{ok("Premium this month", "300.00")},
		CashAfter:   []blockapi.MoneyLine{ok("Cash after assignment", "32000.00")},
		Stress:      []blockapi.MoneyLine{ok("Cash after all assignments", "-5.50"), gap("Underlying down 10%", "missing_quote")},
		MonthChange: []blockapi.MoneyLine{
			ok("Month start", "98000.00"),
			ok("Closed option result", "300.00"),
			gap("Trading result", "missing_quote"),
			ok("Other", "1700.00"),
			ok("Month end", "100000.00"),
		},
		Trend: []blockapi.DayValue{
			{Day: "2026-11-09", NetLiquidation: "99000.50", Currency: "USD"},
			{Day: "2026-11-10", NetLiquidation: "100000.00", Currency: "USD"},
		},
	}
}
func (fakeFigures) Trades(contracts.Day, []blockapi.ExecIn) []blockapi.TradeLine { return nil }
func (fakeFigures) AccountMove(blockapi.AccountMoveIn) blockapi.AccountMoveOut {
	return blockapi.AccountMoveOut{}
}
func (fakeFigures) Moves([]blockapi.CloseIn) []blockapi.MoveLine          { return nil }
func (fakeFigures) Glance([]blockapi.CompanyState) []blockapi.MoneyLine   { return nil }
func (fakeFigures) Changes([]blockapi.CompanyState) []blockapi.ChangeLine { return nil }

func config(acc *fakeAccount, got *blockapi.HouseholdInput) household_page.Config {
	return household_page.Config{
		RunID: "run1", BuildID: "b1", Zone: zone, Aliases: []string{"main", "wife"},
		Account: acc, Ledger: fakeLedger{}, Sessions: fakeSessions{},
		Ledgers: func() compute.OptionLedger { return replayer{} },
		Figures: func() blockapi.PageFigures { return fakeFigures{got: got} },
	}
}

func build(t *testing.T, c household_page.Config, alias string) (map[string]any, []byte) {
	t.Helper()
	b, err := household_page.NewPage(c, alias).Build(context.Background(), nil, day, at)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m, b
}

func gapOf(t *testing.T, m map[string]any, block string) map[string]any {
	t.Helper()
	b, _ := m[block].(map[string]any)
	g, _ := b["gap"].(map[string]any)
	if g == nil {
		t.Fatalf("block %s has no gap: %v", block, b)
	}
	return g
}

func TestUnknownAliasIsGapPage(t *testing.T) {
	t.Parallel()
	acc := account()
	var got blockapi.HouseholdInput
	m, b := build(t, config(acc, &got), "stranger")
	if m["run_status"] != "failed" || len(m["scopes"].([]any)) != 0 {
		t.Fatalf("not a gap page: %s", b)
	}
	for _, blk := range []string{"overview", "cash_after", "stress", "trend", "month_change", "movers"} {
		if g := gapOf(t, m, blk); g["text"] != "Unknown account" {
			t.Fatalf("%s gap = %v", blk, g)
		}
	}
	if len(acc.asked) != 0 {
		t.Fatalf("an unknown alias read the account: %v", acc.asked)
	}
	validate(t, b)
}

func TestOneScopeNoPooling(t *testing.T) {
	t.Parallel()
	acc := account()
	var got blockapi.HouseholdInput
	m, b := build(t, config(acc, &got), "")
	if m["scope"] != "main" {
		t.Fatalf("scope = %v", m["scope"])
	}
	scopes := m["scopes"].([]any)
	if len(scopes) != 1 || scopes[0].(map[string]any)["key"] != "main" {
		t.Fatalf("scopes = %v", scopes)
	}
	for _, a := range acc.asked {
		if a != "main" {
			t.Fatalf("the main page read alias %q", a)
		}
	}
	if got.Alias != "main" || got.Day != "2026-11-10" {
		t.Fatalf("figure input = %+v", got)
	}
	if len(got.Obligations) != 1 || got.Obligations[0].Contracts != "2" || got.Obligations[0].Strike != "50" {
		t.Fatalf("obligations = %+v", got.Obligations)
	}
	if len(got.MonthStart) != 2 || got.MonthStart[0].Value != "98000.00" {
		t.Fatalf("month start = %+v", got.MonthStart)
	}
	if len(got.History) != 3 {
		t.Fatalf("history = %+v", got.History)
	}
	m2, _ := build(t, config(account(), &got), "wife")
	if m2["scope"] != "wife" || len(m2["scopes"].([]any)) != 1 {
		t.Fatalf("second page = %v", m2["scopes"])
	}
	validate(t, b)
}

func TestSecondAccountFailureLeavesMainAlone(t *testing.T) {
	t.Parallel()
	acc := account()
	acc.failOn = "wife"
	var got blockapi.HouseholdInput
	c := config(acc, &got)
	if _, err := household_page.NewPage(c, "wife").Build(context.Background(), nil, day, at); err == nil {
		t.Fatal("a read failure of the second account must be an error of its own page")
	}
	m, _ := build(t, c, "main")
	if m["run_status"] != "partial" {
		t.Fatalf("main page = %v", m["run_status"])
	}
}

func TestMarkBlocksAreMissingQuoteGaps(t *testing.T) {
	t.Parallel()
	var got blockapi.HouseholdInput
	m, _ := build(t, config(account(), &got), "main")
	for blk, reason := range map[string]string{"composition": "missing_quote", "allocation": "missing_quote",
		"concentration": "missing_quote", "movers": "no_data"} {
		if g := gapOf(t, m, blk); g["reason"] != reason {
			t.Fatalf("%s gap = %v", blk, g)
		}
	}
	if g := gapOf(t, m, "stress"); g["reason"] != "missing_quote" {
		t.Fatalf("stress gap = %v", g)
	}
	rows := m["stress"].(map[string]any)["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("stress rows = %v", rows)
	}
	if u := rows[0].(map[string]any)["amount"].(map[string]any)["unit"]; u != "usd_signed" {
		t.Fatalf("negative cash unit = %v", u)
	}
	for _, blk := range []string{"overview", "cash_after", "obligations", "income"} {
		if m[blk].(map[string]any)["gap"] != nil {
			t.Fatalf("%s has a gap", blk)
		}
	}
	acct := m["accounts"].(map[string]any)["rows"].([]any)[0].(map[string]any)
	if acct["stock"].(map[string]any)["gap"] == nil || acct["options"].(map[string]any)["gap"] == nil {
		t.Fatalf("stock and option cells must be gaps: %v", acct)
	}
}

func TestNilReadersGiveGaps(t *testing.T) {
	t.Parallel()
	var got blockapi.HouseholdInput
	full := config(account(), &got)
	cases := map[string]func(*household_page.Config){
		"account":  func(c *household_page.Config) { c.Account = nil },
		"ledger":   func(c *household_page.Config) { c.Ledger = nil },
		"sessions": func(c *household_page.Config) { c.Sessions = nil },
		"ledgers":  func(c *household_page.Config) { c.Ledgers = nil },
		"figures":  func(c *household_page.Config) { c.Figures = nil },
	}
	for name, cut := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := full
			cut(&c)
			m, b := build(t, c, "main")
			if g := gapOf(t, m, "overview"); g["reason"] != "no_reader" {
				t.Fatalf("gap = %v", g)
			}
			if len(m["scopes"].([]any)) != 1 {
				t.Fatalf("scopes = %v", m["scopes"])
			}
			validate(t, b)
		})
	}
}

func TestStaleCaptureIsGapPage(t *testing.T) {
	t.Parallel()
	acc := account()
	acc.values["main"] = map[string][]store.AccountValue{"2026-11-02": vals("2026-11-02", "1.00", "1.00")}
	var got blockapi.HouseholdInput
	m, _ := build(t, config(acc, &got), "main")
	if g := gapOf(t, m, "overview"); g["reason"] != "stale_input" {
		t.Fatalf("gap = %v", g)
	}
}

func TestNoCaptureIsGapPage(t *testing.T) {
	t.Parallel()
	acc := account()
	acc.values["main"] = nil
	var got blockapi.HouseholdInput
	m, _ := build(t, config(acc, &got), "main")
	if g := gapOf(t, m, "overview"); g["reason"] != "no_data" {
		t.Fatalf("gap = %v", g)
	}
}

func TestNilZoneIsError(t *testing.T) {
	t.Parallel()
	var got blockapi.HouseholdInput
	c := config(account(), &got)
	c.Zone = nil
	if _, err := household_page.NewPage(c, "main").Build(context.Background(), nil, day, at); err == nil {
		t.Fatal("want an error")
	}
}

func TestPageMeta(t *testing.T) {
	t.Parallel()
	p := household_page.NewPage(household_page.Config{}, "main")
	if p.Meta.Name != "household_page" || p.Meta.Schema != "page.household_page.v1" {
		t.Fatalf("meta = %+v", p.Meta)
	}
}

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	file := filepath.Base(url)
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", file))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func validate(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile("https://optdesk.invalid/schemas/page.household_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(v); err != nil {
		t.Fatalf("output is invalid: %v\n%s", err, b)
	}
}

func TestOutputValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	var got blockapi.HouseholdInput
	c := config(account(), &got)
	for _, alias := range []string{"main", "", "wife", "stranger", "bad alias!"} {
		_, b := build(t, c, alias)
		validate(t, b)
	}
	// pages that carry the ledger gap and the month-start gap go through the schema too
	withPut := account()
	withPut.stored = []store.Position{shortPut("7974", "7250")}
	m, b := build(t, config(withPut, &got), "main")
	ledgerBlocks(t, m, true)
	validate(t, b)
	noStart := account()
	delete(noStart.values["main"], "2026-10-30")
	m, b = build(t, config(noStart, &got), "main")
	gapOf(t, m, "month_change")
	validate(t, b)
}

type emptyReplayer struct{}

func (emptyReplayer) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	return compute.LedgerResult{}
}

// ledgerOf is a replayer whose ledger holds one open short put of 1 contract.
type ledgerOf struct{ underlying, strike, expiry string }

func (l ledgerOf) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	return compute.LedgerResult{Positions: []compute.LedgerPosition{
		{Underlying: l.underlying, Status: "open", Right: "P", Quantity: "-1", Strike: l.strike, Expiry: l.expiry,
			Multiplier: "1", Currency: "JPY", PremiumIncome: "10"},
	}}
}

func shortPut(sym, strike string) store.Position {
	return store.Position{Symbol: sym, SecType: "option", Right: "put", Quantity: "-1", Currency: "JPY",
		Strike: strike, Expiry: "2026-11-12"}
}

func ledgerBlocks(t *testing.T, m map[string]any, wantGap bool) {
	t.Helper()
	for _, blk := range []string{"obligations", "income", "cash_after"} {
		g, _ := m[blk].(map[string]any)["gap"].(map[string]any)
		if !wantGap {
			if g != nil {
				t.Fatalf("%s has a gap: %v", blk, g)
			}
			continue
		}
		if g == nil || g["reason"] != "no_data" {
			t.Fatalf("%s gap = %v", blk, g)
		}
		text, _ := g["text"].(string)
		if !strings.Contains(text, "7974") || !strings.Contains(text, "option ledger") {
			t.Fatalf("%s gap text = %q", blk, text)
		}
	}
}

func TestStoredShortPutMissingFromLedgerIsGap(t *testing.T) {
	t.Parallel()
	acc := account()
	acc.stored = []store.Position{shortPut("7974", "7250")}
	var got blockapi.HouseholdInput
	c := config(acc, &got)
	c.Ledgers = func() compute.OptionLedger { return emptyReplayer{} }
	m, b := build(t, c, "main")
	ledgerBlocks(t, m, true)
	if rows := m["cash_after"].(map[string]any)["rows"].([]any); len(rows) != 1 {
		t.Fatalf("cash_after rows = %v, want the row kept beside the gap", rows)
	}
	validate(t, b)
}

func TestStoredShortPutInLedgerIsNoGap(t *testing.T) {
	t.Parallel()
	acc := account()
	acc.stored = []store.Position{shortPut("7974", "7250")}
	var got blockapi.HouseholdInput
	c := config(acc, &got)
	c.Ledgers = func() compute.OptionLedger { return ledgerOf{"7974", "7250", "20261112"} }
	m, b := build(t, c, "main")
	ledgerBlocks(t, m, false)
	if rows := m["cash_after"].(map[string]any)["rows"].([]any); len(rows) != 1 {
		t.Fatalf("cash_after rows = %v", rows)
	}
	validate(t, b)
}

func TestNoStoredOptionAndEmptyLedgerIsNoGap(t *testing.T) {
	t.Parallel()
	var got blockapi.HouseholdInput
	c := config(account(), &got)
	c.Ledgers = func() compute.OptionLedger { return emptyReplayer{} }
	m, _ := build(t, c, "main")
	ledgerBlocks(t, m, false)
}

func TestManyMissingShortPutsNameFiveAndFit(t *testing.T) {
	t.Parallel()
	acc := account()
	for i := range 7 {
		acc.stored = append(acc.stored, shortPut("7974", "725"+strings.Repeat("0", i)))
	}
	var got blockapi.HouseholdInput
	c := config(acc, &got)
	c.Ledgers = func() compute.OptionLedger { return emptyReplayer{} }
	m, b := build(t, c, "main")
	text := gapOf(t, m, "obligations")["text"].(string)
	if n := strings.Count(text, "7974"); n > 5 || n == 0 || len([]rune(text)) > 200 || !strings.HasPrefix(text, "7 short put(s)") {
		t.Fatalf("gap text (%d) = %q", len([]rune(text)), text)
	}
	validate(t, b)
}

func TestMissingMonthStartNamesTheDate(t *testing.T) {
	t.Parallel()
	acc := account()
	delete(acc.values["main"], "2026-10-30")
	var got blockapi.HouseholdInput
	m, b := build(t, config(acc, &got), "main")
	text := gapOf(t, m, "month_change")["text"].(string)
	if !strings.Contains(text, "2026-11-01") || text == "Input data is missing: no data yet" {
		t.Fatalf("month_change gap = %q", text)
	}
	validate(t, b)
}

func TestStoredPutDifferentFromLedgerPutIsGap(t *testing.T) {
	t.Parallel()
	acc := account()
	acc.stored = []store.Position{shortPut("7974", "7250")} // default ledger holds one open put (AAA)
	var got blockapi.HouseholdInput
	m, _ := build(t, config(acc, &got), "main")
	ledgerBlocks(t, m, true)
}

func TestOnlyUnmatchedStoredPutsAreCountedAndNamed(t *testing.T) {
	t.Parallel()
	acc := account()
	acc.stored = []store.Position{shortPut("7974", "7250"), shortPut("7974", "7000")}
	var got blockapi.HouseholdInput
	c := config(acc, &got)
	c.Ledgers = func() compute.OptionLedger { return ledgerOf{"7974", "7250", "20261112"} }
	m, _ := build(t, c, "main")
	text := gapOf(t, m, "obligations")["text"].(string)
	if !strings.HasPrefix(text, "1 short put(s)") || !strings.Contains(text, "P7000") || strings.Contains(text, "P7250") {
		t.Fatalf("gap text = %q", text)
	}
}

// moneyFigures answers the household figures with the lines it is given in the obligations block and
// the trend it is given.
type moneyFigures struct {
	fakeFigures
	lines []blockapi.MoneyLine
	trend []blockapi.DayValue
}

func (f moneyFigures) Household(in blockapi.HouseholdInput) blockapi.HouseholdResult {
	r := f.fakeFigures.Household(in)
	if f.lines != nil {
		r.Obligations = f.lines
	}
	if f.trend != nil {
		r.Trend = f.trend
	}
	return r
}

func figuresConfig(f moneyFigures) household_page.Config {
	var got blockapi.HouseholdInput
	f.got = &got
	c := config(account(), &got)
	c.Figures = func() blockapi.PageFigures { return f }
	return c
}

func TestMoneyTextGroupsThousandsAndUsesMinusSign(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ value, cur, text, unit string }{
		{"57849.95", "USD", "$57,849.95", "usd"},
		{"-1684.99", "USD", "\u2212$1,684.99", "usd_signed"},
		{"999.50", "USD", "$999.50", "usd"},
		{"1000000.00", "USD", "$1,000,000.00", "usd"},
		{"-0.50", "USD", "\u2212$0.50", "usd_signed"},
		{"5550", "JPY", "¥5,550", "usd"},
		{"-12345", "JPY", "\u2212¥12,345", "usd_signed"},
	} {
		line := blockapi.MoneyLine{Name: "X", Currency: tc.cur, Amount: compute.FigureValue{Value: tc.value}}
		m, _ := build(t, figuresConfig(moneyFigures{lines: []blockapi.MoneyLine{line}}), "main")
		rows := m["obligations"].(map[string]any)["rows"].([]any)
		amt := rows[0].(map[string]any)["amount"].(map[string]any)
		if amt["text"] != tc.text || amt["unit"] != tc.unit || amt["value"].(float64) != mustFloat(t, tc.value) {
			t.Fatalf("%s %s: %v", tc.value, tc.cur, amt)
		}
	}
}

func mustFloat(t *testing.T, s string) float64 {
	t.Helper()
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestTrendRowsUseGroupedMoney(t *testing.T) {
	t.Parallel()
	trend := []blockapi.DayValue{
		{Day: "2026-11-09", NetLiquidation: "100000.00", Currency: "USD"},
		{Day: "2026-11-10", NetLiquidation: "99000.50", Currency: "USD"},
	}
	m, _ := build(t, figuresConfig(moneyFigures{trend: trend}), "main")
	rows := m["trend"].(map[string]any)["rows"].([]any)
	want := []struct {
		text string
		v    float64
	}{{"$100,000.00", 100000.00}, {"$99,000.50", 99000.50}}
	for i, w := range want {
		v := rows[i].(map[string]any)["value"].(map[string]any)
		if v["text"] != w.text || v["value"].(float64) != w.v {
			t.Fatalf("row %d: %v", i, v)
		}
	}
}
