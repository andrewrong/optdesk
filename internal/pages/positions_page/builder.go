package positions_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

const (
	calendarID         = "XNYS"
	dayLayout          = "2006-01-02"
	stampLayout        = "2006-01-02T15:04:05-07:00"
	sessionLookback    = 150 // calendar days of session days to read
	historyDays        = 90  // session days looked through for a capture
	staleSessions      = 3   // a capture older than this many session days is a stale_input gap
	markMaxAgeSessions = 5   // LedgerParams default of structures_page
	staleAfterDays     = 4
)

var scopeKey = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type staleLabel struct {
	DataTime   string `json:"data_time"`
	StaleAfter string `json:"stale_after"`
	Text       string `json:"text"`
}

type stocksTable struct {
	Title   string     `json:"title"`
	Columns []column   `json:"columns"`
	Rows    []stockRow `json:"rows"`
	Gap     *gapJSON   `json:"gap"`
}

type optionsTable struct {
	Title   string      `json:"title"`
	Columns []column    `json:"columns"`
	Rows    []optionRow `json:"rows"`
	Gap     *gapJSON    `json:"gap"`
}

type viewJSON struct {
	Key      string       `json:"key"`
	Label    string       `json:"label"`
	Sentence string       `json:"sentence"`
	Stocks   stocksTable  `json:"stocks"`
	Options  optionsTable `json:"options"`
	Trades   *tradesTable `json:"trades"`
}

type pageJSON struct {
	Schema        string       `json:"schema"`
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	BuildID       string       `json:"build_id"`
	Scope         string       `json:"scope"`
	GeneratedAt   string       `json:"generated_at"`
	DataTimes     []sourceTime `json:"data_times"`
	RunStatus     string       `json:"run_status"`
	Title         string       `json:"title"`
	Stale         staleLabel   `json:"stale"`
	Views         []viewJSON   `json:"views"`
	Notes         []string     `json:"notes"`
	Gaps          []gapJSON    `json:"gaps"`
}

// capture is what the account holds on its newest session day with positions.
type capture struct {
	day       string
	positions []store.Position
}

// Build writes the positions_page of alias for day; at is generated_at. alias "" is the main account.
func Build(ctx context.Context, c Config, alias string, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, errors.New("positions_page: no zone")
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("positions_page: bad day %q: %w", day, err)
	}
	at = at.In(c.Zone)
	if alias == "" && len(c.Aliases) > 0 {
		alias = c.Aliases[0]
	}
	label := labelAccount + alias + labelClose
	if !slices.Contains(c.Aliases, alias) {
		return marshal(c.gapPage(alias, label, at, "no_data", textUnknown))
	}
	if !c.ready() {
		return marshal(c.gapPage(alias, label, at, "no_reader", textNoReader))
	}
	sessions, err := c.sessionDays(ctx, d)
	if err != nil {
		return nil, err
	}
	snap, err := c.capture(ctx, alias, sessions)
	if err != nil {
		return nil, err
	}
	if snap.day == "" {
		return marshal(c.gapPage(alias, label, at, "no_data", textNoCapture))
	}
	if lag := len(sessions) - 1 - slices.Index(sessions, contracts.Day(snap.day)); lag > staleSessions {
		return marshal(c.gapPage(alias, label, at, "stale_input", textStale))
	}
	price, err := c.prices(ctx, alias, snap.day)
	if err != nil {
		return nil, err
	}
	prem, ledgerAt, err := c.premiums(ctx, alias, sessions, day)
	if err != nil {
		return nil, err
	}
	in := holdingsOf(snap.positions, price)
	res := c.Holdings().Holdings(in)
	if len(res.Rows) != len(in.Rows) {
		return nil, fmt.Errorf("positions_page: holdings gave %d lines for %d rows", len(res.Rows), len(in.Rows))
	}
	stocks, options, skipped := rowsOf(snap.positions, price, res.Rows, prem, text2(label), day)
	trades, err := c.trades(ctx, alias, label, sessions, day)
	if err != nil {
		return nil, err
	}
	return marshal(c.assemble(alias, label, snap, ledgerAt, stocks, options, trades, skipped, at))
}

// ready is true when every reader and constructor is set.
func (c Config) ready() bool {
	return c.Account != nil && c.Marks != nil && c.Ledger != nil && c.Sessions != nil && c.Ledgers != nil &&
		c.Figures != nil && c.Holdings != nil
}

// prices maps the con id of each marked position of the capture day to its price; a mark with no
// price is left out.
func (c Config) prices(ctx context.Context, alias, day string) (map[int64]string, error) {
	marks, err := c.Marks.Marks(ctx, alias, day)
	if err != nil {
		return nil, fmt.Errorf("positions_page: read marks: %w", err)
	}
	price := map[int64]string{}
	for _, m := range marks {
		if m.Basis != mark.BasisNone && m.Price != "" {
			price[m.ConID] = m.Price
		}
	}
	return price, nil
}

func marshal(p pageJSON) ([]byte, error) { return json.Marshal(p) }

// sessionDays returns the session days up to and including d, ascending, at most historyDays.
func (c Config) sessionDays(ctx context.Context, d time.Time) ([]contracts.Day, error) {
	from := contracts.Day(d.AddDate(0, 0, -sessionLookback).Format(dayLayout))
	rows, err := c.Sessions.SessionDays(ctx, calendarID, from, contracts.Day(d.Format(dayLayout)))
	if err != nil {
		return nil, fmt.Errorf("positions_page: read session days: %w", err)
	}
	out := make([]contracts.Day, 0, len(rows))
	for _, r := range rows {
		if r.IsSession {
			out = append(out, contracts.Day(r.SessionDay.Format(dayLayout)))
		}
	}
	slices.Sort(out)
	if len(out) > historyDays {
		out = out[len(out)-historyDays:]
	}
	return out, nil
}

// capture reads the positions of the session days, newest first, until one has rows.
func (c Config) capture(ctx context.Context, alias string, sessions []contracts.Day) (capture, error) {
	for i := len(sessions) - 1; i >= 0; i-- {
		rows, err := c.Account.Positions(ctx, alias, string(sessions[i]))
		if err != nil {
			return capture{}, fmt.Errorf("positions_page: read positions: %w", err)
		}
		if len(rows) > 0 {
			return capture{day: string(sessions[i]), positions: rows}, nil
		}
	}
	return capture{}, nil
}

// premiums replays the ledger of alias and returns the remaining premium of each open contract and the
// newest record time.
func (c Config) premiums(ctx context.Context, alias string, sessions []contracts.Day, day contracts.Day) (map[string]string, time.Time, error) {
	var newest time.Time
	rows, err := c.Ledger.FlexTradeRows(ctx, alias)
	if err != nil {
		return nil, newest, fmt.Errorf("positions_page: read ledger: %w", err)
	}
	trades := make([]compute.LedgerTrade, 0, len(rows))
	for _, r := range rows {
		attrs := map[string]string{}
		if err := json.Unmarshal([]byte(r.Attributes), &attrs); err != nil {
			return nil, newest, fmt.Errorf("positions_page: ledger row %s: %w", r.TransactionID, err)
		}
		trades = append(trades, compute.LedgerTrade{TransactionID: r.TransactionID, Revision: r.Revision, Attributes: attrs})
		if r.RecordedAt.After(newest) {
			newest = r.RecordedAt
		}
	}
	res := c.Ledgers().Replay(compute.LedgerInput{Alias: alias, Trades: trades, SessionDays: sessions, AsOf: day},
		compute.LedgerParams{MarkMaxAgeSessions: markMaxAgeSessions})
	return premiums(res.Positions), newest, nil
}

func (c Config) assemble(alias, label string, snap capture, ledgerAt time.Time, stocks []stockRow, options []optionRow,
	trades tradesTable, skipped int, at time.Time) pageJSON {
	v := viewJSON{
		Key: alias, Label: label, Trades: &trades,
		Sentence: sentenceStocks + strconv.Itoa(len(stocks)) + sentenceRows + strconv.Itoa(len(options)) + sentenceLegs,
		Stocks:   stocksTable{Title: titleStocks, Columns: columnsOf(stockHeads), Rows: stocks},
		Options:  optionsTable{Title: titleOptions, Columns: columnsOf(optionHeads), Rows: options},
	}
	if len(stocks) == 0 {
		v.Stocks.Gap = &gapJSON{Reason: "no_data", Text: textNoHeld}
	}
	if len(options) == 0 {
		v.Options.Gap = &gapJSON{Reason: "no_data", Text: textNoHeld}
	}
	if len(stocks)+len(options) == 0 {
		v.Sentence = sentenceNothing
	}
	p := c.base(alias, at)
	p.RunStatus = "partial"
	p.Views = []viewJSON{v}
	p.Gaps = []gapJSON{gapOf("no_data", titleStocks+", "+titleOptions+": "+textDayProfit)}
	if skipped > 0 {
		p.Notes = append(p.Notes, textSkippedStart+strconv.Itoa(skipped)+textSkippedEnd)
	}
	captured := at
	if len(snap.positions) > 0 && !snap.positions[0].CapturedAt.IsZero() {
		captured = snap.positions[0].CapturedAt.In(c.Zone)
	}
	p.DataTimes = []sourceTime{{Source: "account", DataTime: captured.Format(stampLayout)}}
	if !ledgerAt.IsZero() {
		p.DataTimes = append(p.DataTimes, sourceTime{Source: "option_ledger", DataTime: ledgerAt.In(c.Zone).Format(stampLayout)})
	}
	p.Stale = staleLabel{
		DataTime: captured.Format(stampLayout), StaleAfter: captured.AddDate(0, 0, staleAfterDays).Format(stampLayout),
		Text: textStaleData + snap.day,
	}
	return p
}

// base is the envelope with no view.
func (c Config) base(alias string, at time.Time) pageJSON {
	scope := alias
	if !scopeKey.MatchString(scope) {
		scope = "unknown"
	}
	return pageJSON{
		Schema: "positions_page", SchemaVersion: 3, RunID: c.RunID, BuildID: c.BuildID, Scope: scope,
		GeneratedAt: at.Format(stampLayout), DataTimes: []sourceTime{}, Notes: []string{}, Gaps: []gapJSON{},
		Title: pageTitle, Views: []viewJSON{},
		Stale: staleLabel{DataTime: at.Format(stampLayout), StaleAfter: at.Format(stampLayout), Text: textStaleData + "none"},
	}
}

// gapPage is the page of an alias that has nothing to show: one view whose tables and trades are the one gap.
func (c Config) gapPage(alias, label string, at time.Time, reason, text string) pageJSON {
	g := gapOf(reason, text)
	key := alias
	if key == "" {
		key = "unknown"
	}
	p := c.base(alias, at)
	p.RunStatus = "failed"
	p.Views = []viewJSON{{
		Key: key, Label: label, Sentence: text,
		Stocks:  stocksTable{Title: titleStocks, Columns: columnsOf(stockHeads), Rows: []stockRow{}, Gap: &g},
		Options: optionsTable{Title: titleOptions, Columns: columnsOf(optionHeads), Rows: []optionRow{}, Gap: &g},
		Trades:  &tradesTable{Title: titleTrades, Columns: columnsOf(tradeHeads), Rows: []tradeRow{}, Gap: &g},
	}}
	p.Gaps = []gapJSON{g}
	return p
}
