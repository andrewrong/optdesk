package option_data_page

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeFx struct {
	rate  string
	found bool
	err   error
	mu    sync.Mutex
	calls []string
}

func (f *fakeFx) FxRates(context.Context, string, contracts.Day, contracts.Day) ([]contracts.FxRate, error) {
	return nil, errors.New("unused")
}

func (f *fakeFx) LatestFxRate(_ context.Context, pair string, asOf contracts.Day) (contracts.FxRate, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, pair+"@"+string(asOf))
	return contracts.FxRate{Pair: pair, Rate: f.rate}, f.found, f.err
}

// usdCfg is a one-contract chain with price 3200 in ccy and strike 3200.
func usdCfg(t *testing.T, ccy string, bid *float64, fx *fakeFx) Config {
	t.Helper()
	r := eodRow(t, 1, f64(10), "2026-11-06T10:00:00Z")
	r.UnderlyingPrice, r.UnderlyingCcy = f64(3200), ccy
	r.Bid, r.Ask, r.Last = bid, f64(3200), f64(3200)
	c := cfg(&fakeEod{rows: []contracts.OptionEod{r}}, &fakeQuotes{})
	c.Terms = &fakeTerms{rows: []store.OptionTerms{{InstrumentID: 1, Right: "C", Strike: "3200", ExpirationDate: "2026-12-18"}}}
	if fx != nil {
		c.Fx = fx
	}
	return c
}

func buildUSD(t *testing.T, c Config) ([]byte, map[string]any) {
	t.Helper()
	b, err := Build(t.Context(), c, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateBytes(t, b); err != nil {
		t.Fatal(err)
	}
	return b, buildMap(t, c)
}

func approx(cell any) (string, bool) {
	s, ok := cell.(map[string]any)["approx_usd_text"].(string)
	return s, ok
}

func TestUSDApproxJPY(t *testing.T) {
	t.Parallel()
	fx := &fakeFx{rate: "150.0", found: true}
	_, m := buildUSD(t, usdCfg(t, "JPY", f64(3200), fx))
	cells := map[string]any{"price": m["underlying"].(map[string]any)["price"]}
	row := rowsOf(m)[0].(map[string]any)
	for _, k := range []string{"strike", "bid", "ask", "last"} {
		cells[k] = row[k]
	}
	for k, c := range cells {
		if got, _ := approx(c); got != "≈ US$ 21.33" {
			t.Errorf("%s approx_usd_text = %q", k, got)
		}
	}
	if len(fx.calls) == 0 || fx.calls[0] != "USDJPY@2026-11-06" {
		t.Errorf("calls = %v", fx.calls)
	}
	if _, ok := approx(row["volume"]); ok {
		t.Error("volume must not carry approx_usd_text")
	}
}

func TestUSDApproxSkipsGapCells(t *testing.T) {
	t.Parallel()
	_, m := buildUSD(t, usdCfg(t, "JPY", nil, &fakeFx{rate: "150.0", found: true}))
	bid := rowsOf(m)[0].(map[string]any)["bid"].(map[string]any)
	if bid["kind"] != "gap" {
		t.Errorf("bid = %v", bid)
	}
	if _, ok := bid["approx_usd_text"]; ok {
		t.Error("gap cell has approx_usd_text")
	}
}

func TestUSDNoRateGap(t *testing.T) {
	t.Parallel()
	fx := &fakeFx{}
	_, m := buildUSD(t, usdCfg(t, "JPY", f64(3200), fx))
	row := rowsOf(m)[0].(map[string]any)
	for _, c := range []any{m["underlying"].(map[string]any)["price"], row["strike"], row["bid"], row["ask"], row["last"]} {
		if _, ok := approx(c); ok {
			t.Errorf("figure has approx_usd_text: %v", c)
		}
	}
	gaps := m["gaps"].([]any)
	if len(gaps) != 1 || gaps[0].(map[string]any)["text"] != "no USDJPY rate on or before 2026-11-06" {
		t.Errorf("gaps = %v", gaps)
	}
}

func TestUSDUnchangedForUSD(t *testing.T) {
	t.Parallel()
	fx := &fakeFx{rate: "150.0", found: true}
	with, _ := buildUSD(t, usdCfg(t, "USD", f64(3200), fx))
	without, _ := buildUSD(t, usdCfg(t, "USD", f64(3200), nil))
	if string(with) != string(without) {
		t.Errorf("bytes differ:\n%s\n%s", with, without)
	}
	if len(fx.calls) != 0 {
		t.Errorf("reader called: %v", fx.calls)
	}
}

func TestUSDReaderError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	_, err := Build(t.Context(), usdCfg(t, "JPY", f64(3200), &fakeFx{err: boom}), "2026-11-06", fixedAt)
	if !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
}
