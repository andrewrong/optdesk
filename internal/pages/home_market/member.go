// Package home_market builds the market member of the home page: the document
// page.market_summary.v3 with the index chart, the index table and the volatility indices.
//
// It reads through its Config and the series reader and nothing else. Every number is a stored
// value written as decimal text; a value that is missing, stale or out of range is a Gap, never
// a zero. It imports no other page: the reading function is the one market_page takes, filled by cmd/ot.
package home_market

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	dayLayout  = "2006-01-02"
	timeLayout = "2006-01-02T15:04:05-07:00"

	figIV         = "market.iv_ibkr"
	figRv20       = "market.rv20"
	figIvRv20     = "market.iv_rv20"
	subjectKind   = "underlying"
	seriesChart   = "SP500"
	chartDays     = 30 // newest points of the chart
	chartLookback = 60 // calendar days read for the chart
	levelLookback = 10 // calendar days read for a level
	freshDays     = 4  // a weekend plus one holiday; also the stale limit of the card
	readingDays   = 5
)

// Config is what Build needs besides the series reader.
type Config struct {
	RunID, BuildID, Scope string
	Zone                  *time.Location      // generated_at is written in it; nil is an error of Build
	Readings              compute.ReadingFunc // the method value WinnerReadings of the page-role reader; nil is an error of Build
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type gap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

func mkGap(reason, text string) gap { return gap{Kind: "gap", Reason: reason, Text: text} }

type volPoints struct {
	Kind  string      `json:"kind"`
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

type indexLevel struct {
	Kind  string      `json:"kind"`
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type staleLabel struct {
	DataTime string `json:"data_time"`
	IsStale  bool   `json:"is_stale"`
	Text     string `json:"text"`
}

type chartPoint struct {
	Label string      `json:"label"`
	Close json.Number `json:"close"`
}

type chart struct {
	Points      []chartPoint `json:"points"`
	SeriesNames []string     `json:"series_names"`
	EndLabels   []string     `json:"end_labels"`
	AriaText    string       `json:"aria_text"`
}

type indexRow struct {
	Name        string `json:"name"`
	IV          any    `json:"iv"`
	RV20        any    `json:"rv20"`
	IVMinusRV20 any    `json:"iv_minus_rv20"`
	SourceText  string `json:"source_text,omitempty"`
}

type indexTable struct {
	Columns []string   `json:"columns"`
	Rows    []indexRow `json:"rows"`
}

type volIndex struct {
	Name       string `json:"name"`
	Level      any    `json:"level"`
	Day        string `json:"day"`
	SourceText string `json:"source_text,omitempty"`
}

type card struct {
	Title        string     `json:"title"`
	Say          string     `json:"say"`
	Subtitle     string     `json:"subtitle"`
	Stale        staleLabel `json:"stale"`
	Chart        chart      `json:"chart"`
	IndexTable   indexTable `json:"index_table"`
	VolIndices   []volIndex `json:"vol_indices"`
	VolNote      string     `json:"vol_indices_note"`
	Notes        []string   `json:"notes"`
	Gaps         []gap      `json:"gaps"`
	DataTimesTxt string     `json:"data_times_text"`
}

type viewModel struct {
	Schema        string       `json:"schema"`
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	BuildID       string       `json:"build_id"`
	Scope         string       `json:"scope"`
	GeneratedAt   string       `json:"generated_at"`
	StaleAfter    string       `json:"stale_after"`
	DataTimes     []sourceTime `json:"data_times"`
	RunStatus     string       `json:"run_status"`
	Card          card         `json:"card"`
}

type point struct {
	date time.Time
	text string
}

type dataDays map[string]time.Time

func (m dataDays) add(series string, day time.Time) {
	if cur, ok := m[series]; !ok || day.After(cur) {
		m[series] = day
	}
}

// indexSpec is one row of the index table: the underlying of the reading.
var indexNames = []string{"SPX", "NDX", "RUT"}

type volSpec struct{ name, series string }

var volSpecs = []volSpec{{"VIX", "VIXCLS"}, {"VXN", "VXNCLS"}, {"RVX", "RVXCLS"}}

// dataTimeSeries is the order of data_times.
var dataTimeSeries = []string{"VIXCLS", "VXNCLS", "RVXCLS", seriesChart}

// Build returns the page.market_summary.v3 JSON for day, generatedAt written in c.Zone. A reader
// error is an error of Build, never a Gap.
func Build(ctx context.Context, c Config, r store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error) {
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("home_market: bad day %q: %w", day, err)
	}
	if err := validate(c, r); err != nil {
		return nil, err
	}
	days := dataDays{}
	failed := 0

	table := indexTable{Columns: columns, Rows: []indexRow{}}
	for _, name := range indexNames {
		row, ok, err := buildRow(ctx, c, name, d)
		if err != nil {
			return nil, err
		}
		if !ok {
			failed++
		}
		table.Rows = append(table.Rows, row)
	}

	vols := []volIndex{}
	for _, s := range volSpecs {
		pts, err := storedPoints(ctx, r, s.series, d, levelLookback)
		if err != nil {
			return nil, err
		}
		v := buildVol(s, pts, d)
		if _, isGap := v.Level.(gap); isGap {
			failed++
		}
		if len(pts) > 0 {
			days.add(s.series, pts[0].date)
		}
		vols = append(vols, v)
	}

	cpts, err := storedPoints(ctx, r, seriesChart, d, chartLookback)
	if err != nil {
		return nil, err
	}
	ch := buildChart(cpts)
	if len(cpts) == 0 {
		failed++
	} else {
		days.add(seriesChart, cpts[0].date)
	}

	vm := viewModel{
		Schema:        "market_summary",
		SchemaVersion: 3,
		RunID:         c.RunID,
		BuildID:       c.BuildID,
		Scope:         c.Scope,
		GeneratedAt:   generatedAt.In(c.Zone).Format(timeLayout),
		DataTimes:     []sourceTime{},
		RunStatus:     "ok",
		Card: card{
			Title:      cardTitle,
			Say:        say(vols),
			Subtitle:   cardSubtitle,
			Chart:      ch,
			IndexTable: table,
			VolIndices: vols,
			VolNote:    volNote,
			Notes:      notes,
		},
	}
	if failed > 0 {
		vm.RunStatus = "partial"
	}
	fillTimes(&vm, days, d, c.Zone)
	vm.Card.Gaps = collectGaps(vm.Card)
	return json.Marshal(vm)
}

func validate(c Config, r store.SeriesReader) error {
	switch {
	case c.Zone == nil:
		return errors.New("home_market: nil zone")
	case c.Readings == nil:
		return errors.New("home_market: nil readings")
	case r == nil:
		return errors.New("home_market: nil series reader")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "scope": c.Scope} {
		if !idPattern.MatchString(v) {
			return fmt.Errorf("home_market: bad %s %q", name, v)
		}
	}
	return nil
}

// buildRow is the row of one index: the IV and the 20 day realised volatility from their stored
// readings; the difference from its stored market.iv_rv20 reading. ok is false when the IV is a Gap; a Gap of the
// realised volatility does not count.
func buildRow(ctx context.Context, c Config, name string, d time.Time) (indexRow, bool, error) {
	row := indexRow{
		Name:        name,
		IV:          mkGap("no_data", name+" "+ivNoRow),
		RV20:        mkGap("no_data", name+" "+rv20NoRow),
		IVMinusRV20: mkGap("no_data", name+" "+ivRvNoRow),
	}
	rv, err := newestReading(ctx, c, figRv20, name, d)
	if err != nil {
		return indexRow{}, false, err
	}
	row.RV20 = rv20Cell(name, rv)
	diff, err := newestReading(ctx, c, figIvRv20, name, d)
	if err != nil {
		return indexRow{}, false, err
	}
	row.IVMinusRV20 = ivRvCell(name, diff, d)
	best, err := newestReading(ctx, c, figIV, name, d)
	if err != nil {
		return indexRow{}, false, err
	}
	if best == nil {
		return row, false, nil
	}
	if string(best.State) != "ok" || best.Value == nil {
		row.IV = mkGap("missing_input", name+" "+ivMissing)
		return row, false, nil
	}
	text, err := shiftRound(*best.Value, 2, 1)
	if err != nil || strings.HasPrefix(text, "-") || strings.Trim(text, "0.") == "" {
		row.IV = mkGap("missing_input", name+" "+ivMissing)
		return row, false, nil
	}
	row.IV = volPoints{Kind: "value", Value: json.Number(text), Text: text, Unit: "vol_points"}
	row.SourceText = ivSource
	return row, true, nil
}

// newestReading is the newest stored reading of fig for the index within the last readingDays,
// nil when there is none.
func newestReading(ctx context.Context, c Config, fig, name string, d time.Time) (*contracts.ReadingRecord, error) {
	from := contracts.Day(d.AddDate(0, 0, -readingDays).Format(dayLayout))
	rows, err := c.Readings(ctx, fig, subjectKind, name, from, contracts.Day(d.Format(dayLayout)))
	if err != nil {
		return nil, fmt.Errorf("home_market: read %s %s: %w", fig, name, err)
	}
	var best *contracts.ReadingRecord
	for i := range rows {
		r := &rows[i]
		if best == nil || r.Day.After(best.Day.Time) || (r.Day.Equal(best.Day.Time) && r.Rev > best.Rev) {
			best = r
		}
	}
	return best, nil
}

// rv20Cell is the 20 day realised volatility: the stored value times 100, one decimal, half even.
// A value that rounds to 0.0 is a value; a Gap is for missing input only.
func rv20Cell(name string, r *contracts.ReadingRecord) any {
	if r == nil {
		return mkGap("no_data", name+" "+rv20NoRow)
	}
	if string(r.State) != "ok" || r.Value == nil {
		return mkGap("missing_input", name+" "+rv20Missing)
	}
	text, err := shiftRound(*r.Value, 2, 1)
	if err != nil || strings.HasPrefix(text, "-") {
		return mkGap("missing_input", name+" "+rv20Missing)
	}
	return volPoints{Kind: "value", Value: json.Number(text), Text: text, Unit: "vol_points"}
}

// buildVol is one Cboe index: its newest stored point, a Gap when there is none, when it is
// older than the stale limit or when it is negative.
func buildVol(s volSpec, pts []point, d time.Time) volIndex {
	v := volIndex{Name: s.name, Day: noDay, SourceText: s.series}
	switch {
	case len(pts) == 0:
		v.Level = mkGap("no_data", s.name+levelNoRow)
	case daysBetween(pts[0].date, d) > freshDays:
		v.Day = pts[0].date.Format("01-02")
		v.Level = mkGap("stale", s.name+levelStale+v.Day)
	case strings.HasPrefix(pts[0].text, "-"):
		v.Day = pts[0].date.Format("01-02")
		v.Level = mkGap("missing_input", s.name+levelBad)
	default:
		v.Day = pts[0].date.Format("01-02")
		v.Level = indexLevel{Kind: "value", Value: json.Number(pts[0].text), Text: pts[0].text, Unit: "index_points"}
	}
	return v
}

// buildChart draws the newest chartDays closes, oldest first. No stored 200 day line or band exists,
// so a point carries the close only; the level of a missing day is left out.
func buildChart(newestFirst []point) chart {
	if len(newestFirst) > chartDays {
		newestFirst = newestFirst[:chartDays]
	}
	ch := chart{Points: []chartPoint{}, SeriesNames: seriesNames, EndLabels: []string{noDay, noDay}, AriaText: ariaNone}
	for i := len(newestFirst) - 1; i >= 0; i-- {
		p := newestFirst[i]
		ch.Points = append(ch.Points, chartPoint{Label: p.date.Format("01-02"), Close: json.Number(p.text)})
	}
	if n := len(ch.Points); n > 0 {
		ch.EndLabels = []string{ch.Points[0].Label, ch.Points[n-1].Label}
		ch.AriaText = fmt.Sprintf(ariaFmt, n, ch.EndLabels[0], ch.EndLabels[1])
	}
	return ch
}

func say(vols []volIndex) string {
	if l, ok := vols[0].Level.(indexLevel); ok {
		return sayVix + l.Text + ", " + vols[0].Day
	}
	return sayNone
}

// storedPoints reads the series for lookback days up to d, keeps one point per observation date (the
// greatest RealtimeStart not after d), drops points after d or without a value, newest first.
func storedPoints(ctx context.Context, r store.SeriesReader, id string, d time.Time, lookback int) ([]point, error) {
	rows, err := r.SeriesPoints(ctx, id, contracts.Day(d.AddDate(0, 0, -lookback).Format(dayLayout)), contracts.Day(d.Format(dayLayout)))
	if err != nil {
		return nil, fmt.Errorf("home_market: read %s: %w", id, err)
	}
	kept := map[time.Time]contracts.SeriesPoint{}
	for _, p := range rows {
		obs := dateOf(p.ObservationDate.Time)
		if obs.After(d) || dateOf(p.RealtimeStart.Time).After(d) {
			continue
		}
		if cur, ok := kept[obs]; !ok || p.RealtimeStart.After(cur.RealtimeStart.Time) {
			kept[obs] = p
		}
	}
	out := []point{}
	for obs, p := range kept {
		if p.Value != nil {
			out = append(out, point{date: obs, text: strconv.FormatFloat(*p.Value, 'f', 2, 64)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].date.After(out[j].date) })
	return out, nil
}

// collectGaps copies the Gap objects of the blocks in page order, one per cell.
func collectGaps(c card) []gap {
	out := []gap{}
	for _, row := range c.IndexTable.Rows {
		if g, ok := row.IV.(gap); ok {
			out = append(out, g)
		}
		if g, ok := row.RV20.(gap); ok {
			out = append(out, g)
		}
		if g, ok := row.IVMinusRV20.(gap); ok {
			out = append(out, g)
		}
	}
	for _, v := range c.VolIndices {
		if g, ok := v.Level.(gap); ok {
			out = append(out, g)
		}
	}
	if len(c.Chart.Points) == 0 {
		out = append(out, mkGap("no_data", chartNoData))
	}
	return out
}

func midnight(t time.Time, zone *time.Location) string {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, zone).Format(timeLayout)
}

func at0930(t time.Time, zone *time.Location) string {
	return time.Date(t.Year(), t.Month(), t.Day()+freshDays, 9, 30, 0, 0, zone).Format(timeLayout)
}

func dateOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func daysBetween(from, to time.Time) int { return int(to.Sub(from) / (24 * time.Hour)) }

var decimalText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// shiftRound shifts the decimal text by shift places to the right, rounds it half to even to
// decimals places and writes it with exactly decimals places. The arithmetic is exact (math/big);
// a result of zero has no sign.
func shiftRound(text string, shift, decimals int) (string, error) {
	if !decimalText.MatchString(text) {
		return "", errors.New("home_market: not a decimal text")
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		return "", errors.New("home_market: not a decimal text")
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(shift+decimals)), nil)
	r.Mul(r, new(big.Rat).SetInt(scale))
	neg := r.Sign() < 0
	r.Abs(r)
	q, rem := new(big.Int).QuoRem(r.Num(), r.Denom(), new(big.Int))
	switch rem.Lsh(rem, 1).Cmp(r.Denom()) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	digits := q.String()
	if decimals > 0 {
		if len(digits) <= decimals {
			digits = strings.Repeat("0", decimals-len(digits)+1) + digits
		}
		digits = digits[:len(digits)-decimals] + "." + digits[len(digits)-decimals:]
	}
	if neg && q.Sign() != 0 {
		digits = "-" + digits
	}
	return digits, nil
}

// fillTimes writes data_times, the stale label, data_times_text and stale_after from the newest
// data day of each series; the oldest of them rules the label.
func fillTimes(vm *viewModel, days dataDays, d time.Time, zone *time.Location) {
	var oldest time.Time
	for _, s := range dataTimeSeries {
		t, ok := days[s]
		if !ok {
			continue
		}
		vm.DataTimes = append(vm.DataTimes, sourceTime{Source: s, DataTime: midnight(t, zone)})
		if oldest.IsZero() || t.Before(oldest) {
			oldest = t
		}
	}
	if oldest.IsZero() {
		vm.Card.Stale = staleLabel{DataTime: midnight(d, zone), IsStale: true, Text: noDataText}
		vm.Card.DataTimesTxt = noDataTimes
		vm.StaleAfter = at0930(d, zone)
		return
	}
	vm.Card.Stale = staleLabel{
		DataTime: midnight(oldest, zone),
		IsStale:  daysBetween(oldest, d) > freshDays,
		Text:     staleLabelPrefix + oldest.Format("01-02"),
	}
	vm.Card.DataTimesTxt = dataTimesPrefix + oldest.Format("01-02")
	vm.StaleAfter = at0930(oldest, zone)
}
