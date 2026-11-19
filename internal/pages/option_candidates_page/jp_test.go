package option_candidates_page

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/compute/optionshortlistapi"
	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/candidatecur"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// jpNow is 09:10 JST on 2026-11-19, the fake clock of the Japanese tests; the US build day is the day before.
var jpNow = time.Date(2026, 11, 19, 0, 10, 0, 0, time.UTC)

const (
	jpDay = "2026-11-19"
	usDay = contracts.Day("2026-11-18")
)

// fakeCur holds the yen rows by "day/kind/currency" and records the days it was asked for.
type fakeCur struct {
	mu    sync.Mutex
	rows  map[string][]candidatecur.Row
	asked []string
}

func (f *fakeCur) CandidatesIn(_ context.Context, day contracts.Day, kind, currency string) ([]candidatecur.Row, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := string(day) + "/" + kind + "/" + currency
	f.asked = append(f.asked, key)
	rows := f.rows[key]
	if rows == nil {
		rows = []candidatecur.Row{}
	}
	return rows, nil
}

func loadJPRows(t *testing.T) []candidatecur.Row {
	t.Helper()
	b, err := os.ReadFile("testdata/jp_rows.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []candidatecur.Row
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

// jpConfig is a page of no US rows and the yen rows on jpDay (every row when ids is empty).
func jpConfig(t *testing.T, ids ...int) (Config, *fakeCur) {
	t.Helper()
	var rows []candidatecur.Row
	for _, r := range loadJPRows(t) {
		if len(ids) == 0 {
			rows = append(rows, r)
			continue
		}
		for _, id := range ids {
			if r.InstrumentID == id {
				rows = append(rows, r)
			}
		}
	}
	cur := &fakeCur{rows: map[string][]candidatecur.Row{jpDay + "/short_put/JPY": rows}}
	c := cfg(fakeReader{})
	c.CandCur = cur
	return c, cur
}

type jpOut struct {
	Groups []struct {
		outGroup
		Currency    *string      `json:"currency"`
		Note        *string      `json:"note"`
		LimitCounts []limitCount `json:"limit_counts"`
	} `json:"groups"`
	Tiers    *tiersView     `json:"short_put_tiers"`
	TiersJPY *tiersView     `json:"short_put_tiers_jpy"`
	List     *shortListView `json:"short_put_list"`
	ListJPY  *shortListView `json:"short_put_list_jpy"`
}

func buildJP(t *testing.T, c Config) (jpOut, []byte) {
	t.Helper()
	b, err := Build(context.Background(), c, nil, usDay, jpNow)
	if err != nil {
		t.Fatal(err)
	}
	var p jpOut
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p, b
}

// TestJPGroupTokyoDay: at 09:10 JST on D the US build day is still D-1; the yen group is read from the Tokyo
// date of the build time and finds the rows of D.
func TestJPGroupTokyoDay(t *testing.T) {
	t.Parallel()
	c, cur := jpConfig(t)
	p, _ := buildJP(t, c)
	if len(p.Groups) != 4 {
		t.Fatalf("groups = %d, want 4", len(p.Groups))
	}
	jp := p.Groups[1]
	if jp.Kind != "short_put" || jp.Day == nil || *jp.Day != jpDay || jp.Stale || jp.Main == nil {
		t.Fatalf("yen group = %+v", jp)
	}
	if cur.asked[0] != jpDay+"/short_put/JPY" {
		t.Errorf("first read = %q, want the Tokyo day", cur.asked[0])
	}
	if us := p.Groups[0]; us.Gap == nil || us.Currency != nil || us.Note != nil {
		t.Errorf("US short-put group = %+v", us)
	}
}

// TestJPGroupFields: the yen group says its currency and note; the US groups say neither; a nil reader gives
// no yen group.
func TestJPGroupFields(t *testing.T) {
	t.Parallel()
	c, _ := jpConfig(t)
	p, b := buildJP(t, c)
	jp := p.Groups[1]
	if jp.Currency == nil || *jp.Currency != "JPY" || jp.Note == nil || *jp.Note != jpGroupNoteZh {
		t.Errorf("currency/note = %v / %v", jp.Currency, jp.Note)
	}
	if !strings.Contains(*jp.Note, "东证开盘行情") || strings.Contains(string(b), "later design") {
		t.Errorf("note %q", *jp.Note)
	}
	c.CandCur = nil
	p, b = buildJP(t, c)
	if len(p.Groups) != 3 || bytes.Contains(b, []byte(`"currency"`)) || bytes.Contains(b, []byte(`"limit_counts"`)) ||
		bytes.Contains(b, []byte("_jpy")) {
		t.Errorf("page without a yen reader: %d groups, %s", len(p.Groups), b)
	}
}

// TestJPYTexts: the yen rows print the yen sign and no decimals everywhere, never a dollar sign.
func TestJPYTexts(t *testing.T) {
	t.Parallel()
	c, _ := jpConfig(t)
	c.Names = &namesFake{names: map[int]string{21: "7203", 22: "6954", 23: "6758"}}
	c.Figures = &tierFigs{}
	c.Tiers = &fakeTierer{fn: func(in optiontierapi.Input) optiontierapi.Result {
		vs := make([]optiontierapi.Verdict, len(in.Rows))
		for i, r := range in.Rows {
			vs[i] = checkVerdict(r.Key, "no_jpy_rate")
			vs[i].BreakEven, vs[i].PerContract, vs[i].Cash = r.Strike, "3500", nil
		}
		return resultOf(vs...)
	}}
	_, b := buildJP(t, c)
	var p jpOut
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	row := p.Groups[1].Main.Rows[0]
	for id, want := range map[string]string{"strike": "¥2,900", "bid": "¥35", "break_even": "¥2,865"} {
		if got := row.Cells[id].Text; got != want {
			t.Errorf("%s = %q, want %q", id, got, want)
		}
	}
	jpTiers, _ := json.Marshal(p.TiersJPY)
	jpGroup, _ := json.Marshal(p.Groups[1])
	for name, text := range map[string]string{"group": string(jpGroup), "tiers": string(jpTiers)} {
		if strings.Contains(text, "$") {
			t.Errorf("%s of the yen rows prints a dollar sign: %s", name, text)
		}
	}
	if got := p.TiersJPY.Tiers[1].Rows[0].Premium.Main; got != "¥3,500" {
		t.Errorf("premium = %q", got)
	}
}

// TestJPSecuredAmount: the cash that secures a yen put is printed with the multiplier used.
func TestJPSecuredAmount(t *testing.T) {
	t.Parallel()
	c, _ := jpConfig(t, 1)
	p, _ := buildJP(t, c)
	if got := p.Groups[1].Main.Rows[0].Cells["cash_secured"].Text; got != "¥1,790,000 = ¥17,900 x 100" {
		t.Errorf("cash_secured = %q", got)
	}
	bad := loadJPRows(t)[1]
	bad.Multiplier = 0
	if g, ok := securedCell(bad.Candidate).(gap); !ok || g.Text != jpNoMultiplierZh {
		t.Errorf("no multiplier = %+v", securedCell(bad.Candidate))
	}
}

// markTierer judges every row of the input the way the rule of optiontier does for the yen marks: a row with
// no_jpy_rate or no_jpy_funds is check, never fits.
func markTierer() *fakeTierer {
	return &fakeTierer{fn: func(in optiontierapi.Input) optiontierapi.Result {
		vs := make([]optiontierapi.Verdict, len(in.Rows))
		for i, r := range in.Rows {
			vs[i] = fitsVerdict(r.Key)
			for _, m := range r.Marks {
				if m == "no_jpy_rate" || m == "no_jpy_funds" {
					vs[i] = checkVerdict(r.Key, m)
				}
			}
		}
		res := resultOf(vs...)
		res.Companies = []optiontierapi.CompanyCount{{Company: "21", Check: len(vs)}}
		return res
	}}
}

// TestJPMarkNeverFits: the yen rows go to the judge with their currency and marks, and a row with a mark of
// Decision 14 is never in the tier fits; the US tier view holds no yen row.
func TestJPMarkNeverFits(t *testing.T) {
	t.Parallel()
	c, _ := jpConfig(t, 0, 1)
	judge := markTierer()
	c.Tiers = judge
	p, _ := buildJP(t, c)
	if judge.calls() != 1 {
		t.Fatalf("Judge called %d times", judge.calls())
	}
	for _, r := range judge.in[0].Rows {
		if r.Currency != "JPY" {
			t.Errorf("row %s currency %q", r.Key, r.Currency)
		}
	}
	if p.Tiers != nil || p.TiersJPY == nil {
		t.Fatalf("tiers = %v, jpy = %v", p.Tiers, p.TiersJPY)
	}
	if fits := tierByID(p.TiersJPY, "fits"); fits == nil || len(fits.Rows) != 0 {
		t.Errorf("fits = %+v", fits)
	}
	check := tierByID(p.TiersJPY, "check")
	if check == nil || len(check.Rows) != 2 {
		t.Fatalf("check = %+v", check)
	}
	b := check.Rows[0].Badges[0]
	if b.Code != "no_jpy_rate" && b.Code != "no_jpy_funds" || b.Label == b.Code || !strings.Contains(b.Gloss, "日元") {
		t.Errorf("badge = %+v", b)
	}
}

// TestJPMarkHints: each mark of Decision 16 carries its Chinese hint in the marks cell and none is bare.
func TestJPMarkHints(t *testing.T) {
	t.Parallel()
	c, _ := jpConfig(t, 0)
	p, _ := buildJP(t, c)
	badges := p.Groups[1].Main.Rows[0].Cells["marks"].Badges
	if len(badges) != 2 || badges[0] != "no_jpy_rate "+markHintNoJPYRateZh || badges[1] != "no_jpy_funds "+markHintNoJPYFundsZh {
		t.Errorf("badges = %q", badges)
	}
}

// TestJPEvidenceNotComputed: the evidence cells of a yen row say "not computed" in Chinese and carry no number;
// the evaluator is never asked; the figures need the US rate and are the same gap.
func TestJPEvidenceNotComputed(t *testing.T) {
	t.Parallel()
	c, _ := jpConfig(t, 0)
	ev := &fakeEval{fn: func(optionevidenceapi.Row) (optionevidenceapi.Evidence, error) { return fullEvidence(), nil }}
	figs := &tierFigs{}
	c.Evidence, c.Figures, c.Tiers = ev, figs, markTierer()
	p, b := buildJP(t, c)
	if ev.calls() != 0 || len(figs.got) != 0 {
		t.Errorf("evaluator calls %d, figure calls %d", ev.calls(), len(figs.got))
	}
	row := p.Groups[1].Main.Rows[0]
	for _, id := range append([]string{"itm", "excess"}, evidenceIDs...) {
		cell := row.Cells[id]
		if cell.Kind != "gap" || cell.Text != jpNotComputedZh || cell.Value != nil {
			t.Errorf("%s = %+v", id, cell)
		}
	}
	for _, f := range p.TiersJPY.Tiers[1].Rows[0].Fields {
		switch f.ID {
		case "itm_probability", "vs_cash", "below_buy", "one_sigma", "bid_vs_forecast", "skew", "forecast_vol", "ivp", "vol_edge", "labels":
			if f.Gap == nil || f.Gap.Text != jpNotComputedZh || f.Value != nil {
				t.Errorf("field %s = %+v", f.ID, f)
			}
		}
	}
	if bytes.Contains(b, []byte("21.8%")) || bytes.Contains(b, []byte("89.06")) {
		t.Error("a US figure is printed")
	}
}

// TestJPLimitCountsAllBlocked: a run whose rows are all blocked still shows its counts, one entry per limit.
func TestJPLimitCountsAllBlocked(t *testing.T) {
	t.Parallel()
	c, _ := jpConfig(t, 2, 3, 4, 5)
	p, _ := buildJP(t, c)
	jp := p.Groups[1]
	if jp.Gap != nil || jp.Main != nil || jp.Other == nil || len(jp.Other.Rows) != 4 || jp.Counts.Blocked != 4 {
		t.Fatalf("group = %+v", jp)
	}
	want := []limitCount{
		{"wide_spread", 2, "价差超限，剔除 2 行"}, {"low_open_interest", 1, "未平仓量不足，剔除 1 行"},
		{"small_bid_size", 1, "bid 数量太少，剔除 1 行"}, {"no_bid", 1, "没有 bid，剔除 1 行"}, {"no_quote", 1, "没有报价，剔除 1 行"},
	}
	if len(jp.LimitCounts) != len(want) {
		t.Fatalf("limit_counts = %+v", jp.LimitCounts)
	}
	for i, w := range want {
		if jp.LimitCounts[i] != w {
			t.Errorf("limit_counts[%d] = %+v, want %+v", i, jp.LimitCounts[i], w)
		}
	}
}

// TestJPStripPassesJPY: the valuation strip of a yen company asks the resolver in yen.
func TestJPStripPassesJPY(t *testing.T) {
	t.Parallel()
	c, _ := jpConfig(t, 0)
	c.Names = &namesFake{names: map[int]string{21: "Toyota"}}
	c.Tiers = markTierer()
	entry := valueEntry("Toyota", valueReview())
	entry.Profile.Currency = "JPY"
	buy := &fakeBuy{price: "2950"}
	c.Valuation, c.Buy = &fakeValuation{st: valueState(entry)}, buy
	p, _ := buildJP(t, c)
	if len(buy.in) == 0 || buy.in[0].Currency != "JPY" {
		t.Fatalf("resolver inputs = %+v", buy.in)
	}
	if got := p.TiersJPY.Companies[0].Valuation.Text; got != "¥2,950 (review 2026-11-10)" {
		t.Errorf("strip = %q", got)
	}
}

// TestJPListInYen: the yen short list is built from the yen rows and the US one holds none.
func TestJPListInYen(t *testing.T) {
	t.Parallel()
	c, _ := jpConfig(t, 0)
	c.Tiers = markTierer()
	c.Shortlist = &fakeShortlister{res: shortJPResult()}
	p, _ := buildJP(t, c)
	if p.List != nil || p.ListJPY == nil || len(p.ListJPY.Groups) != 1 {
		t.Fatalf("list = %v, jpy = %+v", p.List, p.ListJPY)
	}
	r := p.ListJPY.Groups[0].Rows[0]
	if p.ListJPY.Groups[0].Spot != "¥3,100" || r.Strike.Main != "¥2,900" || r.Premium.Main != "¥35" {
		t.Errorf("group = %+v", p.ListJPY.Groups[0])
	}
}

// TestJPGolden: the yen cases as one document.
func TestJPGolden(t *testing.T) {
	t.Parallel()
	c, _ := jpConfig(t)
	c.Names = &namesFake{names: map[int]string{21: "7203", 22: "6954", 23: "6758"}}
	c.Tiers = markTierer()
	c.Shortlist = &fakeShortlister{res: shortJPResult()}
	_, b := buildJP(t, c)
	var full map[string]json.RawMessage
	if err := json.Unmarshal(b, &full); err != nil {
		t.Fatal(err)
	}
	var groups []json.RawMessage
	if err := json.Unmarshal(full["groups"], &groups); err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(map[string]json.RawMessage{
		"group": groups[1], "short_put_tiers_jpy": full["short_put_tiers_jpy"], "short_put_list_jpy": full["short_put_list_jpy"],
	})
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/jp_rows.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(compact(t, want), compact(t, got)) {
		t.Errorf("the yen page differs from testdata/jp_rows.golden.json")
	}
}

var _ = store.Candidate{}
var _ = valuation.State{}

// shortJPResult places the first yen row (position 0, Toyota) in one group.
func shortJPResult() optionshortlistapi.Result {
	return optionshortlistapi.Result{
		Groups: []optionshortlistapi.Group{{Company: "21", Best: "0.1", Rows: []optionshortlistapi.Placed{
			{Key: tierKeyHead + "0-0", OTM: "6.4516", AbsDelta: "0.19"}}}},
		Dropped: []optionshortlistapi.Drop{}, Scanned: 1, Shown: 1, DTEMin: "21", DTEMax: "45", Bounds: testBounds,
	}
}
