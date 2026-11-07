package contracts_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func compileDataset(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "contracts", "datasets", name+".r1.json"))
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
	if err := c.AddResource(name+".r1.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(name + ".r1.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func loadRecords(t *testing.T, name string) ([]map[string]any, []byte) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", name+".r1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var recs []map[string]any
	if err := json.Unmarshal(b, &recs); err != nil {
		t.Fatal(err)
	}
	return recs, b
}

func TestDatasetFixtures(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"master.session_day", "obs.series_point", "obs.option_eod", "obs.quote_snapshot"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := compileDataset(t, name)
			recs, raw := loadRecords(t, name)
			if len(recs) == 0 {
				t.Fatal("no records")
			}
			if strings.Contains(string(raw), "synthetic-v1") {
				t.Fatal("a fixture must not carry the marker")
			}
			// one record per line: brackets on their own lines, records between
			if got := bytes.Count(raw, []byte("\n")); got != len(recs)+2 && got != len(recs)+1 {
				t.Fatalf("want one record per line, got %d lines for %d records", got, len(recs))
			}
			for i, r := range recs {
				if err := s.Validate(r); err != nil {
					t.Fatalf("record %d: %v", i, err)
				}
				extra := map[string]any{"extra": 1}
				for k, v := range r {
					extra[k] = v
				}
				if err := s.Validate(extra); err == nil {
					t.Fatalf("record %d with an unknown field must fail", i)
				}
			}
		})
	}
}

func TestSnapshotSchemasReject(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"obs.option_eod", "obs.quote_snapshot"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := compileDataset(t, name)
			recs, _ := loadRecords(t, name)
			mutate := func(f func(map[string]any)) map[string]any {
				m := map[string]any{}
				for k, v := range recs[0] {
					m[k] = v
				}
				f(m)
				return m
			}
			cases := map[string]map[string]any{
				"base":             mutate(func(map[string]any) {}),
				"extra property":   mutate(func(m map[string]any) { m["extra"] = 1 }),
				"status 6":         mutate(func(m map[string]any) { m["status"] = 6 }),
				"status 0":         mutate(func(m map[string]any) { m["status"] = 0 }),
				"currency usd":     mutate(func(m map[string]any) { m["currency"] = "usd" }),
				"capture not uuid": mutate(func(m map[string]any) { m["capture_id"] = "not-a-uuid" }),
				"negative bid":     mutate(func(m map[string]any) { m["bid"] = -1 }),
			}
			for label, rec := range cases {
				err := s.Validate(rec)
				if label == "base" && err != nil {
					t.Fatalf("base record: %v", err)
				}
				if label != "base" && err == nil {
					t.Fatalf("%s must be rejected", label)
				}
			}
			if name == "obs.option_eod" {
				if err := s.Validate(mutate(func(m map[string]any) { m["underlying_ccy"] = "eur" })); err == nil {
					t.Fatal("underlying_ccy eur must be rejected")
				}
				if err := s.Validate(mutate(func(m map[string]any) { m["iv_decimal"] = -1 })); err == nil {
					t.Fatal("iv_decimal -1 must be rejected")
				}
				if err := s.Validate(mutate(func(m map[string]any) { m["delta"] = nil })); err != nil {
					t.Fatalf("null delta: %v", err)
				}
			}
		})
	}
}

func TestSessionDayFixtureValues(t *testing.T) {
	t.Parallel()
	recs, _ := loadRecords(t, "master.session_day")
	type row struct {
		session       bool
		early         bool
		open, closeAt string
	}
	want := map[string]row{
		"2026-10-30": {true, false, "2026-10-30T13:30:00Z", "2026-10-30T20:00:00Z"},
		"2026-11-02": {true, false, "2026-11-02T14:30:00Z", "2026-11-02T21:00:00Z"},
		"2026-11-06": {true, false, "2026-11-06T14:30:00Z", "2026-11-06T21:00:00Z"},
		"2026-11-26": {false, false, "2026-11-26T00:00:00Z", "2026-11-26T00:00:00Z"},
		"2026-11-27": {true, true, "2026-11-27T14:30:00Z", "2026-11-27T18:00:00Z"},
		"2026-12-25": {false, false, "2026-12-25T00:00:00Z", "2026-12-25T00:00:00Z"},
	}
	if len(recs) != len(want) {
		t.Fatalf("want %d records, got %d", len(want), len(recs))
	}
	for _, r := range recs {
		day, _ := r["session_day"].(string)
		w, ok := want[day]
		if !ok {
			t.Fatalf("unexpected day %q", day)
		}
		got := row{r["is_session"] == true, r["early_close"] == true, r["open_at"].(string), r["close_at"].(string)}
		if r["calendar_id"] != "XNYS" || got != w {
			t.Fatalf("%s: got %+v want %+v", day, got, w)
		}
	}

}

func TestSeriesPointFixtureValues(t *testing.T) {
	t.Parallel()
	recs, _ := loadRecords(t, "obs.series_point")
	if len(recs) != 3 {
		t.Fatalf("want 3 records, got %d", len(recs))
	}
	dates := []string{"2026-11-02", "2026-11-03", "2026-11-04"}
	values := []any{4.1, nil, 4.2}
	statuses := []string{"present", "no_value", "present"}
	for i, r := range recs {
		if r["series_id"] != "DGS10" || r["unit"] != "percent" || r["source"] != "fred" ||
			r["observation_date"] != dates[i] || r["value"] != values[i] || r["status"] != statuses[i] {
			t.Fatalf("record %d: %v", i, r)
		}
	}

}

func TestSymbolsFixture(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", "symbols.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := "SYN1\nSYN2\nSYN3\nSYN4\nSYN5\nSYN6\nSYN7\nSYN8\nSYN9\n"
	if string(b) != want {
		t.Fatalf("symbols: got %q", b)
	}

}
