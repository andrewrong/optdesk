package option_boards

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// nameFake answers the names it holds and records each call; an id it does not hold is absent.
type nameFake struct {
	mu    sync.Mutex
	names map[int64]string
	err   error
	calls [][]int64
}

func (f *nameFake) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, slices.Clone(ids))
	if f.err != nil {
		return nil, f.err
	}
	var out []store.InstrumentName
	for _, id := range ids {
		if n, ok := f.names[id]; ok {
			out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: n})
		}
	}
	return out, nil
}

// tickerNames names the underlyings of the fixtures: 1 and 7 are AAPL, 2 and 8 are MSFT.
func tickerNames() *nameFake {
	return &nameFake{names: map[int64]string{1: "AAPL", 2: "MSFT", 7: "AAPL", 8: "MSFT"}}
}

func optionPageOf(t *testing.T, names store.InstrumentNameReader, rows []store.Candidate) map[string]any {
	t.Helper()
	c := OptionConfig{
		RunID: "r", BuildID: "b", Scope: "options", Zone: testZone, Names: names,
		Candidates: &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: rows}}, Eod: fixtureEod(),
	}
	b, err := BuildOption(context.Background(), c, nil, buildDay, buildAt)
	if err != nil {
		t.Fatal(err)
	}
	return decodeMap(t, b)
}

func symbolsOf(t *testing.T, m map[string]any) (cards, table []string, chainTitle string) {
	t.Helper()
	for _, c := range cardsOf(m) {
		cards = append(cards, c.(map[string]any)["symbol"].(string))
	}
	for _, r := range tableRowsOf(m) {
		table = append(table, cellOf(t, r, "symbol")["text"].(string))
	}
	if ch, ok := pane(m)["chain"].(map[string]any); ok {
		chainTitle = ch["title"].(string) + "|" + ch["summary"].(string)
	}
	return cards, table, chainTitle
}

func TestNamesTicker(t *testing.T) {
	t.Parallel()
	f := tickerNames()
	cards, table, chain := symbolsOf(t, optionPageOf(t, f, fixtureRows()))
	if strings.Join(cards, ",") != "AAPL,MSFT" {
		t.Errorf("cards = %v", cards)
	}
	if strings.Join(table, ",") != "AAPL,AAPL,MSFT,MSFT,MSFT" {
		t.Errorf("table = %v", table)
	}
	if !strings.Contains(chain, "Option chain · AAPL") || !strings.Contains(chain, "Selected: AAPL ") {
		t.Errorf("chain = %s", chain)
	}
	if len(f.calls) != 1 || !slices.Equal(f.calls[0], []int64{1, 2}) {
		t.Errorf("calls = %v, want one call for the distinct ids 1 and 2", f.calls)
	}
}

func TestNamesFallbackIssuer(t *testing.T) {
	t.Parallel()
	cases := map[string]store.InstrumentNameReader{
		"nil reader": nil,
		"missing id": &nameFake{names: map[int64]string{9: "ZZZ"}},
		"empty name": &nameFake{names: map[int64]string{1: "", 2: ""}},
		"no names":   &nameFake{},
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cards, table, chain := symbolsOf(t, optionPageOf(t, r, fixtureRows()))
			if strings.Join(cards, ",") != issuerApple+","+issuerMsft {
				t.Errorf("cards = %v", cards)
			}
			if table[0] != issuerApple || table[4] != issuerMsft || !strings.Contains(chain, issuerApple) {
				t.Errorf("table = %v chain = %s", table, chain)
			}
		})
	}
}

func TestNamesPartialFallback(t *testing.T) {
	t.Parallel()
	cards, _, _ := symbolsOf(t, optionPageOf(t, &nameFake{names: map[int64]string{1: "AAPL"}}, fixtureRows()))
	if strings.Join(cards, ",") != "AAPL,"+issuerMsft {
		t.Errorf("cards = %v", cards)
	}
}

func TestNamesReaderError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	c := OptionConfig{
		Zone: testZone, Names: &nameFake{err: boom}, Eod: fixtureEod(),
		Candidates: &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: fixtureRows()}},
	}
	if _, err := BuildOption(context.Background(), c, nil, buildDay, buildAt); !errors.Is(err, boom) {
		t.Errorf("option page err = %v", err)
	}
	sc := stockCfg(stockDay, stockFixture(), "AAPL")
	sc.Names = &nameFake{err: boom}
	if _, err := BuildStock(context.Background(), sc, nil, stockDay, buildAt); !errors.Is(err, boom) {
		t.Errorf("stock page err = %v", err)
	}
}

func TestChainDayNoteShown(t *testing.T) {
	t.Parallel()
	m := optionPageOf(t, tickerNames(), fixtureRows())
	notes := m["notes"].([]any)
	if len(notes) != 2 || notes[1] != "Volume and open interest from 2026-11-09 (chain day)" {
		t.Errorf("notes = %v", notes)
	}
	p, _ := buildStockPayload(t, stockCfg(stockDay, stockFixture(), "AAPL"), stockDay)
	if p.Notice == nil || !strings.HasSuffix(*p.Notice, ". Volume and open interest from 2026-11-09 (chain day)") {
		t.Errorf("notice = %v", p.Notice)
	}
}

func TestStockKeyTicker(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"AAPL", "aapl"} {
		p, _ := buildStockPayload(t, stockCfg(stockDay, stockFixture(), key), stockDay)
		if p.Top.Symbol != "AAPL" || len(p.Matrix.Cells) != 3 {
			t.Errorf("key %q: symbol %q, cells %d", key, p.Top.Symbol, len(p.Matrix.Cells))
		}
		if p.Top.Name == nil || *p.Top.Name != "AAPL" {
			t.Errorf("key %q: top.name = %v", key, p.Top.Name)
		}
	}
}

func TestStockKeyIssuerStillMatches(t *testing.T) {
	t.Parallel()
	p, _ := buildStockPayload(t, stockCfg(stockDay, stockFixture(), issuerApple), stockDay)
	if len(p.Matrix.Cells) != 3 || p.Top.Symbol != "AAPL" {
		t.Errorf("symbol %q, cells %d", p.Top.Symbol, len(p.Matrix.Cells))
	}
	// no names reader: the issuer text is the key and the display name
	c := stockCfg(stockDay, stockFixture(), issuerApple)
	c.Names = nil
	p, _ = buildStockPayload(t, c, stockDay)
	if len(p.Matrix.Cells) != 3 || p.Top.Symbol != issuerApple || p.Top.Name != nil {
		t.Errorf("symbol %q, cells %d, name %v", p.Top.Symbol, len(p.Matrix.Cells), p.Top.Name)
	}
}

func TestStockBareKeyIsTicker(t *testing.T) {
	t.Parallel()
	f := tickerNames()
	c := stockCfg(stockDay, stockFixture(), "")
	c.Names = f
	p, _ := buildStockPayload(t, c, stockDay)
	if p.Top.Symbol != "AAPL" || len(p.Matrix.Cells) != 3 {
		t.Errorf("symbol %q, cells %d", p.Top.Symbol, len(p.Matrix.Cells))
	}
	if len(f.calls) != 1 {
		t.Errorf("names read %d times, want once", len(f.calls))
	}
}
