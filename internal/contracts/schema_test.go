package contracts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// datasetRevision names the revision of a dataset's schema file; r1 unless listed.
// fixtureRevision is the revision of every fixture file.
const fixtureRevision = "r1"

var datasetRevision = map[string]string{"optionledger.flex_fetch": "r2"}

func compileDataset(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	rev := "r1"
	if r, ok := datasetRevision[name]; ok {
		rev = r
	}
	file := name + "." + rev + ".json"
	f, err := os.Open(filepath.Join("..", "..", "contracts", "datasets", file))
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
	if err := c.AddResource(file, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(file)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func loadRecords(t *testing.T, name string) ([]map[string]any, []byte) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", name+"."+fixtureRevision+".json"))
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

	names := []string{
		"master.session_day", "obs.series_point", "obs.option_eod", "obs.quote_snapshot", "readings.reading",
		"candidates.option_candidate", "account.position_mark",
	}
	for _, name := range names {
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
	if len(recs) != 5 {
		t.Fatalf("want 5 records, got %d", len(recs))
	}
	ids := []string{"DGS10", "DGS10", "DGS10", "VIXCLS", "DEXJPUS"}
	units := []string{"percent", "percent", "percent", "index_points", "jpy_per_usd"}
	dates := []string{"2026-11-02", "2026-11-03", "2026-11-04", "2026-11-04", "2026-11-05"}
	values := []any{4.1, nil, 4.2, 18.2, 153.41}
	statuses := []string{"present", "no_value", "present", "present", "present"}
	for i, r := range recs {
		if r["series_id"] != ids[i] || r["unit"] != units[i] || r["source"] != "fred" ||
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

func TestReadingRejects(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "readings.reading")
	recs, _ := loadRecords(t, "readings.reading")
	if len(recs) != 4 {
		t.Fatalf("want 4 records, got %d", len(recs))
	}
	mutate := func(i int, f func(map[string]any)) map[string]any {
		m := map[string]any{}
		for k, v := range recs[i] {
			m[k] = v
		}
		f(m)
		return m
	}
	for i := range recs {
		if err := s.Validate(mutate(i, func(map[string]any) {})); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}
	if recs[0]["state"] != "ok" || len(recs[0]["parameters"].([]any)) != 2 ||
		recs[1]["state"] != "missing_input" || recs[1]["reason_code"] != float64(3) || recs[1]["value"] != nil ||
		recs[2]["rev"] != float64(2) || recs[2]["name"] != recs[0]["name"] {
		t.Fatalf("fixture shape: %v", recs)
	}
	bad := map[string]map[string]any{
		"value 1e3":         mutate(0, func(m map[string]any) { m["value"] = "1e3" }),
		"value number":      mutate(0, func(m map[string]any) { m["value"] = 0.5 }),
		"value 13 decimals": mutate(0, func(m map[string]any) { m["value"] = "0.1234567890123" }),
		"state done":        mutate(0, func(m map[string]any) { m["state"] = "done" }),
		"reason 9":          mutate(1, func(m map[string]any) { m["reason_code"] = 9 }),
		"reason 0":          mutate(1, func(m map[string]any) { m["reason_code"] = 0 }),
		"rev 0":             mutate(0, func(m map[string]any) { m["rev"] = 0 }),
		"origin other":      mutate(0, func(m map[string]any) { m["origin"] = "other" }),
		"unit bogus":        mutate(0, func(m map[string]any) { m["unit"] = "bogus" }),
		"parameter extra": mutate(0, func(m map[string]any) {
			m["parameters"] = []any{map[string]any{"param": "a", "value_num": nil, "value_tag": nil, "x": 1}}
		}),
		"input without keys": mutate(0, func(m map[string]any) {
			m["inputs"] = []any{map[string]any{"input_table": "t", "code_version": "c", "convention_version": "v"}}
		}),
	}
	for label, rec := range bad {
		if err := s.Validate(rec); err == nil {
			t.Fatalf("%s must be rejected", label)
		}
	}
	if err := s.Validate(mutate(0, func(m map[string]any) { m["value"] = "-12345.123456789012"; m["reason_code"] = nil })); err != nil {
		t.Fatalf("signed value with 12 decimals: %v", err)
	}
}

func TestOptionLedgerDatasetFiles(t *testing.T) {
	t.Parallel()
	hash := strings.Repeat("ab", 32)
	row := map[string]any{
		"alias": "a", "level_of_detail": "EXECUTION", "transaction_id": "1234567", "revision": 1,
		"fetch_id": "f1", "row_index": 0, "source": "flex",
		"attributes":        `{"quantity":"-1","tradePrice":"1.25"}`,
		"attributes_sha256": hash, "recorded_at": "2026-01-02T03:04:05Z",
	}
	fetch := map[string]any{
		"fetch_id": "f1", "login": "main", "query_id": "123456", "first_date": "2025-12-01", "last_date": "2026-01-01",
		"started_at": "2026-01-02T03:04:00Z", "finished_at": "2026-01-02T03:04:05Z", "status": "ok",
		"error_kind": "", "error_code": "", "raw_sha256": hash, "parser_version": "1",
		"statements": 2, "statements_unmapped": 1, "rows_parsed": 3, "rows_new": 1, "rows_revised": 1, "rows_unchanged": 1,
	}
	sectionRow := map[string]any{
		"alias": "a", "section": "CashTransaction", "row_key": "777", "revision": 1,
		"fetch_id": "f1", "row_index": 0,
		"attributes":        `{"amount":"1.5"}`,
		"attributes_sha256": hash, "recorded_at": "2026-01-02T03:04:05Z",
	}
	sectionStatus := map[string]any{
		"fetch_id": "f1", "alias": "a", "section": "CashTransaction", "state": "present",
		"row_count": 2, "rows_refused": 0, "recorded_at": "2026-01-02T03:04:05Z",
	}
	with := func(base map[string]any, k string, v any) map[string]any {
		m := map[string]any{}
		for kk, vv := range base {
			m[kk] = vv
		}
		m[k] = v
		return m
	}
	cases := []struct {
		name string
		rec  map[string]any
		bad  map[string]map[string]any
	}{
		{"optionledger.flex_trade_row", row, map[string]map[string]any{
			"revision 0":        with(row, "revision", 0),
			"attributes object": with(row, "attributes", map[string]any{"quantity": "-1"}),
			"source other":      with(row, "source", "manual"),
			"short hash":        with(row, "attributes_sha256", "abc"),
			"account number":    with(row, "account_number", "U1"),
			"alias upper":       with(row, "alias", "A"),
		}},
		{"optionledger.flex_fetch", fetch, map[string]map[string]any{
			"status other":      with(fetch, "status", "done"),
			"count number text": with(fetch, "rows_new", "1"),
			"negative count":    with(fetch, "rows_parsed", -1),
			"query id letters":  with(fetch, "query_id", "abc"),
			"token":             with(fetch, "token", "x"),
		}},
		{"optionledger.flex_section_row", sectionRow, map[string]map[string]any{
			"revision 0":        with(sectionRow, "revision", 0),
			"attributes object": with(sectionRow, "attributes", map[string]any{"amount": "1"}),
			"short hash":        with(sectionRow, "attributes_sha256", "abc"),
			"account number":    with(sectionRow, "account_number", "U1"),
			"account id":        with(sectionRow, "accountId", "U1"),
			"section empty":     with(sectionRow, "section", ""),
			"section spaced":    with(sectionRow, "section", "Cash Transaction"),
			"row key empty":     with(sectionRow, "row_key", ""),
		}},
		{"optionledger.flex_section_status", sectionStatus, map[string]map[string]any{
			"state other":   with(sectionStatus, "state", "missing"),
			"negative":      with(sectionStatus, "rows_refused", -1),
			"account":       with(sectionStatus, "account_number", "U1"),
			"section empty": with(sectionStatus, "section", ""),
		}},
	}
	for _, c := range cases {
		s := compileDataset(t, c.name)
		if err := s.Validate(c.rec); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for label, rec := range c.bad {
			if err := s.Validate(rec); err == nil {
				t.Fatalf("%s: %s must be rejected", c.name, label)
			}
		}
	}
}

func TestEarningsDateFixtureValues(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "obs.earnings_date")
	recs, _ := loadRecords(t, "obs.earnings_date")
	want := []struct {
		status, source string
		hasDate        bool
	}{
		{"listed", "alphavantage", true},
		{"absent", "alphavantage", false},
		{"unsupported", "none", false},
		{"error", "jquants", false},
	}
	if len(recs) != len(want) {
		t.Fatalf("want %d records, got %d", len(want), len(recs))
	}
	for i, w := range want {
		r := recs[i]
		if err := s.Validate(r); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
		if r["status"] != w.status || r["source"] != w.source {
			t.Fatalf("record %d: status %v source %v, want %s %s", i, r["status"], r["source"], w.status, w.source)
		}
		date, present := r["report_date"]
		if !present {
			t.Fatalf("record %d: report_date key must be present", i)
		}
		if (date != nil) != w.hasDate {
			t.Fatalf("record %d: report_date %v, has date want %v", i, date, w.hasDate)
		}
	}

	mutate := func(base map[string]any, f func(map[string]any)) map[string]any {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		f(m)
		return m
	}
	cases := map[string]map[string]any{
		"status unknown":     mutate(recs[0], func(m map[string]any) { m["status"] = "present" }),
		"source null":        mutate(recs[2], func(m map[string]any) { m["source"] = nil }),
		"source fmp":         mutate(recs[0], func(m map[string]any) { m["source"] = "fmp" }),
		"no attempt":         mutate(recs[0], func(m map[string]any) { delete(m, "attempt") }),
		"attempt zero":       mutate(recs[0], func(m map[string]any) { m["attempt"] = 0 }),
		"issuer is a ticker": mutate(recs[0], func(m map[string]any) { m["issuer"] = "AAPL" }),
		"market unknown":     mutate(recs[0], func(m map[string]any) { m["market"] = "HK" }),
		"estimate a number":  mutate(recs[0], func(m map[string]any) { m["estimate"] = 1.5 }),
		"unknown field":      mutate(recs[0], func(m map[string]any) { m["extra"] = 1 }),
	}
	for label, rec := range cases {
		if err := s.Validate(rec); err == nil {
			t.Fatalf("%s must be rejected", label)
		}
	}
}

func TestEarningsDateStatusRulesEnforced(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "obs.earnings_date")
	recs, _ := loadRecords(t, "obs.earnings_date")
	cp := func(b map[string]any) map[string]any {
		m := map[string]any{}
		for k, v := range b {
			m[k] = v
		}
		return m
	}
	unsupportedWithVendor := cp(recs[2]) // status unsupported
	unsupportedWithVendor["source"] = "alphavantage"
	absentWithDate := cp(recs[1]) // status absent
	absentWithDate["report_date"] = "2027-01-29"
	listedWithoutDate := cp(recs[0])
	listedWithoutDate["report_date"] = nil
	for label, r := range map[string]map[string]any{
		"unsupported row with a vendor source": unsupportedWithVendor,
		"absent row with a report_date":        absentWithDate,
		"listed row without a report_date":     listedWithoutDate,
	} {
		if err := s.Validate(r); err == nil {
			t.Errorf("%s must be rejected (Decision 7)", label)
		}
	}
}

func copyRec(b map[string]any, f func(map[string]any)) map[string]any {
	m := map[string]any{}
	for k, v := range b {
		m[k] = v
	}
	f(m)
	return m
}

func TestPriceBarFixtureValues(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "obs.price_bar")
	recs, _ := loadRecords(t, "obs.price_bar")
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d", len(recs))
	}
	for i, r := range recs {
		if err := s.Validate(r); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}
	if recs[0]["basis"] != "trades" || recs[0]["adjust_epoch"] != float64(1) || recs[0]["volume"] == nil {
		t.Fatalf("record 0 must be a trades row of epoch 1 with a volume: %v", recs[0])
	}
	if recs[1]["basis"] != "adjusted_last" || recs[1]["adjust_epoch"] != float64(2) {
		t.Fatalf("record 1 must be an adjusted_last row of epoch 2: %v", recs[1])
	}
	if v, ok := recs[1]["volume"]; !ok || v != nil {
		t.Fatalf("record 1 must carry a null volume, key present: %v", recs[1])
	}
	cases := map[string]map[string]any{
		"basis midpoint":    copyRec(recs[0], func(m map[string]any) { m["basis"] = "midpoint" }),
		"epoch zero":        copyRec(recs[0], func(m map[string]any) { m["adjust_epoch"] = 0 }),
		"close a number":    copyRec(recs[0], func(m map[string]any) { m["close"] = 1.5 }),
		"volume a number":   copyRec(recs[0], func(m map[string]any) { m["volume"] = 10 }),
		"source other":      copyRec(recs[0], func(m map[string]any) { m["source"] = "fred" }),
		"no volume key":     copyRec(recs[0], func(m map[string]any) { delete(m, "volume") }),
		"unknown field":     copyRec(recs[0], func(m map[string]any) { m["extra"] = 1 }),
		"session_day text":  copyRec(recs[0], func(m map[string]any) { m["session_day"] = "yesterday" }),
		"instrument_id str": copyRec(recs[0], func(m map[string]any) { m["instrument_id"] = "x" }),
	}
	for label, rec := range cases {
		if err := s.Validate(rec); err == nil {
			t.Fatalf("%s must be rejected", label)
		}
	}
}

func TestFxRateFixtureValues(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "obs.fx_rate")
	recs, _ := loadRecords(t, "obs.fx_rate")
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d", len(recs))
	}
	for _, r := range recs {
		if err := s.Validate(r); err != nil {
			t.Fatal(err)
		}
	}
	if recs[0]["source"] != "ibkr_midpoint" || recs[1]["source"] != "fred_h10" {
		t.Fatalf("sources: %v %v", recs[0]["source"], recs[1]["source"])
	}
	cases := map[string]map[string]any{
		"pair lowercase": copyRec(recs[0], func(m map[string]any) { m["pair"] = "usdjpy" }),
		"pair slash":     copyRec(recs[0], func(m map[string]any) { m["pair"] = "USD/JPY" }),
		"rate zero":      copyRec(recs[0], func(m map[string]any) { m["rate"] = "0" }),
		"rate zero dec":  copyRec(recs[0], func(m map[string]any) { m["rate"] = "0.000" }),
		"rate negative":  copyRec(recs[0], func(m map[string]any) { m["rate"] = "-1.5" }),
		"rate a number":  copyRec(recs[0], func(m map[string]any) { m["rate"] = 150.2 }),
		"source other":   copyRec(recs[0], func(m map[string]any) { m["source"] = "ibkr" }),
		"unknown field":  copyRec(recs[0], func(m map[string]any) { m["extra"] = 1 }),
	}
	for label, rec := range cases {
		if err := s.Validate(rec); err == nil {
			t.Fatalf("%s must be rejected", label)
		}
	}
}

func TestFxRateSourceFredH10Accepted(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "obs.fx_rate")
	recs, _ := loadRecords(t, "obs.fx_rate")
	ok := copyRec(recs[0], func(m map[string]any) { m["source"] = "fred_h10" })
	if err := s.Validate(ok); err != nil {
		t.Fatalf("fred_h10 must validate: %v", err)
	}
	bad := copyRec(recs[0], func(m map[string]any) { m["source"] = "stooq" })
	if err := s.Validate(bad); err == nil {
		t.Fatal("stooq must be refused")
	}
	var g gen.FxRate
	if err := json.Unmarshal([]byte(`{"pair":"USDJPY","rate_day":"2026-11-05","rate":"153.41","source":"fred_h10","received_at":"2026-11-06T22:15:00Z"}`), &g); err != nil {
		t.Fatalf("generated record must accept fred_h10: %v", err)
	}
	if g.Source != gen.FxRateSourceFredH10 {
		t.Fatalf("source: %v", g.Source)
	}
	if err := json.Unmarshal([]byte(`{"pair":"USDJPY","rate_day":"2026-11-05","rate":"153.41","source":"stooq","received_at":"2026-11-06T22:15:00Z"}`), &g); err == nil {
		t.Fatal("generated record must refuse stooq")
	}
}

func TestSeriesUnitJpyPerUsdAccepted(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "obs.series_point")
	recs, _ := loadRecords(t, "obs.series_point")
	ok := copyRec(recs[0], func(m map[string]any) { m["unit"] = "jpy_per_usd" })
	if err := s.Validate(ok); err != nil {
		t.Fatalf("jpy_per_usd must validate: %v", err)
	}
	bad := copyRec(recs[0], func(m map[string]any) { m["unit"] = "eur_per_usd" })
	if err := s.Validate(bad); err == nil {
		t.Fatal("eur_per_usd must be refused")
	}
	var g gen.SeriesPoint
	row := `{"series_id":"DEXJPUS","observation_date":"2026-11-05","realtime_start":"2026-11-06","realtime_end":"9999-12-31","value":153.41,"status":"present","unit":"%s","currency":"","published_at":"2026-11-06T21:15:00Z","source":"fred","source_revision":"r1"}`
	if err := json.Unmarshal([]byte(fmt.Sprintf(row, "jpy_per_usd")), &g); err != nil || g.Unit != gen.SeriesUnitJpyPerUsd {
		t.Fatalf("generated record must accept jpy_per_usd: %v %v", err, g.Unit)
	}
	if err := json.Unmarshal([]byte(fmt.Sprintf(row, "eur_per_usd")), &g); err == nil {
		t.Fatal("generated record must refuse eur_per_usd")
	}
}

func TestDividendFixtureValues(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "obs.dividend_expectation")
	recs, _ := loadRecords(t, "obs.dividend_expectation")
	want := []struct {
		status, source string
		hasNext        bool
	}{
		{"present", "ibkr_tick_456", true},
		{"no_value", "ibkr_tick_456", false},
		{"unsupported", "none", false},
		{"error", "ibkr_tick_456", false},
	}
	if len(recs) != len(want) {
		t.Fatalf("want %d records, got %d", len(want), len(recs))
	}
	for i, w := range want {
		r := recs[i]
		if err := s.Validate(r); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
		if r["status"] != w.status || r["source"] != w.source {
			t.Fatalf("record %d: status %v source %v, want %s %s", i, r["status"], r["source"], w.status, w.source)
		}
		for _, k := range []string{"next_ex_date", "next_amount"} {
			v, present := r[k]
			if !present {
				t.Fatalf("record %d: %s key must be present", i, k)
			}
			if (v != nil) != w.hasNext {
				t.Fatalf("record %d: %s %v, has value want %v", i, k, v, w.hasNext)
			}
		}
	}

	cases := map[string]map[string]any{
		"status unknown":       copyRec(recs[0], func(m map[string]any) { m["status"] = "listed" }),
		"source null":          copyRec(recs[2], func(m map[string]any) { m["source"] = nil }),
		"source vendor":        copyRec(recs[0], func(m map[string]any) { m["source"] = "fmp" }),
		"unsupported ibkr":     copyRec(recs[2], func(m map[string]any) { m["source"] = "ibkr_tick_456" }),
		"attempt zero":         copyRec(recs[0], func(m map[string]any) { m["attempt"] = 0 }),
		"issuer is a ticker":   copyRec(recs[0], func(m map[string]any) { m["issuer"] = "AAPL" }),
		"amount a number":      copyRec(recs[0], func(m map[string]any) { m["next_amount"] = 0.26 }),
		"no_value with date":   copyRec(recs[1], func(m map[string]any) { m["next_ex_date"] = "2026-11-12" }),
		"no_value with amount": copyRec(recs[1], func(m map[string]any) { m["next_amount"] = "0.26" }),
		"error with date":      copyRec(recs[3], func(m map[string]any) { m["next_ex_date"] = "2026-11-12" }),
		"present without date": copyRec(recs[0], func(m map[string]any) { m["next_ex_date"] = nil }),
		"present no amount":    copyRec(recs[0], func(m map[string]any) { m["next_amount"] = nil }),
		"unknown field":        copyRec(recs[0], func(m map[string]any) { m["extra"] = 1 }),
	}
	for label, rec := range cases {
		if err := s.Validate(rec); err == nil {
			t.Fatalf("%s must be rejected", label)
		}
	}
}
