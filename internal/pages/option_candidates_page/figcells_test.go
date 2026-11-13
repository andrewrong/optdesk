package option_candidates_page

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionfigapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeFigs struct {
	res   optionfigapi.PutFigures
	got   []optionfigapi.PutFigureInput
	calls int
}

func (f *fakeFigs) Put(in optionfigapi.PutFigureInput) optionfigapi.PutFigures {
	f.calls++
	f.got = append(f.got, in)
	return f.res
}

type fakeRate struct {
	pts   []contracts.SeriesPoint
	err   error
	id    string
	from  contracts.Day
	to    contracts.Day
	calls int
}

func (f *fakeRate) SeriesPoints(_ context.Context, id string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	f.calls++
	f.id, f.from, f.to = id, from, to
	return f.pts, f.err
}

func point(day string, v *float64) contracts.SeriesPoint {
	var p contracts.SeriesPoint
	t, _ := time.Parse(time.DateOnly, day)
	p.ObservationDate.Time = t
	p.Value = v
	return p
}

func okFigs() optionfigapi.PutFigures {
	return optionfigapi.PutFigures{ItmPercent: "27.5%", ItmKey: "2755", ExcessPercent: "4.5%", ExcessAbove: true, ExcessKey: "450"}
}

func TestFigureColumnsNilFigures(t *testing.T) {
	t.Parallel()
	g := buildGroup(groupSpecs[0], "2026-11-09", "2026-11-09", []store.Candidate{cand("short_put", 1, "ok")}, nil, rateRead{})
	for _, id := range []string{"itm", "excess"} {
		if c, ok := g.Main.Rows[0].Cells[id].(gap); !ok || c.Reason != "no_reader" {
			t.Errorf("%s = %+v", id, g.Main.Rows[0].Cells[id])
		}
	}
}

func TestFigureColumnsCells(t *testing.T) {
	t.Parallel()
	f := &fakeFigs{res: okFigs()}
	rows := []store.Candidate{cand("short_put", 1, "ok"), cand("short_put", 2, "ok"), cand("short_put", 3, "blocked")}
	g := buildGroup(groupSpecs[0], "2026-11-09", "2026-11-10", rows, f, rateRead{text: "4.5", day: "2026-11-08"})
	if f.calls != 2 {
		t.Fatalf("Put called %d times, want once per ok row", f.calls)
	}
	if in := f.got[0]; in.ChainDay != "2026-11-09" || in.DTE != 39 || in.Strike != "95.00" || in.IV != "0.31" ||
		in.AnnualYield != "0.123456" || in.Rate != "4.5" || in.RateDay != "2026-11-08" || in.MaxRateAgeDays != 7 {
		t.Errorf("input = %+v", in)
	}
	itm := g.Main.Rows[1].Cells["itm"].(valueCell)
	if itm.Text != "27.5%" || string(*itm.Value) != "2755" {
		t.Errorf("itm = %+v", itm)
	}
	ex := g.Main.Rows[1].Cells["excess"].(valueCell)
	if ex.Text != "above 4.5%" || string(*ex.Value) != "450" {
		t.Errorf("excess = %+v", ex)
	}
	if _, ok := g.Other.Rows[0].Cells["itm"]; ok {
		t.Error("the other table has a figure column")
	}
}

func TestFigureBelowAndGaps(t *testing.T) {
	t.Parallel()
	f := &fakeFigs{res: optionfigapi.PutFigures{ExcessPercent: "1.0%", ExcessKey: "-100", ItmGap: "stale_input", ItmWhy: "cash rate is 9 days old"}}
	g := buildGroup(groupSpecs[0], "2026-11-09", "2026-11-09", []store.Candidate{cand("short_put", 1, "ok")}, f, rateRead{})
	if c := g.Main.Rows[0].Cells["excess"].(valueCell); c.Text != "below 1.0%" || string(*c.Value) != "-100" {
		t.Errorf("excess = %+v", c)
	}
	if c, ok := g.Main.Rows[0].Cells["itm"].(gap); !ok || c.Reason != "stale_input" || c.Text != "cash rate is 9 days old" {
		t.Errorf("itm = %+v", g.Main.Rows[0].Cells["itm"])
	}
}

func TestCallGroupsHaveNoFigureColumns(t *testing.T) {
	t.Parallel()
	f := &fakeFigs{res: okFigs()}
	for _, s := range groupSpecs[1:] {
		g := buildGroup(s, "2026-11-09", "2026-11-09", []store.Candidate{cand(s.kind, 1, "ok")}, f, rateRead{})
		for _, col := range g.Main.Columns {
			id := col.ID
			if id == "itm" || id == "excess" {
				t.Errorf("%s has column %s", s.kind, id)
			}
		}
	}
	if f.calls != 0 {
		t.Errorf("Put called %d times for call groups", f.calls)
	}
}

func TestReadRate(t *testing.T) {
	t.Parallel()
	v1, v2 := 4.25, 4.5
	r := &fakeRate{pts: []contracts.SeriesPoint{point("2026-11-05", &v1), point("2026-11-07", &v2), point("2026-11-08", nil)}}
	got, err := readRate(context.Background(), r, "2026-11-09")
	if err != nil || got.text != "4.5" || got.day != "2026-11-07" {
		t.Fatalf("rate = %+v, %v", got, err)
	}
	if r.id != "DGS3MO" || r.from != "2026-10-26" || r.to != "2026-11-09" {
		t.Errorf("read %s %s..%s", r.id, r.from, r.to)
	}
	if got, err = readRate(context.Background(), nil, "2026-11-09"); err != nil || got != (rateRead{}) {
		t.Errorf("nil reader = %+v, %v", got, err)
	}
	if got, err = readRate(context.Background(), &fakeRate{}, "2026-11-09"); err != nil || got != (rateRead{}) {
		t.Errorf("no point = %+v, %v", got, err)
	}
	if _, err = readRate(context.Background(), &fakeRate{err: errors.New("boom")}, "2026-11-09"); err == nil {
		t.Error("reader error is dropped")
	}
}

func TestBuildReadsRateOncePerGroupAndPassesIt(t *testing.T) {
	t.Parallel()
	v := 4.5
	r := &fakeRate{pts: []contracts.SeriesPoint{point("2026-11-08", &v)}}
	f := &fakeFigs{res: okFigs()}
	rows := map[string][]store.Candidate{}
	for _, s := range groupSpecs {
		rows[string(buildDay)+"/"+s.kind] = []store.Candidate{cand(s.kind, 1, "ok")}
	}
	c := cfg(fakeReader{rows: rows})
	c.Figures = f
	if _, err := Build(context.Background(), c, r, buildDay, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if r.calls != len(groupSpecs) {
		t.Errorf("series read %d times", r.calls)
	}
	if len(f.got) != 1 || f.got[0].Rate != "4.5" || f.got[0].RateDay != "2026-11-08" {
		t.Errorf("Put inputs = %+v", f.got)
	}
	if _, err := Build(context.Background(), c, &fakeRate{err: errors.New("boom")}, buildDay, time.Time{}); err == nil {
		t.Error("series error is not an error of Build")
	}
}

func TestEarningsCell(t *testing.T) {
	t.Parallel()
	d := contracts.Day("2026-11-20")
	c := cand("short_put", 1, "ok")
	c.NextEarnings = &d
	c.EarningsState = "listed"
	cell := earningsCell(c).(valueCell)
	if cell.Text != "11-20" || string(*cell.Value) != "20261120" || len(cell.Badges) != 1 || cell.Badges[0] != "listed" {
		t.Errorf("cell = %+v", cell)
	}
	c.NextEarnings = nil
	for state, want := range map[string][2]string{
		"none": {"no_data", "No earnings date known"}, "absent": {"no_data", "No earnings date known"},
		"listed":      {"no_data", "No earnings date known"},
		"unsupported": {"no_data", "Earnings dates are not available for this company"},
		"error":       {"missing_input", "Earnings date could not be read"},
		"":            {"unreadable", "Unreadable"}, "weird": {"unreadable", "Unreadable"},
	} {
		c.EarningsState = state
		if g, ok := earningsCell(c).(gap); !ok || g.Reason != want[0] || g.Text != want[1] {
			t.Errorf("state %q = %+v", state, earningsCell(c))
		}
	}
}
