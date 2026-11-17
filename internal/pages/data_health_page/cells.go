// Package data_health_page holds the rules of the data health page: the cells of a dataset over the window of
// session days and the repair verdict of each cell (docs/design/data-health.md, Decisions 2, 3, 4, 5, 8, 16).
package data_health_page

import (
	"fmt"
	"sort"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

// StateNotRun is the state of a session day with no ledger row of the dataset; the other states are the ledger's.
const StateNotRun = "not_run"

// Cell is one (day, dataset) of the page: the counts of the ledger, its state and the repair it offers.
type Cell struct {
	Day         contracts.Day `json:"day"`
	State       string        `json:"state"`
	Keys        int           `json:"keys"`
	OK          int           `json:"ok"`
	NoValue     int           `json:"no_value"`
	Expired     int           `json:"expired"`
	Missing     int           `json:"missing"`
	MissingKeys []string      `json:"missing_keys"`
	Repair      string        `json:"repair"`
	WhyText     string        `json:"why_text"`
	DueText     string        `json:"due_text"`
}

// CountText is "ok of keys" ("499 of 500") for a cell whose state is not ok, and "" otherwise, also for not_run.
func (c Cell) CountText() string {
	if c.State == string(coverage.StateOK) || c.State == StateNotRun {
		return ""
	}
	return fmt.Sprintf("%d of %d", c.OK, c.Keys)
}

// WindowDays returns the last coverage.WindowSessions session days of the sessions at or before today, ascending.
func WindowDays(sessions []coverage.Session, today contracts.Day) []contracts.Day {
	seen := map[contracts.Day]bool{}
	days := []contracts.Day{}
	for _, s := range sessions {
		if s.IsSession && s.Day <= today && !seen[s.Day] {
			seen[s.Day] = true
			days = append(days, s.Day)
		}
	}
	sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })
	if len(days) > coverage.WindowSessions {
		days = days[len(days)-coverage.WindowSessions:]
	}
	return days
}

// stateOf is the one precedence of a cell with keys in several states: missing > expired > no_value > ok.
func stateOf(r coverage.Cell) string {
	switch {
	case r.Missing > 0:
		return string(coverage.StateMissing)
	case r.Expired > 0:
		return string(coverage.StateExpired)
	case r.NoValue > 0:
		return string(coverage.StateNoValue)
	}
	return string(coverage.StateOK)
}

// DatasetCells returns one Cell of the dataset for each day, in the order of days: the state of the ledger row
// of the day, or not_run when the dataset has none. The repair of every cell is "none" until Apply decides it.
func DatasetCells(rows []coverage.Cell, dataset string, days []contracts.Day) []Cell {
	byDay := map[contracts.Day]coverage.Cell{}
	for _, r := range rows {
		if r.Dataset == dataset {
			byDay[r.Day] = r
		}
	}
	cells := make([]Cell, 0, len(days))
	for _, d := range days {
		r, ok := byDay[d]
		if !ok {
			cells = append(cells, Cell{Day: d, State: StateNotRun, MissingKeys: []string{}, Repair: RepairNone})
			continue
		}
		keys := append([]string{}, r.MissingKeys...)
		cells = append(cells, Cell{
			Day: d, State: stateOf(r), Keys: r.Keys, OK: r.OK, NoValue: r.NoValue,
			Expired: r.Expired, Missing: r.Missing, MissingKeys: keys, Repair: RepairNone,
		})
	}
	return cells
}
