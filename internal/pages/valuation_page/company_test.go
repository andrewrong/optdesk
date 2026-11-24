package valuation_page

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

func companyInput(calc *fakeCalc, e valuation.Entry) Input {
	in := baseInput(calc)
	in.State.Entries = []valuation.Entry{e}
	in.Company = e.Profile.CompanyKey
	return in
}

func obj(v any) map[string]any { return v.(map[string]any) }

func TestCompanyLinesFromCalculator(t *testing.T) {
	t.Parallel()
	calc := newFake()
	cr := &fakeCloses{ids: map[string]int64{"AAA": 3}, bars: []contracts.PriceBar{bar("2026-11-14", "72")}}
	in := companyInput(calc, entryWith("US:AAA", "AAA", "US", "100", "3"))
	in.Closes = cr
	b, err := BuildCompany(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	m := validate(t, b)
	l := obj(m["lines"])
	// the fake's text, only RoundText applied (it adds ".00" to a whole number)
	for k, want := range map[string]string{"c": "90.00", "r": "0.9", "heavy": "63.00", "buy_low": "69.3", "buy_high": "73.8", "fair_low": "83.7",
		"fair_high": "96.3", "trim": "109.8", "clear": "120.00", "core": "150.00", "m": "0.18", "s": "0.22", "zone": "buy", "zone_note": ""} {
		if l[k] != want {
			t.Errorf("lines.%s = %v, want %q", k, l[k], want)
		}
	}
	if obj(l["flags"])["in_buy"] != true || obj(m["close"])["value"] != "72.00" || obj(m["close"])["day"] != "2026-11-14" {
		t.Errorf("flags/close = %v %v", l["flags"], m["close"])
	}
	if obj(m["current"])["v"] != "100" || obj(m["company"])["r_market"] != "US" {
		t.Errorf("current/company = %v %v", m["current"], m["company"])
	}
}

func TestHKCNNoZone(t *testing.T) {
	t.Parallel()
	calc := newFake()
	calc.lines.Zone, calc.lines.Flags, calc.lines.ZoneNote = "", nil, ""
	in := companyInput(calc, entryWith("HK:ZZZ", "", "HKCN", "100", "3"))
	b, err := BuildCompany(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	m := validate(t, b)
	l := obj(m["lines"])
	if l["zone"] != "" || l["flags"] != nil || l["c"] != "90.00" || m["close"] != nil {
		t.Errorf("lines = %v close = %v", l, m["close"])
	}
}

func TestBandFromCalculator(t *testing.T) {
	t.Parallel()
	calc := newFake()
	in := companyInput(calc, entryWith("US:AAA", "AAA", "US", "100", "3"))
	b, err := BuildCompany(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	band := obj(validate(t, b)["band"])
	marks := band["marks"].([]any)
	if len(marks) != len(calc.band.Marks) {
		t.Fatalf("marks = %v", marks)
	}
	for i, want := range calc.band.Marks {
		got := obj(marks[i])
		// positions and names are the calculator's text; values differ only by RoundText
		if got["name"] != want.Name || got["pos"] != want.Pos || got["value"] != want.Value+".00" {
			t.Errorf("mark %d = %v, want %+v", i, got, want)
		}
	}
	if band["min"] != "50.00" || band["max"] != "160.00" {
		t.Errorf("band = %v", band)
	}
}

func rev(id, status, v string, entered string) valuation.Review {
	t, _ := time.Parse(time.DateOnly, entered)
	return valuation.Review{
		ID: 1, ReviewID: id, CompanyKey: "US:AAA", Type: "post_earnings", VersionDate: contracts.Day(entered), Status: status,
		Source: "owner", V: v, P: "3", Trigger: "a", ChangeMemo: "memo " + id, ReportText: "report " + id, EnteredAt: t,
	}
}

func TestHistoryRows(t *testing.T) {
	t.Parallel()
	calc := newFake()
	e := entryWith("US:AAA", "AAA", "US", "88", "3")
	in := companyInput(calc, e)
	void := rev("r2", "void", "88", "2026-11-10")
	void.VoidReason = "typed the wrong V"
	in.Reviews = []valuation.Review{void, rev("r2", "effective", "88", "2026-11-02"), rev("r1", "effective", "100", "2026-10-01")}
	b, err := BuildCompany(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	h := validate(t, b)["history"].([]any)
	if len(h) != 3 {
		t.Fatalf("history = %d rows", len(h))
	}
	void0, eff, first := obj(h[0]), obj(h[1]), obj(h[2])
	if void0["status"] != "void" || void0["void_reason"] != "typed the wrong V" || void0["date"] != "2026-11-10" {
		t.Errorf("void row = %v", void0)
	}
	if eff["v_from"] != "100" || eff["v_to"] != "88" || eff["change_percent"] != "-12.00" || eff["status"] != "effective" ||
		eff["trigger"] != "a" || eff["memo"] != "memo r2" {
		t.Errorf("effective row = %v", eff)
	}
	if first["v_from"] != "" || first["v_to"] != "100" || first["change_percent"] != "" {
		t.Errorf("first row = %v", first)
	}
	if obj(eff["detail"])["report_text"] != "report r2" {
		t.Errorf("detail = %v", eff["detail"])
	}
	if void0["v_from"] != "100" || void0["change_percent"] != "-12.00" {
		t.Errorf("void row repeats the voided row's change: %v", void0)
	}
}

func TestNoCurrentReviewShowsDrafts(t *testing.T) {
	t.Parallel()
	calc := newFake()
	e := entryWith("US:AAA", "AAA", "US", "", "")
	e.Current = nil
	in := companyInput(calc, e)
	draft := rev("r1", "draft", "", "2026-11-05")
	draft.Type, draft.VersionDate = "", ""
	in.Reviews = []valuation.Review{draft}
	b, err := BuildCompany(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	m := validate(t, b)
	if m["lines"] != nil || m["band"] != nil || m["current"] != nil || len(m["history"].([]any)) != 1 || calc.callCount() != 0 {
		t.Errorf("doc = %v", m)
	}
	if obj(m["history"].([]any)[0])["date"] != "2026-11-05" {
		t.Errorf("draft date = %v", m["history"])
	}
}

func TestUnknownCompanyIsGap(t *testing.T) {
	t.Parallel()
	in := baseInput(newFake())
	in.Company = "US:NOPE"
	b, err := BuildCompany(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	m := validate(t, b)
	if m["company"] != nil || len(m["gaps"].([]any)) != 1 {
		t.Errorf("doc = %v", m)
	}
}

func TestBadLinesLeaveNoLines(t *testing.T) {
	t.Parallel()
	calc := newFake()
	calc.linesErr = valuelinesapi.ErrBadInput
	b, err := BuildCompany(context.Background(), companyInput(calc, entryWith("US:AAA", "AAA", "US", "0", "3")))
	if err != nil {
		t.Fatal(err)
	}
	if m := validate(t, b); m["lines"] != nil || m["current"] == nil {
		t.Errorf("doc = %v", m)
	}
}

func TestReportTextPathHidden(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ in, want string }{
		"path":      {"claude/个股报告/GOOG-Alphabet-2026-11-03.md", ""},
		"plain":     {"report r2", "report r2"},
		"two lines": {"notes\nclaude/a.md", "notes\nclaude/a.md"},
	}
	for name, c := range cases {
		if got := reportTextOf(c.in); got != c.want {
			t.Errorf("%s: reportTextOf(%q) = %q, want %q", name, c.in, got, c.want)
		}
	}
}
