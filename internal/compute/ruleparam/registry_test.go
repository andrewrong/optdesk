package ruleparam

import (
	"errors"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
)

var _ settingsapi.NewRuleParamsFunc = New

// table is Decisions 2 and 3 of docs/design/settings-form.md, key by key:
// default, minimum, maximum.
var table = [][4]string{
	{"min_dte", "21", "7", "120"},
	{"max_dte", "45", "14", "180"},
	{"min_bid_size", "1", "1", "100"},
	{"max_spread", "0.1", "0.01", "0.3"},
	{"min_open_interest", "10", "1", "1000"},
	{"div_guard_factor", "1", "0.5", "3"},
	{"max_rate_age_days", "7", "1", "30"},
	{"max_event_age_sessions", "2", "1", "10"},
	{"min_report_gap_days", "60", "30", "100"},
	{"max_strikes", "24", "4", "60"},
	{"capture_target", "0.5", "0.1", "0.95"},
	{"exit_dte", "21", "1", "45"},
}

func TestTwelveKeysInOrder(t *testing.T) {
	t.Parallel()
	specs := New().Specs()
	var keys []string
	for _, s := range specs {
		keys = append(keys, s.Key)
		if s.Label == "" || (s.Kind != "int" && s.Kind != "decimal") {
			t.Errorf("%s: label %q kind %q", s.Key, s.Label, s.Kind)
		}
	}
	want := []string{"min_dte", "max_dte", "min_bid_size", "max_spread", "min_open_interest",
		"div_guard_factor", "max_rate_age_days", "max_event_age_sessions", "min_report_gap_days",
		"max_strikes", "capture_target", "exit_dte"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v", keys)
	}
	specs[0].Key = "changed"
	if New().Specs()[0].Key != "min_dte" {
		t.Fatal("Specs returns the table itself")
	}
}

func TestDefaultsMatchDesign(t *testing.T) {
	t.Parallel()
	specs := New().Specs()
	for i, row := range table {
		s := specs[i]
		if s.Key != row[0] || s.Default != row[1] || s.Min != row[2] || s.Max != row[3] {
			t.Errorf("%s: got default %s min %s max %s, want %v", s.Key, s.Default, s.Min, s.Max, row)
		}
	}
}

func TestEmptyMeansDefault(t *testing.T) {
	t.Parallel()
	p := New()
	for _, s := range p.Specs() {
		got, err := p.Normalize(s.Key, "")
		if err != nil || got != "" {
			t.Errorf("%s: Normalize(\"\") = %q, %v", s.Key, got, err)
		}
		got, err = p.Normalize(s.Key, "  ")
		if err != nil || got != "" {
			t.Errorf("%s: blank = %q, %v", s.Key, got, err)
		}
	}
}

func TestResolveFillsDefaults(t *testing.T) {
	t.Parallel()
	p := New()
	got := p.Resolve(map[string]string{"min_dte": "30", "max_spread": "", "nope": "1"})
	if len(got) != 12 {
		t.Fatalf("len = %d", len(got))
	}
	if got["min_dte"] != "30" || got["max_spread"] != "0.1" || got["exit_dte"] != "21" {
		t.Fatalf("got %v", got)
	}
	if _, ok := got["nope"]; ok {
		t.Fatal("unknown key kept")
	}
	if len(p.Resolve(nil)) != 12 {
		t.Fatal("nil stored")
	}
}

func TestUnknownKeyRefused(t *testing.T) {
	t.Parallel()
	_, err := New().Normalize("min_dtes", "21")
	if !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("err = %v", err)
	}
	if _, err := New().Normalize("", ""); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("empty key err = %v", err)
	}
}
