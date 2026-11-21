package household_page_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/pages/household_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/mark"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// heldAccount holds values and stock positions by alias; a day not in values has none.
type heldAccount struct {
	store.AccountReader
	values    map[string]map[string][]store.AccountValue
	positions map[string][]store.Position
	failPos   string
}

func (h heldAccount) Values(_ context.Context, alias, d string) ([]store.AccountValue, error) {
	return h.values[alias][d], nil
}

func (h heldAccount) Positions(_ context.Context, alias, _ string) ([]store.Position, error) {
	if alias == h.failPos {
		return nil, errors.New("boom")
	}
	return h.positions[alias], nil
}

// heldMarks answers by alias, day.
type heldMarks map[string][]mark.PositionMark

func (m heldMarks) Marks(_ context.Context, alias, d string) ([]mark.PositionMark, error) {
	return m[alias+" "+d], nil
}

// heldFx answers a rate by the day asked: the family reads the newest counted day, an account's own capture day
// has another rate.
type heldFx struct {
	bar.FxRateReader
	byDay map[string]string
}

func (f heldFx) LatestFxRate(_ context.Context, pair string, asOf contracts.Day) (contracts.FxRate, bool, error) {
	r, ok := f.byDay[string(asOf)]
	if !ok {
		return contracts.FxRate{}, false, nil
	}
	return contracts.FxRate{Pair: pair, Rate: r, RateDay: types.SerializableDate{Time: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)}}, true, nil
}

type heldUnder []underlying.Row

func (u heldUnder) Current(context.Context) ([]underlying.Row, error)      { return u, nil }
func (u heldUnder) History(context.Context, int) ([]underlying.Row, error) { return nil, nil }

type heldEarn struct {
	rows map[string][]contracts.EarningsDate
	asks *[][3]string
}

func (e heldEarn) EarningsDates(_ context.Context, issuer string, from, to contracts.Day) ([]contracts.EarningsDate, error) {
	if e.asks != nil {
		*e.asks = append(*e.asks, [3]string{issuer, string(from), string(to)})
	}
	return e.rows[issuer], nil
}

func sday(s string) types.SerializableDate {
	t, _ := time.Parse("2006-01-02", s)
	return types.SerializableDate{Time: t}
}

func earnRow(issuer, status, report, observed string) contracts.EarningsDate {
	r := contracts.EarningsDate{Issuer: issuer, Status: gen.EarningsDateStatus(status), ObservedOn: sday(observed)}
	if report != "" {
		d := sday(report)
		r.ReportDate = &d
	}
	return r
}

func capturedAt(h string) time.Time {
	t, _ := time.Parse(time.RFC3339, "2026-11-"+h)
	return t
}

func netValues(alias, d, net, cur, at string) []store.AccountValue {
	return []store.AccountValue{{Alias: alias, SessionDay: d, Tag: "NetLiquidation", Value: net, Currency: cur, CapturedAt: capturedAt(at)}}
}

func hstock(sym string, con int64, qty, cur string) store.Position {
	return store.Position{Symbol: sym, SecType: "stock", Currency: cur, Multiplier: "1", ConID: con, Quantity: qty}
}

func hmark(con int64, price, cur string) mark.PositionMark {
	return mark.PositionMark{ConID: con, Price: price, Basis: mark.BasisLast, Currency: cur}
}

// heldConfig is the book of the golden: main (USD, 2026-11-10), wife (JPY net, captured 2026-11-09, so its own
// rate is 140 while the family rate is 150) and daughter (no capture, left out).
func heldConfig() household_page.Config {
	acc := heldAccount{
		values: map[string]map[string][]store.AccountValue{
			"main": {"2026-11-10": netValues("main", "2026-11-10", "100000", "USD", "10T01:00:00Z")},
			"wife": {"2026-11-09": netValues("wife", "2026-11-09", "750000", "JPY", "09T01:00:00Z")},
		},
		positions: map[string][]store.Position{
			"main": {hstock("AAA", 1, "100", "USD"), hstock("BBB", 2, "-10", "USD"), hstock("GGG", 3, "10", "USD"),
				hstock("HHH", 4, "20", "USD"), hstock("CCC", 5, "30", "USD"), hstock("DDD", 6, "40", "USD"),
				hstock("EEE", 7, "50", "USD"), hstock("ZERO", 8, "0", "USD"),
				{Symbol: "AAA", SecType: "option", Currency: "USD", Multiplier: "100", ConID: 20, Quantity: "-1"}},
			"wife": {hstock("AAA", 11, "50", "USD"), hstock("4063", 12, "100", "JPY")},
		},
	}
	marks := heldMarks{
		"main 2026-11-10": {hmark(1, "50", "USD"), hmark(2, "30", "USD"), hmark(5, "10", "USD"), hmark(6, "10", "USD"), hmark(7, "10", "USD")},
		"main 2026-11-08": {hmark(4, "10", "USD")},
		"wife 2026-11-09": {hmark(11, "50", "USD"), hmark(12, "3000", "JPY")},
	}
	und := heldUnder{
		{Symbol: "AAA", Market: "US", Currency: "USD", Issuer: "cik:0000000001"},
		{Symbol: "BBB", Market: "US", Currency: "USD", Issuer: "cik:0000000002"},
		{Symbol: "HHH", Market: "US", Currency: "USD"},
		{Symbol: "CCC", Market: "US", Currency: "USD", Issuer: "cik:0000000003"},
		{Symbol: "DDD", Market: "US", Currency: "USD", Issuer: "cik:0000000004"},
		{Symbol: "EEE", Market: "US", Currency: "USD", Issuer: "cik:0000000005"},
		{Symbol: "4063", Market: "JP", Currency: "JPY", Issuer: "edinet:E00001", ReportDateOverride: "2026-11-10"},
	}
	earn := heldEarn{rows: map[string][]contracts.EarningsDate{
		"cik:0000000001": {earnRow("cik:0000000001", "listed", "2026-11-14", "2026-11-10")},
		"cik:0000000002": {earnRow("cik:0000000002", "absent", "", "2026-11-10")},
		"cik:0000000003": {earnRow("cik:0000000003", "unsupported", "", "2026-11-10")},
		"cik:0000000004": {earnRow("cik:0000000004", "error", "", "2026-11-10")},
		"cik:0000000005": {earnRow("cik:0000000005", "listed", "2026-11-05", "2026-11-10")},
	}}
	return household_page.Config{
		RunID: "run1", BuildID: "b1", Zone: zone, Aliases: []string{"main", "wife", "daughter"},
		Account: acc, Sessions: fakeSessions{}, Marks: marks, Holdings: pagefig.NewHoldings, Family: pagefig.NewFamily,
		Fx:         heldFx{byDay: map[string]string{"2026-11-10": "150", "2026-11-09": "140"}},
		Underlying: und, Earnings: earn, Held: pagefig.NewHeld(),
	}
}

func validateHeld(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile("https://optdesk.invalid/schemas/page.held_family_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(v); err != nil {
		t.Fatalf("output is invalid: %v\n%s", err, b)
	}
}

func buildHeld(t *testing.T, c household_page.Config) (map[string]any, []byte) {
	t.Helper()
	p, err := household_page.BuildHeld(context.Background(), c, day)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return decode(t, b), b
}

func heldRows(m map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, r := range m["rows"].([]any) {
		row := r.(map[string]any)
		out[row["company"].(string)] = row
	}
	return out
}

func TestBuildHeldGolden(t *testing.T) {
	t.Parallel()
	_, b := buildHeld(t, heldConfig())
	validateHeld(t, b)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, b, "", "  "); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "held.golden.json")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(golden, append(pretty.Bytes(), '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(want), pretty.Bytes()) {
		t.Fatalf("page differs from %s:\n%s", golden, pretty.Bytes())
	}
}

// TestBuildHeldJpyUsesFamilyRate: the JPY line of wife (captured on a day whose own rate is 140) is valued at the
// family rate 150: 100 x 3000 yen = 2,000 USD, not 2,142.86.
func TestBuildHeldJpyUsesFamilyRate(t *testing.T) {
	t.Parallel()
	m, _ := buildHeld(t, heldConfig())
	row := heldRows(m)["4063"]
	if v := row["value"].(map[string]any); v["text"] != "2,000.00" {
		t.Fatalf("value = %v", v)
	}
}

func TestBuildHeldMergesAccountsAndLeavesOutExcluded(t *testing.T) {
	t.Parallel()
	m, _ := buildHeld(t, heldConfig())
	aaa := heldRows(m)["AAA"]
	accs := aaa["accounts"].([]any)
	if len(accs) != 2 || accs[0].(map[string]any)["alias"] != "main" || accs[1].(map[string]any)["alias"] != "wife" {
		t.Fatalf("accounts = %v", accs)
	}
	if v := aaa["value"].(map[string]any); v["text"] != "7,500.00" {
		t.Fatalf("value = %v", v)
	}
	if !has(strs(m["lead_notes"]), "daughter") {
		t.Fatalf("no line for the account left out: %v", m["lead_notes"])
	}
	if _, ok := heldRows(m)["ZERO"]; ok {
		t.Fatal("a zero quantity is a row")
	}
}

func TestBuildHeldEarningsWindowAndReasons(t *testing.T) {
	t.Parallel()
	c := heldConfig()
	var asks [][3]string
	c.Earnings = heldEarn{rows: c.Earnings.(heldEarn).rows, asks: &asks}
	m, _ := buildHeld(t, c)
	rows := heldRows(m)
	want := map[string]string{"BBB": "no_row", "CCC": "unsupported", "DDD": "row_error", "EEE": "past_date",
		"GGG": "no_underlying", "HHH": "no_issuer"}
	for co, reason := range want {
		if got := rows[co]["earnings"].(map[string]any)["reason"]; got != reason {
			t.Errorf("%s earnings reason %v, want %s", co, got, reason)
		}
	}
	if e := rows["AAA"]["earnings"].(map[string]any); e["date"] != "2026-11-14" || e["days_to"] != float64(4) {
		t.Errorf("AAA earnings = %v", e)
	}
	if e := rows["4063"]["earnings"].(map[string]any); e["date"] != "2026-11-10" || e["days_to"] != float64(0) {
		t.Errorf("4063 override earnings = %v", e)
	}
	found := false
	for _, a := range asks {
		if a[0] == "cik:0000000001" {
			found = a[1] == "2026-07-13" && a[2] == "2026-11-10"
		}
	}
	if !found {
		t.Errorf("earnings window asks %v", asks)
	}
}

func TestBuildHeldNilReadersAreGaps(t *testing.T) {
	t.Parallel()
	c := heldConfig()
	c.Underlying, c.Earnings = nil, nil
	m, _ := buildHeld(t, c)
	if got := heldRows(m)["AAA"]["earnings"].(map[string]any)["reason"]; got != "no_reader" {
		t.Fatalf("earnings reason %v", got)
	}
	core := heldConfig()
	core.Held = nil
	m, b := buildHeld(t, core)
	validateHeld(t, b)
	if m["run_status"] != "failed" || len(m["rows"].([]any)) != 0 || m["gaps"].([]any)[0].(map[string]any)["reason"] != "no_reader" {
		t.Fatalf("no figure: %s", b)
	}
}

func TestBuildHeldAliasFailures(t *testing.T) {
	t.Parallel()
	c := heldConfig()
	acc := c.Account.(heldAccount)
	acc.failPos = "wife"
	c.Account = acc
	m, _ := buildHeld(t, c)
	if !has(strs(m["lead_notes"]), "wife") {
		t.Fatalf("another alias failing is a note: %v", m["lead_notes"])
	}
	if _, ok := heldRows(m)["4063"]; ok {
		t.Fatal("stocks of the failing alias are in a row")
	}
	acc.failPos = "main"
	c.Account = acc
	if _, err := household_page.BuildHeld(context.Background(), c, day); err == nil {
		t.Fatal("the main alias failing is an error")
	}
}

func TestBuildHeldNotesAndCounts(t *testing.T) {
	t.Parallel()
	m, _ := buildHeld(t, heldConfig())
	notes := strings.Join(strs(m["lead_notes"]), "\n")
	for _, sub := range []string{"最多早 5 天", "空头", "只有期权", "daughter", "3 行的收盘价早于", "行没有财报日期"} {
		if !strings.Contains(notes, sub) {
			t.Errorf("lead_notes lack %q:\n%s", sub, notes)
		}
	}
}

var _ blockapi.HeldFigure = pagefig.NewHeld()
