package home_household_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/home_household"
	"github.com/andrewrong/optdesk/internal/pages/home_page"
	"github.com/andrewrong/optdesk/internal/store"
)

var (
	zone = time.FixedZone("owner", 8*3600)
	at   = time.Date(2026, 11, 10, 2, 0, 0, 0, time.UTC)
)

const day = contracts.Day("2026-11-10")

type fakeAccount struct {
	store.AccountReader
	values map[string][]store.AccountValue
	err    error
}

func (f fakeAccount) Values(_ context.Context, _, d string) ([]store.AccountValue, error) {
	return f.values[d], f.err
}

func vals(net, cash string) []store.AccountValue {
	cap := time.Date(2026, 11, 10, 1, 0, 0, 0, time.UTC)
	return []store.AccountValue{
		{Alias: "main", Tag: "NetLiquidation", Value: net, Currency: "USD", CapturedAt: cap},
		{Alias: "main", Tag: "TotalCashValue", Value: cash, Currency: "USD", CapturedAt: cap},
	}
}

type fakeSessions struct{}

func (fakeSessions) SessionDays(_ context.Context, _ string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	f, _ := time.Parse("2006-01-02", string(from))
	e, _ := time.Parse("2006-01-02", string(to))
	var out []contracts.SessionDay
	for d := f; !d.After(e); d = d.AddDate(0, 0, 1) {
		out = append(out, contracts.SessionDay{CalendarID: "XNYS", SessionDay: types.SerializableDate{Time: d},
			IsSession: d.Weekday() != time.Saturday && d.Weekday() != time.Sunday})
	}
	return out, nil
}

func full() map[string][]store.AccountValue {
	return map[string][]store.AccountValue{
		"2026-10-30": vals("98000.00", "40000.00"),
		"2026-11-09": vals("99000.50", "41000.00"),
		"2026-11-10": vals("100000.00", "42000.00"),
	}
}

func config(v map[string][]store.AccountValue) home_household.Config {
	return home_household.Config{
		RunID: "run1", BuildID: "b1", Scope: "scope1", Zone: zone, Alias: "main",
		Account: fakeAccount{values: v}, Sessions: fakeSessions{}, Figures: pagefig.New,
	}
}

func build(t *testing.T, c home_household.Config) (map[string]any, []byte) {
	t.Helper()
	b, err := home_household.Build(context.Background(), c, day, at)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m, b
}

func tilesOf(m map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for k, v := range m["card"].(map[string]any)["tiles"].(map[string]any) {
		out[k] = v.(map[string]any)
	}
	return out
}

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", filepath.Base(url)))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func validate(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile("https://optdesk.invalid/schemas/page.household_summary.v2.json")
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

func TestGolden(t *testing.T) {
	t.Parallel()
	_, b := build(t, config(full()))
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, b, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	golden := filepath.Join("testdata", "ok.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, pretty.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, pretty.Bytes()) {
		t.Fatalf("golden differs:\n%s", pretty.Bytes())
	}
}

func TestTilesCarryTheCaptureFigures(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(full()))
	if m["run_status"] != "ok" || m["run_id"] != "run1" || m["build_id"] != "b1" || m["scope"] != "scope1" {
		t.Fatalf("envelope: %v", m)
	}
	tl := tilesOf(m)
	cases := []struct{ tile, figure, caption string }{
		{"net_worth", "$100,000.00", "USD · Data day 2026-11-10"},
		{"cash", "42.00%", "+$42,000.00"},
		{"day_change", "+$999.50", "+1.01%"},
		{"month_to_date", "+$2,000.00", "+2.04%"},
	}
	for _, c := range cases {
		if got := tl[c.tile]["figure"].(map[string]any)["text"]; got != c.figure {
			t.Errorf("%s figure = %v, want %s", c.tile, got, c.figure)
		}
		if got := tl[c.tile]["caption"].(map[string]any)["text"]; got != c.caption {
			t.Errorf("%s caption = %v, want %s", c.tile, got, c.caption)
		}
	}
}

func reasons(m map[string]any) map[string]string {
	out := map[string]string{}
	for k, t := range tilesOf(m) {
		if g, ok := t["figure"].(map[string]any); ok {
			if r, ok := g["reason"].(string); ok {
				out[k] = r
			}
		}
	}
	return out
}

func TestNilReaderGivesNoReaderOnEveryTile(t *testing.T) {
	t.Parallel()
	for name, mod := range map[string]func(*home_household.Config){
		"sessions": func(c *home_household.Config) { c.Sessions = nil },
		"account":  func(c *home_household.Config) { c.Account = nil },
	} {
		c := config(full())
		mod(&c)
		m, _ := build(t, c)
		r := reasons(m)
		if len(r) != 4 {
			t.Fatalf("%s: %v", name, r)
		}
		for k, v := range r {
			if v != "no_reader" {
				t.Errorf("%s: tile %s reason %s", name, k, v)
			}
		}
		if m["run_status"] != "failed" || m["scope"] != "scope1" {
			t.Errorf("%s: %v", name, m)
		}
	}
}

func TestNoCaptureGivesNoDataOnEveryTile(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(map[string][]store.AccountValue{}))
	r := reasons(m)
	if len(r) != 4 {
		t.Fatalf("%v", r)
	}
	for k, v := range r {
		if v != "no_data" {
			t.Errorf("tile %s reason %s", k, v)
		}
	}
}

func TestOldCaptureGivesStaleInput(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(map[string][]store.AccountValue{"2026-10-30": vals("98000.00", "40000.00")}))
	for k, v := range reasons(m) {
		if v != "stale_input" {
			t.Errorf("tile %s reason %s", k, v)
		}
	}
}

func TestMissingPriorOrMonthStartGapsOnlyThatTile(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(map[string][]store.AccountValue{"2026-11-10": vals("100000.00", "42000.00")}))
	r := reasons(m)
	if len(r) != 2 || r["day_change"] != "missing_input" || r["month_to_date"] != "missing_input" {
		t.Fatalf("reasons %v", r)
	}
	if m["run_status"] != "partial" {
		t.Fatalf("run_status %v", m["run_status"])
	}
	if len(m["card"].(map[string]any)["gaps"].([]any)) != 2 {
		t.Fatalf("card gaps: %v", m["card"])
	}
}

func TestNegativeCashIsBorrowing(t *testing.T) {
	t.Parallel()
	v := full()
	v["2026-11-10"] = vals("100000.00", "-500.00")
	m, _ := build(t, config(v))
	cash := tilesOf(m)["cash"]
	if cash["figure"].(map[string]any)["text"] != "Borrowing" || cash["caption"].(map[string]any)["text"] != "−$500.00" {
		t.Fatalf("cash: %v", cash)
	}
}

func TestReaderErrorIsAnError(t *testing.T) {
	t.Parallel()
	c := config(full())
	c.Account = fakeAccount{err: errors.New("boom")}
	if _, err := home_household.Build(context.Background(), c, day, at); err == nil {
		t.Fatal("a reader error must be an error")
	}
	c.Zone = nil
	if _, err := home_household.Build(context.Background(), c, day, at); err == nil {
		t.Fatal("a nil zone must be an error")
	}
}

func TestComposerAcceptsTheMember(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "home_page", "testdata", "overview_ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	var others map[string]json.RawMessage
	if err := json.Unmarshal(raw, &others); err != nil {
		t.Fatal(err)
	}
	for _, v := range []map[string][]store.AccountValue{full(), {}} {
		c := config(v)
		c.RunID, c.BuildID, c.Scope = "run-test", "build-test", "scope-test"
		_, b := build(t, c)
		in := home_page.Input{
			RunID: "run-test", BuildID: "build-test", Scope: "scope-test", Zone: zone, Title: "Home", Say: "Quiet",
			CardLinks: home_page.CardLinks{Macro: "/macro", Market: "/market", Household: "/household",
				Companies: "/companies", Options: "/options"},
			Household: b, Companies: others["companies"], Macro: others["macro"], Market: others["market"], Options: others["options"],
		}
		if _, err := home_page.Compose(in, at); err != nil {
			t.Fatalf("composer rejects the member: %v", err)
		}
	}
}

func figText(tile map[string]any, k string) string {
	return tile[k].(map[string]any)["text"].(string)
}

func gapTextsOf(m map[string]any) []string {
	var out []string
	for _, g := range m["card"].(map[string]any)["gaps"].([]any) {
		out = append(out, g.(map[string]any)["text"].(string))
	}
	return out
}

func TestOneCapturedDayNamesTheMissingCaptures(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(map[string][]store.AccountValue{"2026-11-10": vals("100000.00", "42000.00")}))
	ts := tilesOf(m)
	const dayText = "Needs a second captured session day; first capture 2026-11-10"
	const mtdText = "Needs the last session day of the previous month; first capture 2026-11-10"
	dc, mtd := ts["day_change"], ts["month_to_date"]
	if figText(dc, "figure") != dayText || figText(dc, "caption") != dayText {
		t.Fatalf("day_change: %v", dc)
	}
	if figText(mtd, "figure") != mtdText || figText(mtd, "caption") != mtdText {
		t.Fatalf("month_to_date: %v", mtd)
	}
	if dc["figure"].(map[string]any)["reason"] != "missing_input" || mtd["figure"].(map[string]any)["reason"] != "missing_input" {
		t.Fatalf("reasons %v", reasons(m))
	}
	g := gapTextsOf(m)
	if len(g) != 2 || g[0] != "Previous trading day: "+dayText || g[1] != "Month to date: "+mtdText {
		t.Fatalf("gaps %q", g)
	}
}

func TestPriorDayWithoutMonthStartOnlyMonthToDateSaysSo(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(map[string][]store.AccountValue{
		"2026-11-09": vals("99000.50", "41000.00"),
		"2026-11-10": vals("100000.00", "42000.00"),
	}))
	ts := tilesOf(m)
	const mtdText = "Needs the last session day of the previous month; first capture 2026-11-10"
	if figText(ts["month_to_date"], "figure") != mtdText || figText(ts["month_to_date"], "caption") != mtdText {
		t.Fatalf("month_to_date: %v", ts["month_to_date"])
	}
	if figText(ts["day_change"], "figure") != "+$999.50" {
		t.Fatalf("day_change: %v", ts["day_change"])
	}
}

func TestPriorDayWithoutNetLiquidationKeepsGenericText(t *testing.T) {
	t.Parallel()
	prior := []store.AccountValue{vals("1", "41000.00")[1]}
	m, _ := build(t, config(map[string][]store.AccountValue{
		"2026-11-09": prior,
		"2026-11-10": vals("100000.00", "42000.00"),
	}))
	dc := tilesOf(m)["day_change"]
	const want = "Input data is missing: no data yet"
	if figText(dc, "figure") != want || figText(dc, "caption") != want {
		t.Fatalf("day_change: %v", dc)
	}
}
