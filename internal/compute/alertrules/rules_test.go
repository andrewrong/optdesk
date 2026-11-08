package alertrules_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/compute/alertrules"
)

const oneRule = `version = 1
public_subjects = ["SPY"]
[[rule]]
id = "r1"
title = "Title"
figure = "market.ivr_own"
figure_version = 1
subject_kind = "underlying"
subject = "SPY"
unit = "decimal"
op = "ge"
threshold = "0.80"
level = "w"
max_data_age_days = 4
`

func TestDefaultRulesLoad(t *testing.T) {
	t.Parallel()
	r, err := alertrules.DefaultRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Rules) != 11 {
		t.Fatalf("rules = %d, want 11", len(r.Rules))
	}
	if len(r.PublicSubjects) != 5 {
		t.Fatalf("public subjects = %v", r.PublicSubjects)
	}
	if len(r.Hash) != 64 {
		t.Fatalf("hash = %q", r.Hash)
	}
	data, _ := os.ReadFile("rules.toml")
	again, err := alertrules.Load(strings.NewReader(string(data)))
	if err != nil || again.Hash != r.Hash {
		t.Fatalf("hash differs for the same bytes: %v", err)
	}
}

func TestDefaultRulesMatchRegister(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../docs/conventions/register.toml")
	if err != nil {
		t.Fatal(err)
	}
	var reg struct {
		Figure []struct {
			ID      string `toml:"id"`
			Version int    `toml:"version"`
		} `toml:"figure"`
	}
	if err := toml.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, f := range reg.Figure {
		have[f.ID+"@"+string(rune('0'+f.Version))] = true
	}
	r, _ := alertrules.DefaultRules()
	for _, rule := range r.Rules {
		if !have[rule.Figure+"@"+string(rune('0'+rule.FigureVersion))] {
			t.Errorf("rule %s: figure %s version %d is not in the register", rule.ID, rule.Figure, rule.FigureVersion)
		}
	}
}

func TestDefaultRulesMatchRulesJson(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../rules/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var js []struct {
		ID         string `json:"id"`
		Version    int    `json:"version"`
		Unit       string `json:"unit"`
		MaxAgeDays int    `json:"max_age_days"`
	}
	if err := json.Unmarshal(data, &js); err != nil {
		t.Fatal(err)
	}
	r, _ := alertrules.DefaultRules()
	for _, rule := range r.Rules {
		found := false
		for _, j := range js {
			if j.ID != rule.Figure || j.Version != rule.FigureVersion {
				continue
			}
			found = true
			if j.Unit != rule.Unit {
				t.Errorf("rule %s: unit %s, rules.json %s", rule.ID, rule.Unit, j.Unit)
			}
			if rule.MaxDataAgeDays > j.MaxAgeDays {
				t.Errorf("rule %s: data age %d above max_age_days %d", rule.ID, rule.MaxDataAgeDays, j.MaxAgeDays)
			}
		}
		if !found {
			t.Errorf("rule %s: figure %s v%d not in rules.json", rule.ID, rule.Figure, rule.FigureVersion)
		}
	}
}

func loadErr(t *testing.T, src, wantKey, wantID string, noValue ...string) {
	t.Helper()
	_, err := alertrules.Load(strings.NewReader(src))
	if err == nil {
		t.Fatal("want an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, wantKey) || !strings.Contains(msg, wantID) {
		t.Fatalf("error %q lacks key %q or id %q", msg, wantKey, wantID)
	}
	for _, v := range noValue {
		if strings.Contains(msg, v) {
			t.Fatalf("error %q holds the value %q", msg, v)
		}
	}
}

func TestLoadAcceptsOneRule(t *testing.T) {
	t.Parallel()
	r, err := alertrules.Load(strings.NewReader(oneRule))
	if err != nil || len(r.Rules) != 1 || r.Rules[0].Threshold != "0.80" {
		t.Fatalf("%v %+v", err, r)
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	t.Parallel()
	loadErr(t, strings.Replace(oneRule, "level = \"w\"", "level = \"w\"\ncolour = \"secretvalue\"", 1), "colour", "", "secretvalue")
}

func TestLoadRejectsDuplicateID(t *testing.T) {
	t.Parallel()
	loadErr(t, oneRule+strings.TrimPrefix(oneRule, "version = 1\npublic_subjects = [\"SPY\"]\n"), "id", "r1")
}

func TestLoadRejectsBadOp(t *testing.T) {
	t.Parallel()
	loadErr(t, strings.Replace(oneRule, `op = "ge"`, `op = "eqq"`, 1), "op", "r1", "eqq")
}

func TestLoadRejectsBadLevel(t *testing.T) {
	t.Parallel()
	loadErr(t, strings.Replace(oneRule, `level = "w"`, `level = "zz"`, 1), "level", "r1", "zz")
}

func TestLoadRejectsBadThreshold(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"abc", "1e3", "1/2", "", ".5", "0x10"} {
		loadErr(t, strings.Replace(oneRule, `threshold = "0.80"`, `threshold = "`+bad+`"`, 1), "threshold", "r1")
	}
}

func TestLoadRejectsLongTitle(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 61)
	loadErr(t, strings.Replace(oneRule, `title = "Title"`, `title = "`+long+`"`, 1), "title", "r1", long)
	ok := strings.Repeat("é", 60)
	if _, err := alertrules.Load(strings.NewReader(strings.Replace(oneRule, `title = "Title"`, `title = "`+ok+`"`, 1))); err != nil {
		t.Fatalf("60 characters must load: %v", err)
	}
}
