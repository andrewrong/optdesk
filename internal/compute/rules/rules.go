// Package rules holds the table of rules behind the figure register: the numbers of every
// figure implemented in Go, embedded from rules.json. It imports the compute root only.
package rules

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
)

//go:embed rules.json
var tableJSON []byte

// entry is one object of rules.json; the field names are its keys.
type entry struct {
	ID          string `json:"id"`
	Version     int    `json:"version"`
	Convention  string `json:"convention"`
	Unit        string `json:"unit"`
	Scale       int    `json:"scale"`
	Compare     string `json:"compare"`
	N           int    `json:"n"`
	Window      int    `json:"window"`
	MinValid    int    `json:"min_valid"`
	Years       int    `json:"years"`
	TradingDays int    `json:"trading_days"`
	MaxAgeDays  int    `json:"max_age_days"`
	Hash        string `json:"hash"`
}

type table struct {
	latest map[string]compute.Rule
	ids    []string
}

// New is a compute.NewRulesFunc. The table is embedded data: a malformed rules.json is a
// build defect, so New panics on it (the package tests parse it).
func New() compute.Rules {
	var entries []entry
	if err := json.Unmarshal(tableJSON, &entries); err != nil {
		panic(fmt.Sprintf("rules: rules.json: %v", err))
	}
	t := table{latest: map[string]compute.Rule{}}
	for _, e := range entries {
		r := compute.Rule{
			ID: e.ID, Version: e.Version, Convention: e.Convention, Unit: e.Unit,
			Scale: e.Scale, Compare: compute.Compare(e.Compare), N: e.N, Window: e.Window,
			MinValid: e.MinValid, Years: e.Years, TradingDays: e.TradingDays,
			MaxAgeDays: e.MaxAgeDays, Hash: e.Hash,
		}
		if old, ok := t.latest[e.ID]; !ok || r.Version > old.Version {
			t.latest[e.ID] = r
		}
	}
	for id := range t.latest {
		t.ids = append(t.ids, id)
	}
	sort.Strings(t.ids)
	return t
}

var _ compute.NewRulesFunc = New

// Rule gives the highest version of id.
func (t table) Rule(id string) (compute.Rule, bool) {
	r, ok := t.latest[id]
	return r, ok
}

// IDs lists the ids in sorted order.
func (t table) IDs() []string { return append([]string(nil), t.ids...) }
