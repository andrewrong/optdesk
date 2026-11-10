package option_boards

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// lookbackDays is how many days before the build day the short puts are searched for rows (Decision 3).
const lookbackDays = 7

// kindShortPut is the one kind of candidate both pages show.
const kindShortPut = "short_put"

// loaded is the short puts found by the lookback. Found is false when none of the days holds a row; Day is then
// empty. Stale is true when Day is before the build day.
type loaded struct {
	Found      bool
	Day        contracts.Day
	Rows       []store.Candidate
	Stale      bool
	ComputedAt time.Time // of the first row; every row of a day is of one run
}

// loadShortPuts reads the build day and, while it has no row, the earlier calendar days back to lookbackDays:
// the first day with rows is the data day. A reader error is an error.
func loadShortPuts(ctx context.Context, r store.CandidateReader, day contracts.Day) (loaded, error) {
	for back := 0; back <= lookbackDays; back++ {
		d, err := daysBack(day, back)
		if err != nil {
			return loaded{}, err
		}
		rows, err := r.Candidates(ctx, d, kindShortPut)
		if err != nil {
			return loaded{}, fmt.Errorf("option_boards: candidates of %s: %w", d, err)
		}
		if len(rows) == 0 {
			continue
		}
		return loaded{Found: true, Day: d, Rows: rows, Stale: back > 0, ComputedAt: rows[0].ComputedAt}, nil
	}
	return loaded{Rows: []store.Candidate{}}, nil
}

// chainLookbackDays is how many days before the data day the option_eod rows are searched for the chain day.
// It copies lookbackDays of internal/jobs/optioncandidates/input.go (a page may not import a job).
const chainLookbackDays = 14

// loadEod makes one OptionEod call per distinct UnderlyingID of the rows over the window dataDay-14..dataDay,
// takes the greatest SessionDay returned as the chain day of that underlying, keeps its rows, and for each
// instrument id keeps the row with the greatest capture_id. An underlying with no row in the window keeps gap
// cells. A nil reader gives an empty map, which the text helpers show as gaps.
func loadEod(ctx context.Context, r store.OptionEodReader, rows []store.Candidate, day contracts.Day) (map[int]contracts.OptionEod, error) {
	out := map[int]contracts.OptionEod{}
	if r == nil {
		return out, nil
	}
	seen := map[int]bool{}
	for _, c := range rows {
		if seen[c.UnderlyingID] {
			continue
		}
		seen[c.UnderlyingID] = true
		from, err := daysBack(day, chainLookbackDays)
		if err != nil {
			return nil, err
		}
		eod, err := r.OptionEod(ctx, int64(c.UnderlyingID), from, day)
		if err != nil {
			return nil, fmt.Errorf("option_boards: option eod of %d: %w", c.UnderlyingID, err)
		}
		chain := ""
		for _, e := range eod {
			if d := eodDay(e, day); d > chain {
				chain = d
			}
		}
		for _, e := range eod {
			if eodDay(e, day) != chain {
				continue
			}
			if old, ok := out[e.InstrumentID]; !ok || old.CaptureID <= e.CaptureID {
				out[e.InstrumentID] = e
			}
		}
	}
	return out, nil
}

// eodDay is the session day text of a row; a zero SessionDay counts as the data day.
func eodDay(e contracts.OptionEod, dataDay contracts.Day) string {
	if e.SessionDay.IsZero() {
		return string(dataDay)
	}
	return e.SessionDay.Format(time.DateOnly)
}

// chainDayNote names the earliest chain day before the data day among the rows of loadEod, or is empty when
// every row is of the data day.
func chainDayNote(eod map[int]contracts.OptionEod, dataDay contracts.Day) string {
	earliest := ""
	for _, e := range eod {
		if d := eodDay(e, dataDay); d < string(dataDay) && (earliest == "" || d < earliest) {
			earliest = d
		}
	}
	if earliest == "" {
		return ""
	}
	return "量与持仓取自 " + earliest + "(链日)"
}
