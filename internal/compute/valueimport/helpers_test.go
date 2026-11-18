package valueimport

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelines"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

func load(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// mutate edits the fixture as a tree of json.Number, so decimal text is kept.
func mutate(t *testing.T, fn func(root map[string]any)) []byte {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(load(t, "synthetic.json")))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		t.Fatal(err)
	}
	fn(root)
	b, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func companies(root map[string]any) []any { return root["companies"].([]any) }
func company0(root map[string]any, i int) map[string]any {
	return companies(root)[i].(map[string]any)
}
func meta(root map[string]any) map[string]any { return root["meta"].(map[string]any) }

func listState() valueimportapi.Stored {
	return valueimportapi.Stored{Underlying: []valueimportapi.StoredUnderlying{
		{Key: "US:SYNA", Symbol: "SYNA", Market: "US"},
		{Key: "US:SYNB B", Symbol: "SYNB B", Market: "US"},
		{Key: "JP:7001", Symbol: "7001", Market: "JP"},
		{Key: "US:SYNG", Symbol: "SYNG", Market: "US"},
	}}
}

func plan(t *testing.T, file []byte, st valueimportapi.Stored, picks map[string]valueimportapi.Pick) valueimportapi.Plan {
	t.Helper()
	p, err := New().Plan(valueimportapi.Input{File: file, Stored: st, Picks: picks, Calc: valuelines.New()})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func rowByName(t *testing.T, p valueimportapi.Plan, name string) valueimportapi.RowPlan {
	t.Helper()
	for _, r := range p.Rows {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no row %s", name)
	return valueimportapi.RowPlan{}
}

// stateAfter is the stored state a plan's inserts would produce.
func stateAfter(base valueimportapi.Stored, p valueimportapi.Plan) valueimportapi.Stored {
	st := base
	st.R = &valueimportapi.StoredR{US: p.R.US, JP: p.R.JP, HKCN: p.R.HKCN, EffectiveDate: p.AsOf, AISwitch: p.R.AISwitch, AIExpiry: p.R.AIExpiry}
	st.Margin, st.MarginDate = p.Margin.Rows, p.Margin.EffectiveDate
	st.Profiles, st.Current = nil, nil
	for _, r := range p.Rows {
		if r.Key == "" || r.Action == valueimportapi.ActionRefused || r.Action == valueimportapi.ActionOlder {
			continue
		}
		st.Profiles = append(st.Profiles, valueimportapi.StoredProfile{Key: r.Key, Name: r.Name, Symbol: r.Symbol,
			Currency: r.Currency, RMarket: r.RMarket, Pool: r.Pool})
		if r.ReviewType != "" && r.Pool != "removed" {
			st.Current = append(st.Current, valueimportapi.StoredCurrent{Key: r.Key, V: r.V, P: r.P, VersionDate: r.VersionDate})
		}
	}
	return st
}

// failCheck is the real calculator with a Check that always reports a problem.
type failCheck struct{ valuelinesapi.Calculator }

func (failCheck) Check(valuelinesapi.CheckInput) []valuelinesapi.Problem {
	return []valuelinesapi.Problem{{Field: "margin", Text: "synthetic problem"}}
}

func num(s string) json.Number { return json.Number(s) }
