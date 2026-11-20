package data_health_page

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"
)

type gwDoc struct {
	Gateways []struct {
		Name    string `json:"name"`
		Verdict string `json:"verdict"`
		Note    string `json:"note"`
		Farms   []struct {
			Name string `json:"name"`
			Up   bool   `json:"up"`
		} `json:"farms"`
	} `json:"gateways"`
}

func buildGateways(t *testing.T, gws []GatewayInput) (gwDoc, map[string]json.RawMessage) {
	t.Helper()
	b, err := Build(context.Background(), Input{Now: time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC), Today: "2026-11-20", Gateways: gws})
	if err != nil {
		t.Fatal(err)
	}
	var d gwDoc
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	return d, raw
}

func TestGatewayVerdicts(t *testing.T) {
	t.Parallel()
	farms := []GatewayFarm{{Name: "ushmds", Up: false}, {Name: "usfarm", Up: true}}
	d, _ := buildGateways(t, []GatewayInput{
		{Name: "a", Answered: true, State: "farm_broken", Farms: farms},
		{Name: "b", Answered: true, State: "soft_lost"},
		{Name: "c", Answered: true, State: "down"},
		{Name: "d", Answered: false, State: "farm_broken"},
		{Name: "e", Answered: true, State: "up"},
	})
	want := []string{"farm_down", "session_retrying", "session_retrying", "unreachable", "session_up"}
	if len(d.Gateways) != len(want) {
		t.Fatalf("%d gateways", len(d.Gateways))
	}
	for i, w := range want {
		if d.Gateways[i].Verdict != w {
			t.Errorf("gateway %d verdict %q, want %q", i, d.Gateways[i].Verdict, w)
		}
	}
	if len(d.Gateways[0].Farms) != 2 || d.Gateways[0].Farms[0].Up || !d.Gateways[0].Farms[1].Up || d.Gateways[0].Farms[0].Name != "ushmds" {
		t.Errorf("farms %+v", d.Gateways[0].Farms)
	}
	if d.Gateways[1].Farms == nil {
		t.Error("farms must be an empty list, not null")
	}
}

func TestGatewayTexts(t *testing.T) {
	t.Parallel()
	d, _ := buildGateways(t, []GatewayInput{
		{Name: "main", Answered: true, State: "up"},
		{Name: "main", Answered: true, State: "down"},
		{Name: "main", Answered: true, State: "farm_broken"},
		{Name: "main", Answered: false},
	})
	idPort := regexp.MustCompile(`\d{4,}|DU|U\d`)
	seen := map[string]bool{}
	for _, g := range d.Gateways {
		seen[g.Verdict] = true
		if g.Note == "" || !strings.ContainsAny(g.Note, "的") || !strings.Contains(g.Note, g.Verdict) {
			t.Errorf("%s: note %q lacks Chinese text or its term", g.Verdict, g.Note)
		}
		if idPort.MatchString(g.Note) {
			t.Errorf("%s: note holds an id or port: %q", g.Verdict, g.Note)
		}
		if g.Verdict == "farm_down" && (!strings.Contains(g.Note, "重新登录") || !strings.Contains(g.Note, "重连")) {
			t.Errorf("farm_down note %q", g.Note)
		}
	}
	if len(seen) != 4 {
		t.Errorf("verdicts seen %v", seen)
	}
}

func TestNoGatewaysOmitsField(t *testing.T) {
	t.Parallel()
	for _, gws := range [][]GatewayInput{nil, {}} {
		_, raw := buildGateways(t, gws)
		if _, ok := raw["gateways"]; ok {
			t.Fatal("gateways key present")
		}
	}
}

func TestUnknownStateNotUp(t *testing.T) {
	t.Parallel()
	d, _ := buildGateways(t, []GatewayInput{
		{Name: "a", Answered: true, State: ""},
		{Name: "b", Answered: true, State: "SESSION_STATE_UNSPECIFIED"},
		{Name: "c", Answered: true, State: "weird"},
	})
	for _, g := range d.Gateways {
		if g.Verdict == "session_up" {
			t.Errorf("%s got session_up", g.Name)
		}
	}
}
