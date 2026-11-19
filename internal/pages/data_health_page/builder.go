// builder.go reads the coverage and builds the document of the page (docs/design/data-health.md, entry 5).
package data_health_page

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

const (
	dayFormat    = "2006-01-02"
	defaultCal   = "XNYS"
	hoursPerDay  = 24
	versionOfDoc = 1
)

// Build returns the JSON of page.data_health_page.v1. A coverage.Reader error gives the gap coverage_unavailable
// and no dataset, never an error.
func Build(ctx context.Context, in Input) ([]byte, error) {
	vm := viewModel{Schema: pageName, SchemaVersion: versionOfDoc, RunID: runID, BuiltAt: in.Now.Format(timeLayout),
		FailuresNote: zhFailuresNote, FailuresNone: zhFailuresNone, Datasets: []Dataset{}, Glossary: glossary(), OverwriteNote: overwriteNote, Gaps: []Gap{}}
	if in.Form != nil {
		vm.Form = &formVM{PostPath: in.Form.PostPath, Expires: in.Form.Expires, MAC: in.Form.MAC}
	}
	vm.Running = runningLines(ctx, in)
	datasets, failuresOK, err := datasetRows(ctx, in)
	if err != nil {
		vm.Gaps = append(vm.Gaps, Gap{Kind: "gap", Reason: gapReason, Text: gapText})
	} else {
		vm.Datasets = datasets
		if !failuresOK {
			vm.Gaps = append(vm.Gaps, Gap{Kind: "gap", Reason: gapFailures, Text: gapFailuresText})
		}
	}
	return json.Marshal(vm)
}

var errNoReader = errors.New("data_health_page: no coverage reader")

// datasetRows builds the dataset rows; failuresOK is false when the repeated failures could not be read (the
// reader lacks coverage.FailureReader, or its read failed) and the rows then carry empty lists.
func datasetRows(ctx context.Context, in Input) (out []Dataset, failuresOK bool, err error) {
	if in.Cov == nil {
		return nil, false, errNoReader
	}
	windows, from, err := windowsByCalendar(ctx, in)
	if err != nil {
		return nil, false, err
	}
	rows, err := in.Cov.Cells(ctx, from, in.Today)
	if err != nil {
		return nil, false, err
	}
	newest, err := in.Cov.NewestObservations(ctx)
	if err != nil {
		return nil, false, err
	}
	ids := datasetIDs(rows)
	failures, failuresOK := readFailures(ctx, in, ids, windows)
	out = make([]Dataset, 0, len(ids))
	for _, id := range ids {
		d := datasetRow(in, id, rows, windows, newest)
		d.Repeated, d.FailuresTotal, d.FailuresHidden, d.FailuresHiddenText = failureView(failures[id])
		out = append(out, d)
	}
	return out, failuresOK, nil
}

// windowsByCalendar returns the window days of each calendar the jobs use and the oldest day of all of them.
func windowsByCalendar(ctx context.Context, in Input) (map[string][]contracts.Day, contracts.Day, error) {
	cals := map[string]bool{defaultCal: len(in.Jobs) == 0}
	for _, j := range in.Jobs {
		if j.Calendar != "" {
			cals[j.Calendar] = true
		}
	}
	today, err := time.Parse(dayFormat, string(in.Today))
	if err != nil {
		return nil, "", err
	}
	lookback := contracts.Day(today.AddDate(0, 0, -coverage.LookbackDays).Format(dayFormat))
	windows := map[string][]contracts.Day{}
	from := in.Today
	for cal, used := range cals {
		if !used {
			continue
		}
		sessions, err := in.Cov.Sessions(ctx, cal, lookback, in.Today)
		if err != nil {
			return nil, "", err
		}
		days := WindowDays(sessions, in.Today)
		windows[cal] = days
		if len(days) > 0 && days[0] < from {
			from = days[0]
		}
	}
	return windows, from, nil
}

func datasetIDs(rows []coverage.Cell) []string {
	seen := map[string]bool{}
	ids := []string{}
	for _, r := range rows {
		if !seen[r.Dataset] {
			seen[r.Dataset] = true
			ids = append(ids, r.Dataset)
		}
	}
	sort.Strings(ids)
	return ids
}

// jobOf is the first job, in Jobs order, that lists the dataset (Decision 17); nil when none does.
func jobOf(jobs []Job, dataset string) *Job {
	for i := range jobs {
		for _, d := range jobs[i].Datasets {
			if d == dataset {
				return &jobs[i]
			}
		}
	}
	return nil
}

func datasetRow(in Input, id string, rows []coverage.Cell, windows map[string][]contracts.Day, newest []coverage.Newest) Dataset {
	job := jobOf(in.Jobs, id)
	cal := calendarOf(in, id, windows)
	var jobID *string
	if job != nil {
		jobID = &job.ID
	}
	cells := DatasetCells(rows, id, windows[cal])
	Apply(cells, job, in.Form, in.Now, in.Today)
	LateText(id, cells, windows[cal], newest)
	gaps := GapDays(cells)
	d := Dataset{DatasetID: id, Label: id, JobID: jobID, Calendar: cal, GapDays: len(gaps),
		RepairAll: RepairAll(gaps, in.Form), Newest: []Newest{}, Cells: cells}
	if id == fredDataset {
		d.Newest = newestRows(newest, in.Today)
	}
	return d
}

// newestRows gives the age in days of each series' newest observation at today, ordered by series id.
func newestRows(newest []coverage.Newest, today contracts.Day) []Newest {
	t, err := time.Parse(dayFormat, string(today))
	out := []Newest{}
	if err != nil {
		return out
	}
	for _, n := range newest {
		age := 0
		if d, err := time.Parse(dayFormat, string(n.Day)); err == nil {
			if a := int(t.Sub(d) / (hoursPerDay * time.Hour)); a > 0 {
				age = a
			}
		}
		out = append(out, Newest{SeriesID: n.SeriesID, Day: n.Day, AgeDays: age})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SeriesID < out[j].SeriesID })
	return out
}
