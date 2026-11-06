package contracts_test

// This file is a self-consistency check, not fixtures. For each live carried
// page schema without a fixture directory it builds the smallest instance from
// the required lists and shows that the schema compiles, that the instance
// validates and that a raised version fails. It writes no fixture file and
// holds no JSON instance text, so no personal data or account value can enter
// the repository.
//
// What it does not prove: nothing about optional fields, nothing about
// additionalProperties, nothing about the shape a builder emits. It shows only
// that refs resolve, every required property has a schema and the patterns can
// be satisfied. The pages still have no renderable ok, gap or stale state.

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// selfCheckFiles holds the 11 live carried schemas that have no fixtures.
// macro_rates has golden files; positions_page, structure_detail v1 and v2 and
// report_page v1 are dead versions.
var selfCheckFiles = []string{
	"page.home_page.v1.json",
	"page.household_risk.v1.json",
	"page.indicator_detail.v1.json",
	"page.ledger_page.v1.json",
	"page.market_summary.v1.json",
	"page.option_result_page.v1.json",
	"page.options_summary.v1.json",
	"page.report_page.v2.json",
	"page.settings_page.v1.json",
	"page.stock_option_page.v1.json",
	"page.structure_detail.v3.json",
}

const (
	patDate     = `^(?:\d{4}-(?:0[13578]|1[02])-(?:0[1-9]|[12]\d|3[01])|\d{4}-(?:0[469]|11)-(?:0[1-9]|[12]\d|30)|\d{4}-02-(?:0[1-9]|1\d|2[0-8])|(?:\d\d(?:0[48]|[2468][048]|[13579][26])|(?:[02468][048]|[13579][26])00)-02-29)$`
	patDateTime = `^(?:\d{4}-(?:0[13578]|1[02])-(?:0[1-9]|[12]\d|3[01])|\d{4}-(?:0[469]|11)-(?:0[1-9]|[12]\d|30)|\d{4}-02-(?:0[1-9]|1\d|2[0-8])|(?:\d\d(?:0[48]|[2468][048]|[13579][26])|(?:[02468][048]|[13579][26])00)-02-29)T(?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d\+08:00$`
)

// versionKey is the property that carries the version const.
const versionKey = "schema_version"

// selfCheckSamples maps each pattern of the files above to one value it matches.
var selfCheckSamples = map[string]string{
	`^(?:/|/[A-Za-z0-9_-][A-Za-z0-9_/-]*)$`: "/",
	patDate:                                 "2026-01-02",
	patDateTime:                             "2026-01-02T03:04:05+08:00",
	`^-?\d+(\.\d+)?$`:                       "1.5",
	`^[0-9]{1,12}$`:                         "1",
	`^[0-9a-f]{64}$`:                        strings.Repeat("a", 64),
	`^[A-Za-z0-9_-]{1,64}$`:                 "id_1",
	`^\d{4}-(0[1-9]|1[0-2])$`:               "2026-01",
	`^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$`: "2026-01-02",
}

func collectPatterns(v any, out map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, c := range x {
			if s, ok := c.(string); ok && k == "pattern" {
				out[s] = true
				continue
			}
			collectPatterns(c, out)
		}
	case []any:
		for _, c := range x {
			collectPatterns(c, out)
		}
	}
}

// Every pattern of the files has a sample, and every sample matches its key.
func TestSelfCheckSamples(t *testing.T) {
	t.Parallel()
	found := map[string]bool{}
	for _, file := range selfCheckFiles {
		if _, err := os.Stat(filepath.Join(schemasDir, file)); os.IsNotExist(err) {
			continue
		}
		collectPatterns(readRaw(t, schemasDir, file), found)
	}
	var missing []string
	for p := range found {
		if _, ok := selfCheckSamples[p]; !ok {
			missing = append(missing, p)
		}
	}
	sort.Strings(missing)
	for _, p := range missing {
		t.Errorf("pattern without a sample: %s", p)
	}
	for p, v := range selfCheckSamples {
		re, err := regexp.Compile(p)
		if err != nil {
			t.Errorf("pattern %s: %v", p, err)
			continue
		}
		if !re.MatchString(v) {
			t.Errorf("sample %q does not match %s", v, p)
		}
	}
}

// checkSelfConsistent compiles file, validates its minimal instance and checks
// that the same instance with the version raised by one fails on the const.
func checkSelfConsistent(t *testing.T, file string) {
	t.Helper()
	s := compileCarried(t, file)
	raw := readRaw(t, schemasDir, file)
	props, _ := raw["properties"].(map[string]any)
	verSchema, _ := props[versionKey].(map[string]any)
	ver, ok := verSchema["const"].(float64)
	if !ok || ver != math.Trunc(ver) {
		t.Fatalf("%s: version key has no integer const", file)
	}
	roundTrip := func(version float64) any {
		inst, ok := minimal(t, schemasDir, file, raw, selfCheckSamples).(map[string]any)
		if !ok {
			t.Fatalf("%s: not an object schema", file)
		}
		inst[versionKey] = version
		b, err := json.Marshal(inst)
		if err != nil {
			t.Fatal(err)
		}
		v, err := jsonschema.UnmarshalJSON(strings.NewReader(string(b)))
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if err := s.Validate(roundTrip(ver)); err != nil {
		t.Fatalf("%s: minimal instance is invalid: %v", file, err)
	}
	err := s.Validate(roundTrip(ver + 1))
	if err == nil {
		t.Fatalf("%s: version %v validates", file, ver+1)
	}
	verr, ok := err.(*jsonschema.ValidationError)
	if !ok {
		t.Fatalf("%s: %v", file, err)
	}
	var ls []leaf
	leaves(verr, &ls)
	if len(ls) != 1 || ls[0] != (leaf{"/" + versionKey, "const"}) {
		t.Errorf("%s: wrong version fails at %v, want the version key const", file, ls)
	}
}

func TestSchemaSelfCheck(t *testing.T) {
	t.Parallel()
	for _, file := range selfCheckFiles {
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			if _, err := os.Stat(filepath.Join(schemasDir, file)); os.IsNotExist(err) {
				t.Skip("file does not exist")
			}
			checkSelfConsistent(t, file)
		})
	}
}
