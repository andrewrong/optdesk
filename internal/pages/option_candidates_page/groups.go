package option_candidates_page

import (
	"context"
	"fmt"
	"strconv"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// lookbackDays is how many days before the build day a kind is searched for rows (Decision 3).
const lookbackDays = 7

const (
	kindShortPut      = "short_put"
	kindCoveredCall   = "covered_call"
	kindHeldCallGuard = "held_call_guard"
)

type groupSpec struct {
	kind string
	main []colDef
}

// groupSpecs are the groups in page order with the columns of the main table (Decision 5).
var groupSpecs = []groupSpec{
	{kindShortPut, []colDef{colRank, colCompany, colExpiry, colDTE, colStrike, colBid, colYield, colBreakEven,
		colDelta, colIV, colMarks, colFit, colEarnings}},
	{kindCoveredCall, []colDef{colRank, colCompany, colAlias, colExpiry, colDTE, colStrike, colBid, colYield,
		colBreakEven, colDelta, colIV, colExtrinsic, colMarks, colEarnings, colExDate, colExAmount}},
	{kindHeldCallGuard, []colDef{colCompany, colAlias, colExpiry, colStrike, colExtrinsic, colExDate, colExAmount,
		colStatus, colGuard}},
}

type counts struct {
	Ok      int `json:"ok"`
	Blocked int `json:"blocked"`
	Unknown int `json:"unknown"`
}

type row struct {
	ID      string         `json:"id"`
	Cells   map[string]any `json:"cells"`
	Reasons []reason       `json:"reasons"`
}

type table struct {
	Columns []column `json:"columns"`
	Rows    []row    `json:"rows"`
}

type group struct {
	Kind   string  `json:"kind"`
	Title  string  `json:"title"`
	Day    *string `json:"day"`
	Stale  bool    `json:"stale"`
	Counts counts  `json:"counts"`
	Main   *table  `json:"main"`
	Other  *table  `json:"other"`
	Gap    *gap    `json:"gap"`
}

func gapGroup(kind string, g gap) group {
	return group{Kind: kind, Title: groupTitles[kind], Gap: &g}
}

// readKind reads the rows of kind of the build day and, when there are none, of each earlier day
// up to lookbackDays back; it returns the first day that has rows. A store error is returned.
func readKind(ctx context.Context, r store.CandidateReader, kind string, day contracts.Day) (contracts.Day, []store.Candidate, error) {
	for i := 0; i <= lookbackDays; i++ {
		d, err := daysBack(day, i)
		if err != nil {
			return "", nil, fmt.Errorf("option_candidates_page: bad day %q: %w", day, err)
		}
		rows, err := r.Candidates(ctx, d, kind)
		if err != nil {
			return "", nil, fmt.Errorf("option_candidates_page: read %s: %w", kind, err)
		}
		if len(rows) > 0 {
			return d, rows, nil
		}
	}
	return "", nil, nil
}

// buildGroup splits the rows of one group in their stored order: ok rows to the main table, the
// others to the second table with their reasons. A group day before the build day is stale.
func buildGroup(spec groupSpec, groupDay, buildDay contracts.Day, rows []store.Candidate) group {
	g := group{Kind: spec.kind, Title: groupTitles[spec.kind], Stale: groupDay < buildDay}
	d := string(groupDay)
	g.Day = &d
	var main, other []store.Candidate
	for _, c := range rows {
		switch c.Status {
		case "ok":
			g.Counts.Ok++
			main = append(main, c)
		case "blocked":
			g.Counts.Blocked++
			other = append(other, c)
		default:
			g.Counts.Unknown++
			other = append(other, c)
		}
	}
	if len(main) > 0 {
		g.Main = buildTable(spec.main, spec.kind, main, g.Stale, false)
	}
	if len(other) > 0 {
		g.Other = buildTable(otherColumns, spec.kind, other, g.Stale, true)
	}
	return g
}

func buildTable(defs []colDef, kind string, rows []store.Candidate, stale, withReasons bool) *table {
	t := &table{Columns: make([]column, len(defs)), Rows: make([]row, len(rows))}
	for i, d := range defs {
		t.Columns[i] = d.column
	}
	for i, c := range rows {
		r := row{ID: "cand-" + kind + "-" + strconv.Itoa(c.InstrumentID) + "-" + strconv.Itoa(i), Cells: map[string]any{}, Reasons: []reason{}}
		for j, d := range defs {
			cell := d.cell(c)
			if vc, ok := cell.(valueCell); ok && j == 0 && stale {
				vc.Badges = append(vc.Badges, staleBadge)
				cell = vc
			}
			r.Cells[d.ID] = cell
		}
		if withReasons {
			if gates, err := parseGates(c.Gates); err == nil {
				r.Reasons = reasonsOf(gates)
			}
		}
		t.Rows[i] = r
	}
	return t
}
