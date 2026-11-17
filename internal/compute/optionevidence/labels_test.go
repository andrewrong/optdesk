package optionevidence_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// TestLabels: the band uses the absolute delta; ex-dividend inside the life sets its label.
func TestLabels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		delta string
		exDiv string
		want  []string
	}{
		{"in band", "-0.19", "", nil},
		{"above band", "-0.35", "", []string{"delta_outside_band"}},
		{"below band", "-0.10", "", []string{"delta_outside_band"}},
		{"positive delta", "0.35", "", []string{"delta_outside_band"}},
		{"ex-div inside", "-0.19", "2026-11-25", []string{"ex_div_inside_life"}},
		{"ex-div after expiry", "-0.19", "2026-12-19", nil},
		{"ex-div on expiry", "-0.19", "2026-12-18", []string{"ex_div_inside_life"}},
	}
	for _, c := range cases {
		r := aaa()
		r.Delta, r.ExDiv = c.delta, contracts.Day(c.exDiv)
		got := eval(t, "0.74", r).Labels
		if got == nil {
			t.Fatalf("%s: Labels is nil", c.name)
		}
		if !slices.Equal(got, append([]string{}, c.want...)) {
			t.Errorf("%s: Labels = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestLabelsDoNotTouchFigures: a row with every label set gives the same figures as one with none.
func TestLabelsDoNotTouchFigures(t *testing.T) {
	t.Parallel()
	none := aaa()
	all := aaa()
	all.Delta, all.ExDiv = "-0.40", "2026-11-25"
	a := eval(t, "0.74", none)
	b := eval(t, "0.74", all)
	if len(a.Labels) != 0 || len(b.Labels) != 2 {
		t.Fatalf("labels: %v %v", a.Labels, b.Labels)
	}
	// "no_edge_at_bid" needs a fair price over the bid; with 0.74 it is not set, so test it too.
	c := eval(t, "1.10", all)
	if len(c.Labels) != 3 {
		t.Fatalf("labels: %v", c.Labels)
	}
	a.Labels, b.Labels = nil, nil
	if !reflect.DeepEqual(a, b) {
		t.Errorf("figures differ: %+v vs %+v", a, b)
	}
	d := eval(t, "0.74", none)
	d.Labels, c.Labels = nil, nil
	c.Fair, c.EdgePerShare, c.EdgePerContract = d.Fair, d.EdgePerShare, d.EdgePerContract
	if !reflect.DeepEqual(c, d) {
		t.Errorf("figures differ: %+v vs %+v", c, d)
	}
}
