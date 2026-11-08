package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const fixturesDir = "../../web/fixtures"

// fixtureSchema maps a directory of web/fixtures to its schema file.
var fixtureSchema = map[string]string{
	"advice-page":            "page.advice_page.v1.json",
	"companies-page":         "page.companies_page.v1.json",
	"full-scan":              "page.full_scan.v1.json",
	"held-companies-summary": "page.held_companies_summary.v1.json",
	"home-page":              "page.home_page.v3.json",
	"household-page":         "page.household_page.v1.json",
	"household-risk":         "page.household_risk.v1.json",
	"household-summary":      "page.household_summary.v1.json",
	"macro-page":             "page.macro_page.v1.json",
	"macro-summary":          "page.macro_summary.v1.json",
	"market-summary":         "page.market_summary.v2.json",
	"market-weather":         "page.market_weather.v1.json",
	"option-page":            "page.option_page.v1.json",
	"options-summary":        "page.options_summary.v2.json",
	"positions-page":         "page.positions_page.v3.json",
	"structures-list":        "page.structures_list.v1.json",
	"tracking":               "page.tracking.v1.json",
}

// expectedFailure names a fixture that must fail, by the instance location
// (JSON pointer) and the keyword of a leaf of the validation error.
type expectedFailure struct {
	path     string // relative to web/fixtures
	location string
	keyword  string
}

func badVersion(dir string) expectedFailure {
	return expectedFailure{path: dir + "/bad-version.json", location: "/schema_version", keyword: "const"}
}

// expectedFailures holds exactly the bad-version.json file of each fixture directory.
var expectedFailures = []expectedFailure{
	badVersion("advice-page"),
	badVersion("companies-page"),
	badVersion("full-scan"),
	badVersion("held-companies-summary"),
	badVersion("household-page"),
	badVersion("household-summary"),
	badVersion("macro-page"),
	badVersion("macro-summary"),
	badVersion("market-summary"),
	badVersion("market-weather"),
	badVersion("option-page"),
	badVersion("options-summary"),
	badVersion("positions-page"),
	badVersion("structures-list"),
	badVersion("tracking"),
}

// isFixtureDir is true for a directory of fixtures of the old lines; web/fixtures/v3 has its own test.
func isFixtureDir(ent os.DirEntry) bool { return ent.IsDir() && ent.Name() != "v3" }

type leaf struct{ location, keyword string }

func leaves(e *jsonschema.ValidationError, out *[]leaf) {
	if len(e.Causes) == 0 {
		kw := ""
		if kp := e.ErrorKind.KeywordPath(); len(kp) > 0 {
			kw = kp[len(kp)-1]
		}
		loc := ""
		for _, p := range e.InstanceLocation {
			loc += "/" + p
		}
		*out = append(*out, leaf{loc, kw})
		return
	}
	for _, c := range e.Causes {
		leaves(c, out)
	}
}

func validateFixture(t *testing.T, s *jsonschema.Schema, path string) error {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	inst, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return s.Validate(inst)
}

func TestFixturesValidate(t *testing.T) {
	t.Parallel()
	expected := map[string]expectedFailure{}
	for _, e := range expectedFailures {
		if _, err := os.Stat(filepath.Join(fixturesDir, e.path)); err != nil {
			t.Errorf("expected-failure entry without a file: %s", e.path)
		}
		expected[e.path] = e
	}
	if len(expectedFailures) != 15 {
		t.Errorf("expected-failure list holds %d entries, want 15", len(expectedFailures))
	}

	t.Run("market_gap_index_iv_no_data", func(t *testing.T) {
		t.Parallel()
		checkMarketGapIndexIvNoData(t)
	})

	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range entries {
		if !isFixtureDir(ent) {
			continue
		}
		dir := ent.Name()
		schemaFile, ok := fixtureSchema[dir]
		if !ok {
			t.Errorf("fixture directory %s has no row in the table", dir)
			continue
		}
		files, err := os.ReadDir(filepath.Join(fixturesDir, dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, fe := range files {
			if fe.IsDir() || !strings.HasSuffix(fe.Name(), ".json") {
				continue
			}
			rel := dir + "/" + fe.Name()
			t.Run(rel, func(t *testing.T) {
				t.Parallel()
				var s *jsonschema.Schema
				if strings.HasSuffix(schemaFile, ".v2.json") || strings.HasSuffix(schemaFile, ".v3.json") {
					s = compilePageV2(t, schemaFile)
				} else {
					s = compileCarried(t, schemaFile)
				}
				err := validateFixture(t, s, filepath.Join(fixturesDir, rel))
				want, mustFail := expected[rel]
				if !mustFail {
					if err != nil {
						t.Fatalf("want valid: %v", err)
					}
					return
				}
				if err == nil {
					t.Fatal("want a validation error, got none")
				}
				verr, ok := err.(*jsonschema.ValidationError)
				if !ok {
					t.Fatalf("unexpected error type %T: %v", err, err)
				}
				var ls []leaf
				leaves(verr, &ls)
				for _, l := range ls {
					if l.location == want.location && l.keyword == want.keyword {
						return
					}
				}
				t.Fatalf("no leaf at %s with keyword %s; leaves %v", want.location, want.keyword, ls)
			})
		}
	}
}

// checkMarketGapIndexIvNoData is the row market_gap_index_iv_no_data of TestFixturesValidate: the market card relies on an index_table row
// whose iv is a common Gap of reason no_data, so gap.json must hold one.
func checkMarketGapIndexIvNoData(t *testing.T) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixturesDir, "market-summary", "gap.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Card struct {
			IndexTable struct {
				Rows []struct {
					Iv struct {
						Reason string `json:"reason"`
					} `json:"iv"`
				} `json:"rows"`
			} `json:"index_table"`
		} `json:"card"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	for _, r := range doc.Card.IndexTable.Rows {
		if r.Iv.Reason == "no_data" {
			return
		}
	}
	t.Fatal("no index_table row of gap.json has iv.reason no_data")
}
