package market_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	timeLayout = "2006-01-02T15:04:05-07:00"
	// staleDisplayDays is the display limit of the page label (the limit of macro_page).
	staleDisplayDays = 4
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type staleLabel struct {
	DataTime string `json:"data_time"`
	IsStale  bool   `json:"is_stale"`
	Text     string `json:"text"`
}

type weather struct {
	Headline string `json:"headline"`
	Sentence string `json:"sentence"`
	Chart    any    `json:"chart"`
}

type viewModel struct {
	Schema        string       `json:"schema"`
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	BuildID       string       `json:"build_id"`
	Scope         string       `json:"scope"`
	GeneratedAt   string       `json:"generated_at"`
	DataTimes     []sourceTime `json:"data_times"`
	RunStatus     string       `json:"run_status"`
	Stale         *staleLabel  `json:"stale"`
	StaleAfter    string       `json:"stale_after,omitempty"`
	Title         string       `json:"title"`
	Sentence      string       `json:"sentence"`
	Notes         []string     `json:"notes"`
	Gaps          []gap        `json:"gaps"`
	Weather       weather      `json:"weather"`
	Indices       indices      `json:"indices"`
	Term          any          `json:"term"`
	Breadth       any          `json:"breadth"`
}

// Build returns the page.market_page.v1 JSON for day. The readings come from c.Readings and
// the term points from r; every number is a stored value that is shifted and rounded as
// decimal text. generatedAt is written in c.Zone. A reader error is an error of Build,
// never a Gap.
func Build(ctx context.Context, c Config, r store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error) {
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("market_page: bad day %q: %w", day, err)
	}
	if err := validate(c, r); err != nil {
		return nil, err
	}
	days := dataDays{}
	failed := 0

	sp, err := readNewest(ctx, c, figSp500Vs200d, seriesSP500, d)
	if err != nil {
		return nil, err
	}
	days.add(seriesSP500, sp.dataDay)
	sentence, weatherOK := weatherSentence(sp)
	if !weatherOK {
		failed++
	}
	idx, rankGaps, err := buildIndices(ctx, c, d, days)
	if err != nil {
		return nil, err
	}
	failed += rankGaps
	ratio, err := readNewest(ctx, c, figVixTerm, seriesVIX, d)
	if err != nil {
		return nil, err
	}
	days.add(seriesVIX, ratio.dataDay)
	tm, termGaps, err := buildTerm(ctx, c, r, d, ratio, days)
	if err != nil {
		return nil, err
	}
	failed += termGaps

	vm := viewModel{
		Schema:        "market_page",
		SchemaVersion: 1,
		RunID:         c.RunID,
		BuildID:       c.BuildID,
		Scope:         c.Scope,
		GeneratedAt:   generatedAt.In(c.Zone).Format(timeLayout),
		DataTimes:     []sourceTime{},
		RunStatus:     "ok",
		Title:         pageTitle,
		Sentence:      pageSentence,
		Notes:         pageNotes,
		Gaps:          []gap{},
		Weather:       weather{Headline: weatherHeadline, Sentence: sentence, Chart: mkGap(reasonNotBuilt, chartNotBuilt)},
		Indices:       idx,
		Term:          tm,
		Breadth:       mkGap(reasonNotBuilt, breadthNotBuilt),
	}
	if failed > 0 {
		vm.RunStatus = "partial"
	}
	vm.Gaps = collectGaps(vm)
	var oldest time.Time
	for _, s := range dataTimeSeries {
		t, ok := days[s]
		if !ok {
			continue
		}
		vm.DataTimes = append(vm.DataTimes, sourceTime{Source: s, DataTime: midnight(t, c.Zone)})
		if oldest.IsZero() || t.Before(oldest) {
			oldest = t
		}
	}
	if !oldest.IsZero() {
		vm.Stale = &staleLabel{
			DataTime: midnight(oldest, c.Zone),
			IsStale:  daysBetween(oldest, d) > staleDisplayDays,
			Text:     staleLabelPrefix + oldest.Format("01-02"),
		}
		vm.StaleAfter = time.Date(oldest.Year(), oldest.Month(), oldest.Day()+staleDisplayDays, 9, 30, 0, 0, c.Zone).Format(timeLayout)
	}
	return json.Marshal(vm)
}

func validate(c Config, r store.SeriesReader) error {
	switch {
	case c.Zone == nil:
		return errors.New("market_page: nil zone")
	case c.Readings == nil:
		return errors.New("market_page: nil readings")
	case r == nil:
		return errors.New("market_page: nil series reader")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "scope": c.Scope} {
		if !idPattern.MatchString(v) {
			return fmt.Errorf("market_page: bad %s %q", name, v)
		}
	}
	return nil
}

// collectGaps copies the Gap objects of the blocks in page order.
func collectGaps(vm viewModel) []gap {
	out := []gap{}
	add := func(cells ...any) {
		for _, c := range cells {
			if g, ok := c.(gap); ok {
				out = append(out, g)
			}
		}
	}
	add(vm.Weather.Chart)
	for _, row := range vm.Indices.Rows {
		add(row.IV, row.RV20, row.IVMinusRV20, row.Rank3Y)
	}
	if t, ok := vm.Term.(term); ok {
		for _, p := range t.Points {
			add(p.Today, p.FiveDaysAgo)
		}
	} else {
		add(vm.Term)
	}
	add(vm.Breadth)
	return out
}

func midnight(t time.Time, zone *time.Location) string {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, zone).Format(timeLayout)
}
