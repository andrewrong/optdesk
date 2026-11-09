package structures_page_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/structures_page"
	"github.com/andrewrong/optdesk/internal/store"
)

const testDay = contracts.Day("2026-11-09")

var testAt = time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC)

type (
	stubLedger   struct{ store.OptionLedgerReader }
	stubSessions struct{ store.SessionDayReader }
	stubAccount  struct{ store.AccountReader }
	stubEod      struct{ store.OptionEodReader }
	stubTerms    struct{ store.OptionTermsReader }
)

func (stubLedger) FlexTradeRows(context.Context, string) ([]store.FlexTradeRow, error) {
	return nil, nil
}

func (stubSessions) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	var out []contracts.SessionDay
	for _, d := range []string{"2026-11-05", "2026-11-06", "2026-11-09"} {
		t, _ := time.Parse("2006-01-02", d)
		out = append(out, contracts.SessionDay{CalendarID: "XNYS", IsSession: true, SessionDay: types.SerializableDate{Time: t}})
	}
	return out, nil
}

// stubAccount holds one stock row of MSFT (instrument 88) in the capture of 2026-11-06.
func (stubAccount) Positions(_ context.Context, _, day string) ([]store.Position, error) {
	if day == "2026-11-06" {
		return []store.Position{{SecType: "STK", Symbol: "MSFT", ConID: 88}}, nil
	}
	return nil, nil
}

// stubEod holds the quotes of the two put contracts of MSFT on 2026-11-06.
func (stubEod) OptionEod(context.Context, int64, contracts.Day, contracts.Day) ([]contracts.OptionEod, error) {
	day := types.SerializableDate{Time: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)}
	row := func(id int, bid, ask, delta, gamma, theta, vega float64) contracts.OptionEod {
		u := 320.0
		return contracts.OptionEod{InstrumentID: id, SessionDay: day, Bid: &bid, Ask: &ask, UnderlyingPrice: &u,
			Delta: &delta, Gamma: &gamma, ThetaPerCalendarDay: &theta, VegaPerVolPoint: &vega}
	}
	return []contracts.OptionEod{row(1, 1.0, 1.2, -0.125, 0.01, 0.02, 0.03), row(2, 1.1, 1.3, -0.25, 0.02, 0.04, 0.06)}, nil
}

func (stubTerms) OptionTerms(context.Context, []int64) ([]store.OptionTerms, error) {
	return []store.OptionTerms{
		{InstrumentID: 1, Right: "P", Strike: "300", ExpirationDate: "2026-12-18"},
		{InstrumentID: 2, Right: "P", Strike: "310", ExpirationDate: "2026-12-18"},
	}, nil
}

type replayStub struct{ res compute.LedgerResult }

func (r replayStub) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	return r.res
}

type ruleStub struct{ res compute.StructureResult }

func (r ruleStub) Evaluate(compute.StructureInput, compute.StructureParams) compute.StructureResult {
	return r.res
}

// fixture is the testdata of a page: what Evaluate returns and the pieces Replay returns.
type fixture struct {
	Result    compute.StructureResult
	Positions []compute.LedgerPosition
}

func readFixture(t *testing.T, name string) fixture {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var fx fixture
	if err := json.Unmarshal(b, &fx); err != nil {
		t.Fatal(err)
	}
	return fx
}

// config wires the fixture; quotes false leaves the quote readers out.
func (fx fixture) config(quotes bool) structures_page.Config {
	c := structures_page.Config{
		RunID: "run1", BuildID: "build1", Zone: time.FixedZone("CST", 8*3600), Aliases: []string{"main"},
		Ledger: stubLedger{}, Sessions: stubSessions{}, Account: stubAccount{},
		Ledgers:    func() compute.OptionLedger { return replayStub{res: compute.LedgerResult{Positions: fx.Positions}} },
		Structures: func() compute.OptionStructures { return ruleStub{res: fx.Result} },
	}
	if quotes {
		c.Eod, c.Terms = stubEod{}, stubTerms{}
	}
	return c
}

func buildWith(t *testing.T, c structures_page.Config, id string) map[string]any {
	t.Helper()
	b, err := structures_page.NewDetailPage(c, "main", id).Build(context.Background(), nil, testDay, testAt)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func (fx fixture) build(t *testing.T, id string, quotes bool) map[string]any {
	t.Helper()
	return buildWith(t, fx.config(quotes), id)
}

func obj(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%v is not an object", v)
	}
	return m
}

func list(t *testing.T, v any) []any {
	t.Helper()
	l, ok := v.([]any)
	if !ok {
		t.Fatalf("%v is not a list", v)
	}
	return l
}

const (
	spreadID = "main:T101:1"
	gapsID   = "main:T200:1"
)

func TestFindByID(t *testing.T) {
	t.Parallel()
	fx := readFixture(t, "detail_spread")
	other := fx.Result.Structures[0]
	other.ID, other.Underlying = "main:T999:1", "ZZZ"
	fx.Result.Other = []compute.Structure{other}
	for id, want := range map[string]string{spreadID: "MSFT 看跌价差", "main:T999:1": "ZZZ 看跌价差"} {
		m := fx.build(t, id, true)
		if m["title"] != want || m["scope"] != "main" || m["schema"] != "structure_detail" || m["schema_version"] != 3.0 {
			t.Errorf("%s: title = %v, scope = %v", id, m["title"], m["scope"])
		}
	}
}

func TestIDNotOpenIsGapPage(t *testing.T) {
	t.Parallel()
	m := readFixture(t, "detail_spread").build(t, "main:T000:9", true)
	if m["sentence"] != "结构已不在持仓中" || m["run_status"] != "partial" {
		t.Fatalf("sentence = %v, status = %v", m["sentence"], m["run_status"])
	}
	for _, k := range []string{"payoff", "greeks"} {
		if g := obj(t, m[k]); g["text"] != "结构已不在持仓中" {
			t.Errorf("%s = %v", k, g)
		}
	}
	if len(list(t, m["legs"])) != 1 || len(list(t, m["fills"])) != 0 || len(list(t, m["timeline"])) != 0 {
		t.Errorf("page = %v", m)
	}
	validate(t, "gap page", m)
}

func TestTilesFromFigures(t *testing.T) {
	t.Parallel()
	m := readFixture(t, "detail_spread").build(t, spreadID, true)
	tl := obj(t, m["tiles"])
	for key, want := range map[string]string{"pnl": "140.0000", "max_gain": "150.0000", "max_loss": "-850.0000", "break_even": "308.5000"} {
		fig := obj(t, obj(t, tl[key])["figure"])
		if fig["text"] != want || fig["unit"] != "usd" {
			t.Errorf("%s = %v, want %s", key, fig, want)
		}
	}
	if fig := obj(t, obj(t, tl["max_loss"])["figure"]); fig["value"] != -850.0 {
		t.Errorf("max_loss value = %v", fig["value"])
	}
	// a gap figure stays a named gap, and a naked call's max loss reads as unbounded
	g := readFixture(t, "detail_gaps").build(t, gapsID, true)
	gt := obj(t, g["tiles"])
	if r := obj(t, obj(t, gt["pnl"])["figure"])["reason"]; r != "missing_quote" {
		t.Errorf("pnl gap = %v", r)
	}
	if loss := obj(t, obj(t, gt["max_loss"])["figure"]); loss["reason"] != "no_rule" || loss["text"] != "无上限" {
		t.Errorf("max_loss = %v", loss)
	}
}

func TestLevelsDistance(t *testing.T) {
	t.Parallel()
	m := readFixture(t, "detail_spread").build(t, spreadID, true)
	var got []string
	for _, l := range list(t, m["key_levels"]) {
		lv := obj(t, l)
		got = append(got, lv["name"].(string)+"="+obj(t, lv["price"])["text"].(string)+" "+lv["distance"].(string))
	}
	want := []string{"买入行权价=300 -6.2500%", "卖出行权价=310 -3.1250%", "盈亏平衡=308.5000 -3.5938%"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("levels = %v, want %v", got, want)
	}
	// without a quote the distance is a gap, never a zero
	g := readFixture(t, "detail_gaps").build(t, gapsID, true)
	if d := obj(t, obj(t, list(t, g["key_levels"])[0])["distance"]); d["reason"] != "missing_quote" {
		t.Errorf("distance = %v", d)
	}
}

func schemaOf(t *testing.T) *jsonschema.Schema {
	t.Helper()
	raw, err := os.ReadFile("../../../contracts/schemas/page.structure_detail.v3.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource("schema.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validate(t *testing.T, name string, m map[string]any) {
	t.Helper()
	b, _ := json.Marshal(m)
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := schemaOf(t).Validate(v); err != nil {
		t.Errorf("%s: %v\n%s", name, err, b)
	}
}

func TestOutputValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	spread, gaps := readFixture(t, "detail_spread"), readFixture(t, "detail_gaps")
	validate(t, "spread", spread.build(t, spreadID, true))
	validate(t, "spread without quote readers", spread.build(t, spreadID, false))
	validate(t, "gaps", gaps.build(t, gapsID, true))
	validate(t, "gaps without quote readers", gaps.build(t, gapsID, false))
	// the spread page is complete: every block is a figure, not a gap
	m := spread.build(t, spreadID, true)
	if m["run_status"] != "ok" || len(list(t, m["gaps"])) != 0 {
		t.Errorf("run_status = %v, gaps = %v", m["run_status"], m["gaps"])
	}
}
