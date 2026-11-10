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

// pos is a position the fake replay returns; the fake figures read none of its fields.
func pos() compute.LedgerPosition { return compute.LedgerPosition{Currency: "USD", Status: "open"} }

func val(v string) compute.FigureValue  { return compute.FigureValue{Value: v} }
func gapv(r string) compute.FigureValue { return compute.FigureValue{Gap: r} }

// figs is a fake Figures: it returns out whatever the input is and records the input.
type figs struct {
	out  home_options.FigureOut
	seen *home_options.FigureIn
}

func (f figs) Options(in home_options.FigureIn) home_options.FigureOut {
	if f.seen != nil {
		*f.seen = in
	}
	return f.out
}

func month(m string, seen bool, sold string, premium, closed compute.FigureValue) home_options.MonthFigure {
	return home_options.MonthFigure{Month: m, Seen: seen, Sold: val(sold), Premium: premium, Closed: closed}
}

// okOut is six months: 2026-06 and 2026-07 are holes, 2026-09 has a close only.
func okOut() home_options.FigureOut {
	return home_options.FigureOut{
		Newest:      "2026-11-04",
		OpenPremium: val("1465.50"),
		Months: []home_options.MonthFigure{
			month("2026-06", false, "0", gapv("no_data"), gapv("no_data")),
			month("2026-07", false, "0", gapv("no_data"), gapv("no_data")),
			month("2026-08", true, "2", val("300.00"), gapv("no_data")),
			month("2026-09", true, "0", val("0.00"), val("120.50")),
			month("2026-10", true, "1", val("225.00"), gapv("no_data")),
			month("2026-11", true, "1", val("-1234.50"), val("-15.25")),
		},
	}
}

func cfgOut(out home_options.FigureOut) home_options.Config {
	return cfg(ledger{aliases: map[string]int{"main": 2}}, replayer{[]compute.LedgerPosition{pos()}}, figs{out: out})
}

func cfg(l store.OptionLedgerReader, r compute.OptionLedger, f home_options.Figures) home_options.Config {
	return home_options.Config{RunID: "run-test", BuildID: "build-test", Scope: "scope-test", Zone: zone, Ledger: l, Replay: r, Figures: f}
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

func card(t *testing.T, c home_options.Config) map[string]any {
	t.Helper()
	return decode(t, build(t, c))["card"].(map[string]any)
}

func tileFigure(c map[string]any, k string) map[string]any {
	return c["tiles"].(map[string]any)[k].(map[string]any)["figure"].(map[string]any)
}

func point(c map[string]any, month string) map[string]any {
	for _, p := range c["premium_series_6m"].([]any) {
		if p.(map[string]any)["month"] == month {
			return p.(map[string]any)
		}
	}
	return nil
}

func TestGoldenOK(t *testing.T) {
	t.Parallel()
	got := build(t, cfgOut(okOut()))
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

// The builder hands the replayed positions, the day and the six month keys to the figures.
func TestFiguresGetPositionsDayAndSixMonths(t *testing.T) {
	t.Parallel()
	var in home_options.FigureIn
	c := cfg(ledger{aliases: map[string]int{"main": 1}}, replayer{[]compute.LedgerPosition{pos()}}, figs{out: okOut(), seen: &in})
	build(t, c)
	want := []string{"2026-06", "2026-07", "2026-08", "2026-09", "2026-10", "2026-11"}
	if in.AsOf != day || len(in.Positions) != 1 || strings.Join(in.Months, ",") != strings.Join(want, ",") {
		t.Fatalf("input = %+v", in)
	}
}

// Premium and closed text of a month are the received decimal text, rearranged and nothing else.
func TestFiguresAreCopiedNotComputed(t *testing.T) {
	t.Parallel()
	k := card(t, cfgOut(okOut()))
	cases := map[string][2]string{ // month: premium text, closed text
		"2026-08": {"+$300.00", ""},
		"2026-09": {"+$0.00", "+$120.50"},
		"2026-11": {"−$1,234.50", "−$15.25"},
	}
	for m, w := range cases {
		pt := point(k, m)
		if got := pt["premium"].(map[string]any)["text"]; got != w[0] {
			t.Errorf("%s premium text = %v, want %s", m, got, w[0])
		}
		if w[1] != "" {
			if got := pt["closed"].(map[string]any)["text"]; got != w[1] {
				t.Errorf("%s closed text = %v, want %s", m, got, w[1])
			}
		}
	}
	if f := point(k, "2026-11")["premium"].(map[string]any); f["value"] != -1234.5 || f["tone"] != "down" {
		t.Errorf("2026-11 premium = %v", f)
	}
	if f := point(k, "2026-09")["premium"].(map[string]any); f["tone"] != "flat" {
		t.Errorf("a zero premium has tone %v", f["tone"])
	}
}

func TestMonthNotSeenIsHoleNotZero(t *testing.T) {
	t.Parallel()
	k := card(t, cfgOut(okOut()))
	if len(k["premium_series_6m"].([]any)) != 6 {
		t.Fatal("the series has not six points")
	}
	for _, m := range []string{"2026-06", "2026-07"} {
		pt := point(k, m)
		for _, f := range []string{"premium", "closed"} {
			if g := pt[f].(map[string]any); g["kind"] != "gap" || g["reason"] != "no_data" {
				t.Errorf("%s %s = %v", m, f, g)
			}
		}
	}
	if g := point(k, "2026-10")["closed"].(map[string]any); g["kind"] != "gap" {
		t.Errorf("a month with a row but no close: closed = %v", g)
	}
	if !strings.Contains(k["premium_series_text"].(string), "2 个月无账本记录") {
		t.Errorf("premium_series_text = %q", k["premium_series_text"])
	}
}

func TestClosedSeriesTextIsBuiltFromLedgerRows(t *testing.T) {
	t.Parallel()
	k := card(t, cfgOut(okOut()))
	if got := k["closed_series_text"].(string); got != "近六个月已平仓盈亏：9月 +$120.50，11月 −$15.25" {
		t.Errorf("closed_series_text = %q", got)
	}
	none := okOut()
	for i := range none.Months {
		none.Months[i].Closed = gapv("no_data")
	}
	got := card(t, cfgOut(none))["closed_series_text"].(string)
	if strings.Contains(got, "$") {
		t.Errorf("closed_series_text invents a figure: %q", got)
	}
}

func TestTilesOfTheMonth(t *testing.T) {
	t.Parallel()
	k := card(t, cfgOut(okOut()))
	if f := tileFigure(k, "sold_count_month"); f["value"] != float64(1) || f["text"] != "1 组" {
		t.Errorf("sold = %v", f)
	}
	if f := tileFigure(k, "premium_month_usd"); f["text"] != "−$1,234.50" {
		t.Errorf("premium_month = %v", f)
	}
	if f := tileFigure(k, "open_premium_usd"); f["text"] != "$1,465.50" || f["unit"] != "usd" {
		t.Errorf("open_premium = %v", f)
	}
	if f := tileFigure(k, "max_loss_usd"); f["kind"] != "gap" {
		t.Errorf("max_loss = %v", f)
	}
	if k["say"] != "本月卖出 1 组，收取权利金 −$1,234.50。" {
		t.Errorf("say = %v", k["say"])
	}
}

// Decision 3: a figure the computation gives as a gap is a gap on the page, never a zero.
func TestEachGapOfTheFiguresIsAGapOnThePage(t *testing.T) {
	t.Parallel()
	out := okOut()
	cur := &out.Months[5]
	cur.Sold, cur.Premium, cur.Closed = gapv("missing_input"), gapv("missing_quote"), gapv("no_data")
	out.OpenPremium = gapv("stale_input")
	k := card(t, cfgOut(out))
	if g := tileFigure(k, "sold_count_month"); g["kind"] != "gap" || g["reason"] != "no_data" {
		t.Errorf("sold = %v", g)
	}
	if g := tileFigure(k, "premium_month_usd"); g["kind"] != "gap" || g["reason"] != "missing_quote" {
		t.Errorf("premium_month = %v", g)
	}
	if g := tileFigure(k, "open_premium_usd"); g["kind"] != "gap" || g["reason"] != "no_data" {
		t.Errorf("open_premium = %v (a reason the schema lacks is no_data)", g)
	}
	if g := point(k, "2026-11")["premium"].(map[string]any); g["kind"] != "gap" {
		t.Errorf("series premium = %v", g)
	}
	if g := point(k, "2026-11")["closed"].(map[string]any); g["kind"] != "gap" {
		t.Errorf("series closed = %v", g)
	}
}

// A value that is not exact two-decimal text is a gap: the builder does not round or repair it.
func TestMalformedValueIsGapNotZero(t *testing.T) {
	t.Parallel()
	out := okOut()
	out.Months[5].Premium = val("12.5")
	out.Months[4].Premium = val("abc")
	out.OpenPremium = val("-3.00") // an open premium is never negative
	k := card(t, cfgOut(out))
	for _, g := range []map[string]any{tileFigure(k, "premium_month_usd"), point(k, "2026-10")["premium"].(map[string]any), tileFigure(k, "open_premium_usd")} {
		if g["kind"] != "gap" {
			t.Errorf("figure = %v, want a gap", g)
		}
	}
}

func TestNilReaderIsNoDataEverywhere(t *testing.T) {
	t.Parallel()
	m := decode(t, build(t, cfg(nil, nil, nil)))
	k := m["card"].(map[string]any)
	for name := range k["tiles"].(map[string]any) {
		if f := tileFigure(k, name); f["kind"] != "gap" {
			t.Errorf("tile %s = %v", name, f)
		}
	}
	for _, p := range k["premium_series_6m"].([]any) {
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
	k := card(t, cfg(ledger{aliases: map[string]int{}}, nil, nil))
	if g := tileFigure(k, "premium_month_usd"); g["kind"] != "gap" {
		t.Errorf("premium_month = %v", g)
	}
}

func TestOtherCurrencyIsNamed(t *testing.T) {
	t.Parallel()
	out := okOut()
	out.OtherCurrency = true
	found := false
	for _, g := range card(t, cfgOut(out))["gaps"].([]any) {
		if g.(map[string]any)["reason"] == "unsupported_currency" {
			found = true
		}
	}
	if !found {
		t.Error("no unsupported_currency gap")
	}
}

func TestLedgerErrorIsError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	_, err := home_options.Build(context.Background(), cfg(ledger{err: boom}, replayer{}, figs{}), day, generated)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestBadConfigIsError(t *testing.T) {
	t.Parallel()
	one := ledger{aliases: map[string]int{"main": 1}}
	for name, c := range map[string]home_options.Config{
		"nil zone":    {RunID: "r", BuildID: "b", Scope: "s"},
		"bad id":      {RunID: "r r", BuildID: "b", Scope: "s", Zone: zone},
		"nil replay":  {RunID: "r", BuildID: "b", Scope: "s", Zone: zone, Ledger: one},
		"nil figures": {RunID: "r", BuildID: "b", Scope: "s", Zone: zone, Ledger: one, Replay: replayer{[]compute.LedgerPosition{pos()}}},
	} {
		if _, err := home_options.Build(context.Background(), c, day, generated); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := home_options.Build(context.Background(), cfg(nil, nil, nil), "nope", generated); err == nil {
		t.Error("bad day: no error")
	}
}
