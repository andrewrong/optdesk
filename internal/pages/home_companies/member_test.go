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

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/pages/home_companies"
	"github.com/andrewrong/optdesk/internal/store"
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
		{"market.ivp_own", "DEMOA"}:     {missingOwn("DEMOA", "2026-11-10", 2, 100)},
		{"market.ivp_ibkr", "DEMOA"}:    {rec("market.ivp_ibkr", "DEMOA", "2026-11-09", "0.730159", "ok")},
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
	if ivp["text"] != "73.02% · IBKR series · 11-09" {
		t.Errorf("ivp text = %v, want 73.02%% · IBKR series · 11-09", ivp["text"])
	}
}

func missingOwn(subject, day string, code int, observations float64) contracts.ReadingRecord {
	r := rec("market.ivp_own", subject, day, "", "missing_input")
	r.ReasonCode = &code
	r.Parameters = []gen.ReadingRecordParametersElem{{Param: "observations", ValueNum: &observations}}
	return r
}

func ivpOf(t *testing.T, view fakeView) map[string]any {
	t.Helper()
	m, _ := build(t, config(home_companies.Config{
		Rows: []home_companies.Row{{Market: "US", Symbol: "DEMOA"}},
		Read: view,
	}))
	row := m["card"].(map[string]any)["rows"].([]any)[0].(map[string]any)
	return row
}

func TestIVPSourceOrder(t *testing.T) {
	t.Parallel()
	ibkr := rec("market.ivp_ibkr", "DEMOA", "2026-11-10", "0.5", "ok")
	cases := []struct {
		name      string
		own       []contracts.ReadingRecord
		ibkr      []contracts.ReadingRecord
		wantKind  string
		wantValue float64
		wantText  string
		wantLabel string
	}{
		{"own wins over ibkr", []contracts.ReadingRecord{rec("market.ivp_own", "DEMOA", "2026-11-10", "0.7", "ok")}, []contracts.ReadingRecord{ibkr},
			"value", 70, "70.00% · own series", "ivp_v1"},
		{"ibkr when own is missing", []contracts.ReadingRecord{missingOwn("DEMOA", "2026-11-10", 2, 100)}, []contracts.ReadingRecord{ibkr},
			"value", 50, "50.00% · IBKR series", "market.ivp_ibkr"},
		{"ibkr when own is not stored", nil, []contracts.ReadingRecord{ibkr},
			"value", 50, "50.00% · IBKR series", "market.ivp_ibkr"},
		{"both missing names own reason", []contracts.ReadingRecord{missingOwn("DEMOA", "2026-11-10", 2, 100)},
			[]contracts.ReadingRecord{rec("market.ivp_ibkr", "DEMOA", "2026-11-10", "", "missing_input")},
			"gap", 0, "IV percentile: short history, 100/252 own IV30 sessions", ""},
		{"stale input", []contracts.ReadingRecord{missingOwn("DEMOA", "2026-11-10", 3, 0)}, nil,
			"gap", 0, "IV percentile: stale input, own IV30 is out of date", ""},
		{"no input", []contracts.ReadingRecord{missingOwn("DEMOA", "2026-11-10", 1, 0)}, nil,
			"gap", 0, "IV percentile: no input, no IV30 reading for the day", ""},
		{"bad input", []contracts.ReadingRecord{missingOwn("DEMOA", "2026-11-10", 5, 0)}, nil,
			"gap", 0, "IV percentile: bad input, own IV30 history is unusable", ""},
		{"no own row", nil, nil,
			"gap", 0, "IV percentile: no own IV30 history is stored", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := okView()
			v.rows[key{"market.ivp_own", "DEMOA"}] = tc.own
			v.rows[key{"market.ivp_ibkr", "DEMOA"}] = tc.ibkr
			row := ivpOf(t, v)
			vol := row["volatility"].(map[string]any)
			ivp := vol["ivp"].(map[string]any)
			if ivp["kind"] != tc.wantKind || ivp["text"] != tc.wantText {
				t.Fatalf("ivp = %v, want kind %s text %q", ivp, tc.wantKind, tc.wantText)
			}
			if tc.wantKind == "value" {
				if ivp["value"].(float64) != tc.wantValue {
					t.Errorf("value = %v, want %v", ivp["value"], tc.wantValue)
				}
				if !strings.Contains(vol["label"].(string), tc.wantLabel) {
					t.Errorf("label = %q, want it to name %s", vol["label"], tc.wantLabel)
				}
			}
		})
	}
}

func notOkRow(name, subject, day, reason string) contracts.ReadingRecord {
	r := rec(name, subject, day, "", "missing_input")
	r.Parameters = []gen.ReadingRecordParametersElem{{Param: "reason", ValueTag: &reason}}
	return r
}

func gapTexts(t *testing.T, read fakeView) (price, hv, iv string) {
	t.Helper()
	m, _ := build(t, config(home_companies.Config{
		Rows: []home_companies.Row{{Issuer: "cik:0000000003", Market: "US", Symbol: "DEMOB"}},
		Read: read,
	}))
	row := m["card"].(map[string]any)["rows"].([]any)[0].(map[string]any)
	vol := row["volatility"].(map[string]any)
	text := func(v any) string { return v.(map[string]any)["text"].(string) }
	return text(row["price"]), text(vol["hv30"]), text(vol["iv30"])
}

func TestBandNotOkFewerThan5YearsNamesTheNeed(t *testing.T) {
	t.Parallel()
	price, _, _ := gapTexts(t, fakeView{rows: map[key][]contracts.ReadingRecord{
		{"valuation.buy_band", "DEMOB"}: {notOkRow("valuation.buy_band", "DEMOB", "2026-11-10", "fewer_than_5_years")},
	}})
	want := "valuation band: needs 5 years of fundamentals, fewer stored; shown once 5 years are loaded"
	if price != want {
		t.Errorf("price text = %q, want %q", price, want)
	}
}

func TestBandNotOkOtherReasonReadsUnderscoresAsSpaces(t *testing.T) {
	t.Parallel()
	price, _, _ := gapTexts(t, fakeView{rows: map[key][]contracts.ReadingRecord{
		{"valuation.buy_band", "DEMOB"}: {notOkRow("valuation.buy_band", "DEMOB", "2026-11-10", "no_price_input")},
	}})
	if want := "valuation band: missing input (no price input)"; price != want {
		t.Errorf("price text = %q, want %q", price, want)
	}
}

func TestNoBandRowSaysNotComputedYet(t *testing.T) {
	t.Parallel()
	price, hv, iv := gapTexts(t, fakeView{})
	want := "valuation band: not computed yet, comes with the next figures run"
	if price != want {
		t.Errorf("price text = %q, want %q", price, want)
	}
	if want := "HV30: not computed yet, comes with the next figures run"; hv != want {
		t.Errorf("hv30 text = %q, want %q", hv, want)
	}
	if want := "IV30: not computed yet, comes with the next figures run"; iv != want {
		t.Errorf("iv30 text = %q, want %q", iv, want)
	}
}

func TestHVNotOkRowWithReasonNamesIt(t *testing.T) {
	t.Parallel()
	_, hv, _ := gapTexts(t, fakeView{rows: map[key][]contracts.ReadingRecord{
		{"market.hv30_own", "DEMOB"}: {notOkRow("market.hv30_own", "DEMOB", "2026-11-10", "short_history")},
	}})
	if want := "HV30: missing input (short history)"; hv != want {
		t.Errorf("hv30 text = %q, want %q", hv, want)
	}
}

func TestChangeGapsSayNotReadByThisCard(t *testing.T) {
	t.Parallel()
	m, _ := build(t, config(home_companies.Config{
		Rows: []home_companies.Row{{Issuer: "cik:0000000003", Market: "US", Symbol: "DEMOB"}},
		Read: fakeView{},
	}))
	row := m["card"].(map[string]any)["rows"].([]any)[0].(map[string]any)
	for k, want := range map[string]string{
		"day_change":      "day change: not read by this card yet",
		"five_day_change": "5-day change: not read by this card yet",
	} {
		if got := row[k].(map[string]any)["text"]; got != want {
			t.Errorf("%s text = %v, want %q", k, got, want)
		}
	}
}

type fakeEarnings struct {
	rows []contracts.EarningsDate
	err  error
}

func (f fakeEarnings) EarningsDates(context.Context, string, contracts.Day, contracts.Day) ([]contracts.EarningsDate, error) {
	return f.rows, f.err
}

func earnRow(status, report string) contracts.EarningsDate {
	r := contracts.EarningsDate{Status: gen.EarningsDateStatus(status)}
	if report != "" {
		d, err := time.Parse("2006-01-02", report)
		if err != nil {
			panic(err)
		}
		r.ReportDate = &types.SerializableDate{Time: d}
	}
	return r
}

func earningsOf(t *testing.T, er store.EarningsDateReader) map[string]any {
	t.Helper()
	m, _ := build(t, config(home_companies.Config{
		Rows:     []home_companies.Row{{Issuer: "cik:0000000003", Market: "US", Symbol: "DEMOB"}},
		Read:     fakeView{},
		Earnings: er,
	}))
	return m["card"].(map[string]any)["rows"].([]any)[0].(map[string]any)["earnings"].(map[string]any)
}

func TestEarningsCell(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		er   store.EarningsDateReader
		want map[string]any
	}{
		{"future", fakeEarnings{rows: []contracts.EarningsDate{earnRow("listed", "2026-11-10"), earnRow("listed", "2026-11-28")}},
			map[string]any{"kind": "value", "date": "2026-11-28", "text": "11-28"}},
		{"today", fakeEarnings{rows: []contracts.EarningsDate{earnRow("listed", "2026-11-10")}},
			map[string]any{"kind": "value", "date": "2026-11-10", "text": "11-10"}},
		{"past", fakeEarnings{rows: []contracts.EarningsDate{earnRow("listed", "2026-11-03")}},
			map[string]any{"kind": "value", "date": "2026-11-03", "text": "已公布 11-03"}},
		{"no row", fakeEarnings{},
			map[string]any{"kind": "gap", "reason": "no_data", "text": "earnings date: none listed"}},
		{"not listed", fakeEarnings{rows: []contracts.EarningsDate{earnRow("none", "")}},
			map[string]any{"kind": "gap", "reason": "no_data", "text": "earnings date: none listed"}},
		{"listed without date", fakeEarnings{rows: []contracts.EarningsDate{earnRow("listed", "")}},
			map[string]any{"kind": "gap", "reason": "no_data", "text": "earnings date: none listed"}},
		{"nil reader", nil,
			map[string]any{"kind": "gap", "reason": "no_reader", "text": "earnings date: reader not set"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := earningsOf(t, tc.er)
			if len(got) != len(tc.want) {
				t.Errorf("cell = %v, want %v", got, tc.want)
			}
			for k, w := range tc.want {
				if got[k] != w {
					t.Errorf("cell = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestEarningsReaderErrorFailsBuild(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	_, err := home_companies.Build(t.Context(), config(home_companies.Config{
		Rows:     []home_companies.Row{{Issuer: "cik:0000000003", Market: "US", Symbol: "DEMOB"}},
		Read:     fakeView{},
		Earnings: fakeEarnings{err: boom},
	}), fixedAt)
	if !errors.Is(err, boom) {
		t.Errorf("Build error = %v, want %v", err, boom)
	}
}
