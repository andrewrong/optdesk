package household_page_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/household_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

type posAccount struct {
	*fakeAccount
	positions []store.Position
	asked     []string
}

func (p *posAccount) Positions(_ context.Context, alias, d string) ([]store.Position, error) {
	p.asked = append(p.asked, alias+" "+d)
	return p.positions, nil
}

// fakeMarks answers byDay[day] for a day it holds and rows for any other day.
type fakeMarks struct {
	rows  []mark.PositionMark
	byDay map[string][]mark.PositionMark
	err   error
	asks  []string
}

func (f *fakeMarks) Marks(_ context.Context, alias, d string) ([]mark.PositionMark, error) {
	f.asks = append(f.asks, alias+" "+d)
	if rows, ok := f.byDay[d]; ok {
		return rows, f.err
	}
	return f.rows, f.err
}

func stock(sym string, con int64, qty, cost string) store.Position {
	return store.Position{Symbol: sym, SecType: "stock", Currency: "USD", Multiplier: "1", ConID: con, Quantity: qty, AvgCost: cost}
}

func markOf(con int64, price string) mark.PositionMark {
	return mark.PositionMark{ConID: con, Price: price, Basis: mark.BasisLast, Currency: "USD"}
}

func holdingsConfig(positions []store.Position, marks *fakeMarks) (household_page.Config, *posAccount) {
	var got blockapi.HouseholdInput
	acc := &posAccount{fakeAccount: account(), positions: positions}
	c := config(acc.fakeAccount, &got)
	c.Account = acc
	c.Marks = marks
	c.Holdings = pagefig.NewHoldings
	return c, acc
}

func twoStocks() []store.Position {
	opt := store.Position{Symbol: "AAA", SecType: "option", Currency: "USD", Multiplier: "100", ConID: 9, Quantity: "-1"}
	return []store.Position{stock("AAA", 1, "100", "200"), stock("BBB", 2, "50", "10"), opt}
}

func TestHoldingBlocksAreBuiltFromPositionsAndMarks(t *testing.T) {
	t.Parallel()
	marks := &fakeMarks{byDay: map[string][]mark.PositionMark{
		"2026-11-10": {markOf(1, "250"), markOf(2, "100")},
		"2026-11-09": {markOf(1, "240"), markOf(2, "105")},
	}}
	for _, rows := range marks.byDay {
		for i := range rows {
			rows[i].MarkedAt = time.Date(2026, 11, 9, 23, 0, 0, 0, time.UTC)
		}
	}
	c, acc := holdingsConfig(twoStocks(), marks)
	m, b := build(t, c, "main")
	validate(t, b)
	for _, blk := range []string{"composition", "concentration", "allocation"} {
		if m[blk].(map[string]any)["gap"] != nil {
			t.Fatalf("%s has a gap: %v", blk, m[blk])
		}
	}
	if m["movers"].(map[string]any)["gap"] != nil || len(m["movers"].(map[string]any)["rows"].([]any)) != 2 {
		t.Fatalf("movers: %s", b)
	}
	if m["run_status"] != "partial" {
		t.Fatalf("run_status = %v", m["run_status"])
	}
	wantReads := []string{"main 2026-11-10", "main 2026-11-10", "main 2026-11-09"}
	if !slices.Equal(acc.asked, wantReads) || !slices.Equal(marks.asks, wantReads) {
		t.Fatalf("reads: %v %v, want %v", acc.asked, marks.asks, wantReads)
	}
	golden := filepath.Join("testdata", "holdings.golden.json")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(golden, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(want), b) {
		t.Fatalf("page differs from %s:\n%s", golden, b)
	}
}

func TestHoldingBlocksNilPathStaysMissingQuote(t *testing.T) {
	t.Parallel()
	for name, mod := range map[string]func(*household_page.Config){
		"marks":    func(c *household_page.Config) { c.Marks = nil },
		"holdings": func(c *household_page.Config) { c.Holdings = nil },
	} {
		c, acc := holdingsConfig(twoStocks(), &fakeMarks{})
		mod(&c)
		m, _ := build(t, c, "main")
		for _, blk := range []string{"composition", "concentration", "allocation"} {
			if g := gapOf(t, m, blk); g["reason"] != "missing_quote" {
				t.Fatalf("%s: %s gap = %v", name, blk, g)
			}
		}
		if len(acc.asked) != 1 {
			t.Fatalf("%s: positions read %d times, want only the build read", name, len(acc.asked))
		}
	}
}

func TestMoversNilPathStaysNoData(t *testing.T) {
	t.Parallel()
	for name, mod := range map[string]func(*household_page.Config){
		"marks":    func(c *household_page.Config) { c.Marks = nil },
		"holdings": func(c *household_page.Config) { c.Holdings = nil },
	} {
		c, _ := holdingsConfig(twoStocks(), &fakeMarks{})
		mod(&c)
		m, _ := build(t, c, "main")
		if g := gapOf(t, m, "movers"); g["reason"] != "no_data" || g["text"] != "The change today needs prices of two trading days: no data yet" {
			t.Fatalf("%s: movers gap = %v", name, g)
		}
	}
}

func TestPositionWithoutMarkIsMissingQuote(t *testing.T) {
	t.Parallel()
	none := mark.PositionMark{ConID: 2, Basis: mark.BasisNone}
	c, _ := holdingsConfig(twoStocks(), &fakeMarks{rows: []mark.PositionMark{markOf(1, "250"), none}})
	m, b := build(t, c, "main")
	validate(t, b)
	for _, blk := range []string{"composition", "concentration", "allocation"} {
		g := gapOf(t, m, blk)
		if g["reason"] != "missing_quote" {
			t.Fatalf("%s gap = %v", blk, g)
		}
	}
	rows := m["composition"].(map[string]any)["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["name"] != "AAA" {
		t.Fatalf("rows = %v", rows)
	}
	if len(m["composition"].(map[string]any)["totals"].([]any)) != 0 {
		t.Fatal("a total that left a stock out must not be shown")
	}
}

func TestHoldingReadErrorsAreErrors(t *testing.T) {
	t.Parallel()
	c, _ := holdingsConfig(twoStocks(), &fakeMarks{err: errors.New("boom")})
	if _, err := household_page.NewPage(c, "main").Build(context.Background(), nil, day, at); err == nil {
		t.Fatal("a mark read failure must be an error of Build")
	}
}

// fakeFx answers LatestFxRate with a fixed rate, nothing found, or an error; it counts the reads.
type fakeFx struct {
	bar.FxRateReader
	rate  string
	err   error
	calls []string
}

func (f *fakeFx) LatestFxRate(_ context.Context, pair string, asOf contracts.Day) (contracts.FxRate, bool, error) {
	f.calls = append(f.calls, pair+" "+string(asOf))
	if f.err != nil || f.rate == "" {
		return contracts.FxRate{}, false, f.err
	}
	return contracts.FxRate{Pair: pair, Rate: f.rate, RateDay: types.SerializableDate{Time: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)}}, true, nil
}

func jpyStock() store.Position {
	return store.Position{Symbol: "4063", SecType: "stock", Currency: "JPY", Multiplier: "1", ConID: 7, Quantity: "100", AvgCost: "3000"}
}

func jpyMark() mark.PositionMark {
	return mark.PositionMark{ConID: 7, Price: "3474.6", Basis: mark.BasisLast, Currency: "JPY"}
}

func jpyConfig(fx *fakeFx) household_page.Config {
	c, _ := holdingsConfig([]store.Position{jpyStock()}, &fakeMarks{rows: []mark.PositionMark{jpyMark()}})
	if fx != nil {
		c.Fx = fx
	}
	return c
}

func TestJpyStockWithRateIsUsdWithShare(t *testing.T) {
	t.Parallel()
	fx := &fakeFx{rate: "150"}
	m, b := build(t, jpyConfig(fx), "main")
	validate(t, b)
	if len(fx.calls) != 1 || fx.calls[0] != "USDJPY 2026-11-10" {
		t.Fatalf("fx calls %v", fx.calls)
	}
	comp := m["composition"].(map[string]any)
	row := comp["rows"].([]any)[0].(map[string]any)
	amt := row["amount"].(map[string]any)
	if amt["currency"] != "USD" || amt["value"].(float64) != 2316.4 || row["share"] == nil {
		t.Fatalf("row %v", row)
	}
	if !strings.Contains(comp["meaning"].(string), "2026-11-09") {
		t.Fatalf("rate day missing from %q", comp["meaning"])
	}
	alloc := m["allocation"].(map[string]any)
	if alloc["gap"] != nil || len(alloc["rows"].([]any)) != 1 {
		t.Fatalf("allocation %v", alloc)
	}
}

func TestJpyStockWithoutRateStaysYenWithGap(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]household_page.Config{
		"not found": jpyConfig(&fakeFx{}),
		"nil fx":    jpyConfig(nil),
		"error":     jpyConfig(&fakeFx{err: errors.New("down")}),
	} {
		m, b := build(t, c, "main")
		validate(t, b)
		comp := m["composition"].(map[string]any)
		row := comp["rows"].([]any)[0].(map[string]any)
		if row["amount"].(map[string]any)["currency"] != "JPY" || row["share"] != nil {
			t.Fatalf("%s: row %v", name, row)
		}
		g := gapOf(t, m, "composition")
		want := "4063: no USDJPY rate on or before 2026-11-10"
		if g["reason"] != "unsupported_currency" || g["text"] != want {
			t.Fatalf("%s: gap %v", name, g)
		}
		if g := gapOf(t, m, "allocation"); g["reason"] != "unsupported_currency" {
			t.Fatalf("%s: allocation gap %v", name, g)
		}
	}
}

func TestFxIsReadOnlyWhenCaptureHoldsJpy(t *testing.T) {
	t.Parallel()
	fx := &fakeFx{rate: "150"}
	c, _ := holdingsConfig(twoStocks(), &fakeMarks{rows: []mark.PositionMark{markOf(1, "250"), markOf(2, "100")}})
	c.Fx = fx
	build(t, c, "main")
	if len(fx.calls) != 0 {
		t.Fatalf("fx read for a USD capture: %v", fx.calls)
	}
}

func accountCells(m map[string]any) (stock, options map[string]any) {
	row := m["accounts"].(map[string]any)["rows"].([]any)[0].(map[string]any)
	return row["stock"].(map[string]any), row["options"].(map[string]any)
}

func shortOption(sym string, con int64) store.Position {
	return store.Position{Symbol: sym, SecType: "option", Currency: "USD", Multiplier: "100", ConID: con, Quantity: "-1",
		Right: "put", Strike: "150", Expiry: "2026-11-20"}
}

func realFigures(c household_page.Config) household_page.Config {
	c.Figures = pagefig.New
	return c
}

func TestStockAndOptionValueAreNumbersWithMarks(t *testing.T) {
	t.Parallel()
	c, _ := holdingsConfig(append(twoStocks()[:2:2], shortOption("AAA", 9)),
		&fakeMarks{rows: []mark.PositionMark{markOf(1, "250"), markOf(2, "100"), markOf(9, "2.50")}})
	m, b := build(t, realFigures(c), "main")
	validate(t, b)
	stock, options := accountCells(m)
	if stock["gap"] != nil || options["gap"] != nil {
		t.Fatalf("cells %v %v", stock, options)
	}
	if v := stock["cell"].(map[string]any)["value"].(float64); v != 30000 {
		t.Fatalf("stock %v", stock)
	}
	if v := options["cell"].(map[string]any)["value"].(float64); v != -250 {
		t.Fatalf("options %v", options)
	}
}

func TestUnmarkedOptionIsNamedInTheCellGap(t *testing.T) {
	t.Parallel()
	c, _ := holdingsConfig([]store.Position{shortOption("TSLA", 9)}, &fakeMarks{})
	m, b := build(t, realFigures(c), "main")
	validate(t, b)
	_, options := accountCells(m)
	if g, _ := options["gap"].(string); !strings.Contains(g, "TSLA") || !strings.Contains(g, "no mark") {
		t.Fatalf("options %v", options)
	}
}

func TestManyUnmarkedOptionsFitTheGapText(t *testing.T) {
	t.Parallel()
	var ps []store.Position
	for i := 0; i < 20; i++ {
		ps = append(ps, shortOption(fmt.Sprintf("LONGSYMBOL%02d", i), int64(100+i)))
	}
	c, _ := holdingsConfig(ps, &fakeMarks{})
	m, b := build(t, realFigures(c), "main")
	validate(t, b)
	_, options := accountCells(m)
	g, _ := options["gap"].(string)
	if len([]rune(g)) > 200 || !strings.Contains(g, "LONGSYMBOL00") || strings.Contains(g, "LONGSYMBOL19") {
		t.Fatalf("gap %d runes: %q", len([]rune(g)), g)
	}
}

func TestJpyPositionsUseRateInAccountCells(t *testing.T) {
	t.Parallel()
	m, _ := build(t, realFigures(jpyConfig(&fakeFx{rate: "150"})), "main")
	stock, _ := accountCells(m)
	if c := stock["cell"].(map[string]any); c["currency"] != "USD" || c["value"].(float64) != 2316.4 {
		t.Fatalf("stock %v", stock)
	}
	m, _ = build(t, realFigures(jpyConfig(nil)), "main")
	stock, _ = accountCells(m)
	if g, _ := stock["gap"].(string); !strings.Contains(g, "no USDJPY rate") {
		t.Fatalf("stock %v", stock)
	}
}
