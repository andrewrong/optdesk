package home_market

import (
	"testing"
	"time"
)

var idxDay = time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)

func day(s string) time.Time {
	t, err := time.Parse(dayLayout, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestIndexIvCellValueWithSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, text, want, src string }{
		{"SPX", "18.25", "18.2", "FRED VIXCLS 11-06"},
		{"NDX", "21.35", "21.4", "FRED VXNCLS 11-06"},
		{"RUT", "23.46", "23.5", "FRED RVXCLS 11-06"},
	} {
		v, ok := indexIvCell(tc.name, []point{{date: idxDay, text: tc.text}}, idxDay).(volPoints)
		if !ok || v.Text != tc.want || string(v.Value) != tc.want || v.Unit != "vol_points" || v.Source != tc.src || v.Kind != "value" {
			t.Errorf("%s: cell = %+v, want %s from %q", tc.name, v, tc.want, tc.src)
		}
	}
}

func TestIndexIvCellOtherDayThanPage(t *testing.T) {
	t.Parallel()
	v, ok := indexIvCell("SPX", []point{{date: day("2026-11-04"), text: "17.90"}}, idxDay).(volPoints)
	if !ok || v.Source != "FRED VIXCLS 11-04" || v.Text != "17.9" {
		t.Fatalf("cell = %+v", v)
	}
}

func TestIndexIvCellNoPoint(t *testing.T) {
	t.Parallel()
	g, ok := indexIvCell("SPX", nil, idxDay).(gap)
	if !ok || g.Reason != "no_data" || g.Text != "SPX"+indexIvNoRow {
		t.Fatalf("cell = %+v", g)
	}
}

func TestIndexIvCellStaleBeyondFreshDays(t *testing.T) {
	t.Parallel()
	if _, ok := indexIvCell("SPX", []point{{date: day("2026-11-02"), text: "17.9"}}, idxDay).(volPoints); !ok {
		t.Fatal("a point freshDays old must be a value")
	}
	g, ok := indexIvCell("SPX", []point{{date: day("2026-11-01"), text: "17.9"}}, idxDay).(gap)
	if !ok || g.Reason != "stale" || g.Text != "SPX"+indexIvStale+"11-01" {
		t.Fatalf("cell = %+v", g)
	}
}

func TestIndexIvCellNegativeIsMissingInput(t *testing.T) {
	t.Parallel()
	g, ok := indexIvCell("NDX", []point{{date: idxDay, text: "-1.00"}}, idxDay).(gap)
	if !ok || g.Reason != "missing_input" {
		t.Fatalf("cell = %+v", g)
	}
}

func TestIndexIvCellUnmappedName(t *testing.T) {
	t.Parallel()
	g, ok := indexIvCell("N225", []point{{date: idxDay, text: "18.2"}}, idxDay).(gap)
	want := "N225 index IV: no volatility index is mapped (SPX, NDX and RUT only)"
	if !ok || g.Reason != "no_data" || g.Text != want {
		t.Fatalf("cell = %+v", g)
	}
}
