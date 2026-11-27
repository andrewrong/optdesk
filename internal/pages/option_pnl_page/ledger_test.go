package option_pnl_page

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// allBooker is sumBooker with the two fields of the compute rule that sumBooker does not fill.
type allBooker struct{ sumBooker }

func (b *allBooker) Combine(rs []optionattrapi.Result) []optionattrapi.Book {
	books := b.sumBooker.Combine(rs)
	for i := range books {
		bk := &books[i]
		bk.AllLots = bk.Lots + bk.NoQuoteLots + bk.UnreconciledLots
		n := func(s string) decimal.Decimal { v, _ := decimal.NewFromString(s); return v }
		bk.LedgerAllPnL = n(bk.LedgerPnL).Add(n(bk.NoQuoteLedgerPnL)).Add(n(bk.UnreconciledLedgerPnL)).String()
	}
	return books
}

func ledgerRig() *rig {
	r := newRig()
	b := &allBooker{}
	r.cfg.Booker, r.book = b, &b.sumBooker
	return r
}

// splitResult is a result that adds up: ledger 50.
func splitResult() optionattrapi.Result { return res("20", "-8", "4", "2", "-2", "", "-2", "0", "14") }

// closed adds an ended lot of company und closed on closedOn; strike keys the result.
func (r *rig) closed(id, und, strike, closedOn string) compute.LedgerPosition {
	p := lotPos(id, und, "c"+id, strike, "closed")
	p.ClosedOn = contracts.Day(closedOn)
	r.replay.positions = append(r.replay.positions, p)
	if _, ok := r.attr.results[strike]; !ok {
		r.attr.results[strike] = splitResult()
	}
	return p
}

func (r *rig) ledger(t *testing.T) *ledgerView {
	t.Helper()
	l := r.build(t).Accounts[0].Ledger
	if l == nil {
		t.Fatal("ledger is null")
	}
	return l
}

func (l *ledgerView) view(id string) ledgerTab {
	for _, v := range l.Views {
		if v.ID == id {
			return v
		}
	}
	return ledgerTab{}
}

func (b bookView) line(label string) (line, bool) {
	for _, l := range b.Lines {
		if l.Label == label {
			return l, true
		}
	}
	return line{}, false
}

func groupLabels(v ledgerTab) []string {
	out := []string{}
	for _, g := range v.Groups {
		out = append(out, g.Label)
	}
	return out
}

func TestLedgerByMonthGroups(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.closed("p1", "AAA", "38.00", "2026-11-13")
	r.closed("p2", "BBB", "38.00", "2026-11-05")
	r.closed("p3", "AAA", "38.00", "2026-09-10")
	m := r.ledger(t).view("by_month")
	if got := strings.Join(groupLabels(m), ","); got != "2026-11,2026-09" {
		t.Fatalf("groups = %s", got)
	}
	if c := m.Groups[0].Books[0].Counts; c != "USD, 2 split lots of 2 ended" {
		t.Errorf("counts = %q", c)
	}
	if tm, _ := m.Groups[0].Books[0].line(lblTimeZh); tm.Text != "$40.00 · 2 lots" {
		t.Errorf("time = %q", tm.Text)
	}
}

func TestLedgerByCompanyGroups(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.closed("p1", "AAA", "38.00", "2026-09-10")
	r.closed("p2", "AAA", "38.00", "2026-11-13")
	c := r.ledger(t).view("by_company")
	if len(c.Groups) != 1 || c.Groups[0].Label != "AAA" || c.Groups[0].Books[0].Counts != "USD, 2 split lots of 2 ended" {
		t.Fatalf("groups = %+v", c.Groups)
	}
}

func TestLedgerMonthOfClosedOn(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	p := r.closed("p1", "AAA", "38.00", "2026-11-05")
	r.replay.positions[0].OpenedOn = "2026-10-20"
	_ = p
	m := r.ledger(t).view("by_month")
	if len(m.Groups) != 1 || m.Groups[0].Label != "2026-11" || len(m.Groups[0].Books) != 1 {
		t.Fatalf("groups = %v", groupLabels(m))
	}
}

func TestLedgerOpenLotsNotCounted(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open"), lotPos("p2", "BBB", "102", "90.00", "open")}
	r.attr.results["90.00"] = splitResult()
	l := r.ledger(t)
	m := l.view("by_month")
	if len(m.Groups) != 1 || m.Groups[0].Summary != noEndedMonthZh || len(m.Groups[0].Books) != 0 {
		t.Fatalf("by month = %+v", m.Groups)
	}
	if len(l.view("by_company").Groups) != 0 {
		t.Fatalf("by company = %+v", l.view("by_company").Groups)
	}
	if !strings.HasSuffix(l.Note, "2 open lots") {
		t.Errorf("note = %q", l.Note)
	}
}

func TestLedgerSumsFromBooker(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.closed("p1", "AAA", "38.00", "2026-11-13")
	r.cfg.Booker = &fixedBooker{books: []optionattrapi.Book{{Currency: "USD", Lots: 3, AllLots: 6, LedgerAllPnL: "537.80", Total: "999.99",
		RentMinusCost: "1.5", LedgerPnL: "7", Difference: "0", AssignedIntrinsic: "0", Time: "1", MoveCost: "1", Direction: "1",
		IVRemark: "1", Unsplit: "0", Entry: "1", Exit: "1", Commission: "-1"}}}
	b := r.ledger(t).view("by_month").Groups[0].Books[0]
	all, _ := b.line(lblAllZh)
	tot, _ := b.line(lblTotalZh)
	if all.Text != "$537.80 · 6 lots" || tot.Text != "$999.99 · 3 lots" || b.Headline.Text != "$1.50 · 3 lots" {
		t.Fatalf("all = %q, total = %q, headline = %q", all.Text, tot.Text, b.Headline.Text)
	}
}

func TestLedgerNoConclusionUnder20(t *testing.T) {
	t.Parallel()
	for n, few := range map[int]bool{19: true, 20: false} {
		r := ledgerRig()
		for i := range n {
			r.closed("p"+itoa(i), "AAA", "38.00", "2026-11-13")
		}
		b := r.ledger(t).view("by_month").Groups[0].Books[0]
		s, has := b.line(lblSampleZh)
		if has != few {
			t.Fatalf("n = %d: Sample line present = %v", n, has)
		}
		muted := b.Headline.Tone == "muted"
		for _, l := range b.Lines {
			if l.Label == lblAllZh {
				if l.Tone == "muted" {
					t.Errorf("n = %d: ledger-all line is muted", n)
				}
				continue
			}
			if (l.Tone == "muted") != few {
				t.Errorf("n = %d: line %q tone %q", n, l.Label, l.Tone)
			}
		}
		if muted != few {
			t.Errorf("n = %d: headline tone %q", n, b.Headline.Tone)
		}
		if few && s.Text != noConclusionZh+" (19 split lots, fewer than 20)" {
			t.Errorf("sample = %q", s.Text)
		}
	}
}

func TestLedgerSampleBesideFigure(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	for _, id := range []string{"p1", "p2", "p3"} {
		r.closed(id, "AAA", "38.00", "2026-11-13")
	}
	b := r.ledger(t).view("by_month").Groups[0].Books[0]
	if !strings.HasSuffix(b.Headline.Text, " · 3 lots") {
		t.Errorf("headline = %q", b.Headline.Text)
	}
	for _, l := range b.Lines {
		if l.Label != lblSampleZh && !strings.HasSuffix(l.Text, " · 3 lots") {
			t.Errorf("line %q = %q", l.Label, l.Text)
		}
	}
	one := ledgerRig()
	one.closed("p1", "AAA", "38.00", "2026-11-13")
	if tm, _ := one.ledger(t).view("by_month").Groups[0].Books[0].line(lblTimeZh); !strings.HasSuffix(tm.Text, " · 1 lot") {
		t.Errorf("time = %q", tm.Text)
	}
}

func TestLedgerTextsFromZhConsts(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.closed("p1", "AAA", "38.00", "2026-10-13")
	l := r.ledger(t)
	b := l.view("by_month").Groups[1].Books[0]
	if tm, _ := b.line(lblTimeZh); tm.Hint == nil || *tm.Hint != ledgerHintTimeZh {
		t.Errorf("time hint = %v", tm.Hint)
	}
	if s, _ := b.line(lblSampleZh); !strings.HasPrefix(s.Text, noConclusionZh) {
		t.Errorf("sample = %q", s.Text)
	}
	if g := l.view("by_month").Groups[0]; g.Summary != noEndedMonthZh {
		t.Errorf("empty month summary = %q", g.Summary)
	}
	if !strings.HasPrefix(l.Note, ledgerNoteZh) {
		t.Errorf("note = %q", l.Note)
	}
	for _, g := range l.view("by_month").Groups {
		for _, bk := range g.Books {
			for _, ln := range append([]line{bk.Headline}, bk.Lines...) {
				if ln.Hint != nil && utf8.RuneCountInString(*ln.Hint) > 200 {
					t.Errorf("hint of %q is over 200 characters", ln.Label)
				}
			}
		}
	}
}

func TestLedgerWindowSixMonths(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.closed("p1", "AAA", "38.00", "2026-05-20")
	r.closed("p2", "BBB", "38.00", "2026-06-01")
	l := r.ledger(t)
	if got := groupLabels(l.view("by_company")); strings.Join(got, ",") != "BBB" {
		t.Errorf("companies = %v", got)
	}
	if got := groupLabels(l.view("by_month")); strings.Join(got, ",") != "2026-11,2026-06" {
		t.Errorf("months = %v", got)
	}
}

func TestLedgerLeftOutPerGroup(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.closed("p1", "BBB", "38.00", "2026-11-13")
	r.closed("p2", "BBB", "nq", "2026-11-12")
	r.attr.results["nq"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "50", Commission: "-1"}
	gapLot := lotPos("p3", "AAA", "c3", "70.00", "expired")
	gapLot.ClosedOn, gapLot.RealizedPnL = "2026-11-11", "12.00"
	r.replay.positions = append(r.replay.positions, gapLot)
	g := r.ledger(t).view("by_month").Groups[0]
	b := g.Books[0]
	if b.LeftOut == nil || !strings.Contains(*b.LeftOut, "1 个没有存档收盘价的仓位（AAA，账本 $12.00）") ||
		!strings.Contains(*b.LeftOut, "1 个没有存档报价的仓位（账本 $50.00；") {
		t.Fatalf("left_out = %v", b.LeftOut)
	}
	all, _ := b.line(lblAllZh)
	if all.Text != "$64.00 · 2 lots" {
		t.Errorf("ledger-all = %q: the gap lot must be in no sum", all.Text)
	}
	for _, call := range r.book.calls {
		for _, x := range call {
			if x.LedgerPnL == "12.00" {
				t.Fatal("a gap lot was given to the Booker")
			}
		}
	}
}

func TestLedgerCurrenciesApart(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.closed("p1", "AAA", "38.00", "2026-11-13")
	r.closed("p2", "BBB", "jpy", "2026-11-12")
	r.replay.positions[1].Currency = "JPY"
	x := splitResult()
	x.Currency = "JPY"
	r.attr.results["jpy"] = x
	g := r.ledger(t).view("by_month").Groups[0]
	if len(g.Books) != 2 || g.Books[0].Currency != "USD" || g.Books[1].Currency != "JPY" || g.Books[0].ID == g.Books[1].ID {
		t.Fatalf("books = %+v", g.Books)
	}
}

func TestLedgerGapAccountNull(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.cfg.Days = nil
	var m struct {
		Accounts []map[string]json.RawMessage `json:"accounts"`
	}
	if err := json.Unmarshal(r.raw(t), &m); err != nil {
		t.Fatal(err)
	}
	if v, ok := m.Accounts[0]["ledger"]; !ok || string(v) != "null" {
		t.Fatalf("ledger = %s (present %v)", v, ok)
	}
}

func collectIDs(v any, out *[]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, c := range x {
			if s, ok := c.(string); ok && k == "id" {
				*out = append(*out, s)
			}
			collectIDs(c, out)
		}
	case []any:
		for _, c := range x {
			collectIDs(c, out)
		}
	}
}

func TestLedgerIdsValid(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.closed("p1", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "38.00", "2026-11-13")
	r.closed("p2", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAB", "38.00", "2026-10-13")
	r.closed("p3", "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB", "38.00", "2026-10-14")
	var v any
	if err := json.Unmarshal(r.raw(t), &v); err != nil {
		t.Fatal(err)
	}
	var ids []string
	collectIDs(v, &ids)
	seen := map[string]bool{}
	lineID := regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	for _, id := range ids {
		if seen[id] {
			t.Errorf("id %q repeats", id)
		}
		seen[id] = true
		if strings.HasPrefix(id, "lg") && !lineID.MatchString(id) {
			t.Errorf("line id %q is not valid", id)
		}
	}
	if len(ids) < 20 {
		t.Fatalf("only %d ids", len(ids))
	}
}

func TestLedgerCompanySummaryRealSpan(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.closed("p1", "AAA", "38.00", "2026-08-14")
	r.closed("p2", "AAA", "38.00", "2026-11-17")
	g := r.ledger(t).view("by_company").Groups[0]
	if !strings.Contains(g.Summary, "lots ended 2026-08-14 to 2026-11-17") || strings.Contains(g.Summary, "2026-06") {
		t.Fatalf("summary = %q", g.Summary)
	}
}

func TestLedgerLeftOutBucketOnly(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.closed("p1", "BBB", "38.00", "2026-11-13")
	gapLot := lotPos("p3", "AAA", "c3", "70.00", "expired")
	gapLot.ClosedOn, gapLot.RealizedPnL = "2026-11-11", "12.00"
	r.replay.positions = append(r.replay.positions, gapLot)
	for _, g := range r.ledger(t).view("by_company").Groups {
		left := g.Books[0].LeftOut
		switch g.Label {
		case "AAA":
			if left == nil || !strings.Contains(*left, "AAA") {
				t.Errorf("AAA left_out = %v", left)
			}
		case "BBB":
			if left != nil {
				t.Errorf("BBB left_out = %s", *left)
			}
		}
	}
}

func TestLedgerOrders(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.closed("p1", "CCC", "38.00", "2026-09-10")
	r.closed("p2", "AAA", "38.00", "2026-11-13")
	r.closed("p3", "BBB", "38.00", "2026-10-02")
	l := r.ledger(t)
	if got := strings.Join(groupLabels(l.view("by_month")), ","); got != "2026-11,2026-10,2026-09" {
		t.Errorf("months = %s", got)
	}
	if got := strings.Join(groupLabels(l.view("by_company")), ","); got != "CCC,AAA,BBB" {
		t.Errorf("companies = %s", got)
	}
}

func TestCardIdsNotRenumbered(t *testing.T) {
	t.Parallel()
	build := func(older bool) []string {
		r := ledgerRig()
		if older {
			r.closed("old", "AAA", "38.00", "2026-09-10")
		}
		r.closed("p1", "BBB", "38.00", "2026-11-13")
		r.closed("p2", "CCC", "38.00", "2026-11-12")
		var ids []string
		for _, l := range r.build(t).Accounts[0].Lots {
			ids = append(ids, l.ID, l.DaysLine.ID)
		}
		return ids
	}
	with, without := build(true), build(false)
	if len(without) == 0 || strings.Join(with, ",") != strings.Join(without, ",") {
		t.Fatalf("with = %v, without = %v", with, without)
	}
}

func TestCardsStayThirtyDays(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.closed("p1", "AAA", "38.00", "2026-10-03")
	r.closed("p2", "BBB", "38.00", "2026-11-13")
	call := lotPos("p8", "AAA", "c8", "38.00", "closed")
	call.Right, call.ClosedOn = "C", "2026-10-03"
	long := lotPos("p9", "AAA", "c9", "38.00", "closed")
	long.Quantity, long.ClosedOn = "2", "2026-10-03"
	r.replay.positions = append(r.replay.positions, call, long)
	vm := r.build(t)
	a := vm.Accounts[0]
	if len(a.Lots) != 1 || !strings.HasPrefix(a.SampleNote, "2 个已结束的仓位，2 个已拆分的仓位") || len(a.Books) != 1 ||
		!strings.HasPrefix(a.Books[0].Counts, "USD book, 1 lot counted") {
		t.Fatalf("lots = %d, note = %q, books = %+v", len(a.Lots), a.SampleNote, a.Books)
	}
	if n := strings.Join(vm.Notes, "|"); strings.Contains(n, "skipped") {
		t.Errorf("notes = %s", n)
	}
	m := a.Ledger.view("by_month")
	if got := strings.Join(groupLabels(m), ","); got != "2026-11,2026-10" || m.Groups[1].Books[0].Counts != "USD, 1 split lot of 1 ended" {
		t.Errorf("months = %s, group = %+v", got, m.Groups[1])
	}
	var g gathered
	lots, _ := selectLots(r.cfg, r.replay.positions, buildAt, &g)
	if g.calls != 0 || g.longs != 0 || len(lots) != 2 || lots[0].card || !lots[1].card {
		t.Errorf("calls = %d, longs = %d, lots = %+v", g.calls, g.longs, lots)
	}
}

func TestLedgerValidatesSchema(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.cfg.StockIDs = fakeStockIDs{ids: map[string]int64{"BBB": 900}}
	r.bars.close[900] = "36.10"
	type parts struct{ time, move, dir, iv, entry, comm, assigned, ledger string }
	for i, p := range []parts{
		{"100", "-40", "20", "10", "-5", "-3", "0", "82"}, {"80", "-50", "15", "12", "-6", "-4", "0", "47"},
		{"82", "-30", "20", "12", "-8", "-3.2", "0", "72.8"}, {"90", "-36", "20", "15", "-7", "-4", "0", "78"},
		{"60", "-30", "20", "15", "-5", "-4", "190", "246"}} {
		strike := "s" + itoa(i)
		if i == 4 {
			a := lotPos("p"+itoa(i), "BBB", "c"+itoa(i), strike, "assigned")
			a.ClosedOn = "2026-11-13"
			r.replay.positions = append(r.replay.positions, a)
		} else {
			r.closed("p"+itoa(i), "AAA", strike, "2026-11-0"+itoa(i+3))
		}
		r.attr.results[strike] = res(p.time, p.move, p.dir, p.iv, p.entry, "", p.comm, p.assigned, p.ledger)
	}
	r.closed("pn", "CCC", "nq", "2026-11-10")
	r.attr.results["nq"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "12.00", Commission: "-1"}
	for i, ledger := range []string{"10.10", "14.20", "17.00"} {
		strike := "o" + itoa(i)
		r.closed("po"+itoa(i), "DDD", strike, "2026-10-1"+itoa(i))
		r.attr.results[strike] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: ledger, Commission: "-1"}
	}
	for i := range 2 {
		r.replay.positions = append(r.replay.positions, lotPos("op"+itoa(i), "EEE", "o"+itoa(i+90), "90.00", "open"))
	}
	raw := r.raw(t)
	golden(t, "ledger", raw)
	onDisk, err := os.ReadFile("testdata/ledger.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	validate(t, "ledger", onDisk)
	b := r.ledger(t).view("by_month").Groups[0].Books[0]
	if tot, _ := b.line(lblTotalZh); tot.Text != "$335.80 · 5 lots" {
		t.Errorf("total = %q", tot.Text)
	}
	if all, _ := b.line(lblAllZh); all.Text != "$537.80 · 6 lots" {
		t.Errorf("ledger-all = %q", all.Text)
	}
}

// unsplit adds an ended lot with no stored quotes (no split) closed on closedOn.
func (r *rig) unsplit(id, closedOn, ledger string) {
	r.closed(id, "AAA", "nq"+id, closedOn)
	r.attr.results["nq"+id] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: ledger, Commission: "-1"}
}

func TestLedgerFoldsMonthsWithoutSplit(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.closed("p1", "AAA", "38.00", "2026-11-13")
	r.unsplit("u1", "2026-08-10", "10")
	r.unsplit("u2", "2026-09-10", "20")
	r.unsplit("u3", "2026-10-10", "30")
	m := r.ledger(t).view("by_month")
	if len(m.Groups) != 2 || m.Groups[0].Label != "2026-11" || !strings.HasPrefix(m.Groups[1].Label, foldedMonthsZh) {
		t.Fatalf("groups = %v", groupLabels(m))
	}
	if !strings.Contains(m.Groups[1].Label, "2026-08") || !strings.Contains(m.Groups[1].Label, "2026-10") || !strings.Contains(m.Groups[1].Summary, foldedNoteZh) {
		t.Errorf("folded group = %+v", m.Groups[1])
	}
}

func TestLedgerFoldedGroupSumsMonths(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	r.unsplit("u1", "2026-08-10", "10")
	r.unsplit("u2", "2026-09-10", "20")
	r.unsplit("u3", "2026-10-10", "30")
	r.ledger(t)
	var folded []optionattrapi.Result
	for _, call := range r.book.calls {
		if len(call) == 3 {
			folded = call
		}
	}
	if folded == nil {
		t.Fatalf("no Combine call with the three lots: %d calls", len(r.book.calls))
	}
	b := (&allBooker{}).Combine(folded)[0]
	if b.AllLots != 3 || b.LedgerAllPnL != "60" {
		t.Fatalf("book = %+v", b)
	}
	g := r.ledger(t).view("by_month").Groups
	line, _ := g[len(g)-1].Books[0].line(lblAllZh)
	if line.Text != "$60.00 · 3 lots" {
		t.Errorf("ledger-all = %q", line.Text)
	}
}

func TestLedgerGroupLateEntryCount(t *testing.T) {
	t.Parallel()
	r := ledgerRig()
	for i := range 4 {
		strike := "38.00"
		if i < 2 {
			strike = "late"
		}
		r.closed("p"+itoa(i), "AAA", strike, "2026-11-13")
	}
	x := splitResult()
	x.LateEntry = true
	r.attr.results["late"] = x
	g := r.ledger(t).view("by_month").Groups[0]
	entry, ok := g.Books[0].line(lblEntryZh)
	if !ok || entry.Hint == nil || *entry.Hint != ledgerHintEntryZh+" · 4 个仓位中有 2 个在第一条存档报价之前成交（报价从 2026-11-07 起存档）" {
		t.Fatalf("entry = %+v", entry)
	}
}

// CashIfAssigned is a stub: no lines, no error.
func (*allBooker) CashIfAssigned([]optionattrapi.OpenPut) ([]optionattrapi.CashDue, error) {
	return nil, nil
}
