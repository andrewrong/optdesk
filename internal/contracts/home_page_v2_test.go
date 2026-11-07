package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const homePageV2 = "page.home_page.v2.json"

// homePageMembers maps each member of the home page to the file its schema is.
var homePageMembers = map[string]string{
	"household": "page.household_summary.v2.json",
	"companies": "page.held_companies_summary.v2.json",
	"macro":     "page.macro_summary.v2.json",
	"market":    "page.market_summary.v3.json",
	"options":   "page.options_summary.v3.json",
}

func homePageMinimal(t *testing.T) map[string]any {
	t.Helper()
	overrides := map[string]string{"^[A-Za-z0-9_-]{1,64}$": "id_1"}
	raw := readRaw(t, schemasDir, homePageV2)
	inst, ok := minimal(t, schemasDir, homePageV2, raw, overrides).(map[string]any)
	if !ok {
		t.Fatal("home_page.v2 is not an object schema")
	}
	return inst
}

func homePageRoundTrip(t *testing.T, m map[string]any) any {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestHomePageV2Valid: the smallest instance, with the five members, is valid.
func TestHomePageV2Valid(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, homePageV2)
	inst := homePageMinimal(t)
	for name := range homePageMembers {
		if _, ok := inst[name].(map[string]any); !ok {
			t.Fatalf("member %s is not built", name)
		}
	}
	if err := s.Validate(homePageRoundTrip(t, inst)); err != nil {
		t.Fatalf("minimal instance is invalid: %v", err)
	}
}

// TestHomePageV2MemberChecked: a member whose schema_version its own file does not allow fails
// at the member's path.
func TestHomePageV2MemberChecked(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, homePageV2)
	for name := range homePageMembers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			inst := homePageMinimal(t)
			inst[name].(map[string]any)["schema_version"] = 99
			err := s.Validate(homePageRoundTrip(t, inst))
			ve, ok := err.(*jsonschema.ValidationError)
			if !ok {
				t.Fatalf("err = %v, want a validation error", err)
			}
			var ls []leaf
			leaves(ve, &ls)
			want := leaf{"/" + name + "/schema_version", "const"}
			for _, l := range ls {
				if l == want {
					return
				}
			}
			t.Errorf("failures = %v, want %v among them", ls, want)
		})
	}
}

// TestHomePageV2NoV1Defs: the file holds no def copied from v1.
func TestHomePageV2NoV1Defs(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join(schemasDir, homePageV2))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "option_trader__") {
		t.Error("the file holds option_trader__")
	}
}
