// Package home_options builds the options member of the home page: a page.options_summary.v4
// document read from the replayed option ledger (store.OptionLedgerReader replayed by a
// compute.OptionLedger, summarised by a compute.OptionResults, as option_tracking_page and
// option_result_page do). The tiles copy fields of the ResultSection that compute returns
// (Premium, Realized, KeptShare); the builder adds, divides, sums and rounds nothing, it only
// places the decimal text (sign, dollar sign, thousands separators). The six month series
// copies ResultMonth.Premium of the sold USD section by close month; a month without a figure is
// the gap no_data, never zero and never a sample.
// The builder reads no clock and makes no request.
package home_options

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	dayLayout   = "2006-01-02"
	monthLayout = "2006-01"
	timeLayout  = "2006-01-02T15:04:05-07:00"
	months      = 6
	staleDays   = 4 // the freshness rule of the home members: data day + 4 days at 09:30
	minus       = "−"
	usd         = "USD"
	sideSold    = "sold"

	title         = "Options overview"
	subtitle      = "From the trade ledger"
	sayNone       = "Option ledger: no records yet."
	sayNoSold     = "Ledger: no closed sold records yet."
	sayPremium    = "Premium from closed sold options totals "
	textNoRows    = "Ledger: no records yet"
	textNoReader  = "Option ledger: not connected yet"
	textNoSold    = "Ledger: no closed sold records"
	textNoDay     = "No ledger day"
	textNotCents  = "Ledger amounts are not whole cents, not rounded on this page"
	textMonthly   = "Settled premium by closing month: no data yet"
	textNoClosed  = "No sold records closed this month"
	textClosedMon = "Monthly closed P/L is not given on this page"
	textSoldCount = "Sold groups this month not computed yet"
	textOpen      = "Open premium not computed yet"
	textMaxLoss   = "Max loss needs the combined structure, the ledger does not give it"
	textOtherCcy  = "Ledger holds non-USD positions, not counted on this page"
	seriesText    = "Settled premium by closing month; expiry, exercise, assignment and buy-back count, fees are not deducted; year and month of the closing day, no time zone conversion; by account alias, not merged"
	closedText    = "Closed P/L, last six months: no data yet, monthly closed P/L is not given on this page"
	seriesCaption = "USD, by closing month: expiry, exercise, assignment and buy-back included, fees are not deducted; the year and month of the closing day; no time zone conversion; by account alias, never merged"
	noteCash      = "Premium counts opening trade cash, fees excluded; sold is positive, bought is negative."
	noteClosed    = "Premium and realised P/L count only closed sold positions, all accounts combined."
)

var monthLabels = [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

var (
	idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	cents     = regexp.MustCompile(`^(-?)([0-9]+)\.([0-9]{2})00$`)
)

// Config carries the identity of the run, the owner's zone, the ledger reader and the two compute
// functions. A nil Ledger gives the no_reader page: every figure is a no_data gap. A nil Replay
// or Results with a Ledger, and a nil Zone, are errors of Build.
type Config struct {
	RunID   string
	BuildID string
	Scope   string
	Zone    *time.Location
	Ledger  store.OptionLedgerReader
	Replay  compute.NewOptionLedgerFunc
	Results compute.NewOptionResultsFunc
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
	Tone  string      `json:"tone"`
}

type tile struct {
	Label   string `json:"label"`
	Figure  any    `json:"figure"`
	Caption string `json:"caption,omitempty"`
}

type tiles struct {
	Sold        tile `json:"sold_count_month"`
	PremiumMon  tile `json:"premium_month_usd"`
	OpenPremium tile `json:"open_premium_usd"`
	MaxLoss     tile `json:"max_loss_usd"`
}

type point struct {
	Month   string `json:"month"`
	Label   string `json:"label"`
	Premium any    `json:"premium"`
	Closed  any    `json:"closed"`
}

type stale struct {
	DataTime string `json:"data_time"`
	Text     string `json:"text"`
	IsStale  bool   `json:"is_stale"`
}

type card struct {
	Title         string   `json:"title"`
	Say           string   `json:"say"`
	Subtitle      string   `json:"subtitle"`
	Stale         stale    `json:"stale"`
	Tiles         tiles    `json:"tiles"`
	Series        []point  `json:"premium_series_6m"`
	SeriesText    string   `json:"premium_series_text"`
	ClosedText    string   `json:"closed_series_text"`
	SeriesCaption string   `json:"premium_series_caption"`
	Notes         []string `json:"notes"`
	Gaps          []gap    `json:"gaps"`
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
	Currency      string       `json:"currency"`
	GeneratedAt   string       `json:"generated_at"`
	StaleAfter    string       `json:"stale_after"`
	DataTimes     []sourceTime `json:"data_times"`
	RunStatus     string       `json:"run_status"`
	Card          card         `json:"card"`
}

// Build returns the page.options_summary.v4 JSON for day; at is generated_at, written in c.Zone.
// The series are the six calendar months ending with the month of day. An error of c.Ledger is an
// error of Build.
func Build(ctx context.Context, c Config, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, errors.New("home_options: nil zone")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "scope": c.Scope} {
		if !idPattern.MatchString(v) {
			return nil, fmt.Errorf("home_options: bad %s %q", name, v)
		}
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("home_options: bad day %q: %w", day, err)
	}
	var in compute.ResultInput
	var sections []compute.ResultSection
	if c.Ledger != nil {
		if in, err = replay(ctx, c, day); err != nil {
			return nil, err
		}
		if len(in.Positions) > 0 {
			if c.Results == nil {
				return nil, errors.New("home_options: nil results")
			}
			sections = c.Results().Summarise(in).Sections
		}
	}
	return json.Marshal(assemble(c, at, d, in.Positions, sections))
}

// replay replays every stored alias as of day; the positions, unresolved and review ids of all of
// them are one ResultInput.
func replay(ctx context.Context, c Config, day contracts.Day) (compute.ResultInput, error) {
	in := compute.ResultInput{AsOf: day}
	aliases, err := c.Ledger.StoredAliases(ctx)
	if err != nil {
		return in, fmt.Errorf("home_options: read aliases: %w", err)
	}
	in.Alias = strings.Join(aliases, ",")
	for _, alias := range aliases {
		rows, err := c.Ledger.FlexTradeRows(ctx, alias)
		if err != nil {
			return in, fmt.Errorf("home_options: read trades of %s: %w", alias, err)
		}
		if len(rows) == 0 {
			continue
		}
		if c.Replay == nil {
			return in, errors.New("home_options: nil replay")
		}
		trades := make([]compute.LedgerTrade, 0, len(rows))
		for _, r := range rows {
			var m map[string]string
			if err := json.Unmarshal([]byte(r.Attributes), &m); err != nil {
				return in, fmt.Errorf("home_options: attributes of %s: %w", r.TransactionID, err)
			}
			trades = append(trades, compute.LedgerTrade{TransactionID: r.TransactionID, Revision: r.Revision, Attributes: m})
		}
		res := c.Replay().Replay(compute.LedgerInput{Alias: alias, Trades: trades, AsOf: day}, compute.LedgerParams{})
		in.Positions = append(in.Positions, res.Positions...)
		in.Unresolved = append(in.Unresolved, res.Unresolved...)
		in.Review = append(in.Review, res.Review...)
	}
	return in, nil
}

// assemble builds the document from the sections of the summary.
func assemble(c Config, at, d time.Time, positions []compute.LedgerPosition, sections []compute.ResultSection) summary {
	noRows := len(positions) == 0
	noText := textNoRows
	if c.Ledger == nil {
		noText = textNoReader
	}
	base, baseText := at, textNoDay
	if newest := newestDay(positions, d); newest != "" {
		if t, err := time.Parse(dayLayout, newest); err == nil {
			base, baseText = t, "Ledger as of "+t.Format("01-02")
		}
	}
	staleAfter := time.Date(base.Year(), base.Month(), base.Day()+staleDays, 9, 30, 0, 0, c.Zone)
	s := summary{
		Schema: "options_summary", SchemaVersion: 4,
		RunID: c.RunID, BuildID: c.BuildID, Scope: c.Scope, Currency: usd,
		GeneratedAt: at.In(c.Zone).Format(timeLayout),
		StaleAfter:  staleAfter.Format(timeLayout),
		DataTimes:   []sourceTime{},
		RunStatus:   "partial",
	}
	if !noRows {
		s.DataTimes = append(s.DataTimes, sourceTime{Source: "option_ledger", DataTime: midnight(base, c.Zone)})
	}
	k := &s.Card
	k.Title, k.Subtitle, k.SeriesCaption = title, subtitle, seriesCaption
	k.SeriesText, k.ClosedText = seriesText, closedText
	k.Stale = stale{DataTime: midnight(base, c.Zone), Text: baseText, IsStale: at.After(staleAfter)}
	k.Notes = []string{noteCash, noteClosed}
	k.Gaps = []gap{noData(textMonthly), noData(textMaxLoss)}
	k.Series = series(d, nil)
	k.Tiles = tiles{
		Sold:        tile{Label: "Sold this month", Figure: noData(textSoldCount)},
		PremiumMon:  tile{Label: "Closed premium", Figure: noData(noText)},
		OpenPremium: tile{Label: "Open premium", Figure: noData(textOpen)},
		MaxLoss:     tile{Label: "Max loss", Figure: noData(textMaxLoss)},
	}
	k.Say = sayNone
	if noRows {
		k.Gaps = append([]gap{noData(noText)}, k.Gaps...)
		return s
	}
	var sold *compute.ResultSection
	for i, sec := range sections {
		switch {
		case sec.Currency != usd:
			k.Gaps = append(k.Gaps, gap{Kind: "gap", Reason: "unsupported_currency", Text: textOtherCcy})
		case sec.Side == sideSold && sold == nil:
			sold = &sections[i]
		}
	}
	k.Tiles.PremiumMon.Figure = noData(textNoSold)
	k.Say = sayNoSold
	if sold == nil {
		return s
	}
	k.Series = series(d, sold)
	if hasFigure(k.Series) {
		k.Gaps = slices.DeleteFunc(k.Gaps, func(g gap) bool { return g.Text == textMonthly })
	}
	k.Tiles.PremiumMon.Caption = caption(*sold)
	f := money(sold.Premium)
	k.Tiles.PremiumMon.Figure = f
	if fig, ok := f.(figure); ok {
		k.Say = sayPremium + fig.Text + "."
	}
	return s
}

// series is the six calendar months ending with the month of d, oldest first. The premium of a
// month is the Premium text of the month of the sold USD section with that Month, placed by money;
// a month the section does not list, or no section, is a gap. The closed figure is always a gap.
func series(d time.Time, sold *compute.ResultSection) []point {
	first := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
	out := make([]point, 0, months)
	for i := months - 1; i >= 0; i-- {
		m := first.AddDate(0, -i, 0)
		key := m.Format(monthLayout)
		var prem any = noData(textNoClosed)
		if sold != nil {
			for _, rm := range sold.Months {
				if rm.Month == key {
					prem = money(rm.Premium)
					break
				}
			}
		}
		out = append(out, point{
			Month: key, Label: monthLabels[m.Month()-1],
			Premium: prem, Closed: noData(textClosedMon),
		})
	}
	return out
}

// hasFigure reports whether any point of the series carries a premium figure.
func hasFigure(pts []point) bool {
	for _, p := range pts {
		if _, ok := p.Premium.(figure); ok {
			return true
		}
	}
	return false
}

// newestDay is the newest ledger day of the positions that is not after d; text order is day order.
func newestDay(positions []compute.LedgerPosition, d time.Time) string {
	limit := d.Format(dayLayout)
	newest := ""
	see := func(day contracts.Day) {
		if v := string(day); v != "" && v <= limit && v > newest {
			newest = v
		}
	}
	for _, p := range positions {
		see(p.OpenedOn)
		see(p.ClosedOn)
		for _, f := range p.Fills {
			see(f.Day)
		}
	}
	return newest
}

// caption places the Realized and KeptShare of the section as text; a field the section leaves
// empty is left out.
func caption(sec compute.ResultSection) string {
	parts := []string{"Opening cash of closed sold positions totals"}
	if r, ok := money(sec.Realized).(figure); ok {
		parts = append(parts, "realised "+r.Text)
	}
	if sec.KeptShare != "" {
		parts = append(parts, "kept share "+sec.KeptShare)
	}
	return strings.Join(parts, "; ")
}

func noData(text string) gap { return gap{Kind: "gap", Reason: "no_data", Text: text} }

func midnight(t time.Time, zone *time.Location) string {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, zone).Format(timeLayout)
}

// money places the decimal text v of a ResultSection (four places, as compute rounds it) as a
// usd_signed figure. Only text is rearranged (the last two places are dropped when they are
// zero, then sign, dollar sign and thousands separators); an empty text, or one with a third or
// fourth place, is a no_data gap and is never rounded here.
func money(v string) any {
	if v == "" {
		return noData(textNoSold)
	}
	m := cents.FindStringSubmatch(v)
	if m == nil {
		return noData(textNotCents)
	}
	neg, digits, frac := m[1] == "-", strings.TrimLeft(m[2], "0"), m[3]
	if digits == "" {
		digits = "0"
	}
	zero := digits == "0" && frac == "00"
	if zero {
		neg = false
	}
	num := digits + "." + frac
	var groups []string
	for len(digits) > 3 {
		groups = append([]string{digits[len(digits)-3:]}, groups...)
		digits = digits[:len(digits)-3]
	}
	amount := "$" + strings.Join(append([]string{digits}, groups...), ",") + "." + frac
	tone, sign := "flat", "+"
	switch {
	case neg:
		tone, sign, num = "down", minus, "-"+num
	case !zero:
		tone = "up"
	}
	return figure{Kind: "value", Value: json.Number(num), Text: sign + amount, Unit: "usd_signed", Tone: tone}
}
