package valueimport

import (
	"errors"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

func TestPlanFirstReviewForNewName(t *testing.T) {
	t.Parallel()
	p := plan(t, load(t, "synthetic.json"), listState(), nil)
	r := rowByName(t, p, "SynthAlpha")
	if r.Action != valueimportapi.ActionInsert || r.ReviewType != "first" || r.Key != "US:SYNA" || !r.NewProfile {
		t.Fatalf("row %+v", r)
	}
	if r.ChangeMemo != "note: alpha note\nq_note: alpha quarter" || r.ReportText != "synthetic report A" || r.SiteBuyTop != "63" {
		t.Fatalf("row %+v", r)
	}
	if p.R.Action != valueimportapi.ActionInsert || p.Margin.Action != valueimportapi.ActionInsert || p.FlaggedCount != 1 {
		t.Fatalf("plan %+v", p)
	}
}

func TestPlanEventReviewWhenValueChanges(t *testing.T) {
	t.Parallel()
	st := listState()
	st.Current = []valueimportapi.StoredCurrent{{Key: "US:SYNA", V: "90", P: "4.5", VersionDate: "2026-01-01"}}
	st.Profiles = []valueimportapi.StoredProfile{{Key: "US:SYNA", Name: "SynthAlpha", Symbol: "SYNA", Currency: "USD", RMarket: "US", Pool: "in_pool"}}
	r := rowByName(t, plan(t, load(t, "synthetic.json"), st, nil), "SynthAlpha")
	if r.Action != valueimportapi.ActionInsert || r.ReviewType != "event" || r.NewProfile {
		t.Fatalf("row %+v", r)
	}
}

func TestPlanSameWhenEqualAsDecimals(t *testing.T) {
	t.Parallel()
	st := listState()
	st.Current = []valueimportapi.StoredCurrent{{Key: "US:SYNA", V: "100", P: "4.50", VersionDate: "2026-01-01"}}
	st.Profiles = []valueimportapi.StoredProfile{{Key: "US:SYNA", Name: "SynthAlpha", RMarket: "US", Pool: "in_pool"}}
	r := rowByName(t, plan(t, load(t, "synthetic.json"), st, nil), "SynthAlpha")
	if r.Action != valueimportapi.ActionSame {
		t.Fatalf("row %+v", r)
	}
}

func TestPlanSecondRunOfSameFileWritesNothing(t *testing.T) {
	t.Parallel()
	file := load(t, "synthetic.json")
	picks := map[string]valueimportapi.Pick{"9001.HK": {Key: "HK:9001", RMarket: "HKCN"}, "SYNF": {Key: "US:SYNF2", RMarket: "US"}}
	first := plan(t, file, listState(), picks)
	second := plan(t, file, stateAfter(listState(), first), picks)
	if second.R.Action != valueimportapi.ActionSame || second.Margin.Action != valueimportapi.ActionSame {
		t.Fatalf("tables %q %q", second.R.Action, second.Margin.Action)
	}
	for _, r := range second.Rows {
		if r.Action != valueimportapi.ActionSame {
			t.Errorf("row %s action %q reason %q", r.Name, r.Action, r.Reason)
		}
	}
	if len(second.Rows) != 6 {
		t.Fatalf("rows %d", len(second.Rows))
	}
}

func TestPlanOlderFileNeverOverwrites(t *testing.T) {
	t.Parallel()
	st := listState()
	// stored tables are newer; they differ from the file only where no buy-zone top depends on it
	st.R = &valueimportapi.StoredR{US: "0.90", JP: "0.80", HKCN: "0.75", EffectiveDate: "2026-03-01"}
	st.Margin = []valuelinesapi.MarginRow{
		{PFrom: "0", M: "0.15", W: "0.10", H: "0.50", S: "0.15"}, {PFrom: "3.0", M: "0.20", W: "0.10", H: "0.50", S: "0.15"},
		{PFrom: "4.0", M: "0.25", W: "0.10", H: "0.50", S: "0.15"}, {PFrom: "4.5", M: "0.30", W: "0.10", H: "0.50", S: "0.15"},
	}
	st.MarginDate = "2026-03-02"
	st.Profiles = []valueimportapi.StoredProfile{{Key: "US:SYNA", Name: "SynthAlpha", RMarket: "US", Pool: "in_pool"}}
	st.Current = []valueimportapi.StoredCurrent{{Key: "US:SYNA", V: "90", P: "4.5", VersionDate: "2026-02-20"}}
	p := plan(t, load(t, "synthetic.json"), st, nil)
	if p.R.Action != valueimportapi.ActionOlder || p.Margin.Action != valueimportapi.ActionOlder {
		t.Fatalf("tables %q %q", p.R.Action, p.Margin.Action)
	}
	if r := rowByName(t, p, "SynthAlpha"); r.Action != valueimportapi.ActionOlder {
		t.Fatalf("row %+v", r)
	}
}

func TestPlanOlderRowIsSkipped(t *testing.T) {
	t.Parallel()
	st := listState()
	st.Profiles = []valueimportapi.StoredProfile{{Key: "US:SYNA", Name: "SynthAlpha", RMarket: "US", Pool: "in_pool"}}
	st.Current = []valueimportapi.StoredCurrent{{Key: "US:SYNA", V: "90", P: "4.5", VersionDate: "2026-02-20"}}
	if r := rowByName(t, plan(t, load(t, "synthetic.json"), st, nil), "SynthAlpha"); r.Action != valueimportapi.ActionOlder {
		t.Fatalf("row %+v", r)
	}
}

func TestPlanRCarriesAISwitchOver(t *testing.T) {
	t.Parallel()
	st := listState()
	st.R = &valueimportapi.StoredR{US: "0.85", JP: "0.8", HKCN: "0.7", EffectiveDate: "2026-01-01", AISwitch: true, AIExpiry: "2026-12-31"}
	p := plan(t, load(t, "synthetic.json"), st, nil)
	if p.R.Action != valueimportapi.ActionInsert || !p.R.AISwitch || p.R.AIExpiry != "2026-12-31" {
		t.Fatalf("r %+v", p.R)
	}
}

func TestPlanNotInFileUnchanged(t *testing.T) {
	t.Parallel()
	st := listState()
	st.Profiles = []valueimportapi.StoredProfile{{Key: "US:OLDCO", Name: "SynthOld", RMarket: "US", Pool: "in_pool"}}
	st.Current = []valueimportapi.StoredCurrent{{Key: "US:OLDCO", V: "1", P: "3", VersionDate: "2025-01-01"}}
	p := plan(t, load(t, "synthetic.json"), st, nil)
	if len(p.NotInFile) != 1 || p.NotInFile[0] != "US:OLDCO" {
		t.Fatalf("not in file %v", p.NotInFile)
	}
	for _, r := range p.Rows {
		if r.Key == "US:OLDCO" {
			t.Fatalf("row for a name not in file: %+v", r)
		}
	}
	if len(p.Unvalued) != 1 || p.Unvalued[0] != "SYNU" {
		t.Fatalf("unvalued %v", p.Unvalued)
	}
}

func TestPlanRemovedNameGetsPoolRemoved(t *testing.T) {
	t.Parallel()
	st := listState()
	st.Profiles = []valueimportapi.StoredProfile{{Key: "US:SYNG", Name: "SynthGone", Symbol: "SYNG", Currency: "USD", RMarket: "US", Pool: "in_pool"}}
	p := plan(t, load(t, "synthetic.json"), st, nil)
	r := rowByName(t, p, "SynthGone")
	if r.Pool != "removed" || r.Action != valueimportapi.ActionProfileOnly || r.ReviewType != "" || r.NewProfile {
		t.Fatalf("row %+v", r)
	}
	if len(p.RemovedNames) != 1 || p.RemovedNames[0] != "SynthGone" {
		t.Fatalf("removed %v", p.RemovedNames)
	}
	// no profile yet: a new profile with pool removed and no review
	r = rowByName(t, plan(t, load(t, "synthetic.json"), listState(), nil), "SynthGone")
	if r.Pool != "removed" || !r.NewProfile || r.Key != "US:SYNG" {
		t.Fatalf("row %+v", r)
	}
	// a name with a valuation in the file is never moved
	b := mutate(t, func(root map[string]any) { company0(root, 0)["name"] = "SynthGone" })
	for _, r := range plan(t, b, listState(), nil).Rows {
		if r.Name == "SynthGone" && r.Pool == "removed" {
			t.Fatalf("moved: %+v", r)
		}
	}
}

func TestPlanNeverPostEarningsNorBackfill(t *testing.T) {
	t.Parallel()
	st := listState()
	st.Profiles = []valueimportapi.StoredProfile{{Key: "US:SYNA", Name: "SynthAlpha", RMarket: "US", Pool: "in_pool"}}
	st.Current = []valueimportapi.StoredCurrent{{Key: "US:SYNA", V: "90", P: "4.5", VersionDate: "2026-01-01"}}
	for _, r := range plan(t, load(t, "synthetic.json"), st, nil).Rows {
		if r.ReviewType != "" && r.ReviewType != "first" && r.ReviewType != "event" {
			t.Errorf("type %q", r.ReviewType)
		}
	}
}

func TestPlanBadFileNamesPathNotValue(t *testing.T) {
	t.Parallel()
	_, err := New().Plan(valueimportapi.Input{File: load(t, "synthetic_bad.json")})
	var fe *valueimportapi.FileError
	if !errors.Is(err, valueimportapi.ErrBadFile) || !errors.As(err, &fe) {
		t.Fatalf("err %v", err)
	}
	if fe.Path != "companies[0].V" || strings.Contains(err.Error(), "secret-looking-text") {
		t.Fatalf("path %q err %q", fe.Path, err)
	}
}

func TestPlanBadFileCases(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		file []byte
		path string
	}{
		"missing as_of":   {mutate(t, func(r map[string]any) { delete(meta(r), "as_of") }), "meta.as_of"},
		"missing HK":      {mutate(t, func(r map[string]any) { delete(meta(r)["R_table_current"].(map[string]any), "HK") }), "meta.R_table_current.HK"},
		"missing B_hi":    {mutate(t, func(r map[string]any) { delete(company0(r, 2), "B_hi") }), "companies[2].B_hi"},
		"wrong type name": {mutate(t, func(r map[string]any) { company0(r, 1)["name"] = 5 }), "companies[1].name"},
		"too big":         {append([]byte(`{"x":"`), make([]byte, valueimportapi.MaxFileBytes)...), "file"},
		"too many": {mutate(t, func(r map[string]any) {
			c := company0(r, 0)
			l := make([]any, valueimportapi.MaxCompanies+1)
			for i := range l {
				l[i] = c
			}
			r["companies"] = l
		}), "companies"},
	}
	for name, c := range cases {
		_, err := New().Plan(valueimportapi.Input{File: c.file})
		var fe *valueimportapi.FileError
		if !errors.As(err, &fe) || fe.Path != c.path {
			t.Errorf("%s: err %v", name, err)
		}
	}
}
