package option_candidates_page

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// blockTierer is the judge of these tests: it answers one fixed Result, Blocks included.
type blockTierer struct{ res optiontierapi.Result }

func (b blockTierer) Judge(optiontierapi.Input) (optiontierapi.Result, error) { return b.res, nil }

var bandParams = optiontierapi.Params{
	DeltaMax: "0.3", ThinDelta: "0.15", MaxCashAgeDays: 3, DaysInYear: 365,
	BandOTMMinPct: "5", BandOTMMaxPct: "15", BandDeltaMin: "0.15", PerCompany: 3,
}

// blockPut is a stored put of underlying id with its spot and buy price.
func blockPut(n, id int, strike, spot string, buy *string) store.Candidate {
	c := tierPut(n, "ok", strike, "0.50")
	c.UnderlyingID, c.Spot, c.BuyPrice = id, new(spot), buy
	return c
}

// blockScenario is four companies: AAA sellable with a fold, BBB above its buy price with mixed rows, CCC with
// rows out of the band, DDD with no buy price and a valuation figure. res holds the blocks.
func blockScenario() (Config, optiontierapi.Result) {
	rows := []store.Candidate{
		blockPut(1, 7, "85.00", "96.50", new("90.00")), blockPut(2, 7, "88.00", "96.50", new("90.00")),
		blockPut(3, 7, "90.00", "96.50", new("90.00")), blockPut(4, 7, "100.00", "96.50", new("90.00")),
		blockPut(5, 7, "95.00", "96.50", new("90.00")),
		blockPut(6, 8, "135.00", "130.00", new("40.00")), blockPut(7, 8, "120.00", "130.00", new("40.00")),
		blockPut(8, 8, "125.00", "130.00", new("40.00")),
		blockPut(9, 9, "48.00", "50.00", new("45.00")), blockPut(10, 9, "44.00", "50.00", new("45.00")),
		blockPut(11, 10, "40.00", "41.20", nil), blockPut(12, 10, "39.00", "41.20", nil),
	}
	k := func(i int) string { return keyOf(rows, i) }
	vs := []optiontierapi.Verdict{
		fitsVerdict(k(0)), checkVerdict(k(1), "no_open_interest"), fitsVerdict(k(2)),
		greyVerdict(k(3), "in_the_money"), aboveBuyVerdict(k(4)),
		greyVerdict(k(5), "in_the_money"), aboveBuyVerdict(k(6)), aboveBuyVerdict(k(7)),
		fitsVerdict(k(8)), checkVerdict(k(9), "delta_above_bound"),
		greyVerdict(k(10), "no_buy_price"), greyVerdict(k(11), "no_buy_price"),
	}
	res := resultOf(vs...)
	res.Params = bandParams
	res.Companies = []optiontierapi.CompanyCount{
		{Company: "7", Fits: 3, Check: 1, Grey: 1}, {Company: "8", Fits: 2, Grey: 1},
		{Company: "9", Fits: 1, Check: 1}, {Company: "10", Grey: 2},
	}
	res.Blocks = []optiontierapi.CompanyBlock{
		{Company: "7", State: optiontierapi.StateSellable, Best: "87.5000", NFit: 3, NCheck: 1, NOpen: 1,
			Shown: []string{k(0), k(1), k(4)}, Folded: []string{k(3), k(2)},
			Why: []optiontierapi.GreyCount{{Code: "in_the_money", N: 1}, {Code: optiontierapi.FoldOverCap, N: 1}}},
		{Company: "9", State: optiontierapi.StateOutBand, NOpen: 2, Folded: []string{k(8), k(9)},
			Why: []optiontierapi.GreyCount{{Code: optiontierapi.FoldOTMLow, N: 1}, {Code: optiontierapi.FoldDeltaOut, N: 1}}},
		{Company: "8", State: optiontierapi.StateSellable, Best: "85.0000", NFit: 2, NOpen: 1,
			Shown: []string{k(6), k(7)}, Folded: []string{k(5)},
			Why: []optiontierapi.GreyCount{{Code: "in_the_money", N: 1}}},
		{Company: "10", State: optiontierapi.StateNoBuyPrice, Folded: []string{k(10), k(11)},
			Why: []optiontierapi.GreyCount{{Code: "no_buy_price", N: 2}}},
	}
	c := tierConfig(rows, blockTierer{res: res})
	c.Names = &namesFake{names: map[int]string{7: "AAA", 8: "BBB", 9: "CCC", 10: "DDD"}}
	c.Valuation, c.Buy = &fakeValuation{st: valueState(valueEntry("DDD", valueReview()))}, &fakeBuy{price: "45"}
	return c, res
}

func scenarioView(t *testing.T) *tiersView {
	t.Helper()
	c, _ := blockScenario()
	return buildTiersOf(t, c)
}

func rowByID(v *tiersView, id string) (tierRow, bool) {
	for _, tv := range v.Tiers {
		for _, r := range tv.Rows {
			if r.ID == id {
				return r, true
			}
		}
	}
	return tierRow{}, false
}

func TestVerdictWordsAndTexts(t *testing.T) {
	t.Parallel()
	cv := companyView{Spot: "$130.00", BuyPrice: new("$40.00")}
	cb := optiontierapi.CompanyBlock{NFit: 2, NCheck: 1, NOpen: 4}
	want := []struct{ state, word, glyph, tone string }{
		{"sellable", "有可卖", "✓", "success"}, {"out_band", "筛选范围内暂无可卖", "!", "warn"},
		{"no_buy_price", "未设买入价·未判定", "–", "muted"},
		{"no_quote", "无报价", "–", "muted"}, {"blocked", "暂无可卖", "–", "muted"},
	}
	for _, w := range want {
		cb.State = w.state
		b := blockViewOf(cb, cv, bandParams)
		if b.Verdict != (verdictView{ID: w.state, Word: w.word, Glyph: w.glyph, Tone: w.tone}) {
			t.Errorf("%s verdict = %+v", w.state, b.Verdict)
		}
		if got := strings.Contains(b.Verdict.Word, "未判定"); got != (w.state == "no_buy_price") {
			t.Errorf("%s word %q: contains 未判定 = %v", w.state, b.Verdict.Word, got)
		}
		if n := len([]rune(b.VerdictText)); n == 0 || n > 200 {
			t.Errorf("%s text has %d characters", w.state, n)
		}
	}
	cb.State = "sellable"
	if got, wantText := blockViewOf(cb, cv, bandParams).VerdictText, "筛选范围内，按年化从高到低，最多 3 个：适合 2 个，待查 1 个；其余在下面折叠。"; got != wantText {
		t.Errorf("sellable text = %q", got)
	}
	cb.State = "out_band"
	if got, wantText := blockViewOf(cb, cv, bandParams).VerdictText, "有 4 个合约能卖，但都不在筛选范围内（价外 5% 到 15%，delta 0.15 到 0.30，持仓量够），原因见下方折叠。"; got != wantText {
		t.Errorf("out_band text = %q", got)
	}
}

func TestFoldedSummaryCounts(t *testing.T) {
	t.Parallel()
	var keys []string
	for i := range 69 {
		keys = append(keys, "r"+strconv.Itoa(i))
	}
	cb := optiontierapi.CompanyBlock{Company: "7", State: "sellable", Folded: keys, Why: []optiontierapi.GreyCount{
		{Code: "in_the_money", N: 16}, {Code: "otm_low", N: 8}, {Code: "over_cap", N: 5}}}
	f := blockViewOf(cb, companyView{}, bandParams).Folded
	if f == nil || f.Title != "未显示 69" || f.Summary != "价内 16 · 价外不足 5% 8 · 超出每家 3 个 5" {
		t.Fatalf("folded = %+v", f)
	}
	if len(f.Reasons) != 3 || f.Reasons[1] != (foldReasonView{Code: "otm_low", Label: "价外不足 5%", Count: "8"}) {
		t.Errorf("reasons = %+v", f.Reasons)
	}
}

func TestShownPlusFoldedIsAllRows(t *testing.T) {
	t.Parallel()
	v := scenarioView(t)
	if v.ByCompany == nil {
		t.Fatal("no by_company")
	}
	byCompany := map[string]map[string]int{}
	for _, tv := range v.Tiers {
		for _, r := range tv.Rows {
			if byCompany[r.Company] == nil {
				byCompany[r.Company] = map[string]int{}
			}
			byCompany[r.Company][r.ID]++
		}
	}
	for _, b := range v.ByCompany.Blocks {
		got := map[string]int{}
		for _, id := range b.RowIDs {
			got[id]++
		}
		if b.Folded != nil {
			sum := 0
			for _, r := range b.Folded.Reasons {
				n, err := strconv.Atoi(r.Count)
				if err != nil {
					t.Fatal(err)
				}
				sum += n
			}
			if sum != len(b.Folded.RowIDs) {
				t.Errorf("%s: reasons add to %d, folded ids %d", b.Company, sum, len(b.Folded.RowIDs))
			}
			for _, id := range b.Folded.RowIDs {
				got[id]++
			}
		}
		if !reflect.DeepEqual(got, byCompany[b.Company]) {
			t.Errorf("%s: shown and folded ids %v, rows %v", b.Company, got, byCompany[b.Company])
		}
	}
}

func TestBlocksOnlyForListedCompanies(t *testing.T) {
	t.Parallel()
	c, res := blockScenario()
	c.Valuation, c.Buy = nil, nil // DDD has no buy price and no figure: it is in the idle line
	v := buildTiersOf(t, c)
	if v.IdleNote == nil || !strings.Contains(*v.IdleNote, "DDD") {
		t.Fatalf("idle note = %v", v.IdleNote)
	}
	var got []string
	for _, b := range v.ByCompany.Blocks {
		got = append(got, b.Company)
	}
	if want := []string{"7", "9", "8"}; !reflect.DeepEqual(got, want) {
		t.Errorf("blocks = %v, want %v (the order of Blocks, no idle company)", got, want)
	}
	if len(res.Blocks) != 4 {
		t.Fatal("scenario changed")
	}
}

func TestRowTierBadge(t *testing.T) {
	t.Parallel()
	v := scenarioView(t)
	b := v.ByCompany.Blocks[0]
	fit, _ := rowByID(v, b.RowIDs[0])
	check, _ := rowByID(v, b.RowIDs[1])
	if fit.Tier == nil || *fit.Tier != (tierBadge{ID: "fits", Word: "适合", Glyph: "✓"}) {
		t.Errorf("fits row tier = %+v", fit.Tier)
	}
	if check.Tier == nil || *check.Tier != (tierBadge{ID: "check", Word: "待查", Glyph: "!"}) {
		t.Errorf("check row tier = %+v", check.Tier)
	}
	for _, bl := range v.ByCompany.Blocks {
		if bl.Folded == nil {
			continue
		}
		for _, id := range bl.Folded.RowIDs {
			if r, ok := rowByID(v, id); !ok || r.Tier != nil {
				t.Errorf("folded row %s: found %v, tier %+v", id, ok, r.Tier)
			}
		}
	}
}

func TestByCompanyAbsentWithoutBlocks(t *testing.T) {
	t.Parallel()
	c, res := blockScenario()
	res.Blocks = nil
	c.Tiers = blockTierer{res: res}
	v := buildTiersOf(t, c)
	if v.ByCompany != nil {
		t.Errorf("by_company = %+v", v.ByCompany)
	}
	for _, tv := range v.Tiers {
		for _, r := range tv.Rows {
			if r.Tier != nil {
				t.Errorf("row %s has a tier", r.ID)
			}
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("by_company")) {
		t.Error("by_company in the payload")
	}
}

func TestJapaneseViewHasBlocks(t *testing.T) {
	t.Parallel()
	c, _ := jpConfig(t, 0, 1)
	probe := markTierer()
	c.Tiers = &fakeTierer{fn: func(in optiontierapi.Input) optiontierapi.Result {
		res := probe.fn(in)
		res.Params = bandParams
		keys := make([]string, len(in.Rows))
		for i, r := range in.Rows {
			keys[i] = r.Key
		}
		res.Blocks = []optiontierapi.CompanyBlock{{Company: "21", State: "sellable", NCheck: len(keys), Shown: keys}}
		return res
	}}
	p, _ := buildJP(t, c)
	if p.TiersJPY == nil || p.TiersJPY.ByCompany == nil || len(p.TiersJPY.ByCompany.Blocks) != 1 {
		t.Fatalf("yen view = %+v", p.TiersJPY)
	}
	if p.Tiers != nil && p.Tiers.ByCompany != nil {
		t.Error("the dollar view has the yen blocks")
	}
	if b := p.TiersJPY.ByCompany.Blocks[0]; len(b.RowIDs) != 2 || b.Folded != nil {
		t.Errorf("block = %+v", b)
	}
}

func TestLegendYieldBasis(t *testing.T) {
	t.Parallel()
	v := scenarioView(t)
	var text string
	for _, l := range v.ByCompany.Legend {
		if l.Tier == "yield" {
			text = l.Text
		}
	}
	for _, part := range []string{"到期天数", "占用的现金（行权价）", "日元公司在日本标签页，不和美元公司排在一起"} {
		if !strings.Contains(text, part) {
			t.Errorf("yield item %q lacks %q", text, part)
		}
	}
	var tiers []string
	for _, l := range v.ByCompany.Legend {
		tiers = append(tiers, l.Tier)
	}
	if want := []string{"fits", "check", "folded", "yield", "cash"}; !reflect.DeepEqual(tiers, want) {
		t.Errorf("legend = %v", tiers)
	}
}

func TestHeadingNote(t *testing.T) {
	t.Parallel()
	v := scenarioView(t)
	if v.ByCompany.HeadingNote != zhHeadingNote || v.ByCompany.HeadingNote == "" {
		t.Errorf("heading note = %q", v.ByCompany.HeadingNote)
	}
	if v.Heads == nil || v.Heads.Tiers != "合约分档" || v.Heads.CompaniesNote != "每家公司两行：上行看价格，下行看估值和合约数量" {
		t.Errorf("heads = %+v", v.Heads)
	}
	for _, c := range v.ByCompany.Columns {
		if c.ID == "premium" && (c.Label != "权利金 / 年化" || deref(c.Hint) != "每份合约的权利金；下一行是年化收益，也是每家公司内排序的依据。") {
			t.Errorf("premium column = %+v", c)
		}
	}
}

func TestByCompanyGolden(t *testing.T) {
	t.Parallel()
	c, _ := blockScenario()
	got, err := Build(context.Background(), c, nil, buildDay, testNow)
	if err != nil {
		t.Fatal(err)
	}
	var page map[string]json.RawMessage
	if err := json.Unmarshal(got, &page); err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	if err := pageSchema(t).Validate(doc); err != nil {
		t.Fatalf("the page breaks its schema: %v", err)
	}
	var tiers struct {
		ByCompany json.RawMessage `json:"by_company"`
	}
	if err := json.Unmarshal(page["short_put_tiers"], &tiers); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/by_company.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jsonschema.UnmarshalJSON(bytes.NewReader(want)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(compact(t, want), compact(t, tiers.ByCompany)) {
		t.Errorf("by_company differs from testdata/by_company.golden.json:\n%s", tiers.ByCompany)
	}
}

func aboveBuyVerdict(key string) optiontierapi.Verdict {
	v := fitsVerdict(key)
	v.Notes = []string{"above_buy_price"}
	return v
}
