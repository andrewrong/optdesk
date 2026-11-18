package option_pnl_page

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
)

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, schemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(schemaDir + file)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func validate(t *testing.T, name string, raw []byte) {
	t.Helper()
	comp := jsonschema.NewCompiler()
	comp.AssertFormat()
	comp.UseLoader(fileLoader{})
	schema, err := comp.Compile(schemaHost + "page.option_pnl_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(v); err != nil {
		t.Errorf("%s: %v", name, err)
	}
}

// golden compares the page with testdata/<name>.golden.json (rewritten with -update).
func golden(t *testing.T, name string, raw []byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		t.Fatal(err)
	}
	buf.WriteByte('\n')
	path := "testdata/" + name + ".golden.json"
	if *update {
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, buf.Bytes()) {
		t.Errorf("%s differs from the page; run with -update and review the diff", path)
	}
}

// res is a result with the figures given as time, move cost, direction, IV, entry, exit, commission,
// assigned intrinsic and ledger; the difference is "0" (the figures add up).
func res(time, move, dir, iv, entry, exit, comm, assigned, ledger string) optionattrapi.Result {
	return optionattrapi.Result{Currency: "USD", Time: time, MoveCost: move, Direction: dir, IVRemark: iv, Unsplit: "0",
		Entry: entry, Exit: exit, Commission: comm, AssignedIntrinsic: assigned, LedgerPnL: ledger, Difference: "0",
		Steps: []optionattrapi.Step{
			{From: "2026-11-10", To: "2026-11-11", CalendarDays: 1, IVFrom: "0.309", IVTo: "0.300", RefIVTo: "0.295"},
			{From: "2026-11-11", To: "2026-11-17", CalendarDays: 6, MultiDay: true, IVFrom: "0.300", IVTo: "0.286", RefIVTo: "0.289"},
		}}
}

// sampleRig is the sample of the design: AAA and CCC open, BBB assigned, DDD expired with no stored close.
func sampleRig() *rig {
	r := newRig()
	aaa := lotPos("p1", "AAA", "101", "90.00", "open")
	aaa.UnrealizedPnL = "147.40"
	bbb := lotPos("p2", "BBB", "202", "38.00", "assigned")
	bbb.RealizedPnL, bbb.BrokerRealized = "268.20", "268.20"
	ccc := lotPos("p3", "CCC", "303", "55.00", "open")
	ccc.UnrealizedPnL = "110.20"
	ddd := lotPos("p4", "DDD", "404", "70.00", "expired")
	ddd.RealizedPnL = "12.00"
	r.replay.positions = []compute.LedgerPosition{aaa, bbb, ccc, ddd}
	r.days.rows[101] = []optionday.Day{dayRow(101, "2026-11-17", "1.20", "1.30", "95")}
	r.days.rows[303] = []optionday.Day{dayRow(303, "2026-11-17", "0.80", "0.90", "60")}
	r.bars.close[900] = "36.10"
	r.attr.results["90.00"] = res("150", "-60", "30", "40", "-10", "", "-2.6", "0", "147.40")
	r.attr.results["38.00"] = res("200", "-90", "40", "0", "-20", "-35", "-16.8", "190", "268.20")
	r.attr.results["55.00"] = res("62", "-36", "25", "24", "40", "", "-4.8", "0", "110.20")
	return r
}

func TestGoldenOk(t *testing.T) {
	t.Parallel()
	golden(t, "ok", sampleRig().raw(t))
}

func TestOutputValidatesSchema(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"ok", "no_quotes", "lot_gap"} {
		raw, err := os.ReadFile("testdata/" + name + ".golden.json")
		if err != nil {
			t.Fatal(err)
		}
		validate(t, name, raw)
	}
	validate(t, "live build", sampleRig().raw(t))
	r := newRig()
	r.cfg.Ledger = nil
	validate(t, "no reader", r.raw(t))
}

func TestSampleBookIdentity(t *testing.T) {
	t.Parallel()
	vm := sampleRig().build(t)
	b := vm.Accounts[0].Books[0]
	want := map[string]string{"b0-main-total": "$335.80", "b0-main-assigned": "$190.00",
		"b0-main-ledger": "$525.80 = total + assignment intrinsic + difference ($0.00)"}
	for _, l := range b.Lines {
		if w, ok := want[l.ID]; ok {
			if l.Text != w {
				t.Errorf("%s = %q, want %q", l.ID, l.Text, w)
			}
			delete(want, l.ID)
		}
	}
	if len(want) != 0 {
		t.Fatalf("lines missing: %v; book = %+v", want, b)
	}
	wantLeft := "1 of 4 lots left out of the sums: 1 with no stored close (DDD, ledger $12.00)"
	if b.Headline.Text != "$226.00" || b.LeftOut == nil || !strings.HasPrefix(*b.LeftOut, wantLeft) {
		t.Fatalf("headline = %q, left_out = %v", b.Headline.Text, b.LeftOut)
	}
}

// fixedBooker answers the same books for every call and records the results.
type fixedBooker struct {
	mu    sync.Mutex
	books []optionattrapi.Book
	calls [][]optionattrapi.Result
}

func (f *fixedBooker) Combine(rs []optionattrapi.Result) []optionattrapi.Book {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, rs)
	return f.books
}

func TestSumsComeFromBooker(t *testing.T) {
	t.Parallel()
	r := sampleRig()
	r.cfg.Booker = &fixedBooker{books: []optionattrapi.Book{{Currency: "USD", Lots: 3, Total: "999.99", RentMinusCost: "1.5",
		LedgerPnL: "7", Difference: "0", AssignedIntrinsic: "0", Time: "1", MoveCost: "1", Direction: "1", IVRemark: "1",
		Unsplit: "0", Entry: "1", Exit: "1", Commission: "-1"}}}
	b := r.build(t).Accounts[0].Books[0]
	found := false
	for _, l := range b.Lines {
		if l.Label == "Total options P&L" {
			found = l.Text == "$999.99"
		}
	}
	if !found || b.Headline.Text != "$1.50" {
		t.Fatalf("book = %+v", b)
	}
}

func TestNoQuotesLotShowsLedgerOnly(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p2", "BBB", "202", "38.00", "expired")}
	r.bars.close[900] = "36.10"
	r.attr.results["38.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, Reason: "no stored quotes", LedgerPnL: "50",
		Commission: "-1.30", Steps: []optionattrapi.Step{}}
	raw := r.raw(t)
	golden(t, "no_quotes", raw)
	var vm viewModel
	if err := json.Unmarshal(raw, &vm); err != nil {
		t.Fatal(err)
	}
	card := vm.Accounts[0].Lots[0]
	if card.Gap != nil || card.IVLine != nil || len(card.Badges) != 1 || card.Badges[0] != "no quotes" {
		t.Fatalf("card = %+v", card)
	}
}

func TestNoQuotesCardNoZeroLines(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p2", "BBB", "202", "38.00", "expired")}
	r.bars.close[900] = "36.10"
	r.attr.results["38.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "50", Commission: "-1.30"}
	card := r.build(t).Accounts[0].Lots[0]
	texts := map[string]string{}
	for _, l := range card.Lines {
		texts[l.Label] = l.Text
		if strings.Contains(l.Text, "0.00") && l.Text != "$50.00" {
			t.Errorf("line %s reads %q", l.Label, l.Text)
		}
	}
	if texts["Ledger says"] != "$50.00" || texts["Commissions"] != "−$1.30" || texts["Reason"] != noQuotesZh ||
		len(card.Lines) != 3 || card.DaysLine.Text != "no stored quote days" {
		t.Fatalf("card = %+v", card)
	}
}

func TestUnreconciledCardShowsReason(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open")}
	r.days.rows[101] = []optionday.Day{dayRow(101, "2026-11-17", "1.20", "1.30", "95")}
	x := res("150", "-60", "30", "40", "-10", "", "-2.6", "0", "147.40")
	x.Difference, x.Reason = "", "ledger mark day 2026-11-17 differs from last quote day 2026-11-16"
	r.attr.results["90.00"] = x
	card := r.build(t).Accounts[0].Lots[0]
	labels := map[string]bool{}
	for _, l := range card.Lines {
		labels[l.Label] = true
	}
	for _, w := range []string{"Time (theta)", "Move cost (gamma)", "Direction (delta)", "Commissions", "Ledger says"} {
		if !labels[w] {
			t.Errorf("no %q line", w)
		}
	}
	if labels["Total options P&L"] {
		t.Error("an unreconciled lot has no total")
	}
	if card.Recon[0].Text != x.Reason || strings.Contains(card.Recon[0].Text, "difference $") || card.DaysLine.Text == "" {
		t.Fatalf("recon = %+v", card.Recon)
	}
}

func TestLotErrorIsCardBookSaysSo(t *testing.T) {
	t.Parallel()
	r := sampleRig()
	eee := lotPos("p5", "EEE", "505", "80.00", "open")
	r.replay.positions = append(r.replay.positions[:1:1], r.replay.positions[1], r.replay.positions[3], eee)
	raw := r.raw(t)
	golden(t, "lot_gap", raw)
	var vm viewModel
	if err := json.Unmarshal(raw, &vm); err != nil {
		t.Fatal(err)
	}
	lots := vm.Accounts[0].Lots
	if len(lots) != 4 || lots[0].Gap != nil || lots[1].Gap != nil || len(lots[0].Lines) == 0 {
		t.Fatalf("lots = %+v", lots)
	}
	if lots[2].Gap == nil || lots[2].Gap.Text != "no stored close of DDD on 2026-11-13: no split" || len(lots[2].Lines) != 0 {
		t.Fatalf("DDD = %+v", lots[2])
	}
	if lots[3].Gap == nil || lots[3].Gap.Text != "no stored quote of EEE within 3 sessions: no split" {
		t.Fatalf("EEE = %+v", lots[3])
	}
	b := vm.Accounts[0].Books[0]
	if b.LeftOut == nil || !strings.HasPrefix(*b.LeftOut, "2 of 4 lots left out of the sums: ") {
		t.Fatalf("left_out = %v", b.LeftOut)
	}
	if len(r.book.calls[0]) != 2 {
		t.Fatalf("the Booker was given %d results, want the 2 attributed", len(r.book.calls[0]))
	}
	for _, call := range r.book.calls {
		for _, x := range call {
			if x.LedgerPnL == "12.00" {
				t.Fatal("a gap lot was given to the Booker")
			}
		}
	}
}

func TestLeftOutNamesEveryClass(t *testing.T) {
	t.Parallel()
	r := sampleRig()
	r.cfg.Booker = &fixedBooker{books: []optionattrapi.Book{{Currency: "USD", Lots: 3, NoQuoteLots: 2, NoQuoteLedgerPnL: "5",
		UnreconciledLots: 1, UnreconciledLedgerPnL: "7", Total: "1", LedgerPnL: "1", Difference: "0"}}}
	got := *r.build(t).Accounts[0].Books[0].LeftOut
	for _, w := range []string{"1 with no stored close (DDD, ledger $12.00)", "2 with no stored quotes (ledger $5.00; " + leftOutNoQuotesZh + ")",
		"1 not reconciled (ledger $7.00)", "4 of 7 lots left out"} {
		if !strings.Contains(got, w) {
			t.Errorf("left_out %q lacks %q", got, w)
		}
	}
}

func TestBuildPassesSeriesReader(t *testing.T) {
	t.Parallel()
	r := sampleRig()
	r.raw(t)
	if len(r.series.series) != 1 || r.series.series[0] != "DGS3MO" {
		t.Fatalf("series read = %v", r.series.series)
	}
	if len(r.attr.ins) == 0 || len(r.attr.ins[0].Rates) != 1 {
		t.Fatalf("inputs = %+v", r.attr.ins)
	}
}

func TestCallLotsSkippedAndCounted(t *testing.T) {
	t.Parallel()
	r := newRig()
	call := lotPos("p9", "AAA", "909", "100.00", "open")
	call.Right = "C"
	r.replay.positions = []compute.LedgerPosition{call, lotPos("p1", "AAA", "101", "90.00", "open")}
	r.days.rows[101] = []optionday.Day{dayRow(101, "2026-11-17", "1.20", "1.30", "95")}
	r.attr.results["90.00"] = res("1", "1", "1", "1", "1", "", "-1", "0", "100")
	vm := r.build(t)
	if len(vm.Accounts[0].Lots) != 1 || !strings.Contains(strings.Join(vm.Notes, "|"), "main: 1 call lots skipped") {
		t.Fatalf("lots = %d, notes = %v", len(vm.Accounts[0].Lots), vm.Notes)
	}
}

func TestLotsOlderThan30DaysNotShown(t *testing.T) {
	t.Parallel()
	r := newRig()
	old := lotPos("p1", "AAA", "101", "90.00", "closed")
	old.ClosedOn = "2026-10-17"
	edge := lotPos("p2", "BBB", "202", "38.00", "closed")
	edge.ClosedOn = "2026-10-18"
	r.replay.positions = []compute.LedgerPosition{old, edge}
	r.attr.results["38.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "50", Commission: "-1"}
	vm := r.build(t)
	if len(vm.Accounts[0].Lots) != 1 || !strings.Contains(vm.Accounts[0].Lots[0].Title, "BBB") {
		t.Fatalf("lots = %+v", vm.Accounts[0].Lots)
	}
}

func TestFewerThan20EndedLots(t *testing.T) {
	t.Parallel()
	const few = ": fewer than 20 split lots: no conclusion about what pays"
	for n, want := range map[int]string{19: "19 ended lots, 0 split lots" + few, 20: "20 ended lots, 0 split lots" + few} {
		r := newRig()
		for i := 0; i < n; i++ {
			r.replay.positions = append(r.replay.positions, lotPos("p"+itoa(i), "BBB", itoa(1000+i), "38.00", "closed"))
		}
		r.attr.results["38.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "50", Commission: "-1"}
		if got := r.build(t).Accounts[0].SampleNote; got != want {
			t.Errorf("n = %d: sample_note = %q, want %q", n, got, want)
		}
	}
}

func TestRollBadge(t *testing.T) {
	t.Parallel()
	r := newRig()
	a := lotPos("p1", "AAA", "101", "38.00", "closed")
	a.ClosedOn = "2026-11-10"
	b := lotPos("p2", "AAA", "102", "38.00", "open")
	b.OpenedOn = "2026-11-10"
	c := lotPos("p3", "CCC", "103", "38.00", "closed")
	r.replay.positions = []compute.LedgerPosition{a, b, c}
	r.days.rows[102] = []optionday.Day{dayRow(102, "2026-11-17", "1.20", "1.30", "95")}
	r.attr.results["38.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "50", Commission: "-1"}
	lots := r.build(t).Accounts[0].Lots
	has := func(l lotView) bool {
		for _, x := range l.Badges {
			if x == "looks like a roll" {
				return true
			}
		}
		return false
	}
	if !has(lots[0]) || !has(lots[1]) || has(lots[2]) {
		t.Fatalf("badges = %v %v %v", lots[0].Badges, lots[1].Badges, lots[2].Badges)
	}
}

func TestNilDaysReaderIsGap(t *testing.T) {
	t.Parallel()
	r := sampleRig()
	r.cfg.Days = nil
	vm := r.build(t)
	a := vm.Accounts[0]
	if a.Gap == nil || !strings.Contains(a.Gap.Text, "No stored quote reader") || vm.RunStatus != "partial" || len(vm.Gaps) != 1 {
		t.Fatalf("account gap = %+v, page gaps = %+v", a.Gap, vm.Gaps)
	}
}

func TestLongPutNoteCount(t *testing.T) {
	t.Parallel()
	r := newRig()
	for _, id := range []string{"p1", "p2"} {
		p := lotPos(id, "AAA", "10"+id, "90.00", "open")
		p.Quantity = "2"
		r.replay.positions = append(r.replay.positions, p)
	}
	vm := r.build(t)
	want := "main: 2 long put lots skipped: " + longPutSkippedZh
	if !strings.Contains(strings.Join(vm.Notes, "|"), want) || len(vm.Accounts[0].Lots) != 0 {
		t.Fatalf("lots = %d, notes = %v", len(vm.Accounts[0].Lots), vm.Notes)
	}
}

func TestNoQuotesTextsAreChinese(t *testing.T) {
	t.Parallel()
	r := sampleRig()
	r.attr.results["38.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "50", Commission: "-1.30"}
	r.cfg.Booker = &fixedBooker{books: []optionattrapi.Book{{Currency: "USD", Lots: 3, NoQuoteLots: 2, NoQuoteLedgerPnL: "5",
		Total: "1", LedgerPnL: "1", Difference: "0"}}}
	long := lotPos("p9", "LLL", "909", "90.00", "open")
	long.Quantity = "2"
	r.replay.positions = append(r.replay.positions, long)
	vm := r.build(t)
	acc := vm.Accounts[0]
	var bbb, aaa *lotView
	for i := range acc.Lots {
		switch {
		case strings.Contains(acc.Lots[i].Title, "BBB"):
			bbb = &acc.Lots[i]
		case strings.Contains(acc.Lots[i].Title, "AAA"):
			aaa = &acc.Lots[i]
		}
	}
	if bbb == nil || aaa == nil {
		t.Fatalf("lots = %+v", acc.Lots)
	}
	reason := ""
	for _, l := range bbb.Lines {
		if l.Label == "Reason" {
			reason = l.Text
		}
	}
	if reason != noQuotesZh {
		t.Errorf("reason = %q", reason)
	}
	for _, l := range aaa.Lines {
		if l.Label == "Reason" && l.Text == noQuotesZh {
			t.Errorf("card with quotes shows the no-quotes text")
		}
	}
	if got := *acc.Books[0].LeftOut; !strings.Contains(got, "2 with no stored quotes (ledger $5.00; "+leftOutNoQuotesZh+")") {
		t.Errorf("left_out = %q", got)
	}
	if !strings.Contains(strings.Join(vm.Notes, "|"), "main: 1 long put lots skipped: "+longPutSkippedZh) {
		t.Errorf("notes = %v", vm.Notes)
	}
}

// oldLot is an ended lot closed outside the 30-day card window, so only the ledger sees it.
func (r *rig) oldLot(id string, gap bool) {
	p := lotPos(id, "BBB", "c"+id, "38.00", "closed")
	p.ClosedOn = "2026-10-01"
	if gap {
		p.RealizedPnL = ""
	}
	r.replay.positions = append(r.replay.positions, p)
}

func TestSampleNoteCountsEndedAndSplit(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.oldLot("p1", false)
	r.oldLot("p2", true)
	r.attr.results["38.00"] = res("20", "-8", "4", "2", "-2", "", "-2", "0", "14")
	if got := r.build(t).Accounts[0].SampleNote; !strings.HasPrefix(got, "2 ended lots, 1 split lots: ") {
		t.Fatalf("sample_note = %q", got)
	}
}

func TestSampleThresholdCountsSplit(t *testing.T) {
	t.Parallel()
	r := newRig()
	for i := 0; i < 25; i++ {
		r.oldLot("p"+itoa(i), i >= 5)
	}
	r.attr.results["38.00"] = res("20", "-8", "4", "2", "-2", "", "-2", "0", "14")
	want := "25 ended lots, 5 split lots: fewer than 20 split lots: no conclusion about what pays"
	if got := r.build(t).Accounts[0].SampleNote; got != want {
		t.Fatalf("sample_note = %q, want %q", got, want)
	}
}
