package option_tracking_page

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
)

type aliasLedger struct {
	fakeLedger
	aliases []string
}

func (l aliasLedger) StoredAliases(context.Context) ([]string, error) { return l.aliases, nil }

// stubTracker answers by alias and counts its calls.
type stubTracker struct {
	results map[string]trackapi.TrackResult
	calls   *int
}

func (s stubTracker) Track(in trackapi.TrackInput, _ trackapi.TrackParams) trackapi.TrackResult {
	if s.calls != nil {
		*s.calls++
	}
	return s.results[in.Alias]
}

func sampleOpen(id string) opt {
	return opt{PositionID: id, Underlying: "AAPL", Right: "C", Strike: "150", Expiry: "2026-12-18", Quantity: "-1",
		Multiplier: "100", Currency: "USD", Status: "open", DTE: 38, Moneyness: "otm", OtmPercent: "3.20",
		Spot: "145.2", Mid: "1.25", MarkDay: "2026-11-09", MarkStatus: "ok", Extrinsic: "1.25", PremiumIncome: "100",
		Commissions: "1.1", Kept: "-25", KeptPercent: "-25.00", Delta: "0.3", Iv: "0.31", NextEarnings: "2026-12-01",
		EarningsInLife: true, NextExDate: "2026-12-05", ExAmount: "0.25", ExInLife: true, EventStatus: "ok",
		Guard: "ok", GuardBasis: "bid", OpenedOn: "2026-10-01"}
}

func sampleClosed(id string) opt {
	o := sampleOpen(id)
	o.Status, o.ClosedOn, o.RealizedPnL = "assigned", "2026-11-03", "98.9"
	return o
}

func okResult() trackapi.TrackResult {
	return trackapi.TrackResult{
		Open:   []opt{sampleOpen("p1")},
		Recent: []opt{sampleClosed("p0")},
		Recon:  []trackapi.TrackRecon{{Underlying: "MSFT", Right: "P", Strike: "300", Quantity: "-1", Expiry: "2026-12-18", Code: "not_in_flex"}},
	}
}

// cfgFor is a Config of fakes with one alias per result; the account has a capture.
func cfgFor(results map[string]trackapi.TrackResult, aliases ...string) Config {
	r := newRig()
	r.positions["2026-11-10"] = []store.Position{{SecType: "STK"}}
	c := r.config()
	c.RunID, c.BuildID, c.Scope, c.Zone = "r1", "b1", "s1", time.UTC
	c.Ledger = aliasLedger{fakeLedger: fakeLedger{rows: []store.FlexTradeRow{{TransactionID: "t1", Revision: 1, Attributes: `{}`}}}, aliases: aliases}
	c.Tracker = stubTracker{results: results}
	c.Params = trackapi.TrackParams{MarkMaxAgeSessions: 3, RecentDays: 30}
	return c
}

func buildRaw(t *testing.T, c Config) []byte {
	t.Helper()
	b, err := Build(context.Background(), c, nil, buildDay, time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func buildPage(t *testing.T, c Config) viewModel {
	t.Helper()
	var vm viewModel
	if err := json.Unmarshal(buildRaw(t, c), &vm); err != nil {
		t.Fatal(err)
	}
	return vm
}

func cellMap(t *testing.T, r row, id string) map[string]any {
	t.Helper()
	m, ok := r.Cells[id].(map[string]any)
	if !ok {
		t.Fatalf("cell %s = %v", id, r.Cells[id])
	}
	return m
}

func TestNoOpenShortsSentence(t *testing.T) {
	t.Parallel()
	res := trackapi.TrackResult{Recent: []opt{sampleClosed("p0")}}
	vm := buildPage(t, cfgFor(map[string]trackapi.TrackResult{"main": res}, "main"))
	a := vm.Accounts[0]
	if a.Open != nil || a.Recent == nil || !strings.Contains(a.Title, "No open short options") {
		t.Fatalf("account = %+v", a)
	}
	c := cfgFor(map[string]trackapi.TrackResult{"main": {}}, "main")
	c.Account = nil
	a = buildPage(t, c).Accounts[0]
	if a.Gap == nil || a.Gap.Text != "No open short options" {
		t.Fatalf("empty account = %+v", a)
	}
}

func TestLongLegsLeftOut(t *testing.T) {
	t.Parallel()
	res := okResult()
	res.LongLegsLeftOut = 3
	vm := buildPage(t, cfgFor(map[string]trackapi.TrackResult{"main": res}, "main"))
	if vm.Accounts[0].LongLegsLeftOut != 3 {
		t.Fatalf("long_legs_left_out = %d", vm.Accounts[0].LongLegsLeftOut)
	}
	if !strings.Contains(strings.Join(vm.Notes, "|"), "3 long legs not listed, see the option positions page") {
		t.Fatalf("notes = %v", vm.Notes)
	}
	if len(vm.Notes) < 1 || vm.Notes[0] != "Flex arrives the next day; positions opened or assigned today may not show yet; see the check table" || vm.MarkNote != "Mid price, the midpoint of the closing bid/ask; not a traded price" {
		t.Fatalf("notes = %v, mark note = %q", vm.Notes, vm.MarkNote)
	}
}

func TestStaleLabelWhenNoMark(t *testing.T) {
	t.Parallel()
	vm := buildPage(t, cfgFor(map[string]trackapi.TrackResult{"main": okResult()}, "main"))
	if vm.Stale != nil {
		t.Fatalf("stale = %+v with a fresh mark", vm.Stale)
	}
	res := okResult()
	res.Open[0].MarkStatus, res.Open[0].Mid = "no_mark", ""
	vm = buildPage(t, cfgFor(map[string]trackapi.TrackResult{"main": res}, "main"))
	if vm.Stale == nil || !vm.Stale.IsStale || vm.Stale.DataTime != "2026-11-10T00:00:00+00:00" {
		t.Fatalf("stale = %+v", vm.Stale)
	}
}

func TestNoReaderIsGap(t *testing.T) {
	t.Parallel()
	calls := 0
	c := cfgFor(nil)
	c.Ledger = nil
	c.Tracker = stubTracker{calls: &calls}
	vm := buildPage(t, c)
	if len(vm.Gaps) != 1 || vm.Gaps[0].Reason != "no_reader" || vm.RunStatus != "partial" {
		t.Fatalf("gaps = %+v status = %s", vm.Gaps, vm.RunStatus)
	}
	if len(vm.Accounts) != 1 || vm.Accounts[0].Gap == nil || vm.Accounts[0].Gap.Reason != "no_reader" || calls != 0 {
		t.Fatalf("accounts = %+v calls = %d", vm.Accounts, calls)
	}
}

func TestAliasWithoutRowsIsGap(t *testing.T) {
	t.Parallel()
	c := cfgFor(nil, "main")
	c.Ledger = aliasLedger{aliases: []string{"main"}}
	a := buildPage(t, c).Accounts[0]
	if a.Gap == nil || a.Gap.Reason != "no_data" || !strings.HasPrefix(a.Gap.Text, "no_flex_rows") || a.Open != nil {
		t.Fatalf("account = %+v", a)
	}
}

func TestNoAccountIsGapOfCheckOnly(t *testing.T) {
	t.Parallel()
	c := cfgFor(map[string]trackapi.TrackResult{"main": okResult()}, "main")
	c.Account = nil
	vm := buildPage(t, c)
	a := vm.Accounts[0]
	if a.Open == nil || a.Recent == nil || a.Recon != nil || a.Gap != nil {
		t.Fatalf("account = %+v", a)
	}
	if len(vm.Gaps) != 1 || vm.Gaps[0].Reason != "no_data" || !strings.HasPrefix(vm.Gaps[0].Text, "no_account") {
		t.Fatalf("gaps = %+v", vm.Gaps)
	}
}

func TestTwoAliasesTwoGroups(t *testing.T) {
	t.Parallel()
	second := okResult()
	second.Open = []opt{sampleOpen("q1"), sampleOpen("q2")}
	c := cfgFor(map[string]trackapi.TrackResult{"a": okResult(), "b": second}, "a", "b")
	a := buildPage(t, c).Accounts
	if len(a) != 2 || a[0].Alias != "a" || a[1].Alias != "b" || len(a[0].Open.Rows) != 1 || len(a[1].Open.Rows) != 2 {
		t.Fatalf("accounts = %+v", a)
	}
	five := cfgFor(map[string]trackapi.TrackResult{}, "a", "b", "c", "d", "e")
	if n := len(buildPage(t, five).Accounts); n != 4 {
		t.Fatalf("%d accounts for five aliases", n)
	}
}

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

func TestOutputValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	comp := jsonschema.NewCompiler()
	comp.AssertFormat()
	comp.UseLoader(fileLoader{})
	schema, err := comp.Compile(schemaHost + "page.option_tracking_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	noMark := okResult()
	noMark.Open[0].MarkStatus, noMark.Open[0].Mid, noMark.Open[0].Spot = "no_mark", "", ""
	noMark.Open[0].Moneyness, noMark.Open[0].OtmPercent, noMark.Open[0].Kept, noMark.Open[0].KeptPercent = "unknown", "", "", ""
	noMark.Open[0].Delta, noMark.Open[0].Iv = "", ""
	noMark.Open[0].EventStatus, noMark.Open[0].Guard, noMark.Open[0].GuardReason = "no_event_row", "unknown", "no_guard_row"
	noReader := cfgFor(nil)
	noReader.Ledger = nil
	cases := map[string]Config{
		"ok":        cfgFor(map[string]trackapi.TrackResult{"main": okResult()}, "main"),
		"no_open":   cfgFor(map[string]trackapi.TrackResult{"main": {Recent: []opt{sampleClosed("p0")}}}, "main"),
		"no_marks":  cfgFor(map[string]trackapi.TrackResult{"main": noMark}, "main"),
		"no_reader": noReader,
	}
	for name, c := range cases {
		v, err := jsonschema.UnmarshalJSON(bytes.NewReader(buildRaw(t, c)))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(v); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
