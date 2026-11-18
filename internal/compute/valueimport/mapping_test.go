package valueimport

import (
	"errors"
	"regexp"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
)

func TestMatchUsesListKeyNotFileMarket(t *testing.T) {
	t.Parallel()
	b := mutate(t, func(r map[string]any) {
		c := company0(r, 0)
		c["market"] = "JP"
		c["R"] = meta(r)["R_table_current"].(map[string]any)["JP"]
		c["B_hi"] = num("63")
	})
	row := rowByName(t, plan(t, b, listState(), nil), "SynthAlpha")
	if row.Key != "US:SYNA" || row.RMarket != "US" || row.Unmatched {
		t.Fatalf("row %+v", row)
	}
}

func TestMatchSpaceSymbol(t *testing.T) {
	t.Parallel()
	row := rowByName(t, plan(t, load(t, "synthetic.json"), listState(), nil), "SynthBravo")
	if row.Key != "US:SYNB B" || row.Symbol != "SYNB B" {
		t.Fatalf("row %+v", row)
	}
}

func TestMatchJPDropsSuffix(t *testing.T) {
	t.Parallel()
	row := rowByName(t, plan(t, load(t, "synthetic.json"), listState(), nil), "SynthCedar")
	if row.Key != "JP:7001" || row.RMarket != "JP" || row.Symbol != "7001" {
		t.Fatalf("row %+v", row)
	}
	for in, want := range map[string]string{"7001.T": "7001", "9001.hk": "9001", "000001.SZ": "000001", "600000.SS": "600000", "brk.b": "BRK B"} {
		if got := cleanTicker(in); got != want {
			t.Errorf("clean %q = %q want %q", in, got, want)
		}
	}
}

func TestMatchTwoRowsFileMarketWins(t *testing.T) {
	t.Parallel()
	st := listState()
	st.Underlying = append(st.Underlying, valueimportapi.StoredUnderlying{Key: "JP:SYNA", Symbol: "SYNA", Market: "JP"})
	if u, ok := match(st.Underlying, "SYNA", "US"); !ok || u.Key != "US:SYNA" {
		t.Fatalf("%v %v", u, ok)
	}
	if _, ok := match(st.Underlying, "SYNA", "HK"); ok {
		t.Fatal("two rows and no market match must be unmatched")
	}
}

func TestUnmatchedGetsProposedKeyAndNeedsTick(t *testing.T) {
	t.Parallel()
	p := plan(t, load(t, "synthetic.json"), listState(), nil)
	d := rowByName(t, p, "SynthDelta")
	if !d.Unmatched || d.ProposedKey != "HK:9001" || d.Key != "" || !d.NeedsTick || d.RMarket != "HKCN" || d.Symbol != "" {
		t.Fatalf("row %+v", d)
	}
	picked := plan(t, load(t, "synthetic.json"), listState(), map[string]valueimportapi.Pick{"9001.HK": {Key: "HK:9001", RMarket: "HKCN"}})
	d = rowByName(t, picked, "SynthDelta")
	if d.Key != "HK:9001" || d.ReviewID == "" || !d.NeedsTick || d.Action != valueimportapi.ActionInsert {
		t.Fatalf("row %+v", d)
	}
}

func TestPickedKeyFormChecked(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"HK 9001", "US:SYNA", ""} {
		p := plan(t, load(t, "synthetic.json"), listState(), map[string]valueimportapi.Pick{"9001.HK": {Key: key, RMarket: "HKCN"}})
		d := rowByName(t, p, "SynthDelta")
		if d.Action != valueimportapi.ActionRefused || d.Key != "" {
			t.Errorf("key %q row %+v", key, d)
		}
	}
}

func TestMarginRowsFromParamsByP(t *testing.T) {
	t.Parallel()
	p := plan(t, load(t, "synthetic.json"), listState(), nil)
	want := []string{"0", "3.0", "4.0", "4.5"}
	if len(p.Margin.Rows) != 4 {
		t.Fatalf("rows %v", p.Margin.Rows)
	}
	for i, r := range p.Margin.Rows {
		if r.PFrom != want[i] {
			t.Errorf("row %d p_from %q", i, r.PFrom)
		}
	}
	if r := p.Margin.Rows[1]; r.M != "0.20" || r.W != "0.10" || r.H != "0.30" || r.S != "0.15" {
		t.Fatalf("order m,w,h,s: %+v", r)
	}
	dup := mutate(t, func(r map[string]any) {
		meta(r)["params_by_P"].(map[string]any)["3"] = []any{"0.1", "0.1", "0.1", "0.1"}
	})
	_, err := New().Plan(valueimportapi.Input{File: dup})
	if !errors.Is(err, valueimportapi.ErrBadFile) {
		t.Fatalf("err %v", err)
	}
}

func TestReviewIDMatchesIDForm(t *testing.T) {
	t.Parallel()
	re := regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	p := plan(t, load(t, "synthetic.json"), listState(), nil)
	seen := map[string]bool{}
	for _, r := range p.Rows {
		if r.ReviewID == "" {
			continue
		}
		if !re.MatchString(r.ReviewID) || len(r.ReviewID) != 4+12+1+8 || seen[r.ReviewID] {
			t.Errorf("id %q", r.ReviewID)
		}
		seen[r.ReviewID] = true
	}
	if reviewID(p.FileSHA256, "US:SYNA") == reviewID(sha([]byte("other")), "US:SYNA") {
		t.Fatal("id must change with the file")
	}
}

func TestMemoLinesFixedOrder(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ note, q, want string }{
		{"a", "b", "note: a\nq_note: b"}, {"", "b", "q_note: b"}, {"a", "", "note: a"}, {"", "", ""},
	} {
		if got := changeMemo(c.note, c.q); got != c.want {
			t.Errorf("%q", got)
		}
	}
}
