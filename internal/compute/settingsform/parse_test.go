package settingsform

import (
	"reflect"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
)

var shown = []int64{1, 2, 3}

func cur() settingsapi.FormState {
	return settingsapi.FormState{
		Params:     map[string]string{"min_dte": "21", "max_spread": "0.1"},
		Exclusions: map[int64]bool{2: true},
	}
}

func TestParseOnlyDifferingFields(t *testing.T) {
	t.Parallel()
	got, err := newForm().Parse(map[string][]string{
		"param.min_dte":    {"21"},
		"param.max_spread": {"0.20"},
		"row":              {"1", "2", "3"},
		"excluded":         {"2", "3"},
		"confirm":          {"1"},
		"expires":          {"1"},
	}, shown, cur())
	if err != nil {
		t.Fatal(err)
	}
	want := settingsapi.FormChange{
		Params:     []settingsapi.FormField{{Key: "max_spread", Text: "0.2"}},
		Exclusions: map[int64]bool{3: true},
		Confirm:    true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	none, err := newForm().Parse(map[string][]string{"param.min_dte": {" 21 "}, "row": {"1", "2"}, "excluded": {"2"}}, shown, cur())
	if err != nil || len(none.Params) != 0 || len(none.Exclusions) != 0 || none.Confirm {
		t.Fatalf("untouched form changed %+v err %v", none, err)
	}
}

func TestParseRefusesUnknownKey(t *testing.T) {
	t.Parallel()
	_, err := newForm().Parse(map[string][]string{"param.nope": {"1"}}, shown, cur())
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("unknown key: %v", err)
	}
	_, err = newForm().Parse(map[string][]string{"param.min_dte": {"abc"}}, shown, cur())
	if err == nil || !strings.Contains(err.Error(), "min_dte") {
		t.Fatalf("bad value should name the key: %v", err)
	}
}

func TestParseRefusesUnshownInstrument(t *testing.T) {
	t.Parallel()
	for _, v := range []map[string][]string{
		{"row": {"9"}},
		{"row": {"1"}, "excluded": {"9"}},
		{"row": {"x"}},
		{"excluded": {"1.5"}},
	} {
		if _, err := newForm().Parse(v, shown, cur()); err == nil {
			t.Errorf("%v accepted", v)
		}
	}
}

func TestParseUncheckedSwitchMeansIncluded(t *testing.T) {
	t.Parallel()
	got, err := newForm().Parse(map[string][]string{"row": {"1", "2"}}, shown, cur())
	if err != nil {
		t.Fatal(err)
	}
	if want := (map[int64]bool{2: false}); !reflect.DeepEqual(got.Exclusions, want) {
		t.Fatalf("got %v want %v", got.Exclusions, want)
	}
}

func TestParseEmptyIsReset(t *testing.T) {
	t.Parallel()
	got, err := newForm().Parse(map[string][]string{"param.min_dte": {""}, "param.max_spread": {"  "}}, shown, cur())
	if err != nil {
		t.Fatal(err)
	}
	want := []settingsapi.FormField{{Key: "min_dte", Text: ""}, {Key: "max_spread", Text: ""}}
	if !reflect.DeepEqual(got.Params, want) {
		t.Fatalf("got %+v want %+v", got.Params, want)
	}
}

func TestParseNamesFirstUnknownKeyInOrder(t *testing.T) {
	t.Parallel()
	for i := 0; i < 20; i++ {
		_, err := newForm().Parse(map[string][]string{"param.zzz": {"1"}, "param.aaa": {"1"}, "param.mmm": {"1"}}, shown, cur())
		if err == nil || !strings.Contains(err.Error(), "aaa") {
			t.Fatalf("first unknown key not named: %v", err)
		}
	}
}
