package risk_page_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/risk_page"
	"github.com/andrewrong/optdesk/internal/store"
)

var update = flag.Bool("update", false, "rewrite the golden file")

var (
	zone = time.FixedZone("owner", 8*3600)
	at   = time.Date(2026, 11, 10, 2, 0, 0, 0, time.UTC)
)

const day = contracts.Day("2026-11-10")

type fakeAccount struct {
	store.AccountReader
	rows map[string][]store.Position // by session day
}

func (f fakeAccount) Positions(_ context.Context, _, d string) ([]store.Position, error) {
	return f.rows[d], nil
}

type fakeLedger struct{}

func (fakeLedger) FlexTradeRows(context.Context, string) ([]store.FlexTradeRow, error) {
	return []store.FlexTradeRow{{TransactionID: "t1", Revision: 1, Attributes: `{"symbol":"AAA"}`, RecordedAt: at}}, nil
}
func (fakeLedger) LatestOkFetch(context.Context, string, string) (store.FlexFetchRecord, bool, error) {
	return store.FlexFetchRecord{}, false, nil
}
func (fakeLedger) StoredAliases(context.Context) ([]string, error) { return nil, nil }

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

type replayer struct{}

func (replayer) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	return compute.LedgerResult{}
}

// structures answers with the given structures whatever the input.
type structures struct{ res compute.StructureResult }

func (s structures) Evaluate(compute.StructureInput, compute.StructureParams) compute.StructureResult {
	return s.res
}

func structure(under, cur, expiry, delta, vega string) compute.Structure {
	return compute.Structure{
		Underlying: under, Currency: cur, Kind: "short_put",
		Legs: []compute.StructureLeg{{Expiry: "2027-01-15"}, {Expiry: expiry}},
		Greeks: compute.GreekSet{
			Delta: compute.FigureValue{Value: delta}, Vega: compute.FigureValue{Value: vega},
		},
	}
}

func stk(sym string, id int64, qty, cur string) store.Position {
	return store.Position{Symbol: sym, SecType: "STK", Currency: cur, ConID: id, Quantity: qty,
		CapturedAt: time.Date(2026, 11, 10, 1, 0, 0, 0, time.UTC)}
}

func okConfig(res compute.StructureResult, stocks ...store.Position) risk_page.Config {
	return risk_page.Config{
		RunID: "run1", BuildID: "b1", Zone: zone, Aliases: []string{"main", "wife"},
		Account:  fakeAccount{rows: map[string][]store.Position{"2026-11-10": stocks}},
		Ledger:   fakeLedger{},
		Sessions: fakeSessions{},
		Ledgers:  func() compute.OptionLedger { return replayer{} },
		Holdings: pagefig.NewHoldings, Structures: func() compute.OptionStructures { return structures{res} },
	}
}

func defaultConfig() risk_page.Config {
	return okConfig(compute.StructureResult{Structures: []compute.Structure{
		structure("AAA", "USD", "2026-12-18", "-30.5", "12.25"),
		structure("BBB", "USD", "2026-12-18", "-10.004", "3.5"),
	}}, stk("AAA", 1, "100", "USD"), stk("BBB", 2, "50", "USD"))
}

func build(t *testing.T, c risk_page.Config, alias string) (map[string]any, []byte) {
	t.Helper()
	b, err := risk_page.NewPage(c, alias).Build(context.Background(), nil, day, at)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	return m, b
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
	s, err := c.Compile("https://optdesk.invalid/schemas/page.household_risk.v1.json")
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

func block(m map[string]any, name string) map[string]any { b, _ := m[name].(map[string]any); return b }

func TestGolden(t *testing.T) {
	t.Parallel()
	_, b := build(t, defaultConfig(), "main")
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		t.Fatal(err)
	}
	out.WriteByte('\n')
	path := filepath.Join("testdata", "ok.golden.json")
	if *update {
		if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("page differs from %s:\n%s", path, out.String())
	}
}

func TestDeltaByUnderlyingVegaByExpiry(t *testing.T) {
	t.Parallel()
	m, _ := build(t, defaultConfig(), "")
	if m["run_status"] != "partial" || m["scope"] != "main" {
		t.Fatalf("status/scope = %v/%v", m["run_status"], m["scope"])
	}
	// AAA: 100 shares - 30.5 = 69.5; BBB: 50 - 10.004 = 39.996 -> 40.00 (cents, half up); total 109.496 -> 109.50
	want := map[string]string{"AAA": "+69.50 美元/美元", "BBB": "+40.00 美元/美元"}
	rows := block(m, "delta")["rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("delta rows = %v", rows)
	}
	for _, r := range rows {
		row := r.(map[string]any)
		cell := row["delta"].(map[string]any)
		if cell["text"] != want[row["name"].(string)] {
			t.Errorf("%v delta = %v", row["name"], cell)
		}
	}
	if tot := block(m, "delta")["total"].(map[string]any); tot["value"] != 109.5 {
		t.Errorf("delta total = %v", tot)
	}
	// one expiry: the nearest leg of each structure, 15.75 in all
	vrows := block(m, "vega")["rows"].([]any)
	if len(vrows) != 1 || vrows[0].(map[string]any)["expiry_text"] != "2026-12-18" {
		t.Fatalf("vega rows = %v", vrows)
	}
	if v := block(m, "vega")["total"].(map[string]any); v["value"] != 15.75 || v["unit"] != "usd_per_vol_point" {
		t.Errorf("vega total = %v", v)
	}
}

func TestNotBuiltBlocks(t *testing.T) {
	t.Parallel()
	m, _ := build(t, defaultConfig(), "main")
	for _, name := range []string{"beta_delta", "sigma"} {
		b := block(m, name)
		if g := b["gap"].(map[string]any); g["reason"] != "not_built" || g["text"] == "" {
			t.Errorf("%s gap = %v", name, g)
		}
		if len(b["rows"].([]any)) != 0 {
			t.Errorf("%s has rows", name)
		}
	}
	n := 0
	for _, g := range m["gaps"].([]any) {
		if g.(map[string]any)["reason"] == "not_built" {
			n++
		}
	}
	if n != 3 {
		t.Errorf("page gaps with not_built = %d, want 3 (beta_delta, sigma, standard_metrics)", n)
	}
}

func TestNilReaderIsGapPage(t *testing.T) {
	t.Parallel()
	c := defaultConfig()
	c.Account = nil
	m, _ := build(t, c, "main")
	if m["run_status"] != "failed" || block(m, "delta")["gap"].(map[string]any)["reason"] != "no_reader" {
		t.Fatalf("page = %v", m)
	}
	c = defaultConfig()
	c.Holdings = nil
	if m, _ := build(t, c, "main"); block(m, "vega")["gap"].(map[string]any)["reason"] != "no_reader" {
		t.Fatalf("nil Holdings: %v", m)
	}
}

func TestNoCaptureIsNoData(t *testing.T) {
	t.Parallel()
	c := okConfig(compute.StructureResult{})
	m, _ := build(t, c, "main")
	if g := block(m, "delta")["gap"].(map[string]any); g["reason"] != "no_data" {
		t.Fatalf("gap = %v", g)
	}
	if g := block(m, "beta_delta")["gap"].(map[string]any); g["reason"] != "no_data" {
		t.Fatalf("beta gap = %v", g)
	}
}

func TestUnknownAliasIsNoData(t *testing.T) {
	t.Parallel()
	for _, alias := range []string{"stranger", "bad alias!"} {
		m, _ := build(t, defaultConfig(), alias)
		if block(m, "delta")["gap"].(map[string]any)["reason"] != "no_data" || m["scope"] == "bad alias!" {
			t.Fatalf("%q: %v", alias, m)
		}
	}
}

func TestForeignStockGivesGapNotZero(t *testing.T) {
	t.Parallel()
	c := okConfig(compute.StructureResult{Structures: []compute.Structure{
		structure("AAA", "USD", "2026-12-18", "-30.5", "12.25"),
		structure("JJJ", "JPY", "2026-12-18", "-1", "1"),
	}}, stk("AAA", 1, "100", "USD"), stk("JJJ", 3, "10", "JPY"))
	m, _ := build(t, c, "main")
	d := block(m, "delta")
	if d["total"] != nil || d["gap"].(map[string]any)["reason"] != "unsupported_currency" {
		t.Fatalf("delta = %v", d)
	}
	if len(d["rows"].([]any)) != 1 {
		t.Fatalf("rows = %v", d["rows"])
	}
	if v := block(m, "vega"); v["total"].(map[string]any)["value"] != 12.25 {
		t.Fatalf("the JPY structure counted in vega: %v", v)
	}
}

func TestGreekGapGivesMissingInput(t *testing.T) {
	t.Parallel()
	s := structure("AAA", "USD", "2026-12-18", "", "")
	s.Greeks = compute.GreekSet{Delta: compute.FigureValue{Gap: "missing_quote"}, Vega: compute.FigureValue{Gap: "missing_quote"}}
	m, _ := build(t, okConfig(compute.StructureResult{Structures: []compute.Structure{s}}, stk("AAA", 1, "100", "USD")), "main")
	if g := block(m, "delta")["gap"].(map[string]any); g["reason"] != "missing_input" {
		t.Fatalf("delta gap = %v", g)
	}
	if v := block(m, "vega"); v["total"] != nil || len(v["rows"].([]any)) != 0 || v["gap"] == nil {
		t.Fatalf("vega = %v", v)
	}
}

func TestNoZoneIsError(t *testing.T) {
	t.Parallel()
	c := defaultConfig()
	c.Zone = nil
	if _, err := risk_page.NewPage(c, "main").Build(context.Background(), nil, day, at); err == nil {
		t.Fatal("no error")
	}
}

func TestMeta(t *testing.T) {
	t.Parallel()
	p := risk_page.NewPage(defaultConfig(), "")
	if p.Meta.Name != "household_risk" || p.Meta.Schema != "page.household_risk.v1" {
		t.Fatalf("meta = %+v", p.Meta)
	}
}
