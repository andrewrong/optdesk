package option_candidates_page

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestTiersFromTierer(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	v := fitsVerdict(keyOf(rows, 0))
	v.Badges, v.Notes = []string{}, []string{"thin_premium"}
	f := &fakeTierer{res: resultOf(v)}
	view := buildTiersOf(t, tierConfig(rows, f))
	fits := tierByID(view, "fits")
	if fits == nil || len(fits.Rows) != 1 || fits.Title != "适合 1" || fits.Word != "适合" || fits.Icon != "check" || fits.Collapsed {
		t.Fatalf("fits = %+v", fits)
	}
	r := fits.Rows[0]
	if r.ID != v.Key || r.Margin.Main != "6.0%" || deref(r.Margin.Sub) != "保本点 $84.58 / 买入价 $90.00" ||
		r.BelowSpot.Main != "12.4%" || deref(r.BelowSpot.Sub) != "1.3 个预期波动*" ||
		r.Premium.Main != "$42.00" || deref(r.Premium.Sub) != "5.8% / 年" {
		t.Errorf("row = %+v", r)
	}
	if len(r.Notes) != 1 || r.Notes[0].Code != "thin_premium" || r.Notes[0].Label != "权利金薄" || r.Line != nil {
		t.Errorf("notes = %+v line %v", r.Notes, r.Line)
	}
	if r.Contract != (contractView{Name: "AAA", Strike: "$85.00", Expiry: "2026-12-18", DTE: "39 天"}) {
		t.Errorf("contract = %+v", r.Contract)
	}
	if f.calls() != 1 {
		t.Errorf("Judge called %d times", f.calls())
	}
}

func TestTiersTierOrderAndCollapsed(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42"), tierPut(2, "ok", "87.50", "0.60"),
		tierPut(3, "ok", "100.00", "5.40"), tierPut(4, "ok", "90.00", "0.94")}
	k := func(i int) string { return keyOf(rows, i) }
	f := &fakeTierer{res: resultOf(checkVerdict(k(1), "no_open_interest"), greyVerdict(k(2), "in_the_money"), greyVerdict(k(3), "no_buy_price"))}
	view := buildTiersOf(t, tierConfig(rows, f))
	if got, want := tierIDs(view), []string{"fits", "check", "grey_in_the_money", "grey_no_buy_price"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tiers = %v, want %v", got, want)
	}
	for _, tr := range view.Tiers {
		if grey := strings.HasPrefix(tr.ID, "grey_"); tr.Collapsed != grey {
			t.Errorf("%s collapsed = %v", tr.ID, tr.Collapsed)
		}
	}
	if tr := tierByID(view, "fits"); len(tr.Rows) != 0 || tr.Title != "适合 0" {
		t.Errorf("empty fits = %+v", tr)
	}
	if tr := tierByID(view, "grey_no_buy_price"); tr.Title != "1 行：没有买入价" || tr.Icon != "minus" || tr.Word != "未判定" {
		t.Errorf("grey = %+v", tr)
	}
	if tr := tierByID(view, "check"); tr.Icon != "alert" || tr.Title != "待查 1" {
		t.Errorf("check = %+v", tr)
	}
}

func TestEveryRowShownOnce(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42"), tierPut(2, "blocked", "87.50", "0.60"),
		tierPut(3, "unknown", "100.00", "5.40"), tierPut(4, "ok", "90.00", "0.00")}
	k := func(i int) string { return keyOf(rows, i) }
	f := &fakeTierer{res: resultOf(fitsVerdict(k(0)), checkVerdict(k(1), "stale_quote"), greyVerdict(k(2), "in_the_money"), greyVerdict(k(3), "no_bid"))}
	view := buildTiersOf(t, tierConfig(rows, f))
	var got []string
	for _, tr := range view.Tiers {
		for _, r := range tr.Rows {
			got = append(got, r.ID)
		}
	}
	want := []string{k(0), k(1), k(2), k(3)}
	slices.Sort(got)
	slices.Sort(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rows shown = %v, read %v", got, want)
	}
	if len(f.in[0].Rows) != 4 {
		t.Errorf("Judge got %d rows of 4", len(f.in[0].Rows))
	}
}

func TestNilTiersIsNull(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	b := buildBytes(t, tierConfig(rows, nil), buildDay)
	if !strings.Contains(string(b), `"short_put_tiers":null`) {
		t.Errorf("short_put_tiers is not null: %s", b)
	}
	var p outPage
	if err := json.Unmarshal(b, &p); err != nil || len(p.Gaps) != 0 {
		t.Errorf("gaps = %+v err %v", p.Gaps, err)
	}
	// a Tierer with no short-put rows is not called and the field is null too
	f := &fakeTierer{}
	c := tierConfig(nil, f)
	if b := buildBytes(t, c, buildDay); !strings.Contains(string(b), `"short_put_tiers":null`) || f.calls() != 0 {
		t.Errorf("no rows: %s, %d calls", b, f.calls())
	}
}

func TestCommonMarkSaidOnce(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	v := fitsVerdict(keyOf(rows, 0))
	v.Infos = []string{"earnings_unconfirmed", "earnings_unknown"}
	res := resultOf(v)
	res.Common = []string{"earnings_unconfirmed"}
	view := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: res}))
	if len(view.Common) != 1 || view.Common[0].Code != "earnings_unconfirmed" ||
		!strings.HasPrefix(view.Common[0].Gloss, "本次运行的每一行都有：财报日未确认。") ||
		!strings.HasSuffix(view.Common[0].Gloss, "这不会改变任何一行的档位。") {
		t.Fatalf("common = %+v", view.Common)
	}
	for _, b := range tierByID(view, "fits").Rows[0].Badges {
		if b.Code == "earnings_unconfirmed" {
			t.Errorf("common mark in a row badge: %+v", b)
		}
	}
	if b := tierByID(view, "fits").Rows[0].Badges; len(b) != 1 || b[0].Code != "earnings_unknown" || b[0].Tone != "info" {
		t.Errorf("badges = %+v", b)
	}
}

func TestGreyRowLine(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "100.00", "5.40")}
	f := &fakeTierer{res: resultOf(greyVerdict(keyOf(rows, 0), "in_the_money"))}
	view := buildTiersOf(t, tierConfig(rows, f))
	r := tierByID(view, "grey_in_the_money").Rows[0]
	if r.Line == nil || !strings.Contains(*r.Line, "intrinsic $3.50, time value $1.90") || !strings.Contains(*r.Line, "BE $96.50") ||
		!strings.Contains(*r.Line, "22.4%/yr") || !strings.Contains(*r.Line, "AAA $100.00") {
		t.Errorf("line = %v", r.Line)
	}
	if len(r.Fields) != 0 {
		t.Errorf("a grey row has fields: %+v", r.Fields)
	}
}

func goldenTiers(t *testing.T, name string) []byte {
	t.Helper()
	c, f := tierScenario(name)
	_ = f
	b, err := Build(context.Background(), c, nil, buildDay, testNow)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func compact(t *testing.T, b []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := json.Compact(&out, b); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func pageSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	comp := jsonschema.NewCompiler()
	comp.AssertFormat()
	comp.UseLoader(fileLoader{})
	schema, err := comp.Compile(schemaHost + "page.option_candidates_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func checkGolden(t *testing.T, name string) {
	t.Helper()
	golden, err := os.ReadFile("testdata/" + name + ".golden.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(golden))
	if err != nil {
		t.Fatal(err)
	}
	if err := pageSchema(t).Validate(v); err != nil {
		t.Errorf("%s: %v", name, err)
	}
	if !bytes.Equal(compact(t, golden), compact(t, goldenTiers(t, name))) {
		t.Errorf("%s: the page differs from its golden file", name)
	}
}

func TestTiersValidatesSchema(t *testing.T) {
	t.Parallel()
	checkGolden(t, "tiers_ok")
}

func TestTiersGapsGolden(t *testing.T) {
	t.Parallel()
	checkGolden(t, "tiers_gaps")
	view := buildTiersOfScenario(t, "tiers_gaps")
	bbb := tierByID(view, "grey_no_buy_price").Rows[0]
	if bbb.Margin.Gap == nil || bbb.Margin.Main != "No buy price set" {
		t.Errorf("BBB margin = %+v", bbb.Margin)
	}
	if r := tierByID(view, "fits").Rows[0]; deref(r.BelowSpot.Sub) != "no IV" {
		t.Errorf("no IV row below_spot = %+v", r.BelowSpot)
	}
	if f := fieldByID(tierByID(view, "fits").Rows[0], "account_cash"); f == nil || f.Gap == nil || f.Gap.Text != "No account cash figure" {
		t.Errorf("cash field = %+v", f)
	}
}

func buildTiersOfScenario(t *testing.T, name string) *tiersView {
	t.Helper()
	c, _ := tierScenario(name)
	return buildTiersOf(t, c)
}

func TestKeptLine(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	res := resultOf(fitsVerdict(keyOf(rows, 0)))
	res.Kept = []optiontierapi.Kept{{Code: "no_buy_price", N: 1, Every: true}, {Code: "in_the_money", N: 2}, {Code: "no_open_interest", N: 3}}
	view := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: res}))
	want := "未进入“适合”的原因（行数）：没有买入价 1（每一行）；价内 2；" + zhKeptLabel_no_open_interest + " 3"
	if view.Kept == nil || *view.Kept != want {
		t.Errorf("kept = %v, want %s", view.Kept, want)
	}
	res.Kept = nil
	if view := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: res})); view.Kept != nil {
		t.Errorf("kept = %v with nothing kept", *view.Kept)
	}
}

func TestFiguresForBlockedAmber(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42"), tierPut(2, "blocked", "87.50", "0.60")}
	rows[1].Gates = `[{"gate":"quote","status":"block","reason":"wide_spread"}]`
	figs := &tierFigs{}
	c := tierConfig(rows, &fakeTierer{res: resultOf(fitsVerdict(keyOf(rows, 0)), checkVerdict(keyOf(rows, 1), "wide_quote"))})
	c.Figures = figs
	view := buildTiersOf(t, c)
	r := tierByID(view, "check").Rows[0]
	if f := fieldByID(r, "itm_probability"); f == nil || deref(f.Value) != "21.8%" {
		t.Errorf("itm = %+v", f)
	}
	if f := fieldByID(r, "vs_cash"); f == nil || !strings.Contains(deref(f.Value), "8.0%") {
		t.Errorf("vs cash = %+v", f)
	}
	var strikes []string
	for _, in := range figs.got {
		strikes = append(strikes, in.Strike)
	}
	if !slices.Contains(strikes, "87.50") {
		t.Errorf("Figures was not called with the blocked amber row: %v", strikes)
	}
}

func TestChineseTextsFit200(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42"), tierPut(2, "ok", "100.00", "5.40")}
	var vs []optiontierapi.Verdict
	for i, g := range []string{"no_buy_price", "no_spot", "in_the_money", "no_bid", "bad_input"} {
		vs = append(vs, greyVerdict(keyOf(rows, i%2), g))
		vs[i].Key += "-" + g
	}
	res := resultOf(vs...)
	res.Common = []string{"earnings_unconfirmed"}
	for code := range badgeTexts {
		res.Kept = append(res.Kept, optiontierapi.Kept{Code: code, N: 12})
	}
	for code := range greySpecs {
		res.Kept = append(res.Kept, optiontierapi.Kept{Code: code, N: 12, Every: true})
	}
	fits := fitsVerdict(keyOf(rows, 0))
	res.Verdicts = append(res.Verdicts, fits)
	res.Order = append(res.Order, fits.Key)
	view := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: res}))
	fit := func(what, s string) {
		if n := utf8.RuneCountInString(s); n < 1 || n > 200 {
			t.Errorf("%s has %d characters: %q", what, n, s)
		}
	}
	for _, l := range view.Legend {
		fit("legend "+l.Tier, l.Text)
	}
	for _, tr := range view.Tiers {
		fit("explain "+tr.ID, tr.Explain)
		for _, r := range tr.Rows {
			if r.Line != nil {
				fit("line "+r.ID, *r.Line)
			}
		}
	}
	fit("kept", *view.Kept)
	for _, m := range view.Common {
		fit("common", m.Gloss)
	}
	for code, b := range badgeTexts {
		fit("gloss "+code, b.gloss)
	}
}

func TestPairMainNeverEmpty(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	v := greyVerdict(keyOf(rows, 0), "no_buy_price")
	v.MarginPct, v.BelowSpotPct, v.PerContract = "", "", ""
	view := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: resultOf(v)}))
	r := tierByID(view, "grey_no_buy_price").Rows[0]
	for name, p := range map[string]pairView{"margin": r.Margin, "below_spot": r.BelowSpot, "premium": r.Premium} {
		if p.Gap == nil || p.Main == "" || p.Main != p.Gap.Text || p.Sub != nil {
			t.Errorf("%s = %+v", name, p)
		}
	}
}

var _ = store.Candidate{}

func keptOf(t *testing.T, kept ...optiontierapi.Kept) *tiersView {
	t.Helper()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	res := resultOf(fitsVerdict(keyOf(rows, 0)))
	res.Kept = kept
	return buildTiersOf(t, tierConfig(rows, &fakeTierer{res: res}))
}

func TestKeptLineChineseNames(t *testing.T) {
	t.Parallel()
	view := keptOf(t, optiontierapi.Kept{Code: "no_open_interest", N: 3, Every: true}, optiontierapi.Kept{Code: "wide_quote", N: 2})
	want := "未进入“适合”的原因（行数）：" + zhKeptLabel_no_open_interest + " 3（每一行）；" + zhKeptLabel_wide_quote + " 2"
	if view.Kept == nil || *view.Kept != want {
		t.Fatalf("kept = %v, want %s", view.Kept, want)
	}
	for _, code := range []string{"No open interest", "Wide quote"} {
		if strings.Contains(*view.Kept, code) {
			t.Errorf("kept has the English label %q", code)
		}
	}
}

func TestKeptNoteOnce(t *testing.T) {
	t.Parallel()
	view := keptOf(t, optiontierapi.Kept{Code: "in_the_money", N: 2})
	if view.KeptNote == nil || *view.KeptNote != "一行可以有几个原因，所以各项行数相加可以大于总行数。" {
		t.Errorf("kept_note = %v", view.KeptNote)
	}
	if view.StoredTitle != "已存表格" {
		t.Errorf("stored_title = %q", view.StoredTitle)
	}
	b, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(b), "一行可以有几个原因"); n != 1 {
		t.Errorf("note appears %d times", n)
	}
	if none := keptOf(t); none.Kept != nil || none.KeptNote != nil {
		t.Errorf("kept = %v, note = %v with nothing kept", none.Kept, none.KeptNote)
	}
}

func TestGreyExplainIsGlossAlone(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "100.00", "5.40")}
	view := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: resultOf(greyVerdict(keyOf(rows, 0), "in_the_money"))}))
	g := tierByID(view, "grey_in_the_money")
	if g.Explain != zhGreyGloss_in_the_money || strings.Contains(g.Explain, explainGrey) {
		t.Errorf("explain = %q", g.Explain)
	}
	if !strings.HasPrefix(tierByID(view, "fits").Explain, noneHead) {
		t.Errorf("fits explain = %q", tierByID(view, "fits").Explain)
	}
}

func TestKeptLongSetEndsWithEllipsis(t *testing.T) {
	t.Parallel()
	var kept []optiontierapi.Kept
	for _, code := range []string{"no_buy_price", "no_spot", "in_the_money", "no_bid", "bad_input",
		"no_open_interest", "low_open_interest", "one_sided_quote", "wide_quote", "stale_quote", "earnings_before_expiry",
		"cash_short", "delta_above_bound", "delta_unknown"} {
		kept = append(kept, optiontierapi.Kept{Code: code, N: 1234, Every: true})
	}
	line := *keptOf(t, kept...).Kept
	if n := utf8.RuneCountInString(line); n > 200 {
		t.Errorf("%d characters", n)
	}
	if !strings.HasSuffix(line, "…") {
		t.Errorf("a cut line does not end with the ellipsis: %s", line)
	}
	short := *keptOf(t, kept[:2]...).Kept
	if strings.HasSuffix(short, "…") {
		t.Errorf("an uncut line ends with the ellipsis: %s", short)
	}
}

func TestIdleCompaniesOneLine(t *testing.T) {
	t.Parallel()
	c, _ := tierScenario("tiers_ok")
	view := buildTiersOf(t, c)
	if len(view.Companies) != 1 || view.Companies[0].Ticker != "AAA" {
		t.Fatalf("companies = %+v", view.Companies)
	}
	if view.IdleNote == nil || *view.IdleNote != "1 家公司没有买入价和估值档案：BBB（它们的行在“没有买入价”一组，按公司折叠）" {
		t.Errorf("idle_note = %v", view.IdleNote)
	}
	if view.BuyHint == nil || !strings.Contains(*view.BuyHint, "公司 > 关注列表") {
		t.Errorf("buy_hint = %v", view.BuyHint)
	}
	// every company has a buy price: no idle line and no hint
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	all := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: resultOf(fitsVerdict(keyOf(rows, 0)))}))
	if all.IdleNote != nil || all.BuyHint != nil || len(all.Companies) != 1 {
		t.Errorf("idle = %v hint = %v companies = %d", all.IdleNote, all.BuyHint, len(all.Companies))
	}
}

func TestIdleLineEndsWithEllipsis(t *testing.T) {
	t.Parallel()
	var rows []store.Candidate
	var vs []optiontierapi.Verdict
	names := map[int]string{}
	var counts []optiontierapi.CompanyCount
	for i := range 60 {
		r := tierPut(i+1, "ok", "40.00", "0.50")
		r.UnderlyingID, r.BuyPrice = 100+i, nil
		names[100+i] = "TICKER" + strings.Repeat("X", 3) + string(rune('A'+i%26))
		rows = append(rows, r)
		vs = append(vs, greyVerdict(keyOf(rows, i), "no_buy_price"))
		counts = append(counts, optiontierapi.CompanyCount{Company: strconv.Itoa(100 + i), Grey: 1})
	}
	res := resultOf(vs...)
	res.Companies = counts
	c := tierConfig(rows, &fakeTierer{res: res})
	c.Names = &namesFake{names: names}
	view := buildTiersOf(t, c)
	if view.IdleNote == nil || utf8.RuneCountInString(*view.IdleNote) > 200 || !strings.Contains(*view.IdleNote, "…（") ||
		!strings.HasPrefix(*view.IdleNote, "60 家公司没有买入价和估值档案：") {
		t.Errorf("idle_note = %v", view.IdleNote)
	}
	if len(view.Companies) != 0 {
		t.Errorf("companies = %d", len(view.Companies))
	}
}

func TestEmptyTierSaysNone(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "100.00", "5.40")}
	view := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: resultOf(greyVerdict(keyOf(rows, 0), "in_the_money"))}))
	for _, id := range []string{"fits", "check"} {
		if e := tierByID(view, id).Explain; !strings.HasPrefix(e, "无：") || utf8.RuneCountInString(e) > 200 {
			t.Errorf("%s explain = %q", id, e)
		}
	}
	if tierByID(view, "fits").Explain == tierByID(view, "check").Explain {
		t.Error("fits and check say the same")
	}
	// a tier with rows keeps its own explanation
	rows = []store.Candidate{tierPut(1, "ok", "85.00", "0.42"), tierPut(2, "ok", "87.50", "0.60")}
	view = buildTiersOf(t, tierConfig(rows, &fakeTierer{res: resultOf(fitsVerdict(keyOf(rows, 0)), checkVerdict(keyOf(rows, 1), "no_open_interest"))}))
	if !strings.HasPrefix(tierByID(view, "fits").Explain, "满足你的全部条件") || tierByID(view, "check").Explain != explainCheck {
		t.Errorf("explains = %q, %q", tierByID(view, "fits").Explain, tierByID(view, "check").Explain)
	}
}

func TestPastEarningsIsUnknown(t *testing.T) {
	t.Parallel()
	for date, want := range map[string]string{"2026-11-08": "unknown", "2026-11-09": "2026-11-09", "2026-11-20": "2026-11-20"} {
		rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
		d := contracts.Day(date)
		rows[0].NextEarnings = &d
		view := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: resultOf(fitsVerdict(keyOf(rows, 0)))}))
		if got := view.Companies[0].NextEarnings; got != want {
			t.Errorf("stored %s shows %q, want %q", date, got, want)
		}
	}
}

func TestSpotHintSet(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	view := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: resultOf(fitsVerdict(keyOf(rows, 0)))}))
	if view.SpotHint == nil || !strings.Contains(*view.SpotHint, "运行时") || !strings.Contains(*view.SpotHint, "不是当天收盘价") {
		t.Errorf("spot_hint = %v", view.SpotHint)
	}
	if view.Companies[0].SpotDay != "2026-11-09" {
		t.Errorf("spot_day = %q", view.Companies[0].SpotDay)
	}
}

func TestColumnHintsShort(t *testing.T) {
	t.Parallel()
	cols := columnsOf("2026-11-09")
	want := []struct {
		id   string
		hint *string
	}{
		{colArrowID, nil}, {colTierID, nil},
		{colContractID, ptr("标的、行权价、到期日、剩余天数")},
		{colMarginID, ptr("保本点比买入价低多少")},
		{colBelowSpotID, ptr("保本点比现价低多少")},
		{colPremiumID, ptr("每份合约的钱 / 年化收益")},
		{colBadgesID, ptr("需要再看一眼的地方")},
	}
	if len(cols) != len(want) {
		t.Fatalf("columns = %d", len(cols))
	}
	for i, w := range want {
		got := cols[i]
		if got.ID != w.id || (got.Hint == nil) != (w.hint == nil) || (w.hint != nil && *got.Hint != *w.hint) {
			t.Errorf("column %d %s hint = %v, want %v", i, got.ID, got.Hint, w.hint)
		}
	}
}

func TestTierNotesNoEnglish(t *testing.T) {
	t.Parallel()
	c, _ := tierScenario("tiers_ok")
	idle := buildTiersOf(t, c)
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	res := resultOf(checkVerdict(keyOf(rows, 0), "wide_quote"))
	res.Kept = []optiontierapi.Kept{{Code: "in_the_money", N: 2}}
	empty := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: res}))
	if idle.BuyHint == nil || empty.Kept == nil || len(empty.Tiers) == 0 {
		t.Fatalf("hint = %v kept = %v tiers = %d", idle.BuyHint, empty.Kept, len(empty.Tiers))
	}
	for name, s := range map[string]string{"buy_hint": *idle.BuyHint, "kept": *empty.Kept, "none_fits": tierByID(empty, "fits").Explain} {
		for _, w := range []string{"Fits", "Check", "Watch list", "Buy"} {
			if strings.Contains(s, w) {
				t.Errorf("%s %q has %q", name, s, w)
			}
		}
	}
}

func TestCompanyCardSpotNote(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ marks, want string }{
		"quote of the rows day": {`["spot_quote:2026-11-09"]`, "存档 11-09"},
		"quote of earlier day":  {`["eod_fallback","chain_day:2026-11-06","spot_quote:2026-11-06"]`, "11-06 报价"},
		"chain price":           {`["eod_fallback","chain_day:2026-11-06","spot_chain"]`, "期权链价 11-06"},
		"chain price open":      {`["spot_chain"]`, "期权链价 11-09"},
		"no spot mark":          {`[]`, "存档 11-09"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
			rows[0].Marks = tc.marks
			res := resultOf(fitsVerdict(keyOf(rows, 0)))
			res.Companies = []optiontierapi.CompanyCount{{Company: "7", Fits: 1}}
			cv := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: res})).Companies[0]
			if cv.SpotNote == nil || *cv.SpotNote != tc.want {
				t.Errorf("note = %v, want %q", cv.SpotNote, tc.want)
			}
		})
	}
}

func TestAboveBuyNoteIsShownOnTheRow(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	v := fitsVerdict(keyOf(rows, 0))
	v.Badges, v.Notes = []string{}, []string{"above_buy_price"}
	view := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: resultOf(v)}))
	fits := tierByID(view, "fits")
	if fits == nil || len(fits.Rows) != 1 || fits.Collapsed {
		t.Fatalf("fits = %+v", fits)
	}
	n := fits.Rows[0].Notes
	if len(n) != 1 || n[0].Code != "above_buy_price" || n[0].Label != "行权价高于买入价" || n[0].Gloss == "" {
		t.Errorf("notes = %+v", n)
	}
}
