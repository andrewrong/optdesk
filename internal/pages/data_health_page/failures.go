// failures.go builds the repeated-failures rows of each dataset (docs/design/repeated-failures.md, Decisions 2-8).
package data_health_page

import (
	"context"
	"sort"
	"strconv"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

const (
	failureRowCap   = 50
	gapFailuresText = zhGapText
	gapFailures     = "failures_unavailable"
)

// FailureRow is one key of the table under a dataset.
type FailureRow struct {
	Key       string        `json:"key"`
	Kind      string        `json:"kind"`
	Days      int           `json:"days"`
	Runs      int           `json:"runs"`
	FirstDay  contracts.Day `json:"first_day"`
	LastDay   contracts.Day `json:"last_day"`
	LastClass string        `json:"last_class"`
	ClassText string        `json:"class_text"`
}

// classText is the Chinese meaning of the class of the newest run.
func classText(class string) string {
	switch class {
	case string(contracts.Retryable):
		return zhClassRetryable
	case string(contracts.Permanent):
		return zhClassPermanent
	case string(contracts.UnknownCalendar):
		return zhClassUnknownCal
	case string(coverage.FailureEmpty):
		return zhClassEmpty
	}
	return zhClassOther
}

// calendarOf is the calendar of a dataset: its job's, else the default, else any calendar with a window.
func calendarOf(in Input, id string, windows map[string][]contracts.Day) string {
	if job := jobOf(in.Jobs, id); job != nil && job.Calendar != "" {
		return job.Calendar
	}
	if _, ok := windows[defaultCal]; ok || jobOf(in.Jobs, id) != nil {
		return defaultCal
	}
	for c := range windows {
		return c
	}
	return defaultCal
}

// readFailures reads the failures once per calendar that a dataset in ids uses, over the last
// coverage.FailureWindowDays window days of the calendar, and keeps the rows of the datasets of that calendar.
// ok is false when the reader does not implement coverage.FailureReader or a read fails.
func readFailures(ctx context.Context, in Input, ids []string, windows map[string][]contracts.Day) (map[string][]coverage.Failure, bool) {
	fr, isReader := in.Cov.(coverage.FailureReader)
	if !isReader {
		return nil, false
	}
	byCal := map[string]map[string]bool{}
	for _, id := range ids {
		cal := calendarOf(in, id, windows)
		if byCal[cal] == nil {
			byCal[cal] = map[string]bool{}
		}
		byCal[cal][id] = true
	}
	out := map[string][]coverage.Failure{}
	for cal, set := range byCal {
		days := windows[cal]
		if len(days) == 0 {
			continue
		}
		if len(days) > coverage.FailureWindowDays {
			days = days[len(days)-coverage.FailureWindowDays:]
		}
		rows, err := fr.RepeatedFailures(ctx, days[0], in.Today)
		if err != nil {
			return nil, false
		}
		for _, f := range rows {
			if set[f.Dataset] {
				out[f.Dataset] = append(out[f.Dataset], f)
			}
		}
	}
	return out, true
}

// failureView orders the rows of one dataset (days, runs, last day descending, key ascending), keeps the first
// failureRowCap and returns them with the total and the number hidden.
func failureView(fs []coverage.Failure) (rows []FailureRow, total, hidden int, hiddenText string) {
	fs = append([]coverage.Failure(nil), fs...)
	sort.Slice(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Days != b.Days {
			return a.Days > b.Days
		}
		if a.Runs != b.Runs {
			return a.Runs > b.Runs
		}
		if a.LastDay != b.LastDay {
			return a.LastDay > b.LastDay
		}
		return a.Key < b.Key
	})
	total = len(fs)
	if len(fs) > failureRowCap {
		fs = fs[:failureRowCap]
	}
	rows = make([]FailureRow, 0, len(fs))
	for _, f := range fs {
		rows = append(rows, FailureRow{Key: f.Key, Kind: string(f.Kind), Days: f.Days, Runs: f.Runs,
			FirstDay: f.FirstDay, LastDay: f.LastDay, LastClass: f.LastClass, ClassText: classText(f.LastClass)})
	}
	hidden = total - len(rows)
	if hidden > 0 {
		hiddenText = zhHiddenPrefix + strconv.Itoa(hidden) + zhHiddenSuffix
	}
	return rows, total, hidden, hiddenText
}
