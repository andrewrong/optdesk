package report_page

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const day = contracts.Day("2026-11-07")

var (
	zone = time.FixedZone("test", 8*3600)
	at   = time.Date(2026, 11, 8, 1, 0, 0, 0, time.UTC)
)

func riskFile(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "risks_"+name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func cfg(market, slot string, risks []byte) Config {
	return Config{Market: market, Slot: slot, Zone: zone,
		Risks: func(context.Context, contracts.Day, time.Time) ([]byte, error) { return risks, nil }}
}

func build(t *testing.T, c Config) map[string]any {
	t.Helper()
	b, err := Build(context.Background(), c, day, at)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func blocksOf(m map[string]any) map[string]any { return m["blocks"].(map[string]any) }

func checkEnvelope(t *testing.T, m map[string]any, market, slot string) {
	t.Helper()
	want := map[string]any{"schema": "report_page", "schema_version": float64(2), "market": market, "slot": slot,
		"session_date": "2026-11-07", "generated_at": "2026-11-08T09:00:00+08:00"}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, want %v", k, m[k], v)
		}
	}
	if sl, ok := m["status_lines"].([]any); !ok || len(sl) != 0 {
		t.Errorf("status_lines = %v", m["status_lines"])
	}
	if v, ok := m["stale"]; !ok || v != nil {
		t.Errorf("stale = %v, want null", v)
	}
	for _, k := range []string{"title", "say"} {
		if s, _ := m[k].(string); s == "" {
			t.Errorf("%s empty", k)
		}
	}
	bn := m["banner"].(map[string]any)
	if bn["left"] == "" || bn["right"] == "" {
		t.Errorf("banner = %v", bn)
	}
}

func TestBuildCloseUS(t *testing.T) {
	t.Parallel()
	checkEnvelope(t, build(t, cfg("US", "close", riskFile(t, "gap"))), "US", "close")
}

func TestBuildOpenJP(t *testing.T) {
	t.Parallel()
	checkEnvelope(t, build(t, cfg("JP", "open", riskFile(t, "gap"))), "JP", "open")
}

func TestBuildRisksBytesEmbedded(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"ok_with_items", "partial", "gap"} {
		raw := riskFile(t, n)
		var want any
		if err := json.Unmarshal(raw, &want); err != nil {
			t.Fatal(err)
		}
		got := blocksOf(build(t, cfg("US", "close", raw)))["risks"]
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: risks = %v, want %v", n, got, want)
		}
	}
}

func TestBuildRisksFuncGetsDayAndTime(t *testing.T) {
	t.Parallel()
	var gotDay contracts.Day
	var gotAt time.Time
	c := Config{Market: "US", Slot: "close", Zone: zone,
		Risks: func(_ context.Context, d contracts.Day, g time.Time) ([]byte, error) {
			gotDay, gotAt = d, g
			return []byte(`{}`), nil
		}}
	if _, err := Build(context.Background(), c, day, at); err != nil {
		t.Fatal(err)
	}
	if gotDay != day || !gotAt.Equal(at) {
		t.Errorf("risks got %v %v", gotDay, gotAt)
	}
}

// gapFields lists the required fields of a block from the schema that may be a Gap.
func gapFields(t *testing.T, def string) (required []string, gapOK map[string]bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", "page.report_page.v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Defs map[string]struct {
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	d := doc.Defs[def]
	gapOK = map[string]bool{}
	for k, p := range d.Properties {
		if bytes.Contains(p, []byte(`"#/$defs/Gap"`)) {
			gapOK[k] = true
		}
	}
	return d.Required, gapOK
}

var gapBlockDefs = map[string]string{
	"actions": "ActionsBlock", "account_moves": "AccountMovesBlock",
	"market_watch": "MarketWatchBlock", "global_assets": "GlobalAssetsBlock",
}

func TestBuildGapBlocks(t *testing.T) {
	t.Parallel()
	bl := blocksOf(build(t, cfg("US", "close", riskFile(t, "gap"))))
	for key, def := range gapBlockDefs {
		b := bl[key].(map[string]any)
		if b["key"] != key {
			t.Errorf("%s: key = %v", key, b["key"])
		}
		for _, f := range []string{"title", "sub", "say", "data_times"} {
			if s, _ := b[f].(string); s == "" {
				t.Errorf("%s.%s empty", key, f)
			}
		}
		if b["state"] != "gap" {
			t.Errorf("%s: state = %v", key, b["state"])
		}
		gs := b["gaps"].([]any)
		if len(gs) != 1 || gs[0].(map[string]any)["reason"] != "not_built" {
			t.Errorf("%s: gaps = %v", key, gs)
		}
		required, gapOK := gapFields(t, def)
		for _, f := range required {
			if !gapOK[f] || f == "gaps" {
				continue
			}
			g, ok := b[f].(map[string]any)
			if !ok || g["reason"] != "not_built" || g["text"] == "" {
				t.Errorf("%s.%s = %v, want a not_built Gap", key, f, b[f])
			}
		}
	}
	if s, _ := bl["global_assets"].(map[string]any)["chart_aria"].(string); s == "" {
		t.Error("chart_aria empty")
	}
}

func TestBuildGapBlockRequiredKeys(t *testing.T) {
	t.Parallel()
	bl := blocksOf(build(t, cfg("JP", "open", riskFile(t, "gap"))))
	for key := range gapBlockDefs {
		b := bl[key].(map[string]any)
		for _, f := range []string{"key", "sub", "say", "data_times"} {
			if _, ok := b[f]; !ok {
				t.Errorf("%s lacks %s", key, f)
			}
		}
	}
	if _, ok := bl["global_assets"].(map[string]any)["chart_aria"]; !ok {
		t.Error("global_assets lacks chart_aria")
	}
}

func TestBuildNoOptionalBlocks(t *testing.T) {
	t.Parallel()
	for _, slot := range []string{"open", "close"} {
		bl := blocksOf(build(t, cfg("US", slot, riskFile(t, "gap"))))
		if len(bl) != 5 {
			t.Errorf("%s: %d blocks, want 5", slot, len(bl))
		}
		for _, k := range []string{"accounts", "candidates", "market_line", "reminders", "system_status", "tiles"} {
			if _, ok := bl[k]; ok {
				t.Errorf("%s: optional block %s present", slot, k)
			}
		}
	}
}

func TestBuildGeneratedAtInZone(t *testing.T) {
	t.Parallel()
	m := build(t, cfg("US", "close", riskFile(t, "gap")))
	got := m["generated_at"].(string)
	if !strings.HasSuffix(got, "+08:00") || got != "2026-11-08T09:00:00+08:00" {
		t.Errorf("generated_at = %s", got)
	}
}

func TestBuildMatchesSchema(t *testing.T) {
	t.Parallel()
	f, err := os.Open(filepath.Join("..", "..", "..", "contracts", "schemas", "page.report_page.v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	const id = "https://option-trader.invalid/schemas/report_page.v2.json"
	comp := jsonschema.NewCompiler()
	if err := comp.AddResource(id, doc); err != nil {
		t.Fatal(err)
	}
	s, err := comp.Compile(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, market := range []string{"US", "JP"} {
		for _, slot := range []string{"open", "close"} {
			for _, n := range []string{"ok_with_items", "partial", "gap"} {
				b, err := Build(context.Background(), cfg(market, slot, riskFile(t, n)), day, at)
				if err != nil {
					t.Fatal(err)
				}
				v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
				if err != nil {
					t.Fatal(err)
				}
				if err := s.Validate(v); err != nil {
					t.Errorf("%s %s %s: %v\n%s", market, slot, n, err, b)
				}
			}
		}
	}
}

func TestBuildBadMarketOrSlot(t *testing.T) {
	t.Parallel()
	risks := riskFile(t, "gap")
	for _, c := range []Config{cfg("EU", "open", risks), cfg("us", "open", risks), cfg("", "close", risks),
		cfg("US", "noon", risks), cfg("JP", "OPEN", risks), cfg("US", "", risks)} {
		if _, err := Build(context.Background(), c, day, at); err == nil {
			t.Errorf("Build(%q, %q) returned no error", c.Market, c.Slot)
		}
	}
}

func TestBuildRisksErrorIsError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	c := Config{Market: "US", Slot: "close", Zone: zone,
		Risks: func(context.Context, contracts.Day, time.Time) ([]byte, error) { return nil, boom }}
	if _, err := Build(context.Background(), c, day, at); !errors.Is(err, boom) {
		t.Errorf("err = %v, want it to wrap boom", err)
	}
	c.Risks = func(context.Context, contracts.Day, time.Time) ([]byte, error) { return []byte("{bad"), nil }
	if _, err := Build(context.Background(), c, day, at); err == nil {
		t.Error("invalid risks JSON returned no error")
	}
}

func TestNewPageMeta(t *testing.T) {
	t.Parallel()
	p := NewPage(cfg("US", "close", riskFile(t, "gap")))
	if p.Meta.Name != "report_page" || p.Meta.Schema != "page.report_page.v2" {
		t.Errorf("meta = %+v", p.Meta)
	}
	b, err := p.Build(context.Background(), nil, day, at)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := Build(context.Background(), cfg("US", "close", riskFile(t, "gap")), day, at)
	if !bytes.Equal(b, want) {
		t.Error("page Build differs from Build")
	}
}
