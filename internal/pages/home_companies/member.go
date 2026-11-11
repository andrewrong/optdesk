// Package home_companies builds the held companies member of the home page, the document
// page.held_companies_summary.v3. It arranges figures stored as readings (the buy band and the
// owner's own volatility figures) and the owner's hand price text; it computes no formula, reads
// no clock of its own, makes no request and writes nothing. It reads the reading view the way
// companies_page does and imports none of that page: a pages sub-package may import only the
// roots and the declarations-only packages of its depends cell.
// Design: docs/design/pages-real-data.md#12 home-member-companies.
package home_companies

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	schemaName    = "held_companies_summary"
	schemaVersion = 3

	dayLayout  = "2006-01-02"
	timeLayout = "2006-01-02T15:04:05-07:00"

	bandName = "valuation.buy_band"
	hvName   = "market.hv30_own"
	ivName   = "market.iv30_own"
	ivpName  = "market.ivp_own"
	ibkrName = "market.ivp_ibkr"

	// lookbackDays is how far before the day a reading is looked for.
	lookbackDays = 10
	// staleDays is the display limit of the freshness label, as macro_page and market_page.
	staleDays = 4
	maxRows   = 30
	maxText   = 200

	noReaderText = "reader not set"
)

// Row is one held company. Issuer is the issuer text of the watch list, Market its market (US or
// JP), Symbol the contract symbol, HandPrice the owner's own buy price as text, empty when none;
// it is shown as it is and never parsed or changed. cmd/ot fills the rows from companiesSubjects.
type Row struct {
	Issuer, Market, Symbol, HandPrice string
}

// Config holds the inputs of Build. A nil Read gives a gap on every figure, never a guess. Rows are
// the held companies; the caller has already chosen them. RunID, BuildID and Scope are the identity
// of the home run, which home_page.Compose checks.
type Config struct {
	Zone    *time.Location
	Today   func() contracts.Day
	RunID   string
	BuildID string
	Scope   string
	Rows    []Row
	Read    store.ReadingViewReader
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
	Tone  string      `json:"tone,omitempty"`
}

type sharpMark struct {
	Kind string `json:"kind"`
}

type volatility struct {
	Source         string `json:"source"`
	Label          string `json:"label"`
	DataDay        string `json:"data_day"`
	HV30           any    `json:"hv30"`
	IV30           any    `json:"iv30"`
	IVP            any    `json:"ivp"`
	Reconciliation string `json:"reconciliation"`
}

type rowOut struct {
	Symbol        string     `json:"symbol"`
	Currency      string     `json:"currency"`
	Price         any        `json:"price"`
	Earnings      any        `json:"earnings"`
	DayChange     any        `json:"day_change"`
	FiveDayChange any        `json:"five_day_change"`
	DaySharp      sharpMark  `json:"day_sharp"`
	FiveDaySharp  sharpMark  `json:"five_day_sharp"`
	Volatility    volatility `json:"volatility"`
}

type column struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type stale struct {
	DataTime string `json:"data_time"`
	IsStale  bool   `json:"is_stale"`
	Text     string `json:"text"`
}

type card struct {
	Title         string   `json:"title"`
	Say           string   `json:"say"`
	Subtitle      string   `json:"subtitle"`
	Stale         stale    `json:"stale"`
	Columns       []column `json:"columns"`
	Rows          []rowOut `json:"rows"`
	SharpNote     string   `json:"sharp_note"`
	Notes         []string `json:"notes"`
	Gaps          []gap    `json:"gaps"`
	DataTimesText string   `json:"data_times_text"`
}

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type document struct {
	Schema        string       `json:"schema"`
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	BuildID       string       `json:"build_id"`
	Scope         string       `json:"scope"`
	GeneratedAt   string       `json:"generated_at"`
	DataTimes     []sourceTime `json:"data_times"`
	StaleAfter    string       `json:"stale_after"`
	RunStatus     string       `json:"run_status"`
	Card          card         `json:"card"`
}

var zoneOwner = time.FixedZone("+08:00", 8*3600)

var columns = []column{
	{ID: "company", Label: "Company"}, {ID: "price", Label: "Price line"}, {ID: "earnings", Label: "Earnings"},
	{ID: "day_change", Label: "Day"}, {ID: "five_day_change", Label: "5 days"},
}

// reading is the newest ok reading of a name and subject, with its day.
type reading struct {
	value  string
	day    time.Time
	source string
}

const (
	ownSource  = "own series"
	ibkrSource = "IBKR series"
)

// Build returns the page.held_companies_summary.v3 JSON read through cfg. Each row has the buy band
// reading (valuation.buy_band) as its price with the owner's hand price beside it, and the owner's own
// HV30, IV30 and IV percentile readings. Earnings, the day changes and the sharp marks have no stored
// source, so they are gaps, and so is the sharp-move note while no sharpmove readings exist.
// generatedAt is written in +08:00 (the zone of cfg when it is that). A reader error is an error of
// Build, never a gap.
func Build(ctx context.Context, cfg Config, generatedAt time.Time) ([]byte, error) {
	if cfg.Today == nil {
		return nil, fmt.Errorf("home_companies: Config.Today is not set")
	}
	today := cfg.Today()
	day, err := time.Parse(dayLayout, string(today))
	if err != nil {
		return nil, fmt.Errorf("home_companies: bad day %q: %w", today, err)
	}
	zone := cfg.zone(generatedAt)
	doc := document{
		Schema: schemaName, SchemaVersion: schemaVersion,
		RunID: cfg.RunID, BuildID: cfg.BuildID, Scope: cfg.Scope,
		GeneratedAt: generatedAt.In(zone).Format(timeLayout),
		RunStatus:   "ok",
		Card: card{
			Title: "Held companies", Subtitle: "Price line and volatility",
			Columns: columns, Rows: []rowOut{}, Notes: []string{}, Gaps: []gap{},
		},
	}
	rows := cfg.Rows
	if len(rows) > maxRows {
		rows = rows[:maxRows]
		doc.Card.Notes = append(doc.Card.Notes, fmt.Sprintf("Showing the first %d of %d companies.", maxRows, len(cfg.Rows)))
	}
	newest := time.Time{}
	priced := 0
	for _, r := range rows {
		cur, ok := currencyOf(r.Market)
		if !ok {
			doc.Card.Gaps = append(doc.Card.Gaps, gap{"gap", "no_data", clip(r.Symbol + ": market " + r.Market + " has no currency")})
			continue
		}
		out, hasPrice, seen, err := cfg.rowOf(ctx, r, cur, day)
		if err != nil {
			return nil, err
		}
		if hasPrice {
			priced++
		}
		if seen.After(newest) {
			newest = seen
		}
		doc.Card.Rows = append(doc.Card.Rows, out)
	}
	if cfg.Read == nil && len(rows) > 0 {
		doc.Card.Gaps = append(doc.Card.Gaps, gap{"gap", "no_reader", "price line and volatility: " + noReaderText})
	}
	doc.Card.Gaps = append(doc.Card.Gaps, gap{"gap", "no_data", "sharp moves: no sharpmove readings"})
	doc.Card.SharpNote = "Sharp moves: no readings yet."
	if len(doc.Card.Rows) > 0 && priced < len(doc.Card.Rows) {
		doc.RunStatus = "partial"
	}
	dataDay := day
	if !newest.IsZero() {
		dataDay = newest
	}
	at := time.Date(dataDay.Year(), dataDay.Month(), dataDay.Day(), 0, 0, 0, 0, zone)
	doc.Card.Stale = stale{
		DataTime: at.Format(timeLayout),
		IsStale:  int(day.Sub(dataDay).Hours()/24) > staleDays,
		Text:     "Data of " + dataDay.Format("01-02"),
	}
	doc.StaleAfter = time.Date(dataDay.Year(), dataDay.Month(), dataDay.Day()+staleDays, 9, 30, 0, 0, zone).Format(timeLayout)
	doc.DataTimes = []sourceTime{{Source: "held_companies", DataTime: at.Format(timeLayout)}}
	doc.Card.DataTimesText = "Data day " + dataDay.Format("01-02")
	doc.Card.Say = fmt.Sprintf("Held companies: %d · %d with a price line.", len(doc.Card.Rows), priced)
	return json.Marshal(doc)
}

// rowOf builds one row; seen is the newest reading day behind it, zero when none.
func (c Config) rowOf(ctx context.Context, r Row, currency string, day time.Time) (out rowOut, hasPrice bool, seen time.Time, err error) {
	out = rowOut{
		Symbol: r.Symbol, Currency: currency,
		Earnings:      mkGap("no_data", "earnings date: no source"),
		DayChange:     mkGap("no_data", "day change: no source"),
		FiveDayChange: mkGap("no_data", "5-day change: no source"),
		DaySharp:      sharpMark{Kind: "none"}, FiveDaySharp: sharpMark{Kind: "none"},
	}
	hand := ""
	if r.HandPrice != "" {
		hand = " · hand " + r.HandPrice
	}
	band, err := c.newest(ctx, bandName, r.Symbol, day)
	if err != nil {
		return out, false, seen, err
	}
	hv, err := c.newest(ctx, hvName, r.Symbol, day)
	if err != nil {
		return out, false, seen, err
	}
	iv, err := c.newest(ctx, ivName, r.Symbol, day)
	if err != nil {
		return out, false, seen, err
	}
	ivp, ivpGap, err := c.ivpOf(ctx, r.Symbol, day)
	if err != nil {
		return out, false, seen, err
	}
	for _, rd := range []*reading{band, hv, iv, ivp} {
		if rd != nil && rd.day.After(seen) {
			seen = rd.day
		}
	}
	switch {
	case c.Read == nil:
		out.Price = mkGap("no_reader", noReaderText+hand)
	case band == nil:
		out.Price = mkGap("no_data", "no band reading"+hand)
	default:
		bandValue, derr := decimal.NewFromString(band.value)
		if derr != nil {
			out.Price = mkGap("no_data", "band reading is not a number"+hand)
			break
		}
		shown := bandValue.String()
		text := shown + hand
		if band.day.Before(day) {
			text = shown + " · " + band.day.Format("01-02") + hand
		}
		out.Price = figure{Kind: "value", Value: json.Number(shown), Text: clip(text), Unit: "price"}
		hasPrice = true
	}
	dataDay := day
	if !seen.IsZero() {
		dataDay = seen
	}
	label := "own · hv30_v1 / iv30_v1 / ivp_v1 · data day " + dataDay.Format("01-02")
	ivpFig := c.percent(ivp, day, "IV percentile", "")
	if ivp != nil {
		ivpFig = c.percent(ivp, day, "IV percentile", ivp.source)
		if ivp.source == ibkrSource {
			label = "own · hv30_v1 / iv30_v1 · " + ibkrName + " · data day " + dataDay.Format("01-02")
		}
	} else if c.Read != nil {
		ivpFig = ivpGap
	}
	out.Volatility = volatility{
		Source: "own", Label: label,
		DataDay: dataDay.Format(dayLayout), Reconciliation: "not shown",
		HV30: c.percent(hv, day, "HV30", ""), IV30: c.percent(iv, day, "IV30", ""), IVP: ivpFig,
	}
	return out, hasPrice, seen, nil
}

// percent writes a stored fraction as a percent figure (two places, half to even), or the gap of a
// reading that is absent.
func (c Config) percent(rd *reading, day time.Time, label, source string) any {
	if c.Read == nil {
		return mkGap("no_reader", label+": "+noReaderText)
	}
	if rd == nil {
		return mkGap("no_data", label+": no stored reading")
	}
	v, err := decimal.NewFromString(rd.value)
	if err != nil {
		return mkGap("no_data", label+": reading is not a number")
	}
	n := v.Shift(2).RoundBank(2)
	text := n.StringFixed(2) + "%"
	if source != "" {
		text += " · " + source
	}
	if rd.day.Before(day) {
		text += " · " + rd.day.Format("01-02")
	}
	return figure{Kind: "value", Value: json.Number(n.StringFixed(2)), Text: text, Unit: "percent", Tone: "none"}
}

// newest returns the newest ok reading of name for symbol within the lookback of day, nil when none
// or when no reader is set.
func (c Config) newest(ctx context.Context, name, symbol string, day time.Time) (*reading, error) {
	rows, err := c.rowsOf(ctx, name, symbol, day)
	if err != nil {
		return nil, err
	}
	return newestOK(rows), nil
}

func (c Config) rowsOf(ctx context.Context, name, symbol string, day time.Time) ([]contracts.ReadingRecord, error) {
	if c.Read == nil {
		return nil, nil
	}
	from := contracts.Day(day.AddDate(0, 0, -lookbackDays).Format(dayLayout))
	rows, err := c.Read.WinnerReadings(ctx, name, "underlying", symbol, from, contracts.Day(day.Format(dayLayout)))
	if err != nil {
		return nil, fmt.Errorf("home_companies: %s of %s: %w", name, symbol, err)
	}
	return rows, nil
}

func newestOK(rows []contracts.ReadingRecord) *reading {
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		if r.State != "ok" || r.Value == nil {
			continue
		}
		return &reading{value: *r.Value, day: r.Day.Time}
	}
	return nil
}

// ivpOf applies the IVP rule of compute-stage2 Decision 3: market.ivp_own when ok, else
// market.ivp_ibkr when ok, else a gap that carries the reason of the stored market.ivp_own row.
func (c Config) ivpOf(ctx context.Context, symbol string, day time.Time) (*reading, gap, error) {
	own, err := c.rowsOf(ctx, ivpName, symbol, day)
	if err != nil {
		return nil, gap{}, err
	}
	if rd := newestOK(own); rd != nil {
		rd.source = ownSource
		return rd, gap{}, nil
	}
	ibkr, err := c.rowsOf(ctx, ibkrName, symbol, day)
	if err != nil {
		return nil, gap{}, err
	}
	if rd := newestOK(ibkr); rd != nil {
		rd.source = ibkrSource
		return rd, gap{}, nil
	}
	return nil, ivpGapOf(own), nil
}

// ivpGapOf names why the own IVP is not ok from its newest stored row.
func ivpGapOf(own []contracts.ReadingRecord) gap {
	const label = "IV percentile: "
	if len(own) == 0 {
		return mkGap("no_data", label+"no own IV30 history is stored")
	}
	r := own[len(own)-1]
	code := 0
	if r.ReasonCode != nil {
		code = *r.ReasonCode
	}
	switch code {
	case 1:
		return mkGap("missing_input", label+"no input, no IV30 reading for the day")
	case 2:
		n := 0
		for _, p := range r.Parameters {
			if p.Param == "observations" && p.ValueNum != nil {
				n = int(*p.ValueNum)
			}
		}
		return mkGap("missing_input", fmt.Sprintf("%sshort history, %d/252 own IV30 sessions", label, n))
	case 3:
		return mkGap("stale_input", label+"stale input, own IV30 is out of date")
	case 5:
		return mkGap("missing_input", label+"bad input, own IV30 history is unusable")
	}
	return mkGap("missing_input", fmt.Sprintf("%sown reading is missing (reason %d)", label, code))
}

// zone is the zone of cfg when generatedAt is +08:00 in it, else +08:00: the schema fixes the offset.
func (c Config) zone(t time.Time) *time.Location {
	if c.Zone == nil {
		return zoneOwner
	}
	if _, off := t.In(c.Zone).Zone(); off != 8*3600 {
		return zoneOwner
	}
	return c.Zone
}

func currencyOf(market string) (string, bool) {
	switch strings.ToUpper(market) {
	case "US":
		return "USD", true
	case "JP":
		return "JPY", true
	}
	return "", false
}

func mkGap(reason, text string) gap { return gap{Kind: "gap", Reason: reason, Text: clip(text)} }

// clip cuts text to the schema limit on a rune boundary.
func clip(s string) string {
	if utf8.RuneCountInString(s) <= maxText {
		return s
	}
	return string([]rune(s)[:maxText])
}
