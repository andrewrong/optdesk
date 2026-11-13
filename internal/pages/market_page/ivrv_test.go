package market_page

import (
	"encoding/json"
	"testing"
)

func ivrvRec(state, value string, code *int, dataDay string) reading {
	r := rec("2026-11-13", state, value, code, dataDay)
	out := reading{rec: &r}
	if dataDay != "" {
		out.dataDay = date(dataDay)
	}
	return out
}

func asFigure(t *testing.T, c any) figure {
	t.Helper()
	f, ok := c.(figure)
	if !ok {
		t.Fatalf("cell = %#v, want a figure", c)
	}
	return f
}

func asGap(t *testing.T, c any) gap {
	t.Helper()
	g, ok := c.(gap)
	if !ok {
		t.Fatalf("cell = %#v, want a gap", c)
	}
	return g
}

func TestIvRvCellSigned(t *testing.T) {
	t.Parallel()
	page := date("2026-11-13")
	for in, want := range map[string]struct{ value, text string }{
		"0.032000": {"3.2", "+3.2"}, "-0.014000": {"-1.4", "-1.4"}, "0.000000": {"0.0", "0.0"},
	} {
		f := asFigure(t, ivRvCell("SPX", ivrvRec("ok", in, nil, "2026-11-13"), page))
		if f.Value.String() != want.value || f.Text != want.text || f.Unit != "vol_points" {
			t.Errorf("%s: %+v, want %v", in, f, want)
		}
	}
}

func TestIvRvCellNamesDayWhenNotPageDay(t *testing.T) {
	t.Parallel()
	page := date("2026-11-13")
	f := asFigure(t, ivRvCell("SPX", ivrvRec("ok", "0.032000", nil, "2026-11-12"), page))
	if f.Text != "+3.2 (11-12)" || f.Value.String() != "3.2" {
		t.Errorf("D-1: %+v", f)
	}
	f = asFigure(t, ivRvCell("SPX", ivrvRec("ok", "0.032000", nil, "2026-11-13"), page))
	if f.Text != "+3.2" {
		t.Errorf("page day: %+v", f)
	}
}

func TestIvRvCellStaleBeyondFiveDays(t *testing.T) {
	t.Parallel()
	page := date("2026-11-13")
	g := asGap(t, ivRvCell("SPX", ivrvRec("ok", "0.032000", nil, "2026-11-07"), page))
	if g.Reason != "stale_input" || g.Text != "SPX "+ivRvStale+"11-07" {
		t.Errorf("6 days: %+v", g)
	}
	f := asFigure(t, ivRvCell("SPX", ivrvRec("ok", "0.032000", nil, "2026-11-08"), page))
	if f.Text != "+3.2 (11-08)" {
		t.Errorf("5 days: %+v", f)
	}
}

func TestIvRvCellStoredCode3IsStale(t *testing.T) {
	t.Parallel()
	page := date("2026-11-13")
	g := asGap(t, ivRvCell("NDX", ivrvRec("missing_input", "", intp(3), "2026-11-10"), page))
	if g.Reason != "stale_input" || g.Text != "NDX "+ivRvStale+"11-10" {
		t.Errorf("with day: %+v", g)
	}
	g = asGap(t, ivRvCell("NDX", ivrvRec("missing_input", "", intp(3), ""), page))
	if g.Reason != "stale_input" || g.Text != "NDX "+ivRvStaleNoDay {
		t.Errorf("no day: %+v", g)
	}
}

func TestIvRvCellNoRowIsNoData(t *testing.T) {
	t.Parallel()
	g := asGap(t, ivRvCell("RUT", reading{}, date("2026-11-13")))
	if g.Reason != "no_data" || g.Text != "RUT implied minus realised: no data, not stored yet" {
		t.Errorf("%+v", g)
	}
}

func TestIvRvCellMissingIsMissingInput(t *testing.T) {
	t.Parallel()
	page := date("2026-11-13")
	for name, r := range map[string]reading{
		"other code": ivrvRec("missing_input", "", intp(1), "2026-11-13"),
		"no code":    ivrvRec("missing_input", "", nil, ""),
		"no day":     ivrvRec("ok", "0.032000", nil, ""),
		"bad value":  ivrvRec("ok", "abc", nil, "2026-11-13"),
	} {
		g := asGap(t, ivRvCell("SPX", r, page))
		if g.Reason != "missing_input" || g.Text != "SPX implied minus realised: no data, inputs incomplete" {
			t.Errorf("%s: %+v", name, g)
		}
	}
}

func TestIvRvCellHasNoThreshold(t *testing.T) {
	t.Parallel()
	page := date("2026-11-13")
	for _, in := range []string{"-0.050000", "0.000000", "0.090000"} {
		c := ivRvCell("SPX", ivrvRec("ok", in, nil, "2026-11-13"), page)
		b, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		if m["kind"] != "value" || len(m) != 4 {
			t.Errorf("%s: %v", in, m)
		}
		for _, k := range []string{"tone", "color", "colour"} {
			if _, ok := m[k]; ok {
				t.Errorf("%s: has %s", in, k)
			}
		}
	}
}
