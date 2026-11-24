package home_household_test

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

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/home_household"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

const (
	dayMixed   = "各账户数据日不同，不显示单日变化"
	cashLine   = "现金分放在各账户，只担保本账户的卖出看跌；合计现金不是可共用的保证金"
	wifeNoData = "账户（wife）今天未采集到，合计不含"
)

// byAlias is the account reader of a book: the rows of each alias by day.
type byAlias struct {
	store.AccountReader
	rows map[string]map[string][]store.AccountValue
	errs map[string]error
}

func (b byAlias) Values(_ context.Context, alias, d string) ([]store.AccountValue, error) {
	return b.rows[alias][d], b.errs[alias]
}

// fakeFamily answers a canned result and keeps the input it was given.
type fakeFamily struct {
	res   blockapi.FamilyResult
	got   *blockapi.FamilyIn
	calls *int
}

func (f fakeFamily) Family(in blockapi.FamilyIn) blockapi.FamilyResult {
	*f.got = in
	*f.calls++
	return f.res
}

func fv(v string) compute.FigureValue   { return compute.FigureValue{Value: v} }
func gapv(r string) compute.FigureValue { return compute.FigureValue{Gap: r} }

// twoOfThree is the result for main and wife counted, daughter left out.
func twoOfThree() blockapi.FamilyResult {
	return blockapi.FamilyResult{
		Currency: "USD", Included: []string{"main", "wife"},
		Excluded: []blockapi.FamilyAccountOut{{Alias: "daughter", Reason: "no_data"}},
		Rows: []blockapi.FamilyAccountOut{
			{Alias: "main", Day: "2026-11-10"}, {Alias: "wife", Day: "2026-11-10"}, {Alias: "daughter", Reason: "no_data"},
		},
		Net: fv("250000.00"), Cash: fv("100000.00"), CashShare: fv("40.00"),
		DayAmount: fv("1500.00"), DayPct: fv("0.60"), MonthAmount: fv("5000.00"), MonthPct: fv("2.04"),
		OldestDay: "2026-11-10",
	}
}

func book(res blockapi.FamilyResult, rows map[string]map[string][]store.AccountValue) (home_household.Config, *blockapi.FamilyIn, *int) {
	got, calls := &blockapi.FamilyIn{}, new(int)
	c := config(nil)
	c.Account = byAlias{rows: rows}
	c.Aliases = []string{"main", "wife", "daughter"}
	c.Family = func() blockapi.FamilyFigure { return fakeFamily{res: res, got: got, calls: calls} }
	return c, got, calls
}

func twoCaptured() map[string]map[string][]store.AccountValue {
	return map[string]map[string][]store.AccountValue{
		"main": full(),
		"wife": {"2026-10-30": vals("20000.00", "8000.00"), "2026-11-09": vals("24000.00", "9000.00"), "2026-11-10": vals("150000.00", "58000.00")},
	}
}

func notesOf(m map[string]any) []string {
	var out []string
	for _, n := range m["card"].(map[string]any)["notes"].([]any) {
		out = append(out, n.(string))
	}
	return out
}

func subtitleOf(m map[string]any) string { return m["card"].(map[string]any)["subtitle"].(string) }

func hasNote(m map[string]any, want string) bool {
	for _, n := range notesOf(m) {
		if n == want {
			return true
		}
	}
	return false
}

func figureOf(m map[string]any, tile string) map[string]any {
	return tilesOf(m)[tile]["figure"].(map[string]any)
}

func TestFamilyCardSumsNetAndCash(t *testing.T) {
	t.Parallel()
	c, got, _ := book(twoOfThree(), twoCaptured())
	m, _ := build(t, c)
	tl := tilesOf(m)
	cases := []struct{ tile, figure, caption string }{
		{"net_worth", "$250,000.00", "USD · 数据日 2026-11-10"},
		{"cash", "40.00%", "+$100,000.00"},
		{"day_change", "+$1,500.00", "+0.60%"},
		{"month_to_date", "+$5,000.00", "+2.04%"},
	}
	for _, k := range cases {
		if g := tl[k.tile]["figure"].(map[string]any)["text"]; g != k.figure {
			t.Errorf("%s figure = %v, want %s", k.tile, g, k.figure)
		}
		if g := tl[k.tile]["caption"].(map[string]any)["text"]; g != k.caption {
			t.Errorf("%s caption = %v, want %s", k.tile, g, k.caption)
		}
	}
	if len(got.Accounts) != 3 || got.Accounts[0].Alias != "main" || got.Accounts[0].Reason != "" ||
		got.Accounts[1].Reason != "" || got.Accounts[2].Reason != "no_data" || got.Accounts[0].Day != "2026-11-10" {
		t.Fatalf("input = %+v", got.Accounts)
	}
	if len(got.Accounts[0].Prior) != 2 || len(got.Accounts[0].MonthStart) != 2 {
		t.Errorf("main earlier captures not passed: %+v", got.Accounts[0])
	}
	if m["run_status"] != "ok" {
		t.Errorf("run_status = %v", m["run_status"])
	}
}

func TestFamilyCardNamesLeftOutAccount(t *testing.T) {
	t.Parallel()
	res := twoOfThree()
	res.Excluded = []blockapi.FamilyAccountOut{
		{Alias: "wife", Reason: "no_data"},
		{Alias: "daughter", Reason: "stale_input", Day: "2026-11-02"},
		{Alias: "kid", Reason: "unsupported_currency"},
	}
	c, _, _ := book(res, twoCaptured())
	m, _ := build(t, c)
	for _, want := range []string{
		wifeNoData,
		"账户（daughter）今天未采集到，最近一次是 2026-11-02（超过 3 个交易日），合计不含",
		"账户（kid）的币种缺少美元汇率或暂不支持，合计不含",
	} {
		if !hasNote(m, want) {
			t.Errorf("notes %q lack %q", notesOf(m), want)
		}
	}
}

func TestFamilyCardChangeGapNamesAccount(t *testing.T) {
	t.Parallel()
	res := twoOfThree()
	res.DayAmount, res.DayPct = gapv("missing_input"), gapv("missing_input")
	res.MonthAmount, res.MonthPct = gapv("missing_input"), gapv("missing_input")
	rows := twoCaptured()
	rows["wife"] = map[string][]store.AccountValue{"2026-11-10": vals("150000.00", "58000.00")}
	c, _, _ := book(res, rows)
	m, _ := build(t, c)
	for _, k := range []string{"day_change", "month_to_date"} {
		g := figureOf(m, k)
		if g["reason"] != "missing_input" || !strings.Contains(g["text"].(string), "账户（wife）") || strings.Contains(g["text"].(string), "main") {
			t.Errorf("%s gap = %v", k, g)
		}
	}
	if m["run_status"] != "partial" {
		t.Errorf("run_status = %v", m["run_status"])
	}
}

func TestFamilyCardSubtitle(t *testing.T) {
	t.Parallel()
	c, _, _ := book(twoOfThree(), twoCaptured())
	m, _ := build(t, c)
	if got, want := subtitleOf(m), "全家合计 · 计入 2 / 3 个账户 · 最早数据日 2026-11-10"; got != want {
		t.Errorf("subtitle = %q, want %q", got, want)
	}
	if m["card"].(map[string]any)["title"] != "Household" {
		t.Errorf("title = %v", m["card"].(map[string]any)["title"])
	}
}

func TestFamilyCardNoneCounted(t *testing.T) {
	t.Parallel()
	g := gapv("no_data")
	res := blockapi.FamilyResult{
		Currency: "USD",
		Excluded: []blockapi.FamilyAccountOut{{Alias: "main", Reason: "no_data"}, {Alias: "wife", Reason: "no_data"}, {Alias: "daughter", Reason: "no_data"}},
		Net:      g, Cash: g, CashShare: g, DayAmount: g, DayPct: g, MonthAmount: g, MonthPct: g,
	}
	c, _, _ := book(res, map[string]map[string][]store.AccountValue{})
	m, _ := build(t, c)
	for _, k := range []string{"net_worth", "cash", "day_change", "month_to_date"} {
		if r := figureOf(m, k)["reason"]; r != "no_data" {
			t.Errorf("%s reason = %v", k, r)
		}
	}
	if m["run_status"] != "failed" {
		t.Errorf("run_status = %v", m["run_status"])
	}
	if got, want := subtitleOf(m), "全家合计 · 计入 0 / 3 个账户 · 最早数据日 无"; got != want {
		t.Errorf("subtitle = %q, want %q", got, want)
	}
	if !hasNote(m, "账户（main）今天未采集到，合计不含") {
		t.Errorf("notes = %q", notesOf(m))
	}
}

func TestFamilyCardCashShare(t *testing.T) {
	t.Parallel()
	c, _, _ := book(twoOfThree(), twoCaptured())
	m, _ := build(t, c)
	f := figureOf(m, "cash")
	if f["unit"] != "share_percent" || f["text"] != "40.00%" || f["value"].(float64) != 40 {
		t.Errorf("share figure = %v", f)
	}
	res := twoOfThree()
	res.CashShare = gapv("missing_input")
	c, _, _ = book(res, twoCaptured())
	m, _ = build(t, c)
	if f := figureOf(m, "cash"); f["text"] != "$100,000.00" {
		t.Errorf("share gap figure = %v, want the amount", f)
	}
	res = twoOfThree()
	res.Cash, res.CashShare = fv("-5000.00"), gapv("missing_input")
	c, _, _ = book(res, twoCaptured())
	m, _ = build(t, c)
	if f := figureOf(m, "cash"); f["text"] != "借款" {
		t.Errorf("negative cash figure = %v", f)
	}
}

func TestFamilyCardDayGapWhenDaysDiffer(t *testing.T) {
	t.Parallel()
	res := twoOfThree()
	res.DayAmount, res.DayPct = gapv("stale_input"), gapv("stale_input")
	res.Rows[1].Day = "2026-11-09"
	rows := twoCaptured()
	delete(rows["wife"], "2026-11-10")
	c, got, _ := book(res, rows)
	m, _ := build(t, c)
	if g := figureOf(m, "day_change"); g["reason"] != "stale_input" || g["text"] != dayMixed {
		t.Errorf("day gap = %v", g)
	}
	if f := figureOf(m, "month_to_date"); f["text"] != "+$5,000.00" {
		t.Errorf("month tile = %v", f)
	}
	if got.Accounts[1].Day != "2026-11-09" {
		t.Errorf("wife day = %q", got.Accounts[1].Day)
	}
	if want := "账户（wife）用的是 2026-11-09 的采集，早于最新交易日，已计入合计"; !hasNote(m, want) {
		t.Errorf("notes %q lack %q", notesOf(m), want)
	}
}

func TestFamilyCardOtherAliasReadError(t *testing.T) {
	t.Parallel()
	res := twoOfThree()
	res.Included = []string{"main"}
	res.Excluded = []blockapi.FamilyAccountOut{{Alias: "wife", Reason: "read_error"}, {Alias: "daughter", Reason: "no_data"}}
	c, got, _ := book(res, twoCaptured())
	c.Account = byAlias{rows: twoCaptured(), errs: map[string]error{"wife": errors.New("boom")}}
	m, _ := build(t, c)
	if got.Accounts[1].Reason != "read_error" || got.Accounts[1].Values != nil {
		t.Errorf("wife input = %+v", got.Accounts[1])
	}
	if !hasNote(m, "账户（wife）读取失败，合计不含") {
		t.Errorf("notes = %q", notesOf(m))
	}
	c.Account = byAlias{rows: twoCaptured(), errs: map[string]error{"main": errors.New("boom")}}
	if _, err := home_household.Build(context.Background(), c, day, at); err == nil {
		t.Error("a first-alias read error must be an error of Build")
	}
}

func TestFamilyCardOneAliasIsToday(t *testing.T) {
	t.Parallel()
	want, err := home_household.Build(context.Background(), config(full()), day, at)
	if err != nil {
		t.Fatal(err)
	}
	for _, aliases := range [][]string{nil, {"main"}} {
		c, _, calls := book(twoOfThree(), nil)
		c.Account = fakeAccount{values: full()}
		c.Figures, c.Aliases = pagefig.New, aliases
		got, err := home_household.Build(context.Background(), c, day, at)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) || *calls != 0 {
			t.Errorf("aliases %v: card differs from today's or the family figure was called (%d)", aliases, *calls)
		}
	}
}

type fakeFx struct{ bar.FxRateReader }

func (fakeFx) LatestFxRate(_ context.Context, pair string, asOf contracts.Day) (contracts.FxRate, bool, error) {
	d, _ := time.Parse("2006-01-02", string(asOf))
	return contracts.FxRate{Pair: pair, Rate: "150.5", RateDay: types.SerializableDate{Time: d}, Source: "fred_h10"}, true, nil
}

func TestFamilyCardNotesHaveCashLine(t *testing.T) {
	t.Parallel()
	c, _, _ := book(twoOfThree(), twoCaptured())
	m, _ := build(t, c)
	n := notesOf(m)
	if n[len(n)-1] != cashLine {
		t.Errorf("notes = %q", n)
	}
	for _, l := range n {
		if strings.Contains(l, "USDJPY") {
			t.Errorf("rate line without a rate: %q", l)
		}
	}
	// A JPY account makes the card read the rate and say it.
	res := twoOfThree()
	res.FxRate, res.FxDay, res.FxSource = "150.5", "2026-11-10", "fred_h10"
	rows := twoCaptured()
	rows["wife"]["2026-11-10"][0].Currency = "JPY"
	c, got, _ := book(res, rows)
	c.Fx = fakeFx{}
	m, _ = build(t, c)
	n = notesOf(m)
	if len(n) < 2 || n[len(n)-2] != "日元账户按 USDJPY 汇率 150.5（日期 2026-11-10，来源 fred_h10）折成美元" || n[len(n)-1] != cashLine {
		t.Errorf("notes = %q", n)
	}
	if got.UsdJpy != "150.5" || got.RateDay != "2026-11-10" || got.RateSource != "fred_h10" {
		t.Errorf("rate input = %+v", got)
	}
}

func TestFamilyCardGolden(t *testing.T) {
	t.Parallel()
	c, _, _ := book(twoOfThree(), twoCaptured())
	_, b := build(t, c)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, b, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	golden := filepath.Join("testdata", "family.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, pretty.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, pretty.Bytes()) {
		t.Fatalf("golden differs:\n%s", pretty.Bytes())
	}
}

// TestFamilyCardStaleCaptureIsMarkedAfterThreeSessionDays: a capture 3 session days before the day is
// counted, one of 4 is passed to the family figure as stale_input and carries no values.
func TestFamilyCardStaleCaptureIsMarkedAfterThreeSessionDays(t *testing.T) {
	t.Parallel()
	for _, k := range []struct{ capture, reason string }{{"2026-11-05", ""}, {"2026-11-04", "stale_input"}} {
		rows := twoCaptured()
		rows["wife"] = map[string][]store.AccountValue{k.capture: vals("150000.00", "58000.00")}
		c, got, _ := book(twoOfThree(), rows)
		build(t, c)
		w := got.Accounts[1]
		if w.Reason != k.reason || w.Day != k.capture || (k.reason != "" && w.Values != nil) || (k.reason == "" && len(w.Values) == 0) {
			t.Errorf("capture %s: wife input = %+v, want reason %q", k.capture, w, k.reason)
		}
	}
}
