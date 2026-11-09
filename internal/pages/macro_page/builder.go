// Package macro_page builds the page.macro_page.v1 view model from stored series points.
// The ease and the events are named gaps, so the page is always partial. It only formats
// stored values; it computes nothing, reads no clock and makes no request.
package macro_page

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	dayLayout  = "2006-01-02"
	timeLayout = "2006-01-02T15:04:05-07:00"

	// freshnessLimitDays is the freshness limit of a series: a weekend plus one holiday.
	freshnessLimitDays = 4
	// lookbackDays is how far before day each series is read.
	lookbackDays = 30
	// maxIndicators is the maxItems of the indicators array in the schema.
	maxIndicators = 20

	title           = "宏观"
	sentence        = "宏观指标来自已存储的序列；松紧度和事件暂无数据。"
	sentenceEase    = "宏观指标来自已存储的序列；事件暂无数据。"
	easeTitle       = "松紧度"
	eventsTitle     = "本周与下周"
	indicatorsTitle = "指标"
	notesTitle      = "说明"
)

var (
	idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

	notes = []string{
		"日期来自 FRED 发布日历和央行日程，只有日期，没有发布时间。",
		"指标取自序列最新存储的一期；没有存储的标为暂缺。",
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
	// Read is optional. Nil keeps the page as it is: the ease is a no_data gap. With it,
	// Build may read stored readings through it; an error of Read is an error of Build,
	// never a gap.
	Read compute.ReadingFunc
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
	Events          []struct{}     `json:"events"`
	Indicators      []indicatorRow `json:"indicators"`
	Notes           []string       `json:"notes"`
	Gaps            []gap          `json:"gaps"`
	Stale           staleLabel     `json:"stale"`
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
	if len(c.Indicators) == 0 {
		return fmt.Errorf("macro_page: no indicators")
	}
	if len(c.Indicators) > maxIndicators {
		return fmt.Errorf("macro_page: %d indicators, at most %d", len(c.Indicators), maxIndicators)
	}
	for i, ind := range c.Indicators {
		if ind.SeriesID == "" || ind.Name == "" {
			return fmt.Errorf("macro_page: indicator %d has an empty series id or name", i)
		}
	}
	return nil
}

// Build returns the page.macro_page.v1 JSON for day, read from r. The page is partial: the
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
		Ease:            gap{Reason: "no_data", Text: "松紧度暂无数据"},
		Events:          []struct{}{},
		Indicators:      []indicatorRow{},
		Notes:           notes,
		Gaps: []gap{
			{Reason: "no_data", Text: "松紧度暂无数据"},
			{Reason: "no_data", Text: "事件日历暂无数据"},
		},
	}
	if c.Read != nil {
		block, reading, err := easeFor(ctx, c, d)
		if err != nil {
			return nil, err
		}
		if block != nil {
			vm.Ease = block
			vm.Sentence = sentenceEase
			vm.Gaps = vm.Gaps[1:]
			vm.DataTimes = append(vm.DataTimes, sourceTime{Source: "macro_ease", DataTime: midnight(reading.dataDay, c.Zone)})
		}
	}
	var oldest *time.Time
	for _, ind := range c.Indicators {
		rows, err := r.SeriesPoints(ctx, ind.SeriesID, from, day)
		if err != nil {
			return nil, fmt.Errorf("macro_page: read %s: %w", ind.SeriesID, err)
		}
		latest := latestPoint(rows, d)
		vm.Indicators = append(vm.Indicators, row(ind, latest, d))
		if latest != nil {
			obs := latest.ObservationDate.Time
			vm.DataTimes = append(vm.DataTimes, sourceTime{Source: ind.SeriesID, DataTime: midnight(obs, c.Zone)})
			if oldest == nil || obs.Before(*oldest) {
				o := obs
				oldest = &o
			}
		}
	}
	base, text := d, "没有数据日"
	if oldest != nil {
		base, text = *oldest, "数据日 "+oldest.Format("01-02")
	}
	vm.Stale = staleLabel{
		DataTime:   midnight(base, c.Zone),
		StaleAfter: time.Date(base.Year(), base.Month(), base.Day()+freshnessLimitDays, 9, 30, 0, 0, c.Zone).Format(timeLayout),
		Text:       text,
	}
	return json.Marshal(vm)
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
		out.Latest = gap{Reason: "no_data", Text: "暂缺"}
		out.Status, out.Label = "missing", "缺"
		return out
	case string(latest.Unit) != "percent":
		out.Latest = gap{Reason: "missing_input", Text: "单位未知"}
	default:
		v := *latest.Value
		out.Latest = figure{Value: v, Text: strconv.FormatFloat(v, 'f', -1, 64) + "%", Unit: "percent"}
	}
	if d.Sub(latest.ObservationDate.Time) > freshnessLimitDays*24*time.Hour {
		out.Status, out.Label = "late", "迟到"
	} else {
		out.Status, out.Label = "good", "及时"
	}
	return out
}
