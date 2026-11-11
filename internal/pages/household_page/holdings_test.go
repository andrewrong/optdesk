package household_page_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/pages/household_page"
	"github.com/andrewrong/optdesk/internal/store"
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

type fakeMarks struct {
	rows []mark.PositionMark
	err  error
	asks []string
}

func (f *fakeMarks) Marks(_ context.Context, alias, d string) ([]mark.PositionMark, error) {
	f.asks = append(f.asks, alias+" "+d)
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
	marks := &fakeMarks{rows: []mark.PositionMark{markOf(1, "250"), markOf(2, "100")}}
	c, acc := holdingsConfig(twoStocks(), marks)
	m, b := build(t, c, "main")
	validate(t, b)
	for _, blk := range []string{"composition", "concentration", "allocation"} {
		if m[blk].(map[string]any)["gap"] != nil {
			t.Fatalf("%s has a gap: %v", blk, m[blk])
		}
	}
	if m["long_term"].(map[string]any)["gap"].(map[string]any)["reason"] != "not_built" ||
		m["movers"].(map[string]any)["gap"].(map[string]any)["reason"] != "no_data" {
		t.Fatalf("long_term or movers: %s", b)
	}
	if m["run_status"] != "partial" {
		t.Fatalf("run_status = %v", m["run_status"])
	}
	const read = "main 2026-11-10"
	if len(acc.asked) != 2 || acc.asked[0] != read || acc.asked[1] != read || len(marks.asks) != 1 || marks.asks[0] != read {
		t.Fatalf("reads: %v %v", acc.asked, marks.asks)
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
