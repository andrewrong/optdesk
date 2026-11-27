// Package macro_page builds the page.macro_page.v1 view model from stored series points, and
// page.macro_page.v2 when its Config has a Liquidity function. The ease and the events are
// named gaps, so the page is always partial. The page formats stored values and the figures
// its Liquidity function returns and computes nothing itself (rounding for display aside); it
// reads no clock and makes no request.
package macro_page

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

const (
	dayLayout  = "2006-01-02"
	timeLayout = "2006-01-02T15:04:05-07:00"

	// freshnessLimitDays is the freshness limit of a grouped daily series and the offset of stale_after.
	freshnessLimitDays = 4
	// dailyFreshnessDays is the freshness limit of a daily series of the Indicators card: a weekend, one FRED
	// publication day and one holiday (a trial value, to be tuned on observed rows).
	dailyFreshnessDays = 6
	// lookbackDays is how far before day each series is read: beyond the monthly freshness limit, so a monthly series inside it is read.
	lookbackDays = 60
	// maxIndicators is the maxItems of the indicators array in the schema.
	maxIndicators = 20

	sentence     = "Macro indicators come from stored series; ease and events: no data yet."
	sentenceEase = "Macro indicators come from stored series; events: no data yet."
	// With an events reader the sentences no longer say the events are missing.
	sentenceEv     = "Macro indicators come from stored series; ease: no data yet."
	sentenceEaseEv = "Macro indicators come from stored series."
)

var (
	idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

	notes = []string{
		"Dates come from the FRED release calendar and central bank schedules; dates only, no release times.",
		"Each indicator is the latest stored period of its series; a series with none stored shows n/a.",
	}
)

// Indicator is one configured row; an empty DetailHref is written as null.
type Indicator struct {
	SeriesID   string
	Name       string
	SourceText string
	DetailHref string
}

// Config carries the identity of the run and the rows of the page. Zone is the owner's zone.
type Config struct {
	RunID      string
	BuildID    string
	Scope      string
	Zone       *time.Location
	Indicators []Indicator
	// Groups is optional: the grouped rows of the Indicators card, written after Indicators with the fields group,
	// change_text, data_day and hint (indicators.go). Empty keeps the payload as it is.
	Groups []Group
	// Window is optional: the option window in days and whether the owner stored it. With it, Build writes the lead
	// block and days_to and in_window on each event (lead.go). Nil keeps the payload as it is.
	Window WindowFunc
	// Read is optional. Nil keeps the page as it is: the ease is a no_data gap. With it,
	// Build may read stored readings through it; an error of Read is an error of Build,
	// never a gap.
	Read compute.ReadingFunc
	// Events is optional. Nil keeps the page as it is: no event rows, the events gap and the old title.
	// With it, Build lists the stored events of the next 60 days and the gaps in them (events.go); an
	// error of Events is an error of Build, never a gap.
	Events macroevent.Reader
	// Liquidity is optional. Nil keeps the page as it is (page.macro_page.v1, no liquidity field, no series read
	// for the block). With it, Build reads the fourteen series of liquidityapi.Input, calls it once and writes
	// page.macro_page.v2 with the funding and credit block (liquidity.go). LiquidityParams and Net go into its
	// Input. An error of a read is an error of Build, never a gap.
	Liquidity       liquidityapi.Func
	LiquidityParams liquidityapi.Params
	Net             liquidityapi.NetFunc
}

type gap struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type figure struct {
	Value float64 `json:"value"`
	Text  string  `json:"text"`
	Unit  string  `json:"unit"`
}

type indicatorRow struct {
	Name       string  `json:"name"`
	Latest     any     `json:"latest"`
	Status     string  `json:"status"`
	Label      string  `json:"label"`
	SourceText string  `json:"source_text"`
	DetailHref *string `json:"detail_href"`
	// Written only for the rows of a group.
	Group      string  `json:"group,omitempty"`
	ChangeText *string `json:"change_text,omitempty"`
	DataDay    *string `json:"data_day,omitempty"`
	Hint       string  `json:"hint,omitempty"`
}

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type staleLabel struct {
	DataTime   string `json:"data_time"`
	StaleAfter string `json:"stale_after"`
	Text       string `json:"text"`
}

type viewModel struct {
	Schema          string         `json:"schema"`
	SchemaVersion   int            `json:"schema_version"`
	RunID           string         `json:"run_id"`
	BuildID         string         `json:"build_id"`
	Scope           string         `json:"scope"`
	GeneratedAt     string         `json:"generated_at"`
	DataTimes       []sourceTime   `json:"data_times"`
	RunStatus       string         `json:"run_status"`
	Title           string         `json:"title"`
	Sentence        string         `json:"sentence"`
	EaseTitle       string         `json:"ease_title"`
	EventsTitle     string         `json:"events_title"`
	IndicatorsTitle string         `json:"indicators_title"`
	NotesTitle      string         `json:"notes_title"`
	Ease            any            `json:"ease"`
	Events          []eventRow     `json:"events"`
	Indicators      []indicatorRow `json:"indicators"`
	Notes           []string       `json:"notes"`
	Gaps            []gap          `json:"gaps"`
	Stale           staleLabel     `json:"stale"`
	// Written only with a Liquidity function (version 2).
	LiquidityTitle string          `json:"liquidity_title,omitempty"`
	Liquidity      *liquidityBlock `json:"liquidity,omitempty"`
	// Written only with a Window function.
	Lead   *lead `json:"lead,omitempty"`
	events []macroevent.Event
}

func validate(c Config) error {
	if c.Zone == nil {
		return fmt.Errorf("macro_page: nil zone")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "scope": c.Scope} {
		if !idPattern.MatchString(v) {
			return fmt.Errorf("macro_page: bad %s %q", name, v)
		}
	}
	all := slices.Clone(c.Indicators)
	for _, g := range c.Groups {
		all = append(all, g.Indicators...)
	}
	if len(all) == 0 {
		return fmt.Errorf("macro_page: no indicators")
	}
	if len(all) > maxIndicators {
		return fmt.Errorf("macro_page: %d indicators, at most %d", len(all), maxIndicators)
	}
	for i, ind := range all {
		if ind.SeriesID == "" || ind.Name == "" {
			return fmt.Errorf("macro_page: indicator %d has an empty series id or name", i)
		}
	}
	return nil
}

// Build returns the page.macro_page.v1 JSON (v2 with c.Liquidity) for day, read from r. The page is partial: the
// events are a no_data gap, and so is the ease unless c.Read returns a stored macro.ease
// reading with a value. generatedAt is written in c.Zone.
func Build(ctx context.Context, r store.SeriesReader, c Config, day contracts.Day, generatedAt time.Time) ([]byte, error) {
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("macro_page: bad day %q: %w", day, err)
	}
	if err := validate(c); err != nil {
		return nil, err
	}
	if r == nil { // a missing reader reads no series: every figure is a gap, never a panic
		r = noSeries{}
	}
	from := contracts.Day(d.AddDate(0, 0, -lookbackDays).Format(dayLayout))
	vm := viewModel{
		Schema:          "macro_page",
		SchemaVersion:   1,
		RunID:           c.RunID,
		BuildID:         c.BuildID,
		Scope:           c.Scope,
		GeneratedAt:     generatedAt.In(c.Zone).Format(timeLayout),
		DataTimes:       []sourceTime{},
		RunStatus:       "partial",
		Title:           title,
		Sentence:        sentence,
		EaseTitle:       easeTitle,
		EventsTitle:     eventsTitle,
		IndicatorsTitle: indicatorsTitle,
		NotesTitle:      notesTitle,
		Ease:            gap{Reason: "no_data", Text: "Ease: no data yet"},
		Events:          []eventRow{},
		Indicators:      []indicatorRow{},
		Notes:           notes,
		Gaps: []gap{
			{Reason: "no_data", Text: "Ease: no data yet"},
			{Reason: "no_data", Text: "Event calendar: no data yet"},
		},
	}
	eventGaps, err := withEvents(ctx, r, c, d, &vm)
	if err != nil {
		return nil, err
	}
	if c.Read != nil {
		block, reading, err := easeFor(ctx, c, d)
		if err != nil {
			return nil, err
		}
		if block != nil {
			vm.Ease = block
			vm.Sentence = sentenceEase
			if c.Events != nil {
				vm.Sentence = sentenceEaseEv
			}
			vm.Gaps = vm.Gaps[1:]
			vm.DataTimes = append(vm.DataTimes, sourceTime{Source: "macro_ease", DataTime: midnight(reading.dataDay, c.Zone)})
		}
	}
	vm.Gaps = append(vm.Gaps, eventGaps...)
	if err := withLiquidity(ctx, r, c, d, &vm); err != nil {
		return nil, err
	}
	var oldest *staleSrc
	for _, ind := range c.Indicators {
		rows, err := r.SeriesPoints(ctx, ind.SeriesID, from, day)
		if err != nil {
			return nil, fmt.Errorf("macro_page: read %s: %w", ind.SeriesID, err)
		}
		latest := latestPoint(rows, d)
		vm.Indicators = append(vm.Indicators, row(ind, latest, d))
		if latest != nil {
			oldest = noteData(&vm, c.Zone, oldest, ind.SeriesID, ind.Name, latest.ObservationDate.Time, true)
		}
	}
	oldest, err = withGroupsAndLead(ctx, r, c, d, generatedAt, &vm, oldest)
	if err != nil {
		return nil, err
	}
	vm.Stale = staleOf(d, oldest, c.Zone)
	return json.Marshal(vm)
}

// staleSrc is the series that holds the oldest data day.
type staleSrc struct {
	day  time.Time
	id   string
	name string
}

// staleOf is the stale label: the oldest latest observation of the indicators and the series that holds it, else the page day.
func staleOf(d time.Time, oldest *staleSrc, zone *time.Location) staleLabel {
	base, text := d, "No data day"
	if oldest != nil {
		base = oldest.day
		rh := rhythmOf(oldest.id)
		limit := rh.limitDays()
		if d.Sub(base) > time.Duration(limit)*24*time.Hour {
			text = fmt.Sprintf("Data day %s, %s is late (%s series, limit %d days)", base.Format("01-02"), oldest.name, rh, limit)
		} else {
			text = fmt.Sprintf("Data day %s, %s (%s series, FRED publishes with a lag)", base.Format("01-02"), oldest.name, rh)
		}
	}
	return staleLabel{
		DataTime:   midnight(base, zone),
		StaleAfter: time.Date(base.Year(), base.Month(), base.Day()+freshnessLimitDays, 9, 30, 0, 0, zone).Format(timeLayout),
		Text:       text,
	}
}

// rhythm is the publication rhythm of a series.
type rhythm string

const (
	rhythmDaily   rhythm = "daily"
	rhythmWeekly  rhythm = "weekly"
	rhythmMonthly rhythm = "monthly"
)

// seriesRhythm maps the series ids of the Indicators card that are not daily to their rhythm.
var seriesRhythm = map[string]rhythm{
	"DTWEXBGS":        rhythmWeekly,
	"DEXJPUS":         rhythmWeekly,
	"NFCI":            rhythmWeekly,
	"IRLTLT01JPM156N": rhythmMonthly,
}

func rhythmOf(id string) rhythm {
	if r, ok := seriesRhythm[id]; ok {
		return r
	}
	return rhythmDaily
}

// limitDays is the freshness limit of the rhythm in calendar days.
func (r rhythm) limitDays() int {
	switch r {
	case rhythmMonthly:
		return monthlyFreshnessDays
	case rhythmWeekly:
		return weeklyFreshnessDays
	case rhythmDaily:
		return dailyFreshnessDays
	}
	return dailyFreshnessDays
}

func midnight(t time.Time, zone *time.Location) string {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, zone).Format(timeLayout)
}

// latestPoint drops the rows first published after d, keeps the greatest RealtimeStart per
// observation date, and returns the kept row of the latest date that has a value.
func latestPoint(rows []contracts.SeriesPoint, d time.Time) *contracts.SeriesPoint {
	kept := map[string]contracts.SeriesPoint{}
	for _, p := range rows {
		if p.RealtimeStart.After(d) {
			continue
		}
		k := p.ObservationDate.Format(dayLayout)
		if cur, ok := kept[k]; !ok || p.RealtimeStart.After(cur.RealtimeStart.Time) {
			kept[k] = p
		}
	}
	var best *contracts.SeriesPoint
	for _, p := range kept {
		if p.Value == nil {
			continue
		}
		if best == nil || p.ObservationDate.After(best.ObservationDate.Time) {
			p := p
			best = &p
		}
	}
	return best
}

func row(ind Indicator, latest *contracts.SeriesPoint, d time.Time) indicatorRow {
	out := indicatorRow{Name: ind.Name, SourceText: ind.SourceText}
	if ind.DetailHref != "" {
		h := ind.DetailHref
		out.DetailHref = &h
	}
	switch {
	case latest == nil:
		out.Latest = gap{Reason: "no_data", Text: "no data yet"}
		out.Status, out.Label = "missing", "n/a"
		return out
	case string(latest.Unit) != "percent":
		out.Latest = gap{Reason: "missing_input", Text: gapUnitUnknown}
	default:
		v := *latest.Value
		out.Latest = figure{Value: v, Text: strconv.FormatFloat(v, 'f', -1, 64) + "%", Unit: "percent"}
	}
	if d.Sub(latest.ObservationDate.Time) > time.Duration(rhythmOf(ind.SeriesID).limitDays())*24*time.Hour {
		out.Status, out.Label = "late", labelLate
	} else {
		out.Status, out.Label = "good", labelOnTime
	}
	return out
}

// noteData adds the data time of a series to vm and, when it counts for the stale label, moves oldest back to it.
func noteData(vm *viewModel, zone *time.Location, oldest *staleSrc, id, name string, obs time.Time, counts bool) *staleSrc {
	vm.DataTimes = append(vm.DataTimes, sourceTime{Source: id, DataTime: midnight(obs, zone)})
	if counts && (oldest == nil || obs.Before(oldest.day)) {
		return &staleSrc{day: obs, id: id, name: name}
	}
	return oldest
}

// withGroupsAndLead appends the grouped rows, with their data times (a monthly or weekly series is listed but does not set the
// stale label) and, with c.Window, writes the event flags and the lead block. It returns the oldest data day.
func withGroupsAndLead(ctx context.Context, r store.SeriesReader, c Config, d, generatedAt time.Time, vm *viewModel, oldest *staleSrc) (*staleSrc, error) {
	grouped, days, err := buildGroups(ctx, r, c.Groups, d)
	if err != nil {
		return nil, err
	}
	vm.Indicators = append(vm.Indicators, grouped...)
	for _, g := range days {
		oldest = noteData(vm, c.Zone, oldest, g.id, g.name, g.day, !g.monthly && !g.weekly)
	}
	if c.Window == nil {
		return oldest, nil
	}
	window, stored := windowOf(ctx, c, generatedAt)
	markWindow(vm.Events, d, window)
	vm.Lead, err = buildLead(ctx, r, vm.events, d, window, stored)
	return oldest, err
}

// noSeries is the reader of a nil SeriesReader: it holds no points.
type noSeries struct{}

func (noSeries) SeriesPoints(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SeriesPoint, error) {
	return nil, nil
}
