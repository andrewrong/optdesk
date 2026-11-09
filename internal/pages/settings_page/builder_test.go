package settings_page

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

const macText = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

var (
	zone    = time.FixedZone("CST", 8*3600)
	fixedAt = time.Date(2026, 11, 9, 2, 0, 0, 0, time.UTC)
)

type fakeParams struct{ specs []settingsapi.RuleParamSpec }

func (f fakeParams) Specs() []settingsapi.RuleParamSpec { return f.specs }
func (fakeParams) Normalize(_, text string) (string, error) {
	return text, nil
}
func (fakeParams) Resolve(m map[string]string) map[string]string { return m }

type fakeOwner struct {
	params []ownerset.Param
	excl   []ownerset.Exclusion
	hist   []ownerset.HistoryRow
	err    error
	atSeen time.Time
	nSeen  int
}

func (f *fakeOwner) Params(_ context.Context, at time.Time) ([]ownerset.Param, error) {
	f.atSeen = at
	return f.params, f.err
}
func (f *fakeOwner) Exclusions(context.Context, time.Time) ([]ownerset.Exclusion, error) {
	return f.excl, nil
}
func (f *fakeOwner) History(_ context.Context, n int) ([]ownerset.HistoryRow, error) {
	f.nSeen = n
	return f.hist, nil
}
func (*fakeOwner) Version(context.Context, time.Time) (int64, error) { return 0, nil }
func (*fakeOwner) Close() error                                      { return nil }

type fakeCatalog struct{ ids []int64 }

func (f fakeCatalog) OptionEodLatest(context.Context, contracts.Day) ([]store.OptionEodLatest, error) {
	var out []store.OptionEodLatest
	for _, id := range f.ids {
		out = append(out, store.OptionEodLatest{UnderlyingID: id, SessionDay: "2026-11-06"})
	}
	return out, nil
}

type fakeNames map[int64]string

func (f fakeNames) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, id := range ids {
		if n, ok := f[id]; ok {
			out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: n})
		}
	}
	return out, nil
}

var specs = []settingsapi.RuleParamSpec{
	{Key: "min_dte", Label: "Minimum days to expiry", Unit: "days", Kind: "int", Default: "21", Min: "7", Max: "120"},
	{Key: "max_spread", Label: "Maximum spread", Unit: "USD", Kind: "decimal", Default: "0.1", Min: "0.01", Max: "0.3"},
}

func baseConfig() Config {
	return Config{
		RunID: "run-1", BuildID: "b1", Scope: "all", Zone: zone,
		Params:  fakeParams{specs: specs},
		Owner:   &fakeOwner{},
		Catalog: fakeCatalog{},
		Names:   fakeNames{},
	}
}

func build(t *testing.T, c Config) map[string]any {
	t.Helper()
	b, err := Build(t.Context(), c, nil, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateBytes(t, b); err != nil {
		t.Fatalf("schema: %v\n%s", err, b)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func list(m map[string]any, k string) []map[string]any {
	var out []map[string]any
	for _, x := range m[k].([]any) {
		out = append(out, x.(map[string]any))
	}
	return out
}

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	const name = "page.settings_page.v2.json"
	f, err := os.Open(filepath.Join("..", "..", "..", "contracts", "schemas", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	const base = "https://optdesk.invalid/schemas/"
	if err := c.AddResource(base+name, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(base + name)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validateBytes(t *testing.T, b []byte) error {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return compileSchema(t).Validate(v)
}

func TestBuildFillsOneRowPerKey(t *testing.T) {
	t.Parallel()
	c := baseConfig()
	o := &fakeOwner{params: []ownerset.Param{{Key: "max_spread", Value: "0.15"}, {Key: "unknown", Value: "9"}}}
	c.Owner = o
	rows := list(build(t, c), "params")
	if len(rows) != 2 || rows[0]["key"] != "min_dte" || rows[1]["key"] != "max_spread" {
		t.Fatalf("rows = %v", rows)
	}
	if rows[0]["value"] != "" || rows[1]["value"] != "0.15" {
		t.Fatalf("values = %v", rows)
	}
	if !o.atSeen.Equal(fixedAt) {
		t.Fatalf("read at %v", o.atSeen)
	}
}

func TestBuildShowsDefaultsAndBounds(t *testing.T) {
	t.Parallel()
	r := list(build(t, baseConfig()), "params")[0]
	want := map[string]any{"label": "Minimum days to expiry", "unit": "days", "kind": "int", "default": "21", "min": "7", "max": "120", "value": ""}
	for k, v := range want {
		if r[k] != v {
			t.Errorf("%s = %v, want %v", k, r[k], v)
		}
	}
}

func TestBuildExclusionRowsSorted(t *testing.T) {
	t.Parallel()
	c := baseConfig()
	c.Catalog = fakeCatalog{ids: []int64{3, 5, 9}}
	c.Names = fakeNames{3: "AAA", 5: "BBB", 9: "CCC"}
	c.Owner = &fakeOwner{excl: []ownerset.Exclusion{{InstrumentID: 5, Excluded: true}}}
	rows := list(build(t, c), "rows")
	if len(rows) != 3 {
		t.Fatalf("rows = %v", rows)
	}
	for i, w := range []struct {
		id  float64
		sym string
		ex  bool
	}{{3, "AAA", false}, {5, "BBB", true}, {9, "CCC", false}} {
		if rows[i]["instrument_id"] != w.id || rows[i]["symbol"] != w.sym || rows[i]["excluded"] != w.ex {
			t.Errorf("row %d = %v", i, rows[i])
		}
	}
}

func TestBuildKeepsFlaggedInstrumentWithoutChain(t *testing.T) {
	t.Parallel()
	c := baseConfig()
	c.Catalog = fakeCatalog{ids: []int64{3, 9}}
	c.Names = fakeNames{3: "AAA", 9: "CCC"}
	c.Owner = &fakeOwner{excl: []ownerset.Exclusion{{InstrumentID: 5, Excluded: true}}}
	rows := list(build(t, c), "rows")
	if len(rows) != 3 || rows[1]["instrument_id"] != float64(5) || rows[1]["excluded"] != true || rows[1]["symbol"] != "#5" {
		t.Fatalf("rows = %v", rows)
	}
}

func TestBuildReadOnlyHasNullForm(t *testing.T) {
	t.Parallel()
	m := build(t, baseConfig())
	if m["form"] != nil {
		t.Fatalf("form = %v", m["form"])
	}
	g := m["gap"].(map[string]any)
	if g["reason"] != "not_built" || g["text"] == "" {
		t.Fatalf("gap = %v", g)
	}
	c := baseConfig()
	c.Form = &settingsapi.FormToken{Expires: "1789000000", MAC: macText}
	m = build(t, c)
	f := m["form"].(map[string]any)
	if f["post_path"] != "/owner-settings" || f["expires"] != "1789000000" || f["mac"] != macText || m["gap"] != nil {
		t.Fatalf("form = %v gap = %v", f, m["gap"])
	}
}

func TestBuildConfirmLaidOut(t *testing.T) {
	t.Parallel()
	c := baseConfig()
	c.Confirm = &Confirm{
		Rows: []ConfirmRow{{Label: "Minimum days to expiry", Old: "21", New: "30"}, {Label: "AAA", Old: "off", New: "on"}},
		Form: settingsapi.FormToken{Expires: "1789000100", MAC: macText},
	}
	m := build(t, c)
	cf := m["confirm"].(map[string]any)
	rows := list(cf, "rows")
	if len(rows) != 2 || rows[0]["old"] != "21" || rows[0]["new"] != "30" || rows[1]["label"] != "AAA" {
		t.Fatalf("rows = %v", rows)
	}
	if f := cf["form"].(map[string]any); f["expires"] != "1789000100" || f["mac"] != macText {
		t.Fatalf("form = %v", f)
	}
	if build(t, baseConfig())["confirm"] != nil {
		t.Fatal("confirm without a Confirm")
	}
}

func TestBuildHistoryNewestFirst(t *testing.T) {
	t.Parallel()
	c := baseConfig()
	c.Names = fakeNames{7: "AAA"}
	o := &fakeOwner{hist: []ownerset.HistoryRow{
		{Kind: "exclusion", InstrumentID: 7, Excluded: true, SetAt: time.Date(2026, 11, 8, 4, 0, 0, 0, time.UTC)},
		{Kind: "param", Key: "min_dte", Value: "30", SetAt: time.Date(2026, 11, 7, 4, 0, 0, 0, time.UTC)},
	}}
	c.Owner = o
	c.HistoryN = 50
	h := list(build(t, c), "history")
	if o.nSeen != 20 {
		t.Fatalf("asked for %d rows", o.nSeen)
	}
	if len(h) != 2 || h[0]["kind"] != "exclusion" || h[1]["kind"] != "param" {
		t.Fatalf("history = %v", h)
	}
	if h[0]["at"] != "2026-11-08T12:00:00+08:00" || h[0]["name"] != "AAA" || h[0]["instrument_id"] != float64(7) ||
		h[0]["excluded"] != true || h[0]["key"] != nil {
		t.Fatalf("exclusion row = %v", h[0])
	}
	if h[1]["key"] != "min_dte" || h[1]["value"] != "30" || h[1]["name"] != "Minimum days to expiry" || h[1]["instrument_id"] != nil {
		t.Fatalf("param row = %v", h[1])
	}
	c.HistoryN = 0
	o.nSeen = -1
	if got := list(build(t, c), "history"); len(got) != 0 || o.nSeen != -1 {
		t.Fatalf("history read with HistoryN 0: %v", got)
	}
}

func TestBuildNilOwnerIsGap(t *testing.T) {
	t.Parallel()
	c := baseConfig()
	c.Owner = nil
	c.Form = &settingsapi.FormToken{Expires: "1", MAC: macText}
	m := build(t, c)
	g := m["gap"].(map[string]any)
	if g["reason"] != "no_reader" || m["run_status"] != "partial" {
		t.Fatalf("gap = %v status = %v", g, m["run_status"])
	}
	if len(list(m, "params")) != 0 || len(list(m, "rows")) != 0 || m["form"] != nil {
		t.Fatalf("page carries data without a reader: %v", m)
	}
}

func TestBuildNoticeKinds(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"saved", "nothing_changed", "error"} {
		c := baseConfig()
		c.Notice = &Notice{Kind: k, Text: "text of " + k}
		n := build(t, c)["notice"].(map[string]any)
		if n["kind"] != k || n["text"] != "text of "+k {
			t.Errorf("notice = %v", n)
		}
	}
	if build(t, baseConfig())["notice"] != nil {
		t.Fatal("notice without a Notice")
	}
	c := baseConfig()
	c.Notice = &Notice{Kind: "weird", Text: "x"}
	if _, err := Build(t.Context(), c, nil, "2026-11-06", fixedAt); err == nil {
		t.Fatal("bad notice kind built")
	}
}

func TestBuildValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	c := baseConfig()
	c.Catalog = fakeCatalog{ids: []int64{3}}
	c.Names = fakeNames{3: "AAA"}
	c.Form = &settingsapi.FormToken{Expires: "1789000000", MAC: macText}
	c.HistoryN = 5
	c.Owner = &fakeOwner{hist: []ownerset.HistoryRow{{Kind: "param", Key: "min_dte", Value: "30", SetAt: fixedAt}}}
	c.Notice = &Notice{Kind: "saved", Text: "Saved 1 change."}
	build(t, c) // validates inside
	for _, bad := range []func(*Config){
		func(c *Config) { c.Zone = nil },
		func(c *Config) { c.RunID = "bad id" },
		func(c *Config) { c.Params = nil },
	} {
		cc := c
		bad(&cc)
		if _, err := Build(t.Context(), cc, nil, "2026-11-06", fixedAt); err == nil {
			t.Error("bad config built")
		}
	}
	boom := errors.New("boom")
	cc := c
	cc.Owner = &fakeOwner{err: boom}
	if _, err := Build(t.Context(), cc, nil, "2026-11-06", fixedAt); !errors.Is(err, boom) {
		t.Fatalf("reader error = %v", err)
	}
}

func TestBuildRefusesMoreRowsThanTheSchemaHolds(t *testing.T) {
	t.Parallel()
	ids := make([]int64, 1001)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	c := baseConfig()
	c.Catalog = fakeCatalog{ids: ids}
	if _, err := Build(t.Context(), c, nil, "2026-11-06", fixedAt); err == nil {
		t.Fatal("1001 underlyings built, a flag could be cut silently")
	}
}
