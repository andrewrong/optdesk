package home_companies_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/pages/home_companies"
)

const schemaHost = "https://optdesk.invalid/schemas/"

var fixedAt = time.Date(2026, 11, 10, 13, 0, 0, 0, time.UTC)

type key struct{ name, subject string }

// fakeView returns the rows stored under (name, subject), cut to the asked days.
type fakeView struct {
	rows map[key][]contracts.ReadingRecord
	err  error
}

func (f fakeView) WinnerReadings(_ context.Context, name, _, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []contracts.ReadingRecord
	for _, r := range f.rows[key{name, subject}] {
		d := r.Day.Format("2006-01-02")
		if d >= string(from) && d <= string(to) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (fakeView) Close() error { return nil }

func rec(name, subject, day, value, state string) contracts.ReadingRecord {
	r := contracts.ReadingRecord{Name: name, Subject: subject, SubjectKind: "underlying", State: gen.ReadingRecordState(state)}
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		panic(err)
	}
	r.Day.Time = d
	if value != "" {
		r.Value = &value
	}
	return r
}

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, schemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", file))
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
	s, err := c.Compile(schemaHost + "page.held_companies_summary.v3.json")
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

func config(read home_companies.Config) home_companies.Config {
	read.Today = func() contracts.Day { return "2026-11-10" }
	read.RunID, read.BuildID, read.Scope = "run-1", "build-1", "home"
	return read
}

func okView() fakeView {
	return fakeView{rows: map[key][]contracts.ReadingRecord{
		{"valuation.buy_band", "DEMOA"}: {rec("valuation.buy_band", "DEMOA", "2026-11-10", "142.500000", "ok")},
		{"market.hv30_own", "DEMOA"}:    {rec("market.hv30_own", "DEMOA", "2026-11-10", "0.3125", "ok")},
		{"market.iv30_own", "DEMOA"}:    {rec("market.iv30_own", "DEMOA", "2026-11-10", "0.401", "ok")},
		{"market.ivp_own", "DEMOA"}:     {rec("market.ivp_own", "DEMOA", "2026-11-09", "0.730159", "ok")},
	}}
}

func build(t *testing.T, cfg home_companies.Config) (map[string]any, []byte) {
	t.Helper()
	b, err := home_companies.Build(t.Context(), cfg, fixedAt)
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

func TestGolden(t *testing.T) {
	t.Parallel()
	_, b := build(t, config(home_companies.Config{
		Rows: []home_companies.Row{{Issuer: "cik:0000000001", Market: "US", Symbol: "DEMOA", HandPrice: "85.5"}},
		Read: okView(),
	}))
	var got bytes.Buffer
	if err := json.Indent(&got, b, "", "  "); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "ok.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got.String()) != strings.TrimSpace(string(want)) {
		t.Errorf("output differs from testdata/ok.golden.json:\n%s", got.String())
	}
}

func TestRowWithNoReadingIsGaps(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(home_companies.Config{
		Rows: []home_companies.Row{{Issuer: "cik:0000000002", Market: "US", Symbol: "NONE", HandPrice: "42"}},
		Read: okView(),
	}))
	card := m["card"].(map[string]any)
	row := card["rows"].([]any)[0].(map[string]any)
	price := row["price"].(map[string]any)
	if price["kind"] != "gap" || price["reason"] != "no_data" || !strings.Contains(price["text"].(string), "hand 42") {
		t.Errorf("price = %v, want a no_data gap that keeps the hand price", price)
	}
	if m["run_status"] != "partial" {
		t.Errorf("run_status = %v, want partial", m["run_status"])
	}
}

func TestSharpMoveIsAGap(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(home_companies.Config{Read: okView()}))
	card := m["card"].(map[string]any)
	found := false
	for _, g := range card["gaps"].([]any) {
		if strings.Contains(g.(map[string]any)["text"].(string), "sharp") {
			found = true
		}
	}
	if !found || card["sharp_note"] == "" {
		t.Errorf("want a sharp-move gap and a note, got gaps %v note %v", card["gaps"], card["sharp_note"])
	}
}

func TestNilReadGivesNoReaderGaps(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(home_companies.Config{
		Rows: []home_companies.Row{{Market: "US", Symbol: "DEMOA", HandPrice: "85.5"}},
	}))
	card := m["card"].(map[string]any)
	row := card["rows"].([]any)[0].(map[string]any)
	if got := row["price"].(map[string]any)["reason"]; got != "no_reader" {
		t.Errorf("price reason = %v, want no_reader", got)
	}
	if got := row["volatility"].(map[string]any)["hv30"].(map[string]any)["reason"]; got != "no_reader" {
		t.Errorf("hv30 reason = %v, want no_reader", got)
	}
}

func TestReaderErrorIsAnError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	_, err := home_companies.Build(t.Context(), config(home_companies.Config{
		Rows: []home_companies.Row{{Market: "US", Symbol: "DEMOA"}},
		Read: fakeView{err: boom},
	}), fixedAt)
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want it to wrap the reader error", err)
	}
}

func TestMissingTodayIsAnError(t *testing.T) {
	t.Parallel()
	if _, err := home_companies.Build(t.Context(), home_companies.Config{}, fixedAt); err == nil {
		t.Error("want an error when Today is not set")
	}
}

func TestOlderReadingCarriesItsDay(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(home_companies.Config{
		Rows: []home_companies.Row{{Market: "JP", Symbol: "DEMOA"}},
		Read: okView(),
	}))
	row := m["card"].(map[string]any)["rows"].([]any)[0].(map[string]any)
	if row["currency"] != "JPY" {
		t.Errorf("currency = %v, want JPY", row["currency"])
	}
	ivp := row["volatility"].(map[string]any)["ivp"].(map[string]any)
	if ivp["text"] != "73.02% · 11-09" {
		t.Errorf("ivp text = %v, want 73.02%% · 11-09", ivp["text"])
	}
}
