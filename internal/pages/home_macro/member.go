// Package home_macro builds the macro member of the home page: a page.macro_summary.v2
// document read from the stored macro.ease reading. It computes nothing beyond rounding the
// stored value and the week change, reads no clock and makes no request. The events have no
// stored source, so they are empty and named as a not_built gap. With a Liquidity function the
// card's say is the dated reserves fact of that function's result (reservesSay) instead of the Ease.
package home_macro

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	dayLayout  = "2006-01-02"
	timeLayout = "2006-01-02T15:04:05-07:00"

	easeName    = "macro.ease"
	easeVersion = 2
	easeKind    = "market"
	easeSubject = "WALCL"
	weekDays    = 7
	staleDays   = 4 // the freshness rule of macro_page: data day + 4 days at 09:30
	bandLow     = "33.5"
	bandHigh    = "66.5"
	minus       = "−"

	title        = "Macro"
	sayEase      = "Ease %s, %s."
	sayNoEase    = "Ease: no data yet."
	textNoData   = "Ease: no data yet"
	textNoReader = "Ease: not connected yet"
	textNoWeek   = "Change vs last week: no data yet"
	textEvents   = "Event calendar: not connected yet"
	textNoDay    = "No data day"
	noteDates    = "Event dates follow the release calendar, no release time."
)

var (
	idPattern   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	decimalText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)
	bandWords   = map[string]string{"tighter": "tighter", "steady": "neutral", "looser": "looser"}
)

// Config carries the identity of the run, the owner's zone and the reading function. A nil
// Read gives the ease as a no_reader gap.
type Config struct {
	RunID   string
	BuildID string
	Scope   string
	Zone    *time.Location
	Read    compute.ReadingFunc
	// Series, Liquidity and LiquidityParams are optional. Only with Liquidity set, say is the dated reserves
	// fact built from the figures Liquidity returns over the WRESBAL series read through Series (a nil Series
	// is a gap); with Liquidity nil, say is the Ease sentence as before. An error of Series is an error of Build.
	Series          store.SeriesReader
	Liquidity       liquidityapi.Func
	LiquidityParams liquidityapi.Params
}

type gap struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type figure struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
	Tone  string      `json:"tone,omitempty"`
}

type band struct {
	Low  figure `json:"low"`
	High figure `json:"high"`
}

type ease struct {
	Figure        figure   `json:"figure"`
	WeekChange    any      `json:"week_change"`
	MarkerPercent figure   `json:"marker_percent"`
	NeutralBand   band     `json:"neutral_band"`
	Ticks         []string `json:"ticks"`
	ReadingText   string   `json:"reading_text"`
	AriaText      string   `json:"aria_text"`
}

type stale struct {
	DataTime   string `json:"data_time"`
	StaleAfter string `json:"stale_after"`
	Text       string `json:"text"`
}

type card struct {
	Title         string   `json:"title"`
	Say           string   `json:"say"`
	Stale         stale    `json:"stale"`
	Ease          any      `json:"ease"`
	Events        []string `json:"events"`
	Notes         []string `json:"notes"`
	Gaps          []gap    `json:"gaps"`
	DataTimesText string   `json:"data_times_text"`
}

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type summary struct {
	Schema        string       `json:"schema"`
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	BuildID       string       `json:"build_id"`
	Scope         string       `json:"scope"`
	GeneratedAt   string       `json:"generated_at"`
	DataTimes     []sourceTime `json:"data_times"`
	RunStatus     string       `json:"run_status"`
	Card          card         `json:"card"`
}

// reading is the winner macro.ease reading of one day that has a value.
type reading struct {
	value   *big.Rat
	band    string
	dataDay time.Time
}

// Build returns the page.macro_summary.v2 JSON for day; at is generated_at, written in
// c.Zone. The events are empty with a not_built gap, so the run status is partial. The ease
// is a no_reader gap with a nil c.Read, a no_data gap with no stored reading with a value
// for day, and the block of the stored macro.ease reading otherwise. An error of c.Read is
// an error of Build.
func Build(ctx context.Context, c Config, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, errors.New("home_macro: nil zone")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "scope": c.Scope} {
		if !idPattern.MatchString(v) {
			return nil, fmt.Errorf("home_macro: bad %s %q", name, v)
		}
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("home_macro: bad day %q: %w", day, err)
	}
	s := summary{
		Schema: "macro_summary", SchemaVersion: 2,
		RunID: c.RunID, BuildID: c.BuildID, Scope: c.Scope,
		GeneratedAt: at.In(c.Zone).Format(timeLayout),
		DataTimes:   []sourceTime{},
		RunStatus:   "partial",
		Card: card{
			Title:  title,
			Events: []string{},
			Notes:  []string{noteDates},
			Gaps:   []gap{{Reason: "not_built", Text: textEvents}},
		},
	}
	base := d
	baseText := textNoDay
	g := gap{Reason: "no_data", Text: textNoData}
	s.Card.Say = sayNoEase
	s.Card.DataTimesText = textNoData + ". " + noteDates
	if c.Read == nil {
		g = gap{Reason: "no_reader", Text: textNoReader}
	} else {
		block, cur, err := easeFor(ctx, c.Read, d)
		if err != nil {
			return nil, err
		}
		if block != nil {
			s.Card.Ease = block
			s.Card.Say = fmt.Sprintf(sayEase, bandWords[cur.band], weekSay(block))
			base, baseText = cur.dataDay, "Data day "+cur.dataDay.Format("01-02")
			s.DataTimes = append(s.DataTimes, sourceTime{Source: "macro_ease", DataTime: midnight(base, c.Zone)})
			s.Card.DataTimesText = "Ease data day " + base.Format("01-02") + ". " + noteDates
		}
	}
	if c.Liquidity != nil {
		say, err := reservesSay(ctx, c, d)
		if err != nil {
			return nil, err
		}
		s.Card.Say = say
		if say == sayNoReserves {
			s.Card.Gaps = append(s.Card.Gaps, gap{Reason: "no_data", Text: textNoReserves})
		}
	}
	if s.Card.Ease == nil {
		s.Card.Ease = g
		s.Card.Gaps = append([]gap{g}, s.Card.Gaps...)
	}
	s.Card.Stale = stale{
		DataTime:   midnight(base, c.Zone),
		StaleAfter: time.Date(base.Year(), base.Month(), base.Day()+staleDays, 9, 30, 0, 0, c.Zone).Format(timeLayout),
		Text:       baseText,
	}
	return json.Marshal(s)
}

func midnight(t time.Time, zone *time.Location) string {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, zone).Format(timeLayout)
}

// weekSay is the week-change sentence of the card, from the stored ease block.
func weekSay(b *ease) string {
	if f, ok := b.WeekChange.(figure); ok {
		return "vs last week " + f.Text
	}
	return textNoWeek
}

// readEase reads macro.ease for day; nil with no error means no usable reading.
func readEase(ctx context.Context, read compute.ReadingFunc, day time.Time) (*reading, error) {
	d := contracts.Day(day.Format(dayLayout))
	rows, err := read(ctx, easeName, easeKind, easeSubject, d, d)
	if err != nil {
		return nil, fmt.Errorf("home_macro: read %s: %w", easeName, err)
	}
	var best *contracts.ReadingRecord
	for i := range rows {
		r := &rows[i]
		if r.Name != easeName || r.Version != easeVersion || r.Day.Format(dayLayout) != string(d) {
			continue
		}
		if string(r.State) != "ok" || r.Value == nil {
			continue
		}
		if best == nil || r.Rev > best.Rev {
			best = r
		}
	}
	if best == nil {
		return nil, nil
	}
	if !decimalText.MatchString(*best.Value) {
		return nil, fmt.Errorf("home_macro: %s value %q is not decimal text", easeName, *best.Value)
	}
	v, _ := new(big.Rat).SetString(*best.Value)
	if v.Sign() < 0 || v.Cmp(big.NewRat(100, 1)) > 0 {
		return nil, fmt.Errorf("home_macro: %s value %q outside 0 to 100", easeName, *best.Value)
	}
	out := &reading{value: v}
	out.band, out.dataDay = readParams(best, day)
	if _, ok := bandWords[out.band]; !ok {
		return nil, nil
	}
	return out, nil
}

// readParams returns the band tag and the newest <input>.data_day of a reading; fallback is
// the data day when none is stored.
func readParams(r *contracts.ReadingRecord, fallback time.Time) (string, time.Time) {
	var tag string
	var newest time.Time
	for _, p := range r.Parameters {
		if p.ValueTag == nil {
			continue
		}
		switch {
		case p.Param == "band":
			tag = *p.ValueTag
		case strings.HasSuffix(p.Param, ".data_day"):
			if t, err := time.Parse(dayLayout, *p.ValueTag); err == nil && t.After(newest) {
				newest = t
			}
		}
	}
	if newest.IsZero() {
		newest = fallback
	}
	return tag, newest
}

// roundHalfUp rounds r to places decimals, halves away from zero, as exact decimal text.
func roundHalfUp(r *big.Rat, places int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	x := new(big.Rat).Mul(r, new(big.Rat).SetInt(scale))
	neg := x.Sign() < 0
	x.Abs(x)
	x.Add(x, big.NewRat(1, 2))
	n := new(big.Int).Quo(x.Num(), x.Denom())
	if neg {
		n.Neg(n)
	}
	return new(big.Rat).SetFrac(n, scale).FloatString(places)
}

func signedText(num string) string {
	switch {
	case strings.HasPrefix(num, "-"):
		return minus + num[1:]
	case strings.Trim(num, "0.") == "":
		return num
	}
	return "+" + num
}

func track(num, text string) figure {
	return figure{Value: json.Number(num), Text: text, Unit: "percent_of_track"}
}

// easeFor builds the ease block from the readings of day and of the day a week before; nil
// when day has no usable reading.
func easeFor(ctx context.Context, read compute.ReadingFunc, day time.Time) (*ease, *reading, error) {
	cur, err := readEase(ctx, read, day)
	if err != nil || cur == nil {
		return nil, nil, err
	}
	prev, err := readEase(ctx, read, day.AddDate(0, 0, -weekDays))
	if err != nil {
		return nil, nil, err
	}
	whole := roundHalfUp(cur.value, 0)
	word := bandWords[cur.band]
	b := &ease{
		Figure:        figure{Value: json.Number(roundHalfUp(cur.value, 1)), Text: whole, Unit: "index_points"},
		MarkerPercent: track(roundHalfUp(cur.value, 2), whole),
		NeutralBand:   band{Low: track(bandLow, bandLow+"%"), High: track(bandHigh, bandHigh+"%")},
		Ticks:         []string{"0", "25", "50", "75", "100"},
		WeekChange:    gap{Reason: "no_data", Text: textNoWeek},
	}
	weekText := textNoWeek
	if prev != nil {
		num := roundHalfUp(new(big.Rat).Sub(cur.value, prev.value), 1)
		text := signedText(num)
		tone := "flat"
		if strings.HasPrefix(text, "+") {
			tone = "up"
		} else if strings.HasPrefix(text, minus) {
			tone = "down"
		}
		b.WeekChange = figure{Value: json.Number(num), Text: text, Unit: "index_points", Tone: tone}
		weekText = "vs last week " + text
	}
	b.ReadingText = "Now " + whole + ", " + word + "; fixed scale 0 to 100."
	b.AriaText = "Ease " + whole + ", " + word + ", " + weekText +
		"; fixed scale 0 to 100, neutral band " + bandLow + " to " + bandHigh + "."
	return b, cur, nil
}

// Design: docs/design/liquidity-view.md, Decision 4.

const (
	reservesID       = "WRESBAL"
	reservesLookback = 60 // days read before day: the 4-week change needs 28
	sayNoReserves    = "Reserves: no data yet."
	textNoReserves   = "Reserves: no data yet"
	sayReserves      = "Reserves %s, 4 weeks %s (%s)."
)

// reservesSay is "Reserves $3.0tn, 4 weeks −$40bn (2026-11-11)." from the Reserves and Reserves4w figures of
// c.Liquidity over the WRESBAL series alone; sayNoReserves with no Series or a figure with a Reason.
func reservesSay(ctx context.Context, c Config, d time.Time) (string, error) {
	if c.Series == nil {
		return sayNoReserves, nil
	}
	day := contracts.Day(d.Format(dayLayout))
	from := contracts.Day(d.AddDate(0, 0, -reservesLookback).Format(dayLayout))
	rows, err := c.Series.SeriesPoints(ctx, reservesID, from, day)
	if err != nil {
		return "", fmt.Errorf("home_macro: read %s: %w", reservesID, err)
	}
	res := c.Liquidity(liquidityapi.Input{Day: day, WRESBAL: seriesOf(rows, d), Params: c.LiquidityParams})
	if res.Reserves.Reason != "" || res.Reserves4w.Reason != "" {
		return sayNoReserves, nil
	}
	level, ok1 := new(big.Rat).SetString(res.Reserves.Value)
	delta, ok2 := new(big.Rat).SetString(res.Reserves4w.Value)
	if !ok1 || !ok2 || res.Reserves.Day == "" {
		return sayNoReserves, nil
	}
	return fmt.Sprintf(sayReserves, level2text(level), bnSigned(delta), res.Reserves.Day), nil
}

var (
	oneTn = big.NewRat(1_000_000_000_000, 1)
	oneBn = big.NewRat(1_000_000_000, 1)
)

// level2text is whole USD as $x.xtn from one trillion, else $xbn.
func level2text(v *big.Rat) string {
	if v.Cmp(oneTn) >= 0 {
		return "$" + roundHalfUp(new(big.Rat).Quo(v, oneTn), 1) + "tn"
	}
	return "$" + roundHalfUp(new(big.Rat).Quo(v, oneBn), 0) + "bn"
}

// bnSigned is a change in billions, signed with the minus sign: −$40bn, +$48bn, $0bn.
func bnSigned(v *big.Rat) string {
	n := roundHalfUp(new(big.Rat).Quo(v, oneBn), 0)
	switch signedText(n) {
	case n:
		return "$" + n + "bn"
	case "+" + n:
		return "+$" + n + "bn"
	}
	return minus + "$" + n[1:] + "bn"
}

// seriesOf keeps, per observation date, the row of the greatest RealtimeStart not after d, drops rows with no
// value or a status other than present, and returns the days ascending; nil when none is left.
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
	slices.Sort(days)
	var out compute.Series
	for _, k := range days {
		out = append(out, compute.Point{Day: contracts.Day(k), Value: *kept[k].Value})
	}
	return out
}
