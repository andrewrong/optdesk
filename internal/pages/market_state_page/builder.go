package market_state_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	dayLayout  = "2006-01-02"
	timeLayout = "2006-01-02T15:04:05-07:00"
	// staleDisplayDays is the display limit of the stale label (the limit of market_page).
	staleDisplayDays = 4
	// seriesBackDays is how far back the FRED series are read; the high-yield series starts at the floor day.
	seriesBackDays = 1500
	highYieldFloor = "2000-01-01"
)

// Series ids read from the store.
const (
	seriesSP500 = "SP500"
	seriesVIX   = "VIXCLS"
	seriesVXV   = "VXVCLS"
	seriesHY    = "BAMLH0A0HYM2"
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

type gap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type figure struct {
	Kind  string      `json:"kind"`
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

type percentile struct {
	Text   string `json:"text"`
	Window string `json:"window"`
}

type marketRow struct {
	ID         string      `json:"id"`
	Label      string      `json:"label"`
	Figure     any         `json:"figure"`
	Window     string      `json:"window"`
	Day        any         `json:"day"`
	Percentile *percentile `json:"percentile,omitempty"`
	History    any         `json:"history"`
}

type marketBlock struct {
	Title string      `json:"title"`
	Rows  []marketRow `json:"rows"`
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
	Title         string       `json:"title"`
	Notes         []string     `json:"notes"`
	Gaps          []gap        `json:"gaps"`
	Day           string       `json:"day"`
	Freshness     string       `json:"freshness_text"`
	Market        marketBlock  `json:"market"`
	Sectors       sectorBlock  `json:"sectors"`
}

// Build returns the page.market_state_page.v1 JSON for day. Series come from r, readings from
// c.Readings, bars from c.Bars; the figures are copied from c.Market and c.Liquidity and only
// shifted and rounded as decimal text. A reader error is an error of Build, never a Gap.
func Build(ctx context.Context, c Config, r store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error) {
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("market_state_page: bad day %q: %w", day, err)
	}
	if err := validate(c, r); err != nil {
		return nil, err
	}
	in := marketstateapi.Input{Day: day, Figures: c.Figures, Units: c.Units, Rules: c.Rules, Params: c.MarketParams}
	lq := liquidityapi.Input{Day: day, Params: c.LiquidityParams}
	if c.Market != nil || c.Liquidity != nil {
		if err := readSeries(ctx, r, d, &in, &lq); err != nil {
			return nil, err
		}
	}
	bars, err := readBars(ctx, c, d)
	if err != nil {
		return nil, err
	}
	if bars.ok {
		in.Benchmark, in.Funds, in.Industry = bars.bench, bars.funds, bars.industry
	}
	var ms marketstateapi.Result
	if c.Market != nil {
		ms = c.Market(in)
	}
	var lr liquidityapi.Result
	if c.Liquidity != nil {
		lr = c.Liquidity(lq)
	}
	rd, err := readReadings(ctx, c, d)
	if err != nil {
		return nil, err
	}
	rows, days := buildRows(c, rd, ms, lr)
	sec := buildSectors(c, ms, bars.ok && c.Market != nil)
	if sec.EndDayText != "" {
		days.add(spySymbol, sec.EndDayText)
	}
	vm := viewModel{
		Schema: "market_state_page", SchemaVersion: 1,
		RunID: c.RunID, BuildID: c.BuildID, Scope: c.Scope,
		GeneratedAt: generatedAt.In(c.Zone).Format(timeLayout),
		DataTimes:   []sourceTime{}, RunStatus: "ok",
		Title: pageTitle, Notes: []string{}, Day: string(day),
		Freshness: freshnessText(sec.EndDayText),
		Market:    marketBlock{Title: marketTitle, Rows: rows},
		Sectors:   sec.sectorBlock,
	}
	vm.Gaps = collectGaps(vm)
	if len(vm.Gaps) > 0 {
		vm.RunStatus = "partial"
	}
	var oldest time.Time
	for _, s := range days.sources() {
		t, err := time.Parse(dayLayout, days[s])
		if err != nil {
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
			IsStale:  int(d.Sub(oldest).Hours()/24) > staleDisplayDays,
			Text:     staleLabelPrefix + oldest.Format("01-02"),
		}
	}
	return json.Marshal(vm)
}

func validate(c Config, r store.SeriesReader) error {
	if c.Zone == nil {
		return errors.New("market_state_page: nil zone")
	}
	if r == nil {
		return errors.New("market_state_page: nil series reader")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "scope": c.Scope} {
		if !idPattern.MatchString(v) {
			return fmt.Errorf("market_state_page: bad %s %q", name, v)
		}
	}
	return nil
}

// readSeries reads the four FRED series into the inputs of both figure functions.
func readSeries(ctx context.Context, r store.SeriesReader, d time.Time, in *marketstateapi.Input, lq *liquidityapi.Input) error {
	from := contracts.Day(d.AddDate(0, 0, -seriesBackDays).Format(dayLayout))
	to := contracts.Day(d.Format(dayLayout))
	get := func(id string, f contracts.Day) (compute.Series, error) {
		rows, err := r.SeriesPoints(ctx, id, f, to)
		if err != nil {
			return nil, fmt.Errorf("market_state_page: read %s: %w", id, err)
		}
		return seriesOf(rows, d), nil
	}
	var err error
	if in.SP500, err = get(seriesSP500, from); err != nil {
		return err
	}
	if in.VIXCLS, err = get(seriesVIX, from); err != nil {
		return err
	}
	if in.VXVCLS, err = get(seriesVXV, from); err != nil {
		return err
	}
	if in.BAMLH0A0HYM2, err = get(seriesHY, highYieldFloor); err != nil {
		return err
	}
	lq.BAMLH0A0HYM2 = in.BAMLH0A0HYM2
	return nil
}

// seriesOf keeps, per observation date, the row of the greatest RealtimeStart not after d, drops
// rows with no value or a status other than present, and returns the days ascending.
func seriesOf(rows []contracts.SeriesPoint, d time.Time) compute.Series {
	kept := map[string]contracts.SeriesPoint{}
	for _, p := range rows {
		if p.RealtimeStart.After(d) || (p.Status != "" && p.Status != "present") {
			continue
		}
		k := p.ObservationDate.Format(dayLayout)
		if cur, ok := kept[k]; !ok || p.RealtimeStart.After(cur.RealtimeStart.Time) {
			kept[k] = p
		}
	}
	days := make([]string, 0, len(kept))
	for k, p := range kept {
		if p.Value != nil {
			days = append(days, k)
		}
	}
	if len(days) == 0 {
		return nil
	}
	slices.Sort(days)
	out := make(compute.Series, 0, len(days))
	for _, k := range days {
		out = append(out, compute.Point{Day: contracts.Day(k), Value: *kept[k].Value})
	}
	return out
}

// dataDays holds the newest data day (ISO text) per source.
type dataDays map[string]string

func (m dataDays) add(source, day string) {
	if day == "" {
		return
	}
	if cur, ok := m[source]; !ok || day > cur {
		m[source] = day
	}
}

func (m dataDays) sources() []string {
	out := make([]string, 0, len(m))
	for s := range m {
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

func midnight(t time.Time, zone *time.Location) string {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, zone).Format(timeLayout)
}

// freshnessText says what the figures are and how old each source is.
func freshnessText(endDay string) string {
	last := freshnessNone
	if t, err := time.Parse(dayLayout, endDay); err == nil {
		last = t.Format("Mon") + " " + endDay
	}
	return freshnessHead + last + freshnessTail
}

// collectGaps copies the Gap objects of the market rows and the end day in page order.
func collectGaps(vm viewModel) []gap {
	out := []gap{}
	for _, row := range vm.Market.Rows {
		if g, ok := row.Figure.(gap); ok {
			out = append(out, g)
		}
		if g, ok := row.History.(gap); ok {
			out = append(out, g)
		}
	}
	if g, ok := vm.Sectors.EndDay.(gap); ok {
		out = append(out, g)
	}
	return out
}
