package option_candidates_page

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/store"
)

// bbbOf is the strip of BBB of tiers_ok, which has no buy price: a valuation figure keeps it off the idle line.
func bbbOf(t *testing.T) companyView {
	t.Helper()
	c, _ := tierScenario("tiers_ok")
	c.Valuation, c.Buy = &fakeValuation{st: valueState(valueEntry("BBB", valueReview()))}, &fakeBuy{price: "50"}
	for _, cv := range buildTiersOf(t, c).Companies {
		if cv.Ticker == "BBB" {
			return cv
		}
	}
	t.Fatal("no BBB strip")
	return companyView{}
}

// countedCompany is the first company of a one-row page whose answer carries the given counts.
func countedCompany(t *testing.T, fits, check, grey int) companyView {
	t.Helper()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	res := resultOf(fitsVerdict(keyOf(rows, 0)))
	res.Companies = []optiontierapi.CompanyCount{{Company: "7", Fits: fits, Check: check, Grey: grey}}
	return buildTiersOf(t, tierConfig(rows, &fakeTierer{res: res})).Companies[0]
}

func TestCompanyTierFromCounts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name              string
		fits, check, grey int
		id, word, glyph   string
	}{
		{"fits wins over check", 1, 2, 3, "fits", "适合", "✓"},
		{"check when no fit", 0, 2, 3, "check", "待查", "!"},
		{"grey when only grey", 0, 0, 3, "grey", "未判定", "–"},
		{"all zero is grey", 0, 0, 0, "grey", "未判定", "–"},
	} {
		got := countedCompany(t, tc.fits, tc.check, tc.grey).Tier
		if got != (tierBadge{ID: tc.id, Word: tc.word, Glyph: tc.glyph}) {
			t.Errorf("%s: tier = %+v", tc.name, got)
		}
	}
}

func TestCompanyCountParts(t *testing.T) {
	t.Parallel()
	cv := countedCompany(t, 12, 0, 3)
	if cv.CountParts != (countParts{Fits: "12", Check: "0", Grey: "3"}) {
		t.Errorf("count_parts = %+v", cv.CountParts)
	}
	if cv.Counts != "12 fit, 0 check, 3 collapsed" {
		t.Errorf("counts = %q", cv.Counts)
	}
}

func TestSpotNote(t *testing.T) {
	t.Parallel()
	cs := buildTiersOfScenario(t, "tiers_ok").Companies
	if cs[0].SpotText == nil || *cs[0].SpotText != "$96.50" || cs[0].SpotNote == nil || *cs[0].SpotNote != "存档 11-09" {
		t.Errorf("spot = %v note = %v", cs[0].SpotText, cs[0].SpotNote)
	}
	if cs[0].Spot != "$96.50" || cs[0].SpotDay != "2026-11-09" {
		t.Errorf("old spot fields = %q, %q", cs[0].Spot, cs[0].SpotDay)
	}
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	rows[0].Spot = nil
	res := resultOf(fitsVerdict(keyOf(rows, 0)))
	res.Companies = []optiontierapi.CompanyCount{{Company: "7", Fits: 1}}
	no := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: res})).Companies[0]
	if no.SpotText != nil || no.SpotNote != nil || no.Spot != "no spot" {
		t.Errorf("no spot: %+v", no)
	}
}

func TestCompanyZhTexts(t *testing.T) {
	t.Parallel()
	cs := buildTiersOfScenario(t, "tiers_ok").Companies
	if cs[0].PriceNote != nil || cs[0].EarningsText != "来源暂无日期（未查到，不等于没有）" {
		t.Errorf("AAA = %v %q", cs[0].PriceNote, cs[0].EarningsText)
	}
	if bbb := bbbOf(t); bbb.PriceNote == nil || *bbb.PriceNote != "没有买入价" || bbb.BuyPriceNote == nil || *bbb.BuyPriceNote != "not judged" {
		t.Errorf("BBB = %+v", bbb)
	}
	c, _ := tierScenario("tiers_ok")
	rr := c.Candidates.(fakeReader)
	day := store.Candidate{}.NextEarnings
	_ = day
	rows := rr.rows["2026-11-09/short_put"]
	for i := range rows {
		if rows[i].UnderlyingID == 7 {
			d := buildDay
			rows[i].NextEarnings = &d
		}
	}
	got := buildTiersOf(t, c).Companies[0]
	if got.EarningsText != string(buildDay) || got.NextEarnings != string(buildDay) {
		t.Errorf("earnings = %q / %q", got.EarningsText, got.NextEarnings)
	}
}

func TestStripSourceBadge(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ marks, code, label string }{
		{`["buy_from_hand"]`, "hand", "手填"},
		{`["buy_from_valuation"]`, "valuation", "估值"},
	} {
		cs := stripsOf(t, func(rows []store.Candidate) {
			for i := range rows {
				if rows[i].UnderlyingID == 7 {
					rows[i].Marks = tc.marks
				}
			}
		})
		b := cs[0].SourceBadge
		if b == nil || b.Code != tc.code || b.Label != tc.label || b.Gloss == "" || b.Tone != "info" {
			t.Errorf("%s: source_badge = %+v", tc.code, b)
		}
		if old := cs[0].BuySource; old == nil || old.Code != tc.code {
			t.Errorf("%s: old buy_source = %+v", tc.code, old)
		}
	}
	none := bbbOf(t)
	if none.SourceBadge != nil {
		t.Errorf("none: source_badge = %+v", none.SourceBadge)
	}
	if none.BuySource == nil || none.BuySource.Code != "none" || none.BuySource.Label != "Not judged" {
		t.Errorf("none: old buy_source = %+v", none.BuySource)
	}
}

func TestStripLabelsChinese(t *testing.T) {
	t.Parallel()
	v := buildTiersOfScenario(t, "tiers_ok")
	want := stripLabels{
		Spot: "现价", BuyPrice: "买入价", NextEarnings: "下次财报", Contracts: "合约",
		Fits: "适合", Check: "待查", Grey: "折叠", GreyHint: "折叠 = 未判定",
	}
	if v.StripLabels != want {
		t.Errorf("strip_labels = %+v", v.StripLabels)
	}
	const note = "现价是任务为这一天运行时取到的价格，不是当天收盘价。“闸门”用的买入价就是上面的买入价：有估值买入区间就用它，没有再用手填价格。"
	if v.CardNote == nil || *v.CardNote != note {
		t.Errorf("card_note = %v", v.CardNote)
	}
	if v.SpotHint == nil || *v.SpotHint != "现价是任务为这一天运行时取到的价格，不是当天收盘价。" {
		t.Errorf("spot_hint = %v", v.SpotHint)
	}
}

// valueCompany is the first strip of tiers_ok with a figure of 91.20 for AAA and the marks of its stored rows.
func valueCompany(t *testing.T, marks string) *valueLineView {
	t.Helper()
	c, _ := valueConfig(valueState(valueEntry("AAA", valueReview())), &fakeBuy{price: "91.2"})
	rr := c.Candidates.(fakeReader)
	rows := rr.rows["2026-11-09/short_put"]
	for i := range rows {
		rows[i].Marks = marks
	}
	return companiesOfBuild(t, c)[0].Valuation
}

func TestValueRunStateUsed(t *testing.T) {
	t.Parallel()
	v := valueCompany(t, `["buy_from_valuation"]`)
	if v.RunState != "used" || v.LineLabel != "估值买入线" || v.LineText != "$91.20" || v.Note == nil || *v.Note != "复核于 2026-11-10，运行就用它" {
		t.Errorf("valuation = %+v", v)
	}
	if v.Text != "$91.20 (review 2026-11-10)" || v.Label != "Valuation buy line B_hi" {
		t.Errorf("old fields = %q %q", v.Text, v.Label)
	}
}

func TestValueRunStateNotUsed(t *testing.T) {
	t.Parallel()
	c, _ := valueConfig(valueState(valueEntry("BBB", valueReview())), &fakeBuy{price: "50"})
	var bbb *companyView
	cs := buildTiersOf(t, c).Companies
	for i := range cs {
		if cs[i].Ticker == "BBB" {
			bbb = &cs[i]
		}
	}
	if bbb == nil || bbb.Valuation == nil {
		t.Fatal("no BBB strip with a valuation")
	}
	v := bbb.Valuation
	if v.RunState != "not_used" || v.LineText != "$50.00" || v.Note == nil || *v.Note != "复核于 2026-11-10，最近一次运行没用到" {
		t.Errorf("valuation = %+v", v)
	}
	if !strings.Contains(*v.Note, "最近一次运行没用到") {
		t.Errorf("note = %q", *v.Note)
	}
	if hand := valueCompany(t, `["buy_from_hand"]`); hand.RunState != "not_used" {
		t.Errorf("a hand price is not the valuation: run_state = %q", hand.RunState)
	}
}

func TestValueRunStateNone(t *testing.T) {
	t.Parallel()
	c, _ := valueConfig(valueState(), &fakeBuy{price: "91.2"})
	v := companiesOfBuild(t, c)[0].Valuation
	if v.RunState != "none" || v.LineLabel != "估值买入线" || v.LineText != "无估值档案" || v.Note == nil ||
		*v.Note != "导入估值，或在“公司 > 关注列表”手填买入价" {
		t.Errorf("no_profile = %+v", v)
	}
	for _, reason := range []string{"removed", "no_review", "bad_input"} {
		c, _ := valueConfig(valueState(valueEntry("AAA", valueReview())), &fakeBuy{price: "91.2", reason: reason})
		got := companiesOfBuild(t, c)[0].Valuation
		if reasonOf(companyView{Valuation: got}) == "" || got.RunState != "none" || got.Note != nil || got.LineText != got.Text {
			t.Errorf("%s: valuation = %+v", reason, got)
		}
	}
}

func TestValueRunStateReadFailed(t *testing.T) {
	t.Parallel()
	c, fv := valueConfig(valueState(), &fakeBuy{price: "91.2"})
	fv.err = errors.New("db down")
	for _, cv := range companiesOfBuild(t, c) {
		v := cv.Valuation
		if reasonOf(cv) != "read_failed" || v.RunState != "none" || v.Note != nil || v.LineText != valueReasonReadFailedZh {
			t.Errorf("valuation = %+v", v)
		}
	}
}

func TestHeadLabels(t *testing.T) {
	t.Parallel()
	partial := buildPage(t, map[string][]store.Candidate{"2026-11-09/short_put": {cand("short_put", 1, "ok")}}, buildDay)
	all := buildPage(t, map[string][]store.Candidate{
		"2026-11-09/short_put": {cand("short_put", 1, "ok")}, "2026-11-09/covered_call": {cand("covered_call", 2, "ok")},
		"2026-11-09/held_call_guard": {cand("held_call_guard", 3, "ok")},
	}, buildDay)
	if partial.RunStatus != "partial" || all.RunStatus != "ok" {
		t.Fatalf("run status = %q, %q", partial.RunStatus, all.RunStatus)
	}
	for _, tc := range []struct {
		p    outPage
		text string
	}{{all, "完成"}, {partial, "部分完成"}} {
		if tc.p.HeadLabels != (headLabels{DataDay: "数据日", RunStatus: "运行状态", RunStatusText: tc.text, RuleVersion: "规则版本"}) {
			t.Errorf("status %q: head_labels = %+v", tc.p.RunStatus, tc.p.HeadLabels)
		}
	}
	if v := buildTiersOfScenario(t, "tiers_ok"); v.StoredTitle != "已存表格" {
		t.Errorf("stored_title = %q", v.StoredTitle)
	}
}

var asciiLetters = regexp.MustCompile(`[A-Za-z]+`)

// allowedWords are the option terms that stay as they are, glossed beside.
var allowedWords = map[string]bool{"Delta": true, "DGS3MO": true, "IV": true}

func checkNoEnglish(t *testing.T, what, s string) {
	t.Helper()
	for _, w := range asciiLetters.FindAllString(s, -1) {
		if !allowedWords[w] {
			t.Errorf("%s %q holds the English word %q", what, s, w)
		}
	}
}

func TestTierViewZhWords(t *testing.T) {
	t.Parallel()
	v := buildTiersOfScenario(t, "tiers_ok")
	p := buildPage(t, map[string][]store.Candidate{"2026-11-09/short_put": {cand("short_put", 1, "ok")}}, buildDay)
	exact := func(what, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %q, want %q", what, got, want)
		}
	}
	exact("page title", p.Title, "期权候选")
	exact("group put", p.Groups[0].Title, "先卖出看跌")
	exact("group call", groupTitles[kindCoveredCall], "备兑看涨")
	exact("group guard", groupTitles[kindHeldCallGuard], "持仓看涨保护")
	for i, want := range []string{"✓ 适合 2", "! 待查 1", "– 未判定 2", "账户现金"} {
		exact("legend title", v.Legend[i].Title, want)
	}
	for id, want := range map[string]string{"fits": "适合", "check": "待查", "grey_in_the_money": "未判定"} {
		exact("tier word "+id, tierByID(v, id).Word, want)
	}
	for id, want := range map[string]string{"fits": "适合 2", "check": "待查 1", "grey_in_the_money": "1 行：价内", "grey_no_buy_price": "1 行：没有买入价"} {
		exact("tier title "+id, tierByID(v, id).Title, want)
	}
	for i, want := range []string{"详情", "档位", "合约", "安全边际", "距现价", "权利金 / 年化", "徽标"} {
		exact("column", v.Columns[i].Label, want)
		if h := v.Columns[i].Hint; h != nil {
			head, _, _ := strings.Cut(*h, dayHint)
			if utf8.RuneCountInString(head) > 40 {
				t.Errorf("hint of %s has %d characters before the day", want, utf8.RuneCountInString(head))
			}
		}
	}
	for tail, want := range map[string]string{
		"no_spot": " 行：没有现价", "in_the_money": " 行：价内", "no_bid": " 行：没有买价",
		"above_buy_price": " 行：保本点高于买入价", "bad_input": " 行：数字读不出", "no_buy_price": " 行：没有买入价",
	} {
		exact("grey tail "+tail, greySpecs[tail].titleTail, want)
	}
	badges := map[string]string{
		"no_open_interest": "无未平仓量", "low_open_interest": "未平仓量低", "one_sided_quote": "单边报价", "wide_quote": "报价价差大",
		"stale_quote": "报价过时", "earnings_before_expiry": "到期前有财报", "cash_short": "现金不足", "delta_above_bound": "Delta 超上限",
		"delta_unknown": "Delta 未知", "thin_premium": "权利金薄", "earnings_unconfirmed": "财报日未确认",
		"earnings_beyond_horizon": "财报日在期限之外", "earnings_unknown": "财报日未知", "cash_unknown": "现金未知",
		"no_jpy_rate": "无日元现金利率", "no_jpy_funds": "无日元账户",
	}
	if len(badges) != len(badgeTexts) {
		t.Errorf("%d badge labels listed, %d known", len(badges), len(badgeTexts))
	}
	for code, want := range badges {
		exact("badge "+code, badgeOf(code, badgeToneNone, store.Candidate{}).Label, want)
		checkNoEnglish(t, "badge "+code, want)
	}
	for _, tr := range v.Tiers {
		checkNoEnglish(t, "tier word", tr.Word)
		checkNoEnglish(t, "tier title", tr.Title)
		for _, r := range tr.Rows[:min(len(tr.Rows), 1)] {
			for _, s := range []*string{r.Margin.Sub, r.BelowSpot.Sub, r.Premium.Sub} {
				if s != nil && asciiLetters.MatchString(*s) {
					t.Errorf("row sub text %q has an ASCII word", *s)
				}
			}
		}
	}
	for _, l := range v.Legend {
		checkNoEnglish(t, "legend title", l.Title)
	}
	for _, c := range v.Columns {
		checkNoEnglish(t, "column", c.Label)
	}
}

func TestShortListTextsUnchanged(t *testing.T) {
	t.Parallel()
	if tierGreyWord != "Not judged" || notJudged != "not judged" {
		t.Errorf("tierGreyWord = %q notJudged = %q", tierGreyWord, notJudged)
	}
	if b := buySourceBadge("none"); b == nil || b.Label != "Not judged" {
		t.Errorf("none badge = %+v", b)
	}
	if b := buySourceBadge("hand"); b == nil || b.Label != "Hand price" {
		t.Errorf("hand badge = %+v", b)
	}
	if b := buySourceBadge("valuation"); b == nil || b.Label != "Valuation buy zone" {
		t.Errorf("valuation badge = %+v", b)
	}
	l := okList(t)
	r := l.Groups[1].Rows[0]
	if r.Buy.Sub == nil || *r.Buy.Sub != "BE $84.58 vs $90.00, margin 6.0%" {
		t.Errorf("short list buy sub = %v", r.Buy.Sub)
	}
	if r.Expiry.Sub == nil || *r.Expiry.Sub != "39d" {
		t.Errorf("short list expiry sub = %v", r.Expiry.Sub)
	}
}
