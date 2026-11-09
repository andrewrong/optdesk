package option_candidates_page

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

func TestChainDayLookback(t *testing.T) {
	t.Parallel()
	rows := map[string][]store.Candidate{
		"2026-11-06/short_put":    {cand("short_put", 1, "ok")},
		"2026-11-04/covered_call": {cand("covered_call", 2, "ok")},
	}
	p := buildPage(t, rows, buildDay)
	put, call := p.Groups[0], p.Groups[1]
	if put.Day == nil || *put.Day != "2026-11-06" || call.Day == nil || *call.Day != "2026-11-04" {
		t.Fatalf("days = %v, %v", put.Day, call.Day)
	}
	if call.Gap != nil || call.Main == nil {
		t.Errorf("call group is a gap: %+v", call)
	}
	if p.ChainDay == nil || *p.ChainDay != "2026-11-06" {
		t.Errorf("chain_day = %v", p.ChainDay)
	}
	if p.Stale == nil || !p.Stale.IsStale {
		t.Errorf("stale = %+v", p.Stale)
	}
	// seven days back is still found, eight is not
	edge := buildPage(t, map[string][]store.Candidate{"2026-11-02/short_put": {cand("short_put", 1, "ok")}}, buildDay)
	if edge.Groups[0].Day == nil || *edge.Groups[0].Day != "2026-11-02" {
		t.Errorf("7 days back not found: %+v", edge.Groups[0])
	}
	far := buildPage(t, map[string][]store.Candidate{"2026-11-01/short_put": {cand("short_put", 1, "ok")}}, buildDay)
	if far.Groups[0].Gap == nil {
		t.Errorf("8 days back found: %+v", far.Groups[0])
	}
}

func TestStaleWhenOlderDay(t *testing.T) {
	t.Parallel()
	rows := map[string][]store.Candidate{
		"2026-11-09/short_put":    {cand("short_put", 1, "ok")},
		"2026-11-07/covered_call": {cand("covered_call", 2, "ok")},
	}
	p := buildPage(t, rows, buildDay)
	if p.Groups[0].Stale || !p.Groups[1].Stale {
		t.Errorf("stale flags = %v, %v", p.Groups[0].Stale, p.Groups[1].Stale)
	}
	if p.Stale == nil || p.Stale.Text != "候选行来自 2026-11-07,不是当日" {
		t.Errorf("stale label = %+v", p.Stale)
	}
	fresh := buildPage(t, map[string][]store.Candidate{"2026-11-09/short_put": {cand("short_put", 1, "ok")}}, buildDay)
	if fresh.Stale != nil {
		t.Errorf("fresh page is stale: %+v", fresh.Stale)
	}
}

func TestKindWithoutRowsIsGap(t *testing.T) {
	t.Parallel()
	rows := map[string][]store.Candidate{
		"2026-11-09/short_put":    {cand("short_put", 1, "ok")},
		"2026-11-09/covered_call": {cand("covered_call", 2, "ok")},
	}
	p := buildPage(t, rows, buildDay)
	g := p.Groups[2]
	if g.Gap == nil || g.Gap.Reason != "no_rows" || g.Day != nil || g.Main != nil || g.Other != nil {
		t.Errorf("guard group = %+v", g)
	}
	if len(p.Gaps) != 0 {
		t.Errorf("page gaps = %+v", p.Gaps)
	}
	if p.RuleVersion == nil || *p.RuleVersion != "r1" || p.ComputedAt == nil {
		t.Errorf("rule_version = %v, computed_at = %v", p.RuleVersion, p.ComputedAt)
	}
}

func TestStaleRowsMarked(t *testing.T) {
	t.Parallel()
	blocked := cand("short_put", 2, "blocked")
	rows := map[string][]store.Candidate{
		"2026-11-07/short_put":    {cand("short_put", 1, "ok"), blocked},
		"2026-11-09/covered_call": {cand("covered_call", 3, "ok")},
	}
	p := buildPage(t, rows, buildDay)
	for _, tb := range []*outTable{p.Groups[0].Main, p.Groups[0].Other} {
		for _, r := range tb.Rows {
			first := r.Cells[tb.Columns[0].ID]
			if !reflect.DeepEqual(first.Badges, []string{"过期"}) {
				t.Errorf("stale row %s first cell badges = %v", r.ID, first.Badges)
			}
		}
	}
	for _, r := range p.Groups[1].Main.Rows {
		for id, c := range r.Cells {
			for _, b := range c.Badges {
				if b == "过期" {
					t.Errorf("fresh row %s cell %s has the stale badge", r.ID, id)
				}
			}
		}
	}
}

func TestColumnsPerKind(t *testing.T) {
	t.Parallel()
	rows := map[string][]store.Candidate{}
	for _, s := range groupSpecs {
		rows["2026-11-09/"+s.kind] = []store.Candidate{cand(s.kind, 1, "ok"), cand(s.kind, 2, "blocked")}
	}
	p := buildPage(t, rows, buildDay)
	want := [][]string{
		{"rank", "company", "expiry", "dte", "strike", "bid", "yield", "break_even", "delta", "iv", "marks", "fit", "next_earnings"},
		{"rank", "company", "alias", "expiry", "dte", "strike", "bid", "yield", "break_even", "delta", "iv", "extrinsic",
			"marks", "next_earnings", "next_ex_date", "ex_amount"},
		{"company", "alias", "expiry", "strike", "extrinsic", "next_ex_date", "ex_amount", "status", "guard"},
	}
	for i, g := range p.Groups {
		if got := g.Main.ids(); !reflect.DeepEqual(got, want[i]) {
			t.Errorf("%s main columns = %v", g.Kind, got)
		}
		if got := g.Other.ids(); !reflect.DeepEqual(got, []string{"company", "contract", "alias", "status", "reasons"}) {
			t.Errorf("%s other columns = %v", g.Kind, got)
		}
	}
}
