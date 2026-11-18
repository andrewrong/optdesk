package valueimportapi_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

// fakeImporter returns a plan with every action and records the input it saw.
type fakeImporter struct{ seen valueimportapi.Input }

var _ valueimportapi.Importer = (*fakeImporter)(nil)

func (f *fakeImporter) Plan(in valueimportapi.Input) (valueimportapi.Plan, error) {
	f.seen = in
	if len(in.File) == 0 {
		return valueimportapi.Plan{}, &valueimportapi.FileError{Path: "meta.as_of"}
	}
	return valueimportapi.Plan{
		FileSHA256: "abc",
		AsOf:       "2026-01-02",
		R:          valueimportapi.RPlan{Action: valueimportapi.ActionInsert, US: "0.9", JP: "0.8", HKCN: "0.7", AISwitch: true, AIExpiry: "2026-06-01"},
		Margin: valueimportapi.MarginPlan{
			Action:        valueimportapi.ActionOlder,
			Rows:          []valuelinesapi.MarginRow{{PFrom: "0", M: "0.1", W: "0.1", H: "0.2", S: "0.1"}},
			EffectiveDate: "2026-01-02",
		},
		Rows: []valueimportapi.RowPlan{
			{Key: "US:AAA", Action: valueimportapi.ActionInsert, ReviewType: "first", NeedsTick: true},
			{Key: "US:BBB", Action: valueimportapi.ActionSame},
			{Key: "JP:1111", Action: valueimportapi.ActionOlder},
			{Key: "", Action: valueimportapi.ActionRefused, Reason: "x", Unmatched: true, ProposedKey: "HK:0700"},
			{Key: "US:CCC", Action: valueimportapi.ActionProfileOnly, NewProfile: true},
		},
		NotInFile: []string{"US:DDD"}, Unvalued: []string{"EEE"}, RemovedNames: []string{"FFF"}, FlaggedCount: 2,
	}, nil
}

// TestImporterContractBothSides: a caller reads the fake's plan unchanged, and the fake sees the caller's input.
func TestImporterContractBothSides(t *testing.T) {
	t.Parallel()
	f := &fakeImporter{}
	var imp valueimportapi.Importer = f
	in := valueimportapi.Input{
		File: []byte("{}"),
		Stored: valueimportapi.Stored{
			Underlying: []valueimportapi.StoredUnderlying{{Key: "US:AAA", Symbol: "AAA", Market: "US"}},
			R:          &valueimportapi.StoredR{US: "0.9", EffectiveDate: "2025-01-01"},
			Latest:     "4.2",
		},
		Picks: map[string]valueimportapi.Pick{"X": {Key: "HK:0700", RMarket: "HKCN"}},
	}
	p, err := imp.Plan(in)
	if err != nil {
		t.Fatal(err)
	}
	if f.seen.Picks["X"].Key != "HK:0700" || f.seen.Stored.R.US != "0.9" {
		t.Fatalf("input changed: %+v", f.seen)
	}
	if p.R.Action != valueimportapi.ActionInsert || p.Margin.Action != valueimportapi.ActionOlder || p.R.AIExpiry != "2026-06-01" {
		t.Fatalf("plan changed: %+v", p)
	}
	want := []string{
		valueimportapi.ActionInsert, valueimportapi.ActionSame, valueimportapi.ActionOlder,
		valueimportapi.ActionRefused, valueimportapi.ActionProfileOnly,
	}
	if len(p.Rows) != len(want) {
		t.Fatalf("rows %d", len(p.Rows))
	}
	for i, a := range want {
		if p.Rows[i].Action != a {
			t.Errorf("row %d action %q want %q", i, p.Rows[i].Action, a)
		}
	}
	if p.Rows[3].ProposedKey != "HK:0700" || p.FlaggedCount != 2 || p.NotInFile[0] != "US:DDD" {
		t.Fatalf("plan changed: %+v", p)
	}
	var plan valueimportapi.PlanFunc = f.Plan
	if _, err := plan(valueimportapi.Input{}); !errors.Is(err, valueimportapi.ErrBadFile) {
		t.Fatalf("empty input: %v", err)
	}
}

// TestConstantsAreTheDesignValues: the constants are those of the design.
func TestConstantsAreTheDesignValues(t *testing.T) {
	t.Parallel()
	if valueimportapi.MaxFileBytes != 1048576 || valueimportapi.MaxCompanies != 500 {
		t.Fatal("size constants")
	}
	if valueimportapi.RoundTolerance != "0.005" || valueimportapi.RoundToleranceAbs != "0.5" {
		t.Fatal("tolerances")
	}
	names := map[string]string{
		"insert": valueimportapi.ActionInsert, "same": valueimportapi.ActionSame, "older": valueimportapi.ActionOlder,
		"refused": valueimportapi.ActionRefused, "profile_only": valueimportapi.ActionProfileOnly,
	}
	for want, got := range names {
		if want != got {
			t.Errorf("action %q is %q", want, got)
		}
	}
}

// TestFileErrorNamesPathOnly: the text of a FileError holds the path and nothing of the input.
func TestFileErrorNamesPathOnly(t *testing.T) {
	t.Parallel()
	err := error(&valueimportapi.FileError{Path: "companies[3].V"})
	if !strings.Contains(err.Error(), "companies[3].V") {
		t.Fatalf("no path: %q", err)
	}
	if !errors.Is(err, valueimportapi.ErrBadFile) {
		t.Fatal("not ErrBadFile")
	}
	if strings.Contains(err.Error(), "999.99") {
		t.Fatal("value leaked")
	}
}
