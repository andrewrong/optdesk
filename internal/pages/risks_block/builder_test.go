package risks_block

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var zone = time.FixedZone("CST", 8*3600)

func at() time.Time { return time.Date(2026, 11, 8, 9, 0, 0, 0, zone) }

func fired(id, title, figure, level, detail, day string) compute.AlertResult {
	return compute.AlertResult{RuleID: id, Title: title, Figure: figure, Level: level, Detail: detail,
		State: "fired", Day: contracts.Day(day), DataDay: contracts.Day(day)}
}

func clear(id string) compute.AlertResult {
	return compute.AlertResult{RuleID: id, Title: id, Level: "w", State: "clear", Day: "2026-11-06", DataDay: "2026-11-06"}
}

func noData(id, title, reason string) compute.AlertResult {
	return compute.AlertResult{RuleID: id, Title: title, Level: "w", State: "no_data", Reason: reason}
}

func build(t *testing.T, res []compute.AlertResult, err error) []byte {
	t.Helper()
	in := Input{
		Evaluate: func(context.Context, compute.AlertRules, compute.ReadingFunc, contracts.Day) ([]compute.AlertResult, error) {
			return res, err
		},
		Read: func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
			return nil, nil
		},
		Today: "2026-11-07", GeneratedAt: at(),
	}
	b, e := Build(context.Background(), in)
	if err != nil {
		if e == nil {
			t.Fatal("no error")
		}
		return nil
	}
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

type tc struct {
	name string
	res  []compute.AlertResult
}

var cases = []tc{
	{"ok_with_items", []compute.AlertResult{
		fired("vix_pctile_high", "VIX percentile high", "market.vix_pctile_3y", "w", "VIX 3-year percentile 0.93", "2026-11-06"),
		clear("a"),
		fired("sp500_far_below_200d", "S&P far below 200-day average", "market.sp500_vs_200d", "d", "Gap -0.06", "2026-11-05"),
	}},
	{"ok_no_alert", []compute.AlertResult{clear("a"), clear("b")}},
	{"partial", []compute.AlertResult{
		fired("vix_pctile_high", "VIX percentile high", "market.vix_pctile_3y", "w", "VIX 3-year percentile 0.93", "2026-11-06"),
		noData("spy_ivr_high", "SPY IV rank high", "missing_input"),
	}},
	{"gap", []compute.AlertResult{noData("a", "A1", "no_reading"), noData("b", "B1", "unit_mismatch")}},
}

func TestBlockOkWithItems(t *testing.T) {
	t.Parallel()
	m := decode(t, build(t, cases[0].res, nil))
	if m["state"] != "ok" || m["key"] != "risks" || m["title"] != "Risk" {
		t.Fatalf("block = %v", m)
	}
	if m["say"] != "2026-11-07 2 market alerts." {
		t.Errorf("say = %v", m["say"])
	}
	items := m["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items = %v", items)
	}
	first := items[0].(map[string]any)
	if first["kind"] != "market_alert" || first["icon_key"] != "volatility" || first["title"] != "VIX percentile high" ||
		first["detail"] != "VIX 3-year percentile 0.93 (data day 2026-11-06)" {
		t.Errorf("item 0 = %v", first)
	}
	if items[1].(map[string]any)["icon_key"] != "trend" {
		t.Errorf("item 1 = %v", items[1])
	}
	if len(m["gaps"].([]any)) != 0 {
		t.Errorf("gaps = %v", m["gaps"])
	}
	if m["data_times"] != "Newest reading day 2026-11-06; alert data day 2026-11-06, 2026-11-05; generated at 2026-11-08T09:00:00+08:00" {
		t.Errorf("data_times = %v", m["data_times"])
	}
}

func TestBlockOkNoAlert(t *testing.T) {
	t.Parallel()
	m := decode(t, build(t, cases[1].res, nil))
	if m["state"] != "ok" || m["say"] != "2026-11-07 No market alerts." || len(m["items"].([]any)) != 0 {
		t.Errorf("block = %v", m)
	}
}

func TestBlockPartialNoData(t *testing.T) {
	t.Parallel()
	m := decode(t, build(t, cases[2].res, nil))
	if m["state"] != "partial" || len(m["items"].([]any)) != 1 {
		t.Fatalf("block = %v", m)
	}
	g := m["gaps"].([]any)
	if len(g) != 1 || g[0].(map[string]any)["text"] != "SPY IV rank high: n/a" || g[0].(map[string]any)["reason"] != "missing_input" {
		t.Errorf("gaps = %v", g)
	}
}

func TestBlockGapAllNoData(t *testing.T) {
	t.Parallel()
	m := decode(t, build(t, cases[3].res, nil))
	if m["state"] != "gap" {
		t.Fatalf("block = %v", m)
	}
	items := m["items"].(map[string]any)
	if items["reason"] != "no_data" || items["text"] == "" {
		t.Errorf("items = %v", items)
	}
	g := m["gaps"].([]any)
	if len(g) != 2 || g[1].(map[string]any)["text"] != "B1: n/a" {
		t.Errorf("gaps = %v", g)
	}
}

func TestBlockStaleDataIsStaleInput(t *testing.T) {
	t.Parallel()
	for reason, want := range map[string]string{
		"stale_data": "stale_input", "missing_input": "missing_input", "no_data_day": "missing_input",
		"no_reading": "no_data", "unit_mismatch": "no_data",
	} {
		res := []compute.AlertResult{clear("a"), noData("r", "Rule", reason)}
		g := decode(t, build(t, res, nil))["gaps"].([]any)
		if got := g[0].(map[string]any)["reason"]; got != want {
			t.Errorf("%s: reason = %v, want %s", reason, got, want)
		}
	}
}

func TestBlockLevelsCarried(t *testing.T) {
	t.Parallel()
	var res []compute.AlertResult
	for _, l := range []string{"n", "w", "d"} {
		res = append(res, fired("r"+l, "Rule"+l, "market.ivr_own", l, "x", "2026-11-06"))
	}
	items := decode(t, build(t, res, nil))["items"].([]any)
	for i, l := range []string{"n", "w", "d"} {
		if items[i].(map[string]any)["level"] != l {
			t.Errorf("item %d level = %v", i, items[i])
		}
	}
	bad := fired("r", "Rule", "market.ivr_own", "x", "x", "2026-11-06")
	in := Input{
		Evaluate: func(context.Context, compute.AlertRules, compute.ReadingFunc, contracts.Day) ([]compute.AlertResult, error) {
			return []compute.AlertResult{bad}, nil
		},
		Read: func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
			return nil, nil
		},
		Today: "2026-11-07", GeneratedAt: at(),
	}
	if _, err := Build(context.Background(), in); err == nil {
		t.Error("unknown level accepted")
	}
}

func TestBlockRequiredFieldsPresent(t *testing.T) {
	t.Parallel()
	for _, c := range cases {
		m := decode(t, build(t, c.res, nil))
		for _, k := range []string{"key", "title", "say", "sub", "data_times", "state", "gaps", "items"} {
			if v, ok := m[k]; !ok || v == nil || v == "" {
				t.Errorf("%s: field %s = %v", c.name, k, v)
			}
		}
	}
	m := decode(t, build(t, nil, nil))
	if _, ok := m["gaps"].([]any); !ok {
		t.Errorf("gaps of an empty rule set = %v", m["gaps"])
	}
}

func schema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	path := filepath.Join("..", "..", "..", "contracts", "schemas", "page.report_page.v2.json")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	const id = "https://option-trader.invalid/schemas/report_page.v2.json"
	c := jsonschema.NewCompiler()
	if err := c.AddResource(id, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(id + "#/$defs/RisksBlock")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestBlockMatchesSchema(t *testing.T) {
	t.Parallel()
	s := schema(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b := build(t, c.res, nil)
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Validate(doc); err != nil {
				t.Fatalf("%v\n%s", err, b)
			}
			want, err := os.ReadFile(filepath.Join("testdata", c.name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var w, g any
			if err := json.Unmarshal(want, &w); err != nil {
				t.Fatal(err)
			}
			_ = json.Unmarshal(b, &g)
			if !reflect.DeepEqual(w, g) {
				t.Errorf("got %s\nwant %s", b, want)
			}
		})
	}
}

func TestBlockReaderErrorIsError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	build(t, nil, boom)
	in := Input{
		Rules: compute.AlertRules{Rules: []compute.AlertRule{{ID: "r"}}},
		Evaluate: func(ctx context.Context, _ compute.AlertRules, read compute.ReadingFunc, d contracts.Day) ([]compute.AlertResult, error) {
			_, err := read(ctx, "f", "market", "S", d, d)
			return nil, err
		},
		Read: func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
			return nil, boom
		},
		Today: "2026-11-07", GeneratedAt: at(),
	}
	if _, err := Build(context.Background(), in); !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}
}

func TestBlockManyFiredStaysInSchema(t *testing.T) {
	t.Parallel()
	var res []compute.AlertResult
	for i := 0; i < 16; i++ {
		res = append(res, fired(fmt.Sprintf("r%d", i), "Title", "market.vix_pctile_3y", "w", "d", fmt.Sprintf("2026-11-%02d", i+1)))
	}
	res = append(res, fired("long", "Title", "market.ivr_own", "w", strings.Repeat("x", 300), "2026-11-06"))
	s := schema(t)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(build(t, res, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(doc); err != nil {
		t.Fatalf("many fired rules break the schema: %v", err)
	}
}
