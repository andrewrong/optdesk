// Package home_household builds the household member of the home page: the document
// page.household_summary.v2 with the four tiles net worth, cash, day change and month to date of the
// main account. The figures come from blockapi.PageFigures, the builder copies their decimal text,
// computes nothing, reads no clock and pools no account.
package home_household

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	calendarID      = "XNYS"
	dayLayout       = "2006-01-02"
	timeLayout      = "2006-01-02T15:04:05-07:00"
	sessionLookback = 150 // calendar days of session days to read
	staleSessions   = 3   // a capture older than this many session days is a stale_input gap
	staleAfterDays  = 4
	schemaName      = "household_summary"
	schemaVersion   = 2
)

// Config is what Build needs. RunID, BuildID and Scope are the identity of the member, which the home
// composer checks. Zone is the owner's zone (nil is an error of Build). Alias is the main account.
// A nil Account or Sessions gives each tile the gap no_reader; a nil Figures is an error of Build; a
// reader error is an error of Build, never a gap.
type Config struct {
	RunID, BuildID, Scope string
	Zone                  *time.Location
	Alias                 string
	Account               store.AccountReader
	Sessions              store.SessionDayReader
	Figures               blockapi.NewPageFiguresFunc
}

// capture is the account on the newest day with values, the day before it with values, and the last
// session day of the month before the data day.
type capture struct {
	day        string
	values     []store.AccountValue
	prior      []store.AccountValue
	monthStart []store.AccountValue
}

// Build writes the household member of day; at is generated_at.
func Build(ctx context.Context, c Config, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, errors.New("home_household: no zone")
	}
	if c.Figures == nil {
		return nil, errors.New("home_household: no figures")
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("home_household: bad day %q: %w", day, err)
	}
	at = at.In(c.Zone)
	switch {
	case c.Account == nil || c.Sessions == nil:
		return marshal(gapDoc(c, at, "no_reader", textNoReader))
	case c.Alias == "":
		return marshal(gapDoc(c, at, "no_data", textNoData))
	}
	sessions, err := c.sessionDays(ctx, d)
	if err != nil {
		return nil, err
	}
	snap, idx, err := c.capture(ctx, sessions)
	if err != nil {
		return nil, err
	}
	if snap.day == "" {
		return marshal(gapDoc(c, at, "no_data", textNoData))
	}
	if len(sessions)-1-idx > staleSessions {
		return marshal(gapDoc(c, at, "stale_input", textStale))
	}
	f := c.Figures()
	res := f.Household(blockapi.HouseholdInput{
		Alias: c.Alias, Day: contracts.Day(snap.day), Values: valuesIn(snap.values),
	})
	day1 := f.AccountMove(blockapi.AccountMoveIn{Alias: c.Alias, Values: valuesIn(snap.values), Prior: valuesIn(snap.prior)})
	mtd := f.AccountMove(blockapi.AccountMoveIn{Alias: c.Alias, Values: valuesIn(snap.values), Prior: valuesIn(snap.monthStart)})
	return marshal(c.assemble(snap, res, day1, mtd, at))
}

func marshal(d doc) ([]byte, error) { return json.Marshal(d) }

func valuesIn(vs []store.AccountValue) []blockapi.ValueIn {
	out := make([]blockapi.ValueIn, 0, len(vs))
	for _, v := range vs {
		out = append(out, blockapi.ValueIn{Tag: v.Tag, Value: v.Value, Currency: v.Currency})
	}
	return out
}

// sessionDays returns the session days up to and including d, ascending.
func (c Config) sessionDays(ctx context.Context, d time.Time) ([]contracts.Day, error) {
	from := contracts.Day(d.AddDate(0, 0, -sessionLookback).Format(dayLayout))
	rows, err := c.Sessions.SessionDays(ctx, calendarID, from, contracts.Day(d.Format(dayLayout)))
	if err != nil {
		return nil, fmt.Errorf("home_household: read session days: %w", err)
	}
	out := make([]contracts.Day, 0, len(rows))
	for _, r := range rows {
		if r.IsSession {
			out = append(out, contracts.Day(r.SessionDay.Format(dayLayout)))
		}
	}
	slices.Sort(out)
	return out, nil
}

// capture reads the values of the session days, newest first: the first day with rows is the data day
// (idx its place in sessions), the next one with rows the prior day, and the last session day before
// the month of the data day the month start.
func (c Config) capture(ctx context.Context, sessions []contracts.Day) (cp capture, idx int, err error) {
	read := func(s contracts.Day) ([]store.AccountValue, error) {
		rows, err := c.Account.Values(ctx, c.Alias, string(s))
		if err != nil {
			return nil, fmt.Errorf("home_household: read values: %w", err)
		}
		return rows, nil
	}
	i := len(sessions) - 1
	for ; i >= 0; i-- {
		rows, err := read(sessions[i])
		if err != nil {
			return cp, 0, err
		}
		if len(rows) > 0 {
			cp.day, cp.values, idx = string(sessions[i]), rows, i
			break
		}
	}
	if cp.day == "" {
		return cp, 0, nil
	}
	for i--; i >= 0; i-- {
		rows, err := read(sessions[i])
		if err != nil {
			return cp, 0, err
		}
		if len(rows) > 0 {
			cp.prior = rows
			break
		}
	}
	for j := len(sessions) - 1; j >= 0; j-- {
		if string(sessions[j]) < cp.day[:7] {
			if cp.monthStart, err = read(sessions[j]); err != nil {
				return cp, 0, err
			}
			break
		}
	}
	return cp, idx, nil
}

// assemble writes the document of a capture. Each tile that has no figure is a gap and is listed in
// the gaps of the card.
func (c Config) assemble(cp capture, res blockapi.HouseholdResult, day1, mtd blockapi.AccountMoveOut, at time.Time) doc {
	d := base(c, at)
	var net, cash blockapi.MoneyLine
	if len(res.Accounts) >= 2 {
		net, cash = res.Accounts[0], res.Accounts[1]
	}
	cur := net.Currency
	d.Card.Tiles = tiles{
		NetWorth:    netWorthTile(net, cp.day),
		Cash:        cashTile(cash, net),
		DayChange:   moveTile(labelDayChange, explainDayChange, day1.DayAmount, day1.DayPct, cur),
		MonthToDate: moveTile(labelMonthToDate, explainMonthToDate, mtd.DayAmount, mtd.DayPct, cur),
	}
	if len(cp.prior) == 0 {
		d.Card.Tiles.DayChange = sayMissing(d.Card.Tiles.DayChange, textNeedsPrior+cp.day)
	}
	if len(cp.monthStart) == 0 {
		d.Card.Tiles.MonthToDate = sayMissing(d.Card.Tiles.MonthToDate, textNeedsMonthStart+cp.day)
	}
	captured := at
	if len(cp.values) > 0 && !cp.values[0].CapturedAt.IsZero() {
		captured = cp.values[0].CapturedAt.In(c.Zone)
	}
	d.DataTimes = []sourceTime{{Source: "account", DataTime: captured.Format(timeLayout)}}
	d.StaleAfter = captured.AddDate(0, 0, staleAfterDays).Format(timeLayout)
	d.Card.Stale = staleLabel{DataTime: captured.Format(timeLayout), StaleAfter: d.StaleAfter, Text: textStaleLabel + cp.day}
	d.Card.Subtitle = textSubtitle + cp.day
	d.Card.DataTimesText = textDataDay + cp.day
	d.RunStatus = "ok"
	for _, t := range []struct {
		label string
		gap   *gapJSON
	}{
		{labelNetWorth, d.Card.Tiles.NetWorth.gap()}, {labelCash, d.Card.Tiles.Cash.gap()},
		{labelDayChange, d.Card.Tiles.DayChange.gap()}, {labelMonthToDate, d.Card.Tiles.MonthToDate.gap()},
	} {
		if t.gap != nil {
			d.RunStatus = "partial"
			d.Card.Gaps = append(d.Card.Gaps, gapJSON{Reason: t.gap.Reason, Text: t.label + ": " + t.gap.Text})
		}
	}
	return d
}

// sayMissing words the gap of a change tile that has no figure for want of a capture day: the reason
// stays missing_input, only the text says which capture is missing. Any other tile is returned as it is.
func sayMissing(t tile, text string) tile {
	if g := t.gap(); g == nil || g.Reason != "missing_input" {
		return t
	}
	g := gapJSON{Reason: "missing_input", Text: text}
	t.Figure, t.Caption = g, g
	return t
}

// base is the envelope with an empty card.
func base(c Config, at time.Time) doc {
	return doc{
		Schema: schemaName, SchemaVersion: schemaVersion, RunID: c.RunID, BuildID: c.BuildID, Scope: c.Scope,
		Currency: "USD", GeneratedAt: at.Format(timeLayout), DataTimes: []sourceTime{},
		StaleAfter: at.AddDate(0, 0, staleAfterDays).Format(timeLayout),
		Card: card{
			Title: cardTitle, Subtitle: cardTitle, DataTimesText: textNoCapture, Notes: []string{}, Gaps: []gapJSON{},
			Stale: staleLabel{
				DataTime: at.Format(timeLayout), StaleAfter: at.AddDate(0, 0, staleAfterDays).Format(timeLayout),
				Text: textStaleLabel + "none",
			},
		},
	}
}

// gapDoc is the member of an account that has nothing to show: the four tiles are the one gap.
func gapDoc(c Config, at time.Time, reason, text string) doc {
	d := base(c, at)
	d.RunStatus = "failed"
	d.Card.Subtitle = text
	g := gapJSON{Reason: reason, Text: text}
	d.Card.Tiles = tiles{
		NetWorth:    tile{Label: labelNetWorth, Figure: g, Caption: caption{Text: text}},
		Cash:        tile{Label: labelCash, Figure: g, Caption: g},
		DayChange:   tile{Label: labelDayChange, Figure: g, Caption: g},
		MonthToDate: tile{Label: labelMonthToDate, Figure: g, Caption: g},
	}
	for _, l := range []string{labelNetWorth, labelCash, labelDayChange, labelMonthToDate} {
		d.Card.Gaps = append(d.Card.Gaps, gapJSON{Reason: reason, Text: l + ": " + text})
	}
	return d
}

// netWorthTile is the net liquidation of the capture; only US dollars have a place in the schema.
func netWorthTile(net blockapi.MoneyLine, day string) tile {
	t := tile{Label: labelNetWorth, Explain: explainNetWorth, Caption: caption{Text: textDataDay + day}}
	if g := lineGap(net, false); g != "" {
		t.Figure = gapOf(g)
		return t
	}
	t.Figure = usdFigure{Value: json.Number(net.Amount.Value), Text: "$" + grouped(net.Amount.Value), Unit: "USD"}
	t.Caption = caption{Text: "USD · " + textDataDay + day}
	return t
}

// cashTile shows the share of cash in net liquidation, and the amount below it. A negative cash is a
// loan and has no share.
func cashTile(cash, net blockapi.MoneyLine) tile {
	t := tile{Label: labelCash, Explain: explainCash}
	if g := lineGap(cash, true); g != "" {
		t.Figure, t.Caption = gapOf(g), gapOf(g)
		return t
	}
	t.Caption = signedFigure(cash.Amount.Value)
	share := cash.Share.Value
	switch {
	case strings.HasPrefix(cash.Amount.Value, "-"):
		t.Figure = caption{Text: textBorrowing}
	case cash.Share.Gap != "" || share == "" || net.Amount.Gap != "" || overHundred(share):
		t.Figure = caption{Text: "$" + grouped(cash.Amount.Value)}
	default:
		t.Figure = shareFigure{Value: json.Number(share), Text: share + "%", Unit: "share_percent"}
	}
	return t
}

// moveTile is a tile of a change: the signed dollars and the percent.
func moveTile(label, explain string, amount, pct compute.FigureValue, cur string) tile {
	t := tile{Label: label, Explain: explain}
	switch {
	case amount.Gap != "":
		g := gapOf(amount.Gap)
		t.Figure, t.Caption = g, g
		return t
	case cur != "USD":
		g := gapOf("unsupported_currency")
		t.Figure, t.Caption = g, g
		return t
	}
	t.Figure = signedFigure(amount.Value)
	if pct.Gap != "" {
		t.Caption = gapOf(pct.Gap)
	} else {
		t.Caption = percentFigure(pct.Value)
	}
	return t
}

// lineGap is the reason a line has no US-dollar cell, "" when it has one. A negative amount has no
// place in the unsigned figure of net worth, a signed one (cash) keeps it.
func lineGap(l blockapi.MoneyLine, signed bool) string {
	switch {
	case l.Amount.Gap != "":
		return l.Amount.Gap
	case !decimalText.MatchString(l.Amount.Value):
		return "missing_input"
	case l.Currency != "USD":
		return "unsupported_currency"
	case !signed && strings.HasPrefix(l.Amount.Value, "-"):
		return "missing_input"
	}
	return ""
}

func gapOf(reason string) gapJSON { return gapJSON{Reason: reason, Text: gapText(reason)} }

// overHundred reports whether a decimal percent text is above 100.
func overHundred(s string) bool {
	whole, frac, _ := strings.Cut(s, ".")
	n, err := strconv.Atoi(whole)
	if err != nil {
		return true
	}
	return n > 100 || (n == 100 && strings.Trim(frac, "0") != "")
}

// The JSON shapes of page.household_summary.v2. Numbers are json.Number: the decimal text the figures
// carry, written as it is; nothing here parses, rounds or adds a figure.

type gapJSON struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type caption struct {
	Text string `json:"text"`
}

type usdFigure struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

type shareFigure usdFigure

type signed struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Tone  string      `json:"tone"`
	Unit  string      `json:"unit"`
}

// tile is a tile of the card; Figure and Caption are one of the shapes the schema allows for the tile.
type tile struct {
	Label   string `json:"label"`
	Figure  any    `json:"figure"`
	Caption any    `json:"caption"`
	Explain string `json:"explain,omitempty"`
}

// gap is the gap of the figure of the tile, nil when it has a figure.
func (t tile) gap() *gapJSON {
	if g, ok := t.Figure.(gapJSON); ok {
		return &g
	}
	return nil
}

type tiles struct {
	NetWorth    tile `json:"net_worth"`
	DayChange   tile `json:"day_change"`
	MonthToDate tile `json:"month_to_date"`
	Cash        tile `json:"cash"`
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

type card struct {
	Title         string     `json:"title"`
	Subtitle      string     `json:"subtitle"`
	Stale         staleLabel `json:"stale"`
	Tiles         tiles      `json:"tiles"`
	Notes         []string   `json:"notes"`
	Gaps          []gapJSON  `json:"gaps"`
	DataTimesText string     `json:"data_times_text"`
}

type doc struct {
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

var decimalText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// grouped writes the integer part of unsigned decimal text in groups of three digits.
func grouped(v string) string {
	v = strings.TrimPrefix(v, "-")
	whole, frac, hasFrac := strings.Cut(v, ".")
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if hasFrac {
		b.WriteString("." + frac)
	}
	return b.String()
}

// tone is up, down or flat by the sign of decimal text.
func tone(v string) string {
	switch {
	case strings.HasPrefix(v, "-"):
		if strings.Trim(v, "-0.") == "" {
			return "flat"
		}
		return "down"
	case strings.Trim(v, "0.") == "":
		return "flat"
	}
	return "up"
}

// sign is the sign character of the text of a signed figure: + or U+2212.
func sign(v string) string {
	if strings.HasPrefix(v, "-") && tone(v) != "flat" {
		return "\u2212"
	}
	return "+"
}

func signedFigure(v string) signed {
	return signed{Value: json.Number(v), Text: sign(v) + "$" + grouped(v), Tone: tone(v), Unit: "usd_signed"}
}

func percentFigure(v string) signed {
	return signed{Value: json.Number(v), Text: sign(v) + strings.TrimPrefix(v, "-") + "%", Tone: tone(v), Unit: "percent"}
}

// The words of the member, written once.
const (
	cardTitle      = "Household"
	textSubtitle   = "Main account · Data day "
	textDataDay    = "Data day "
	textStaleLabel = "Account data day "
	textNoReader   = "No reader: no data yet"
	textNoData     = "This account has no capture yet"
	textNoCapture  = "No capture"
	textStale      = "The latest capture is over 3 trading days old"
	textBorrowing  = "Borrowing"

	labelNetWorth    = "Net value"
	labelCash        = "Cash"
	labelDayChange   = "Previous trading day"
	labelMonthToDate = "Month to date"

	explainNetWorth = "Net liquidation of the main account (broker NetLiquidation, USD), from the latest capture."
	explainCash     = "Cash of the main account (TotalCashValue, USD). The large figure is the share of net value, " +
		"the small one the amount; negative cash is margin borrowing."
	explainDayChange = "Net liquidation of the latest capture minus the previous captured trading day, USD; " +
		"includes deposits and withdrawals, not pure return. The percentage is this change divided " +
		"by the previous day."
	explainMonthToDate = "Net liquidation of the latest capture minus the last trading day of the previous month, " +
		"USD; includes deposits and withdrawals, not time-weighted return."
)

const (
	textNeedsPrior      = "Needs a second captured session day; first capture "
	textNeedsMonthStart = "Needs the last session day of the previous month; first capture "
)

var gapTexts = map[string]string{
	"missing_quote":        "Needs position prices: no data yet",
	"missing_input":        "Input data is missing: no data yet",
	"unsupported_currency": "Exchange rate is missing or currency is unsupported: no data yet",
	"no_data":              textNoData,
	"no_reader":            textNoReader,
	"stale_input":          textStale,
}

func gapText(reason string) string {
	if t, ok := gapTexts[reason]; ok {
		return t
	}
	return "no data yet"
}
