package home_options_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/pages/home_options"
)

// fakeStructures returns the same result for every alias and records each input.
type fakeStructures struct {
	res  compute.StructureResult
	seen *[]compute.StructureInput
}

func (f fakeStructures) Evaluate(in compute.StructureInput, _ compute.StructureParams) compute.StructureResult {
	*f.seen = append(*f.seen, in)
	return f.res
}

// fakeTiles returns out whatever the input and records it.
type fakeTiles struct {
	out  blockapi.OptionTilesOut
	seen *blockapi.OptionTilesIn
	n    *int
}

func (f fakeTiles) Tiles(in blockapi.OptionTilesIn) blockapi.OptionTilesOut {
	*f.seen = in
	*f.n++
	return f.out
}

type probe struct {
	evaluated []compute.StructureInput
	tilesIn   blockapi.OptionTilesIn
	calls     int
}

// withFigures sets the two fields on the config; the probe records what compute was given.
func withFigures(c home_options.Config, p *probe, res compute.StructureResult, out blockapi.OptionTilesOut) home_options.Config {
	c.Structures = func() compute.OptionStructures { return fakeStructures{res: res, seen: &p.evaluated} }
	c.Tiles = func() blockapi.OptionTiles { return fakeTiles{out: out, seen: &p.tilesIn, n: &p.calls} }
	return c
}

func figCfg(p *probe, res compute.StructureResult, out blockapi.OptionTilesOut) home_options.Config {
	return withFigures(okCfg(), p, res, out)
}

func tileOf(c map[string]any, k string) map[string]any {
	return c["tiles"].(map[string]any)[k].(map[string]any)
}

func usd(v, dollars string) blockapi.UsdText { return blockapi.UsdText{Value: v, Dollars: dollars} }

func TestFiguresFromTheFakeTiles(t *testing.T) {
	t.Parallel()
	var p probe
	structs := compute.StructureResult{
		Structures: []compute.Structure{{ID: "main:a", Kind: "short_put"}},
		Other:      []compute.Structure{{ID: "main:b", Kind: "other"}},
	}
	out := blockapi.OptionTilesOut{SoldCount: 2, OpenPremium: usd("1234.50", "1235"), MaxLossUsd: usd("2000.00", "2000"), OpenCount: 2}
	c := card(t, withFigures(cfg(ledger{aliases: map[string]int{"a": 1, "b": 1}}, results{}, pos()), &p, structs, out))
	if f := tileFigure(c, "sold_count_month"); f["value"] != float64(2) || f["unit"] != "structures" || f["text"] != "2" {
		t.Fatalf("sold = %v", f)
	}
	if f := tileFigure(c, "open_premium_usd"); f["text"] != "$1,235" || f["value"] != 1234.5 || f["unit"] != "usd" {
		t.Fatalf("open = %v", f)
	}
	if f := tileFigure(c, "max_loss_usd"); f["text"] != "$2,000" || f["value"] != float64(2000) {
		t.Fatalf("loss = %v", f)
	}
	checkCaptions(t, c)
	checkProbe(t, &p)
}

func checkCaptions(t *testing.T, c map[string]any) {
	t.Helper()
	if got := tileOf(c, "sold_count_month")["caption"]; got != "Ledger as of 11-04, all accounts combined; lots opened this month" {
		t.Fatalf("sold caption = %v", got)
	}
	if got := tileOf(c, "open_premium_usd")["caption"]; got != "Ledger as of 11-04, all accounts combined; remaining part of each lot" {
		t.Fatalf("open caption = %v", got)
	}
	if got := tileOf(c, "max_loss_usd")["caption"]; got != "Ledger as of 11-04, all accounts combined" {
		t.Fatalf("loss caption = %v", got)
	}
}

// Evaluate once per alias with the four fields of the design and no others; one Tiles call with everything.
func checkProbe(t *testing.T, p *probe) {
	t.Helper()
	if len(p.evaluated) != 2 || p.calls != 1 {
		t.Fatalf("evaluated %d, tiles calls %d", len(p.evaluated), p.calls)
	}
	for _, in := range p.evaluated {
		if in.Alias == "" || len(in.Positions) != 1 || in.AsOf != day || in.Quotes != nil || in.Held != nil || in.HeldDay != "" || in.SessionDays != nil {
			t.Fatalf("input = %+v", in)
		}
	}
	if p.tilesIn.Day != day || len(p.tilesIn.Positions) != 2 || len(p.tilesIn.Structures) != 4 {
		t.Fatalf("tiles input = %+v", p.tilesIn)
	}
}

func TestUncoveredCallIsTheMaxLossGap(t *testing.T) {
	t.Parallel()
	var p probe
	out := blockapi.OptionTilesOut{
		OpenPremium: usd("10.00", "10"), MaxLossUsd: usd("12345.67", "12346"), MaxLossGap: "no_data", Uncovered: 1, OpenCount: 2,
	}
	c := card(t, figCfg(&p, compute.StructureResult{}, out))
	g := tileFigure(c, "max_loss_usd")
	want := "1 calls without assigned shares in the ledger; other structures at least $12,346"
	if g["kind"] != "gap" || g["reason"] != "no_data" || g["text"] != want {
		t.Fatalf("gap = %v", g)
	}
}

// An uncovered call beside an unnumbered structure: the text still says "at least" the numbered rest.
func TestUncoveredCallWithUnnumberedStructure(t *testing.T) {
	t.Parallel()
	var p probe
	out := blockapi.OptionTilesOut{MaxLossUsd: usd("300.00", "300"), MaxLossGap: "no_data", Uncovered: 1, Unnumbered: 1, OpenCount: 3}
	g := tileFigure(card(t, figCfg(&p, compute.StructureResult{}, out)), "max_loss_usd")
	if g["text"] != "1 calls without assigned shares in the ledger; other structures at least $300" {
		t.Fatalf("gap = %v", g)
	}
	out.MaxLossUsd = blockapi.UsdText{}
	g = tileFigure(card(t, figCfg(&p, compute.StructureResult{}, out)), "max_loss_usd")
	if g["text"] != "1 calls without assigned shares in the ledger; other structures at least $0" {
		t.Fatalf("gap = %v", g)
	}
}

func TestUnnumberedStructureCaption(t *testing.T) {
	t.Parallel()
	var p probe
	out := blockapi.OptionTilesOut{MaxLossUsd: usd("300.00", "300"), OpenPremium: usd("5.00", "5"), Unnumbered: 2, OpenCount: 3}
	c := card(t, figCfg(&p, compute.StructureResult{}, out))
	if got := tileOf(c, "max_loss_usd")["caption"].(string); !strings.HasSuffix(got, "At least: 2 structures have no loss number") {
		t.Fatalf("caption = %q", got)
	}
	if f := tileFigure(c, "max_loss_usd"); f["kind"] != "value" {
		t.Fatalf("figure = %v", f)
	}
}

func TestOnlyNonUsdIsUnsupportedNotZero(t *testing.T) {
	t.Parallel()
	var p probe
	out := blockapi.OptionTilesOut{OpenPremiumGap: "unsupported_currency", MaxLossGap: "unsupported_currency", OpenCount: 2, OtherCurrency: 2}
	c := card(t, figCfg(&p, compute.StructureResult{}, out))
	for _, k := range []string{"open_premium_usd", "max_loss_usd"} {
		g := tileFigure(c, k)
		if g["kind"] != "gap" || g["reason"] != "unsupported_currency" || g["text"] != "Only non-USD structures are open (2), not converted" {
			t.Fatalf("%s = %v", k, g)
		}
	}
}

func TestNoOpenStructureIsZeroWithItsCaption(t *testing.T) {
	t.Parallel()
	var p probe
	out := blockapi.OptionTilesOut{OpenPremium: usd("0.00", "0"), MaxLossUsd: usd("0.00", "0")}
	c := card(t, figCfg(&p, compute.StructureResult{}, out))
	for _, k := range []string{"open_premium_usd", "max_loss_usd"} {
		f := tileFigure(c, k)
		if f["kind"] != "value" || f["text"] != "$0" || f["value"] != float64(0) {
			t.Fatalf("%s = %v", k, f)
		}
		got := tileOf(c, k)["caption"].(string)
		if !strings.Contains(got, "no open structures in the ledger") || !strings.HasPrefix(got, "Ledger as of 11-04, all accounts combined") {
			t.Fatalf("%s caption = %q", k, got)
		}
	}
}

func TestNetPaidIsTheNoDataGap(t *testing.T) {
	t.Parallel()
	var p probe
	out := blockapi.OptionTilesOut{OpenPremiumGap: "no_data", MaxLossUsd: usd("1.00", "1"), OpenCount: 1}
	g := tileFigure(card(t, figCfg(&p, compute.StructureResult{}, out)), "open_premium_usd")
	if g["kind"] != "gap" || g["reason"] != "no_data" || g["text"] != "Net paid, not shown" {
		t.Fatalf("gap = %v", g)
	}
}

func TestUnreadableAndNonUsdCaptions(t *testing.T) {
	t.Parallel()
	var p probe
	out := blockapi.OptionTilesOut{OpenPremium: usd("1.00", "1"), MaxLossUsd: usd("1.00", "1"), OpenCount: 4, Unreadable: 1, OtherCurrency: 2}
	c := card(t, figCfg(&p, compute.StructureResult{}, out))
	for _, k := range []string{"open_premium_usd", "max_loss_usd"} {
		got := tileOf(c, k)["caption"].(string)
		for _, w := range []string{"2 non-USD structures not counted", "1 structures with unreadable rows not counted"} {
			if !strings.Contains(got, w) {
				t.Fatalf("%s caption lacks %q: %q", k, w, got)
			}
		}
	}
	plain := card(t, figCfg(&p, compute.StructureResult{}, blockapi.OptionTilesOut{OpenCount: 1}))
	if got := tileOf(plain, "open_premium_usd")["caption"].(string); strings.Contains(got, "non-USD") || strings.Contains(got, "unreadable") {
		t.Fatalf("caption = %q", got)
	}
}

func gapTexts(c map[string]any) string {
	b, _ := json.Marshal(c["gaps"])
	return string(b)
}

func TestReviewLineAppearsAndDisappears(t *testing.T) {
	t.Parallel()
	var p probe
	with := figCfg(&p, compute.StructureResult{}, blockapi.OptionTilesOut{})
	with.Replay = func() compute.OptionLedger {
		return reviewReplayer{replayer{[]compute.LedgerPosition{pos()}}, []string{"u1"}, []string{"r1", "r2"}}
	}
	c := card(t, with)
	if !strings.Contains(gapTexts(c), "3 ledger rows need review, figures may be incomplete") || strings.Count(gapTexts(c), "need review") != 1 {
		t.Fatalf("gaps = %v", c["gaps"])
	}
	for _, g := range c["gaps"].([]any) {
		if m := g.(map[string]any); strings.Contains(m["text"].(string), "need review") && m["reason"] != "no_data" {
			t.Fatalf("gap = %v", m)
		}
	}
	if without := card(t, figCfg(&p, compute.StructureResult{}, blockapi.OptionTilesOut{})); strings.Contains(gapTexts(without), "need review") {
		t.Fatalf("gaps = %v", without["gaps"])
	}
}

type reviewReplayer struct {
	replayer
	unresolved, review []string
}

func (r reviewReplayer) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	return compute.LedgerResult{Positions: r.positions, Unresolved: r.unresolved, Review: r.review}
}

func TestCardGapLeavesOnlyWithBothFields(t *testing.T) {
	t.Parallel()
	var p probe
	both := figCfg(&p, compute.StructureResult{}, blockapi.OptionTilesOut{})
	noTiles, noStructures := both, both
	noTiles.Tiles = nil
	noStructures.Structures = nil
	const gapText = "Max loss needs the combined structure, the ledger does not give it"
	if strings.Contains(gapTexts(card(t, both)), gapText) {
		t.Fatal("the card gap stays with both fields set")
	}
	for name, c := range map[string]home_options.Config{"tiles nil": noTiles, "structures nil": noStructures} {
		k := card(t, c)
		if !strings.Contains(gapTexts(k), gapText) {
			t.Errorf("%s: the card gap is gone", name)
		}
		if g := tileFigure(k, "max_loss_usd"); g["kind"] != "gap" || g["text"] != gapText {
			t.Errorf("%s: tile = %v", name, g)
		}
		if strings.Contains(gapTexts(k), "need review") {
			t.Errorf("%s: review line without figures", name)
		}
	}
}

// Every caption and gap text with the largest counts stays inside the 200 characters of the schema.
func TestTextsFitTheSchemaBound(t *testing.T) {
	t.Parallel()
	const big = 100000
	huge := usd("999999999.99", "1000000000")
	cases := []blockapi.OptionTilesOut{
		{SoldCount: big, OpenPremium: huge, MaxLossUsd: huge, OpenCount: big, Unnumbered: big, Unreadable: big, OtherCurrency: big},
		{SoldCount: big, OpenPremiumGap: "unsupported_currency", MaxLossGap: "unsupported_currency", OpenCount: big, OtherCurrency: big},
		{OpenPremiumGap: "no_data", MaxLossGap: "no_data", MaxLossUsd: huge, Uncovered: big, OpenCount: big, Unreadable: big, OtherCurrency: big},
	}
	for i, out := range cases {
		var p probe
		c := card(t, figCfg(&p, compute.StructureResult{}, out))
		for k, v := range c["tiles"].(map[string]any) {
			tl := v.(map[string]any)
			if s, _ := tl["caption"].(string); len([]rune(s)) > 200 {
				t.Errorf("case %d %s caption is %d characters", i, k, len([]rune(s)))
			}
			if s, _ := tl["figure"].(map[string]any)["text"].(string); len([]rune(s)) > 200 {
				t.Errorf("case %d %s text is %d characters", i, k, len([]rune(s)))
			}
		}
	}
	rows := make([]string, big)
	var p probe
	with := figCfg(&p, compute.StructureResult{}, blockapi.OptionTilesOut{})
	with.Replay = func() compute.OptionLedger {
		return reviewReplayer{replayer{[]compute.LedgerPosition{pos()}}, rows, nil}
	}
	for _, g := range card(t, with)["gaps"].([]any) {
		if s := g.(map[string]any)["text"].(string); len([]rune(s)) > 200 {
			t.Errorf("gap text is %d characters", len([]rune(s)))
		}
	}
}

func TestGoldenFigures(t *testing.T) {
	t.Parallel()
	var p probe
	out := blockapi.OptionTilesOut{SoldCount: 3, OpenPremium: usd("1234.50", "1235"), MaxLossUsd: usd("4000.00", "4000"), OpenCount: 2}
	got := build(t, figCfg(&p, compute.StructureResult{}, out))
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, got, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	want, err := os.ReadFile(filepath.Join("testdata", "figures.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pretty.Bytes(), want) {
		t.Fatalf("golden differs:\n%s", pretty.Bytes())
	}
	if strings.Contains(pretty.String(), "the ledger summary gives no") {
		t.Fatal("a tile still carries the old gap text")
	}
}
