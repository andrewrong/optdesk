package option_candidates_page

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/store"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
	buildDay   = contracts.Day("2026-11-09")
)

// fakeReader holds rows by "day/kind"; a day or kind without rows gives an empty slice.
type fakeReader struct {
	rows map[string][]store.Candidate
	err  error
}

func (f fakeReader) Candidates(_ context.Context, day contracts.Day, kind string) ([]store.Candidate, error) {
	if f.err != nil {
		return nil, f.err
	}
	rows := f.rows[string(day)+"/"+kind]
	if rows == nil {
		rows = []store.Candidate{}
	}
	return rows, nil
}

func intp(n int) *int       { return &n }
func strp(s string) *string { return &s }

// cand is a stored row with plain values; tests change the fields they care about.
func cand(kind string, id int, status string) store.Candidate {
	right := "put"
	if kind != "short_put" {
		right = "call"
	}
	c := store.Candidate{
		Kind: kind, InstrumentID: id, Issuer: "ACME", Expiry: "2026-12-18", DTE: 39, Right: right,
		Strike: "95.00", BidEod: "1.20", BreakEven: "93.80", AnnualizedYield: strp("0.123456"),
		Delta: strp("-0.25"), Iv: strp("0.31"), Status: status, Gates: "[]", Marks: "[]", Fits: "{}",
		EarningsState: "listed", RuleVersion: "r1",
		ComputedAt: time.Date(2026, 11, 9, 21, 30, 0, 0, time.UTC),
	}
	if status == "ok" {
		c.Rank = intp(id)
	}
	return c
}

type outCell struct {
	Kind   string       `json:"kind"`
	Reason string       `json:"reason"`
	Text   string       `json:"text"`
	Value  *json.Number `json:"value"`
	Badges []string     `json:"badges"`
	Tone   string       `json:"tone"`
}

type outRow struct {
	ID      string             `json:"id"`
	Cells   map[string]outCell `json:"cells"`
	Reasons []reason           `json:"reasons"`
}

type outTable struct {
	Columns []column `json:"columns"`
	Rows    []outRow `json:"rows"`
}

func (t *outTable) ids() []string {
	out := []string{}
	for _, c := range t.Columns {
		out = append(out, c.ID)
	}
	return out
}

type outGroup struct {
	Kind   string    `json:"kind"`
	Day    *string   `json:"day"`
	Stale  bool      `json:"stale"`
	Counts counts    `json:"counts"`
	Main   *outTable `json:"main"`
	Other  *outTable `json:"other"`
	Gap    *gap      `json:"gap"`
}

type outPage struct {
	ChainDay     *string     `json:"chain_day"`
	ComputedAt   *string     `json:"computed_at"`
	RuleVersion  *string     `json:"rule_version"`
	RunStatus    string      `json:"run_status"`
	Stale        *staleLabel `json:"stale"`
	Gaps         []gap       `json:"gaps"`
	Notes        []string    `json:"notes"`
	EstimateNote string      `json:"estimate_note"`
	Groups       []outGroup  `json:"groups"`
}

func cfg(r store.CandidateReader) Config {
	return Config{RunID: "run1", BuildID: "b1", Scope: "s1", Zone: time.UTC, Candidates: r}
}

func buildBytes(t *testing.T, c Config, day contracts.Day) []byte {
	t.Helper()
	b, err := Build(context.Background(), c, nil, day, time.Date(2026, 11, 9, 22, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func buildPage(t *testing.T, rows map[string][]store.Candidate, day contracts.Day) outPage {
	t.Helper()
	var p outPage
	if err := json.Unmarshal(buildBytes(t, cfg(fakeReader{rows: rows}), day), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func rowIDs(t *outTable) []string {
	out := []string{}
	for _, r := range t.Rows {
		out = append(out, r.ID)
	}
	return out
}

func TestOrderKept(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{cand("short_put", 30, "ok"), cand("short_put", 10, "ok"), cand("short_put", 20, "ok")}
	for i := range rows {
		rows[i].Rank = intp(i + 1)
	}
	p := buildPage(t, map[string][]store.Candidate{"2026-11-09/short_put": rows}, buildDay)
	main := p.Groups[0].Main
	if got := strings.Join(rowIDs(main), ","); got != "cand-short_put-30-0,cand-short_put-10-1,cand-short_put-20-2" {
		t.Fatalf("row ids = %s", got)
	}
	for i, r := range main.Rows {
		if want := fmt.Sprint(i + 1); r.Cells["rank"].Text != want {
			t.Errorf("row %d rank = %q, want %s", i, r.Cells["rank"].Text, want)
		}
	}
}

func TestOkAndOtherSplit(t *testing.T) {
	t.Parallel()
	blocked := cand("short_put", 2, "blocked")
	blocked.Gates = `[{"gate":"events","status":"block","reason":"earnings_before_expiry"}]`
	unknown := cand("short_put", 3, "unknown")
	unknown.Gates = `[{"gate":"chain_fresh","status":"unknown","reason":"stale_chain"}]`
	rows := []store.Candidate{cand("short_put", 1, "ok"), blocked, unknown, cand("short_put", 4, "ok"), cand("short_put", 5, "blocked")}
	p := buildPage(t, map[string][]store.Candidate{"2026-11-09/short_put": rows}, buildDay)
	g := p.Groups[0]
	if got := strings.Join(rowIDs(g.Main), ","); got != "cand-short_put-1-0,cand-short_put-4-1" {
		t.Errorf("main = %s", got)
	}
	if got := strings.Join(rowIDs(g.Other), ","); got != "cand-short_put-2-0,cand-short_put-3-1,cand-short_put-5-2" {
		t.Errorf("other = %s", got)
	}
	if g.Counts != (counts{Ok: 2, Blocked: 2, Unknown: 1}) {
		t.Errorf("counts = %+v", g.Counts)
	}
	for _, r := range g.Main.Rows {
		if len(r.Reasons) != 0 {
			t.Errorf("main row %s has reasons", r.ID)
		}
	}
	if r := g.Other.Rows[0].Reasons; len(r) != 1 || r[0].Status != "block" {
		t.Errorf("blocked reasons = %+v", r)
	}
	if r := g.Other.Rows[1].Reasons; len(r) != 1 || r[0].Status != "unknown" {
		t.Errorf("unknown reasons = %+v", r)
	}
}

func TestNoRunIsGap(t *testing.T) {
	t.Parallel()
	rows := map[string][]store.Candidate{}
	for _, s := range groupSpecs {
		rows["2026-11-01/"+s.kind] = []store.Candidate{cand(s.kind, 1, "ok")}
	}
	p := buildPage(t, rows, buildDay)
	if len(p.Gaps) != 1 || p.Gaps[0].Reason != "no_run" {
		t.Fatalf("gaps = %+v", p.Gaps)
	}
	if p.ChainDay != nil || p.Stale != nil {
		t.Errorf("chain_day = %v, stale = %v", p.ChainDay, p.Stale)
	}
	for _, g := range p.Groups {
		if g.Gap == nil || g.Gap.Reason != "no_rows" || g.Day != nil || g.Main != nil || g.Other != nil {
			t.Errorf("group %s = %+v", g.Kind, g)
		}
	}
}

func TestNilReaderIsGap(t *testing.T) {
	t.Parallel()
	var p outPage
	if err := json.Unmarshal(buildBytes(t, cfg(nil), buildDay), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Gaps) != 1 || p.Gaps[0].Reason != "no_reader" {
		t.Fatalf("gaps = %+v", p.Gaps)
	}
	if len(p.Groups) != 3 || p.Groups[0].Gap == nil || p.Groups[0].Gap.Reason != "no_reader" {
		t.Errorf("groups = %+v", p.Groups)
	}
}

func TestEstimateNote(t *testing.T) {
	t.Parallel()
	p := buildPage(t, nil, buildDay)
	if !strings.Contains(p.EstimateNote, "不是成交价") || len(p.EstimateNote) > 200 {
		t.Errorf("estimate_note = %q", p.EstimateNote)
	}
	rows := map[string][]store.Candidate{"2026-11-09/short_put": {cand("short_put", 1, "ok")}}
	main := buildPage(t, rows, buildDay).Groups[0].Main
	for _, c := range main.Columns {
		if c.ID == "bid" && (c.Label != "Bid (EOD 估)" || c.Hint == nil || *c.Hint != "收盘 bid,下一交易日才卖,不是成交价") {
			t.Errorf("bid column = %+v", c)
		}
	}
}

func TestJPNote(t *testing.T) {
	t.Parallel()
	p := buildPage(t, nil, buildDay)
	if len(p.Notes) != 1 || p.Notes[0] != "日本期权候选:后续设计 (option-candidates-jp)" {
		t.Errorf("notes = %v", p.Notes)
	}
}

func TestStoreErrorFailsBuild(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	_, err := Build(context.Background(), cfg(fakeReader{err: boom}), nil, buildDay, time.Time{})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, err := Build(context.Background(), Config{RunID: "r", BuildID: "b", Scope: "s"}, nil, buildDay, time.Time{}); err == nil {
		t.Error("nil zone gives no error")
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
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	schema, err := c.Compile(schemaHost + "page.option_candidates_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ok_rows", "blocked_rows", "no_rows"} {
		raw, err := os.ReadFile("testdata/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		rows := map[string][]store.Candidate{}
		if err := json.Unmarshal(raw, &rows); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		b := buildBytes(t, cfg(fakeReader{rows: rows}), buildDay)
		v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(v); err != nil {
			t.Errorf("%s: %v\n%s", name, err, b)
		}
	}
}
