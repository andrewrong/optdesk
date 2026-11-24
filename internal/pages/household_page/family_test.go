package household_page_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/pages/household_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeFamily answers with a fixed result and records the input it was given.
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

func fv(v string) compute.FigureValue { return compute.FigureValue{Value: v} }

func counted(alias, day, net, cash string) blockapi.FamilyAccountOut {
	return blockapi.FamilyAccountOut{Alias: alias, Day: day, Net: fv(net), Cash: fv(cash)}
}

func left(alias, reason, day string) blockapi.FamilyAccountOut {
	return blockapi.FamilyAccountOut{Alias: alias, Reason: reason, Day: day}
}

// famResult is a result of rows in input order: the counted ones are included, the others excluded.
func famResult(net, cash, share string, rows ...blockapi.FamilyAccountOut) blockapi.FamilyResult {
	r := blockapi.FamilyResult{Currency: "USD", Rows: rows, Net: fv(net), Cash: fv(cash), CashShare: fv(share)}
	for _, x := range rows {
		if x.Reason == "" {
			r.Included = append(r.Included, x.Alias)
			if r.OldestDay == "" || x.Day < r.OldestDay {
				r.OldestDay = x.Day
			}
		} else {
			r.Excluded = append(r.Excluded, x)
		}
	}
	return r
}

type famRig struct {
	c     household_page.Config
	in    *blockapi.FamilyIn
	calls *int
	acc   *fakeAccount
}

func rig(res blockapi.FamilyResult, aliases ...string) famRig {
	var figs blockapi.HouseholdInput
	acc := account()
	c := config(acc, &figs)
	c.Aliases = aliases
	r := famRig{c: c, in: &blockapi.FamilyIn{}, calls: new(int), acc: acc}
	c.Family = func() blockapi.FamilyFigure { return fakeFamily{res: res, got: r.in, calls: r.calls} }
	r.c = c
	return r
}

func (r famRig) build(t *testing.T) (map[string]any, []byte) {
	t.Helper()
	b, err := household_page.BuildFamily(context.Background(), r.c, day, at)
	if err != nil {
		t.Fatal(err)
	}
	return decode(t, b), b
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("not json: %s", b)
	}
	return m
}

func strs(v any) []string {
	var out []string
	for _, s := range v.([]any) {
		out = append(out, s.(string))
	}
	return out
}

func accountRows(m map[string]any) []map[string]any {
	var out []map[string]any
	for _, r := range m["accounts"].(map[string]any)["rows"].([]any) {
		out = append(out, r.(map[string]any))
	}
	return out
}

func has(lines []string, sub string) bool {
	return slices.ContainsFunc(lines, func(l string) bool { return strings.Contains(l, sub) })
}

func TestFamilyPageSumsAllAliases(t *testing.T) {
	t.Parallel()
	r := rig(famResult("105000.00", "43000.00", "40.95",
		counted("main", "2026-11-10", "100000.00", "42000.00"), counted("wife", "2026-11-10", "5000.00", "1000.00")), "main", "wife")
	m, b := r.build(t)
	if m["scope"] != "household" || m["run_status"] != "partial" || m["title"] != "家庭合计" {
		t.Fatalf("page head: %s", b)
	}
	scopes := m["scopes"].([]any)
	if len(scopes) != 1 || scopes[0].(map[string]any)["key"] != "household" || len(scopes[0].(map[string]any)["accounts"].([]any)) != 2 {
		t.Fatalf("scopes = %v", scopes)
	}
	if got := []string{r.in.Accounts[0].Alias, r.in.Accounts[1].Alias}; !slices.Equal(got, []string{"main", "wife"}) || r.in.Accounts[1].Day != "2026-11-10" {
		t.Fatalf("figure input = %+v", r.in)
	}
	ov := m["overview"].(map[string]any)
	rows := ov["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["name"] != "全家净值（计入 2 / 2 个账户）" {
		t.Fatalf("overview rows = %v", rows)
	}
	if tot := ov["totals"].([]any); len(tot) != 1 || tot[0].(map[string]any)["text"] != "$105,000.00" {
		t.Fatalf("totals = %v", ov["totals"])
	}
	ar := accountRows(m)
	if len(ar) != 2 || ar[0]["net_liquidation"].(map[string]any)["cell"].(map[string]any)["text"] != "$100,000.00" ||
		ar[1]["cash"].(map[string]any)["cell"].(map[string]any)["text"] != "$1,000.00" || ar[0]["whose"] != "本人" || ar[1]["whose"] != "其他账户" {
		t.Fatalf("account rows = %v", ar)
	}
	if cols := strs(m["accounts"].(map[string]any)["columns"]); !slices.Equal(cols, []string{"账户", "净值", "现金", "股票", "期权", "数据日"}) {
		t.Fatalf("columns = %v", cols)
	}
	checkDetail(t, m, ar)
	validate(t, b)
}

func checkDetail(t *testing.T, m map[string]any, ar []map[string]any) {
	t.Helper()
	for _, row := range ar {
		for _, k := range []string{"stock", "options"} {
			if f := row[k].(map[string]any); f["cell"] != nil || f["gap"] != "见单个账户" {
				t.Fatalf("%s = %v", k, f)
			}
		}
	}
	for _, blk := range []string{"composition", "movers", "trend", "stress", "income", "month_change", "cash_after",
		"obligations", "concentration", "allocation"} {
		if g := gapOf(t, m, blk); g["reason"] != "not_built" || !strings.Contains(g["text"].(string), "单个账户页") {
			t.Fatalf("%s gap = %v", blk, g)
		}
	}
	if len(m["gaps"].([]any)) != 10 || len(m["data_times"].([]any)) != 2 {
		t.Fatalf("gaps %v, data_times %v", m["gaps"], m["data_times"])
	}
}

func TestFamilyPageNamesLeftOutAccount(t *testing.T) {
	t.Parallel()
	r := rig(famResult("100000.00", "42000.00", "42",
		counted("main", "2026-11-10", "100000.00", "42000.00"), left("daughter", "no_data", "")), "main", "daughter")
	m, b := r.build(t)
	if r.in.Accounts[1].Reason != "no_data" {
		t.Fatalf("daughter passed as %+v", r.in.Accounts[1])
	}
	ar := accountRows(m)
	if len(ar) != 2 {
		t.Fatalf("rows = %v", ar)
	}
	for _, k := range []string{"net_liquidation", "cash"} {
		if f := ar[1][k].(map[string]any); f["cell"] != nil || f["gap"] != "未采集到" {
			t.Fatalf("%s = %v", k, f)
		}
	}
	if notes := strs(m["lead_notes"]); len(notes) != 1 || notes[0] != "账户（daughter）今天未采集到，合计不含" {
		t.Fatalf("lead_notes = %v", notes)
	}
	if g := m["gaps"].([]any)[0].(map[string]any); g["reason"] != "no_data" {
		t.Fatalf("gaps = %v", m["gaps"])
	}
	if rows := m["overview"].(map[string]any)["rows"].([]any); rows[0].(map[string]any)["name"] != "全家净值（计入 1 / 2 个账户）" {
		t.Fatalf("overview = %v", rows)
	}
	validate(t, b)
}

func TestFamilyPageOlderDayIsCountedAndSaid(t *testing.T) {
	t.Parallel()
	r := rig(famResult("105000.00", "43000.00", "40",
		counted("main", "2026-11-10", "100000.00", "42000.00"), counted("wife", "2026-11-09", "5000.00", "1000.00")), "main", "wife")
	r.acc.values["wife"] = map[string][]store.AccountValue{"2026-11-09": vals("2026-11-09", "5000.00", "1000.00")}
	m, b := r.build(t)
	if r.in.Accounts[1].Reason != "" || r.in.Accounts[1].Day != "2026-11-09" {
		t.Fatalf("wife passed as %+v", r.in.Accounts[1])
	}
	if notes := strs(m["lead_notes"]); len(notes) != 1 || notes[0] != "账户（wife）用的是 2026-11-09 的采集，早于最新交易日，已计入合计" {
		t.Fatalf("lead_notes = %v", notes)
	}
	if ar := accountRows(m); ar[1]["data_day"] != "2026-11-09" {
		t.Fatalf("wife row = %v", ar[1])
	}
	validate(t, b)
}

func TestFamilyPageStaleIsLeftOut(t *testing.T) {
	t.Parallel()
	r := rig(famResult("100000.00", "42000.00", "42",
		counted("main", "2026-11-10", "100000.00", "42000.00"), left("wife", "stale_input", "2026-11-04")), "main", "wife")
	r.acc.values["wife"] = map[string][]store.AccountValue{"2026-11-04": vals("2026-11-04", "5000.00", "1000.00")}
	m, b := r.build(t)
	if a := r.in.Accounts[1]; a.Reason != "stale_input" || a.Day != "2026-11-04" || len(a.Values) != 0 {
		t.Fatalf("wife passed as %+v", a)
	}
	if notes := strs(m["lead_notes"]); len(notes) != 1 || !strings.Contains(notes[0], "最近一次是 2026-11-04") {
		t.Fatalf("lead_notes = %v", notes)
	}
	if f := accountRows(m)[1]["net_liquidation"].(map[string]any); f["gap"] != "采集过旧" || f["cell"] != nil {
		t.Fatalf("net = %v", f)
	}
	validate(t, b)
}

func TestFamilyPageValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	out := []blockapi.FamilyAccountOut{left("main", "no_data", ""), left("wife", "read_error", "")}
	none := blockapi.FamilyResult{Currency: "USD", Net: compute.FigureValue{Gap: "no_data"}, Cash: compute.FigureValue{Gap: "no_data"},
		Rows: out, Excluded: out}
	two := famResult("105000.00", "43000.00", "40.95",
		counted("main", "2026-11-10", "100000.00", "42000.00"), counted("wife", "2026-11-10", "5000.00", "1000.00"))
	for name, res := range map[string]blockapi.FamilyResult{
		"two":  two,
		"none": none,
	} {
		_, b := rig(res, "main", "wife").build(t)
		validate(t, b)
		_ = name
	}
	r := rig(blockapi.FamilyResult{}, "main", "wife")
	r.c.Family = nil
	m, b := r.build(t)
	if m["run_status"] != "failed" || m["scope"] != "household" || gapOf(t, m, "overview")["reason"] != "no_reader" {
		t.Fatalf("no family figure: %s", b)
	}
	validate(t, b)
}

func TestViewsOnNamedAccountPage(t *testing.T) {
	t.Parallel()
	var got blockapi.HouseholdInput
	m, b := build(t, config(account(), &got), "wife")
	views := m["views"].([]any)
	if len(views) != 3 {
		t.Fatalf("views = %v", views)
	}
	want := []struct {
		key, href string
		cur       bool
	}{{"household", "/household", false}, {"main", "/household?account=main", false}, {"wife", "/household?account=wife", true}}
	for i, w := range want {
		v := views[i].(map[string]any)
		if v["key"] != w.key || v["href"] != w.href || v["current"] != w.cur {
			t.Fatalf("view %d = %v", i, v)
		}
	}
	validate(t, b)
	_, fb := rig(famResult("1", "1", "1", counted("main", "2026-11-10", "1", "1")), "main", "wife").build(t)
	if v := decode(t, fb)["views"].([]any)[0].(map[string]any); v["current"] != true {
		t.Fatalf("family view = %v", v)
	}
}

func TestOneAliasConfigHasNoViews(t *testing.T) {
	t.Parallel()
	var got blockapi.HouseholdInput
	c := config(account(), &got)
	c.Aliases = []string{"main"}
	m, _ := build(t, c, "main")
	if _, ok := m["views"]; ok {
		t.Fatalf("views = %v", m["views"])
	}
}

func TestFamilyPageOtherAliasReadErrorIsNamed(t *testing.T) {
	t.Parallel()
	r := rig(famResult("100000.00", "42000.00", "42",
		counted("main", "2026-11-10", "100000.00", "42000.00"), left("wife", "read_error", "")), "main", "wife")
	r.acc.failOn = "wife"
	m, b := r.build(t)
	if r.in.Accounts[1].Reason != "read_error" {
		t.Fatalf("wife passed as %+v", r.in.Accounts[1])
	}
	if f := accountRows(m)[1]["cash"].(map[string]any); f["gap"] != "读取失败" || f["cell"] != nil {
		t.Fatalf("cash = %v", f)
	}
	if notes := strs(m["lead_notes"]); len(notes) != 1 || notes[0] != "账户（wife）读取失败，合计不含" {
		t.Fatalf("lead_notes = %v", notes)
	}
	validate(t, b)
}

func TestFamilyPageFirstAliasErrorFails(t *testing.T) {
	t.Parallel()
	r := rig(blockapi.FamilyResult{}, "main", "wife")
	r.acc.failOn = "main"
	if _, err := household_page.BuildFamily(context.Background(), r.c, day, at); err == nil {
		t.Fatal("a reader error of the first alias must be an error")
	}
	if *r.calls != 0 {
		t.Fatal("the figure was called after the error")
	}
}

func TestFamilyPageStatesCountAndRate(t *testing.T) {
	t.Parallel()
	res := famResult("100000.00", "42000.00", "42", counted("main", "2026-11-10", "100000.00", "42000.00"),
		left("wife", "no_data", ""), left("daughter", "no_data", ""))
	res.FxRate, res.FxDay, res.FxSource = "150", "2026-11-09", "boj"
	r := rig(res, "main", "wife", "daughter")
	fx := &fakeFx{rate: "150"}
	r.c.Fx = fx
	m, _ := r.build(t)
	if len(fx.calls) != 0 {
		t.Fatalf("a book of USD captures read the rate: %v", fx.calls)
	}
	if !has(strs(m["lead_notes"]), "USDJPY 汇率 150（日期 2026-11-09，来源 boj）") {
		t.Fatalf("lead_notes = %v", m["lead_notes"])
	}
	if rows := m["overview"].(map[string]any)["rows"].([]any); rows[0].(map[string]any)["name"] != "全家净值（计入 1 / 3 个账户）" {
		t.Fatalf("overview = %v", rows)
	}
	jpy := func() []store.AccountValue {
		v := vals("2026-11-10", "15000000", "3000000")
		for i := range v {
			v[i].Currency = "JPY"
		}
		return v
	}
	r.acc.values["wife"] = map[string][]store.AccountValue{"2026-11-10": jpy()}
	r.build(t)
	if !slices.Equal(fx.calls, []string{"USDJPY 2026-11-10"}) {
		t.Fatalf("rate reads = %v", fx.calls)
	}
	if in := r.in; in.UsdJpy != "150" || in.RateDay != "2026-11-09" || in.RateSource != "" {
		t.Fatalf("rate passed = %q %q %q", in.UsdJpy, in.RateDay, in.RateSource)
	}
}

func TestFamilyPageCashNote(t *testing.T) {
	t.Parallel()
	r := rig(famResult("105000.00", "43000.00", "40.95",
		counted("main", "2026-11-10", "100000.00", "42000.00"), counted("wife", "2026-11-10", "5000.00", "1000.00")), "main", "wife")
	r.c.Statement = []string{"wife"}
	m, _ := r.build(t)
	notes := strs(m["notes"])
	if notes[0] != "现金分放在各账户，只担保本账户的卖出看跌；合计现金不是可共用的保证金" || len(notes) != 2 || !strings.Contains(notes[1], "2026-11-10") {
		t.Fatalf("notes = %v", notes)
	}
	row := m["overview"].(map[string]any)["rows"].([]any)[0].(map[string]any)
	if row["note"] != "现金 $43,000.00，占合计净值 41.0%" {
		t.Fatalf("cash line = %v", row["note"])
	}
}

func TestFamilyPageRepeatedAliasCountsOnce(t *testing.T) {
	t.Parallel()
	r := rig(famResult("105000.00", "43000.00", "40.95",
		counted("main", "2026-11-10", "100000.00", "42000.00"), counted("wife", "2026-11-10", "5000.00", "1000.00")), "main", "wife", "main")
	m, b := r.build(t)
	if len(r.in.Accounts) != 2 || len(accountRows(m)) != 2 {
		t.Fatalf("figure input %+v, rows %v", r.in.Accounts, accountRows(m))
	}
	if tags := m["scopes"].([]any)[0].(map[string]any)["accounts"].([]any); len(tags) != 2 {
		t.Fatalf("scope accounts = %v", tags)
	}
	validate(t, b)
}

func TestFamilyPageAliasNamedHouseholdIsLeftOut(t *testing.T) {
	t.Parallel()
	r := rig(famResult("100000.00", "42000.00", "42",
		counted("main", "2026-11-10", "100000.00", "42000.00"), left("household", "read_error", "")), "main", "household")
	m, b := r.build(t)
	if slices.Contains(r.acc.asked, "household") {
		t.Fatalf("the alias household was read: %v", r.acc.asked)
	}
	if a := r.in.Accounts[1]; a.Alias != "household" || a.Reason != "read_error" {
		t.Fatalf("passed as %+v", a)
	}
	if !has(strs(m["lead_notes"]), "账户（household）读取失败") {
		t.Fatalf("lead_notes = %v", m["lead_notes"])
	}
	validate(t, b)
}

func TestFamilyPageOneAliasIsNamedPage(t *testing.T) {
	t.Parallel()
	r := rig(blockapi.FamilyResult{}, "main")
	_, b := r.build(t)
	var got blockapi.HouseholdInput
	c := config(account(), &got)
	c.Aliases = []string{"main"}
	_, want := build(t, c, "")
	if string(b) != string(want) || *r.calls != 0 {
		t.Fatalf("one-alias family page differs from Build:\n%s\n%s", b, want)
	}
}

func TestFamilyViewsOnePerDistinctAlias(t *testing.T) {
	t.Parallel()
	r := rig(famResult("100000.00", "42000.00", "42",
		counted("main", "2026-11-10", "100000.00", "42000.00"), left("household", "read_error", "")), "main", "household", "main")
	m, _ := r.build(t)
	cur := 0
	for _, v := range m["views"].([]any) {
		if v.(map[string]any)["current"] == true {
			cur++
		}
	}
	if n := len(m["views"].([]any)); n != 2 || cur != 1 {
		t.Fatalf("views = %v", m["views"])
	}
}

func TestFamilyPageRepeatedOnlyAliasIsNamedPage(t *testing.T) {
	t.Parallel()
	r := rig(blockapi.FamilyResult{}, "main", "main")
	m, _ := r.build(t)
	if m["scope"] != "main" || *r.calls != 0 {
		t.Fatalf("scope = %v", m["scope"])
	}
	if _, ok := m["views"]; ok {
		t.Fatalf("views = %v", m["views"])
	}
}

func TestFamilyAccountRowsUseFamilyScope(t *testing.T) {
	t.Parallel()
	r := rig(famResult("105000.00", "43000.00", "40.95",
		counted("main", "2026-11-10", "100000.00", "42000.00"), counted("wife", "2026-11-10", "5000.00", "1000.00")), "main", "wife")
	m, b := r.build(t)
	keys := map[any]bool{}
	for _, s := range m["scopes"].([]any) {
		keys[s.(map[string]any)["key"]] = true
	}
	ar := accountRows(m)
	if len(ar) != 2 {
		t.Fatalf("account rows = %s", b)
	}
	for _, row := range ar {
		if row["scope"] != "household" || !keys[row["scope"]] {
			t.Fatalf("row scope = %v, scope keys = %v", row["scope"], keys)
		}
	}
}
