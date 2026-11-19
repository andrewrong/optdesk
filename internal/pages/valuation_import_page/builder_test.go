package valuation_import_page

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

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

func validate(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile(schemaHost + "page.valuation_import_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(inst); err != nil {
		t.Fatalf("schema: %v\n%s", err, b)
	}
}

// samplePlan holds invented companies and numbers.
func samplePlan() valueimportapi.Plan {
	return valueimportapi.Plan{
		FileSHA256: strings.Repeat("ab", 32),
		AsOf:       "2026-01-15",
		R:          valueimportapi.RPlan{Action: "insert", US: "0.08", JP: "0.07", HKCN: "0.09", Note: "invented note"},
		Margin: valueimportapi.MarginPlan{Action: "same", Rows: []valuelinesapi.MarginRow{
			{PFrom: "4.5", M: "0.10", W: "0.15", H: "0.20", S: "0.25"},
			{PFrom: "3.0", M: "0.15", W: "0.20", H: "0.25", S: "0.30"},
		}},
		Rows: []valueimportapi.RowPlan{
			{Key: "US:ZZA", Ticker: "ZZA", Name: "Zeta Alpha Corp", Currency: "USD", RMarket: "US",
				Action: "insert", ReviewType: "event", V: "120.00", P: "4.5", VersionDate: "2026-01-10",
				FileBuyTop: "96.00", SiteBuyTop: "96.10"},
			{Key: "JP:9901", Ticker: "9901.T", Name: "Yamabiko Example KK", Currency: "JPY", RMarket: "JP",
				Action: "same", V: "3400.00", P: "4.0", VersionDate: "2025-12-20", FileBuyTop: "2600", SiteBuyTop: "2600"},
			{Ticker: "QQB", Name: "Quartz Beta Ltd", Currency: "HKD", RMarket: "HKCN", NewProfile: true,
				Unmatched: true, ProposedKey: "HK:0999", Action: "refused",
				Reason: "the site computes another buy-zone top", V: "55.00", P: "3.5", VersionDate: "2026-01-12",
				FileBuyTop: "40.00", SiteBuyTop: "44.00", NeedsTick: true},
			{Key: "US:ZZC", Ticker: "ZZC", Name: "Zenith Gamma Inc", Currency: "USD", RMarket: "US",
				Action: "older", V: "80.00", P: "4.0", VersionDate: "2025-11-01", FileBuyTop: "60.00", SiteBuyTop: "60.00"},
			{Ticker: "NNE", Name: "Nova Epsilon Ltd", Currency: "HKD", RMarket: "HKCN", NewProfile: true,
				Unmatched: true, ProposedKey: "HK:0555", Action: "insert", ReviewType: "first", V: "20.00", P: "4.0",
				VersionDate: "2026-01-11", FileBuyTop: "15.00", SiteBuyTop: "15.00", NeedsTick: true},
		},
		NotInFile: []string{"US:ZZD"}, Unvalued: []string{"ZZE"}, RemovedNames: []string{"ZZF"}, FlaggedCount: 2,
	}
}

func previewInput() Input {
	return Input{View: ViewPreview, Plan: samplePlan(), LatestYield: "0.041", FileName: "sample-valuations.json"}
}

func build(t *testing.T, in Input) []byte {
	t.Helper()
	b, err := Build(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	return b
}

func sameJSON(t *testing.T, got []byte, path string) {
	t.Helper()
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("output differs from %s:\n%s", path, got)
	}
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestBuildEmptyView(t *testing.T) {
	t.Parallel()
	b := build(t, Input{View: ViewEmpty, FileName: "ignored.json", Plan: samplePlan()})
	m := decode(t, b)
	if m["view"] != "empty" || m["file"] != nil || m["r"] != nil || m["margin"] != nil || m["post_path"] != "valuation-import-post" {
		t.Errorf("empty view: %s", b)
	}
	if len(m["rows"].([]any)) != 0 || len(m["result"].([]any)) != 0 {
		t.Errorf("empty view has rows: %s", b)
	}
}

func TestBuildPreviewMatchesFixture(t *testing.T) {
	t.Parallel()
	sameJSON(t, build(t, previewInput()), "testdata/preview.json")
}

func TestBuildResultMatchesFixture(t *testing.T) {
	t.Parallel()
	in := previewInput()
	in.View = ViewResult
	in.Applied = []Outcome{
		{Key: "US:ZZA", Outcome: OutcomeInserted},
		{Key: "JP:9901", Outcome: OutcomeSame},
		{Key: "QQB", Outcome: OutcomeRefused, Text: "the site computes another buy-zone top"},
		{Key: "US:ZZC", Outcome: OutcomeOlder},
		{Key: "NNE", Outcome: OutcomeNotConfirmed},
	}
	sameJSON(t, build(t, in), "testdata/result.json")
}

func TestBuildCountsEqualRows(t *testing.T) {
	t.Parallel()
	m := decode(t, build(t, previewInput()))
	got := map[string]int{}
	ticks := 0
	for _, r := range m["rows"].([]any) {
		row := r.(map[string]any)
		got[row["action"].(string)]++
		if row["needs_tick"] == true {
			ticks++
		}
	}
	c := m["counts"].(map[string]any)
	for _, a := range []string{"insert", "same", "older", "refused"} {
		if int(c[a].(float64)) != got[a] {
			t.Errorf("counts.%s = %v, rows say %d", a, c[a], got[a])
		}
	}
	if int(c["needs_tick"].(float64)) != ticks {
		t.Errorf("counts.needs_tick = %v, rows say %d", c["needs_tick"], ticks)
	}
}

func TestBuildStepTwoFromStepOnePayload(t *testing.T) {
	t.Parallel()
	var step1 struct {
		File struct {
			SHA256 string `json:"sha256"`
		} `json:"file"`
		Rows []struct {
			Ticker      string `json:"ticker"`
			ProposedKey string `json:"proposed_key"`
			NeedsTick   bool   `json:"needs_tick"`
			Action      string `json:"action"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(build(t, previewInput()), &step1); err != nil {
		t.Fatal(err)
	}
	ticked := map[string]bool{}
	picks := map[string]string{}
	for _, r := range step1.Rows {
		if r.NeedsTick {
			ticked[r.Ticker] = true
			picks[r.Ticker] = r.ProposedKey
		}
	}
	if len(ticked) != 2 || picks["NNE"] != "HK:0555" || step1.File.SHA256 != strings.Repeat("ab", 32) {
		t.Fatalf("payload read wrong: %v %v %q", ticked, picks, step1.File.SHA256)
	}
	// The confirm writes what is ticked; the rest is listed.
	var applied []Outcome
	for _, r := range step1.Rows {
		o := Outcome{Key: r.Ticker, Outcome: OutcomeNotConfirmed}
		if r.Action == "insert" && (!r.NeedsTick || ticked[r.Ticker]) {
			o.Outcome = OutcomeInserted
			if k := picks[r.Ticker]; k != "" {
				o.Key = k
			}
		}
		applied = append(applied, o)
	}
	in := previewInput()
	in.View = ViewResult
	in.Applied = applied
	m := decode(t, build(t, in))
	if m["view"] != "result" {
		t.Fatalf("view %v", m["view"])
	}
	res := m["result"].([]any)
	if len(res) != len(step1.Rows) || res[4].(map[string]any)["key"] != "HK:0555" || res[4].(map[string]any)["outcome"] != "inserted" {
		t.Errorf("result: %v", res)
	}
}

func TestBuildFileNameCutAndNoPath(t *testing.T) {
	t.Parallel()
	in := previewInput()
	in.FileName = `C:\Users\x/../` + strings.Repeat("é", 80) + ".json"
	f := decode(t, build(t, in))["file"].(map[string]any)
	name := f["name"].(string)
	if strings.ContainsAny(name, `/\`) || len([]rune(name)) != 64 {
		t.Errorf("name %q", name)
	}
}

func TestBuildUnmatchedAndOlderRows(t *testing.T) {
	t.Parallel()
	rows := decode(t, build(t, previewInput()))["rows"].([]any)
	older := rows[3].(map[string]any)
	if older["reason"] != TextOlder {
		t.Errorf("older reason %v", older["reason"])
	}
	un := rows[4].(map[string]any)
	if un["unmatched"] != true || un["proposed_key"] != "HK:0555" || un["review_type"] != TextFirst {
		t.Errorf("unmatched row %v", un)
	}
}

func TestEveryReasonHasChineseHint(t *testing.T) {
	t.Parallel()
	for _, s := range Reasons {
		if !strings.ContainsFunc(s, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
			t.Errorf("no Chinese hint: %q", s)
		}
	}
	for en, zh := range hinted {
		if !strings.HasPrefix(zh, en) {
			t.Errorf("sentence %q does not keep %q", zh, en)
		}
	}
}

func TestOlderReasonFromImporterKeepsHint(t *testing.T) {
	t.Parallel()
	in := previewInput()
	in.Plan.Rows[3].Reason = "older than stored"
	in.Plan.Rows[4].Reason = "unmatched name"
	rows := decode(t, build(t, in))["rows"].([]any)
	if got := rows[3].(map[string]any)["reason"]; got != TextOlder {
		t.Errorf("reason %v, want %q", got, TextOlder)
	}
	if got := rows[4].(map[string]any)["reason"]; got != TextUnmatched {
		t.Errorf("reason %v, want %q", got, TextUnmatched)
	}
}

func TestBuildLabelsInEveryView(t *testing.T) {
	t.Parallel()
	res := previewInput()
	res.View = ViewResult
	empty := Input{View: ViewEmpty}
	for name, in := range map[string]Input{"empty": empty, "preview": previewInput(), "result": res} {
		labels, ok := decode(t, build(t, in))["labels"].(map[string]any)
		if !ok {
			t.Fatalf("%s: no labels", name)
		}
		want := map[string]string{
			"r": TextR, "buy_zone_top_site": TextBuyZoneTopSite,
			"buy_zone_top_file": TextBuyZoneTopFile, "general_tick": TextGeneralTick,
		}
		for k, v := range newLabels() {
			want[k] = v
		}
		if len(labels) != len(want) {
			t.Errorf("%s: labels %v", name, labels)
		}
		for k, v := range want {
			if labels[k] != v {
				t.Errorf("%s: labels[%s] = %v, want %q", name, k, labels[k], v)
			}
		}
	}
}

func newLabels() map[string]string {
	return map[string]string{
		"title":             TextTitle,
		"intro":             TextIntro,
		"file":              TextFile,
		"preview":           TextPreview,
		"not_in_file":       TextNotInFile,
		"unvalued":          TextUnvalued,
		"removed":           TextRemoved,
		"count_insert":      TextCountInsert,
		"count_same":        TextCountSame,
		"count_older":       TextCountOlder,
		"count_refused":     TextCountRefused,
		"count_needs_tick":  TextCountNeedsTick,
		"stored":            TextStored,
		"older_than_stored": TextOlderThanStored,
		"margin_table":      TextMarginTable,
		"result":            TextResult,
		"same_file_again":   TextSameFileAgain,
		"confirm":           TextConfirm,
	}
}

func TestBuildEmptyViewLabels(t *testing.T) {
	t.Parallel()
	b := build(t, Input{View: ViewEmpty})
	labels, ok := decode(t, b)["labels"].(map[string]any)
	if !ok {
		t.Fatalf("no labels: %s", b)
	}
	for k, v := range newLabels() {
		if labels[k] != v {
			t.Errorf("labels[%s] = %v, want %q", k, labels[k], v)
		}
	}
	if len(newLabels()) != 18 {
		t.Errorf("want 18 new labels, have %d", len(newLabels()))
	}
	if !strings.Contains(TextBuyZoneTopSite, "买入区间上沿") || !strings.Contains(TextBuyZoneTopFile, "买入区间上沿") {
		t.Errorf("buy-zone labels lack the hint")
	}
}
