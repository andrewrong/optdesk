package home_options_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/home_options"
	"github.com/andrewrong/optdesk/internal/store"
)

const day = "2026-11-05"

var zone = time.FixedZone("CST", 8*3600)
var generated = time.Date(2026, 11, 6, 9, 30, 0, 0, zone)

// ledger is a fake reader: one alias per key, a row per transaction (the attributes are not read by the fake replay).
type ledger struct {
	aliases map[string]int
	err     error
}

func (l ledger) FlexTradeRows(_ context.Context, alias string) ([]store.FlexTradeRow, error) {
	if l.err != nil {
		return nil, l.err
	}
	rows := []store.FlexTradeRow{}
	for i := 0; i < l.aliases[alias]; i++ {
		rows = append(rows, store.FlexTradeRow{Alias: alias, TransactionID: string(rune('a' + i)), Revision: 1, Attributes: "{}"})
	}
	return rows, nil
}

func (ledger) LatestOkFetch(context.Context, string, string) (store.FlexFetchRecord, bool, error) {
	return store.FlexFetchRecord{}, false, nil
}

func (l ledger) StoredAliases(context.Context) ([]string, error) {
	if l.err != nil {
		return nil, l.err
	}
	out := []string{}
	for a := range l.aliases {
		out = append(out, a)
	}
	return out, nil
}

// replayer returns fixed positions whatever the trades are.
type replayer struct{ positions []compute.LedgerPosition }

func (r replayer) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	return compute.LedgerResult{RuleVersion: 1, Positions: r.positions}
}

func short(open contracts.Day, qty, price, status string, closedOn contracts.Day, realized string) compute.LedgerPosition {
	p := compute.LedgerPosition{
		Currency: "USD", Multiplier: "100", Status: status, OpenedOn: open, ClosedOn: closedOn, RealizedPnL: realized,
		Fills: []compute.LedgerFill{{Day: open, Action: "open", Quantity: qty, Price: price}},
	}
	if closedOn != "" {
		p.Fills = append(p.Fills, compute.LedgerFill{Day: closedOn, Action: "close", Quantity: "1", Price: "0.5"})
	}
	return p
}

func positions() []compute.LedgerPosition {
	return []compute.LedgerPosition{
		short("2026-08-12", "-2", "1.50", "closed", "2026-09-03", "120.5"),
		short("2026-10-02", "-1", "2.25", "open", "", ""),
		short("2026-11-03", "-3", "0.80", "open", "", ""),
		short("2026-11-04", "1", "0.10", "open", "", ""),
	}
}

func cfg(l store.OptionLedgerReader, r compute.OptionLedger) home_options.Config {
	return home_options.Config{RunID: "run-test", BuildID: "build-test", Scope: "scope-test", Zone: zone, Ledger: l, Replay: r}
}

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", filepath.Base(url)))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func build(t *testing.T, c home_options.Config) []byte {
	t.Helper()
	b, err := home_options.Build(context.Background(), c, day, generated)
	if err != nil {
		t.Fatal(err)
	}
	sc := jsonschema.NewCompiler()
	sc.AssertFormat()
	sc.UseLoader(fileLoader{})
	s, err := sc.Compile("https://optdesk.invalid/schemas/page.options_summary.v4.json")
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
	return b
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func withRows() home_options.Config {
	return cfg(ledger{aliases: map[string]int{"main": 2}}, replayer{positions()})
}

func TestGoldenOK(t *testing.T) {
	t.Parallel()
	got := build(t, withRows())
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, got, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	want, err := os.ReadFile(filepath.Join("testdata", "ok.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pretty.Bytes(), want) {
		t.Fatalf("golden differs:\n%s", pretty.Bytes())
	}
}

func TestMonthWithNoLedgerRowIsHoleNotZero(t *testing.T) {
	t.Parallel()
	card := decode(t, build(t, withRows()))["card"].(map[string]any)
	series := card["premium_series_6m"].([]any)
	if len(series) != 6 {
		t.Fatalf("series has %d points", len(series))
	}
	byMonth := map[string]map[string]any{}
	for _, p := range series {
		pt := p.(map[string]any)
		byMonth[pt["month"].(string)] = pt
	}
	// 2026-06 and 2026-07 have no row; 2026-08 has one
	for _, m := range []string{"2026-06", "2026-07"} {
		g := byMonth[m]["premium"].(map[string]any)
		if g["kind"] != "gap" || g["reason"] != "no_data" {
			t.Errorf("%s premium = %v", m, g)
		}
	}
	if f := byMonth["2026-08"]["premium"].(map[string]any); f["text"] != "+$300" {
		t.Errorf("2026-08 premium = %v", f)
	}
	// 2026-09 has a close row only: premium is a real zero, closed is the realized result
	if f := byMonth["2026-09"]["premium"].(map[string]any); f["text"] != "+$0" || f["tone"] != "flat" {
		t.Errorf("2026-09 premium = %v", f)
	}
	if f := byMonth["2026-09"]["closed"].(map[string]any); f["text"] != "+$120" {
		t.Errorf("2026-09 closed = %v", f)
	}
	if g := byMonth["2026-10"]["closed"].(map[string]any); g["kind"] != "gap" {
		t.Errorf("2026-10 closed = %v", g)
	}
}

func TestClosedSeriesTextIsBuiltFromLedgerRows(t *testing.T) {
	t.Parallel()
	card := decode(t, build(t, withRows()))["card"].(map[string]any)
	if got := card["closed_series_text"].(string); got != "近六个月已平仓盈亏：9月 +$120" {
		t.Errorf("closed_series_text = %q", got)
	}
	empty := cfg(ledger{aliases: map[string]int{"main": 1}}, replayer{[]compute.LedgerPosition{short("2026-11-03", "-1", "1", "open", "", "")}})
	card = decode(t, build(t, empty))["card"].(map[string]any)
	if got := card["closed_series_text"].(string); strings.Contains(got, "+$") || strings.Contains(got, "$0") {
		t.Errorf("closed_series_text invents a figure: %q", got)
	}
}

func TestTilesOfTheMonth(t *testing.T) {
	t.Parallel()
	card := decode(t, build(t, withRows()))["card"].(map[string]any)
	tiles := card["tiles"].(map[string]any)
	fig := func(k string) map[string]any { return tiles[k].(map[string]any)["figure"].(map[string]any) }
	if f := fig("sold_count_month"); f["value"] != float64(1) || f["text"] != "1 组" {
		t.Errorf("sold = %v", f)
	}
	// 3 x 0.80 x 100 sold, 1 x 0.10 x 100 bought
	if f := fig("premium_month_usd"); f["text"] != "+$230" {
		t.Errorf("premium_month = %v", f)
	}
	// open sold rows: 2.25 x 100 (October) and 240 (November)
	if f := fig("open_premium_usd"); f["text"] != "$465" {
		t.Errorf("open_premium = %v", f)
	}
	if f := fig("max_loss_usd"); f["kind"] != "gap" {
		t.Errorf("max_loss = %v", f)
	}
}

func TestNilReaderIsNoDataEverywhere(t *testing.T) {
	t.Parallel()
	m := decode(t, build(t, cfg(nil, nil)))
	card := m["card"].(map[string]any)
	for k, v := range card["tiles"].(map[string]any) {
		if f := v.(map[string]any)["figure"].(map[string]any); f["kind"] != "gap" {
			t.Errorf("tile %s = %v", k, f)
		}
	}
	for _, p := range card["premium_series_6m"].([]any) {
		if g := p.(map[string]any)["premium"].(map[string]any); g["kind"] != "gap" {
			t.Errorf("point = %v", p)
		}
	}
	if m["run_status"] != "partial" {
		t.Errorf("run_status = %v", m["run_status"])
	}
}

func TestEmptyLedgerIsNoDataEverywhere(t *testing.T) {
	t.Parallel()
	card := decode(t, build(t, cfg(ledger{aliases: map[string]int{}}, nil)))["card"].(map[string]any)
	if g := card["tiles"].(map[string]any)["premium_month_usd"].(map[string]any)["figure"].(map[string]any); g["kind"] != "gap" {
		t.Errorf("premium_month = %v", g)
	}
}

func TestOtherCurrencyIsLeftOutAndNamed(t *testing.T) {
	t.Parallel()
	p := short("2026-11-03", "-1", "1", "open", "", "")
	p.Currency = "EUR"
	card := decode(t, build(t, cfg(ledger{aliases: map[string]int{"main": 1}}, replayer{[]compute.LedgerPosition{p}})))["card"].(map[string]any)
	found := false
	for _, g := range card["gaps"].([]any) {
		if g.(map[string]any)["reason"] == "unsupported_currency" {
			found = true
		}
	}
	if !found {
		t.Errorf("gaps = %v", card["gaps"])
	}
}

func TestLedgerErrorIsError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	_, err := home_options.Build(context.Background(), cfg(ledger{err: boom}, replayer{}), day, generated)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestBadConfigIsError(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]home_options.Config{
		"nil zone":   {RunID: "r", BuildID: "b", Scope: "s"},
		"bad id":     {RunID: "r r", BuildID: "b", Scope: "s", Zone: zone},
		"nil replay": {RunID: "r", BuildID: "b", Scope: "s", Zone: zone, Ledger: ledger{aliases: map[string]int{"main": 1}}},
	} {
		if _, err := home_options.Build(context.Background(), c, day, generated); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := home_options.Build(context.Background(), cfg(nil, nil), "nope", generated); err == nil {
		t.Error("bad day: no error")
	}
}
