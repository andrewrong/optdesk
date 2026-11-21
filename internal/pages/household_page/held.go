package household_page

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// The held family page: one row per company the family holds. Every figure is copied from
// blockapi.HeldFigure; this file reads the book and lays the text out.

const (
	heldScope       = "held-family"
	earningsDays    = 120 // days of stored earnings rows read up to the page day
	heldSourceTime  = "account"
	heldTextNoRead  = "No reader: no data yet"
	heldTextNoEarn  = "No underlying or earnings reader: every next earnings date is a gap"
	heldValueNoMark = "gap: no mark"
	heldValueGap    = "gap: no value"
	heldShareGap    = "gap"
)

// heldEarningsTexts are the words of a cell with no date, by reason.
var heldEarningsTexts = map[string]string{
	"no_row":        "no date: the earnings job has no row for this company",
	"unsupported":   "no date: the earnings job does not support this company",
	"row_error":     "no date: the earnings row is an error",
	"no_reader":     "no date: no earnings reader",
	"no_underlying": "no date: no underlying row for this symbol",
	"no_issuer":     "no date: underlying row has no issuer",
	"past_date":     "no date: the stored date is already past",
}

// HeldPage is page.held_family_page.v1.
type HeldPage struct {
	Schema        string        `json:"schema"`
	SchemaVersion int           `json:"schema_version"`
	RunID         string        `json:"run_id"`
	BuildID       string        `json:"build_id"`
	Scope         string        `json:"scope"`
	GeneratedAt   string        `json:"generated_at"`
	DataTimes     []HeldTime    `json:"data_times"`
	RunStatus     string        `json:"run_status"`
	Stale         *HeldStale    `json:"stale"`
	Title         string        `json:"title"`
	Notes         []string      `json:"notes"`
	Gaps          []HeldGap     `json:"gaps"`
	Total         HeldTotal     `json:"total"`
	Columns       []HeldColumn  `json:"columns"`
	Rows          []HeldPageRow `json:"rows"`
	RowsSum       HeldCell      `json:"rows_sum"`
	LeadNotes     []string      `json:"lead_notes"`
}

// HeldTime is the capture time of one source.
type HeldTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

// HeldStale is the stale label of the common schema.
type HeldStale struct {
	DataTime string `json:"data_time"`
	Text     string `json:"text"`
	IsStale  bool   `json:"is_stale"`
}

// HeldGap is a named gap of the page.
type HeldGap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

// HeldTotal is the family total the shares are taken of.
type HeldTotal struct {
	Text         string `json:"text"`
	Currency     string `json:"currency"`
	Day          string `json:"day"`
	AccountsText string `json:"accounts_text"`
}

// HeldColumn is one column of the table.
type HeldColumn struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Hint  string `json:"hint"`
}

// HeldAccount is one holding account, named by its alias word.
type HeldAccount struct {
	Alias string `json:"alias"`
	Label string `json:"label"`
}

// HeldCell is a figure as text, or a gap with its reason.
type HeldCell struct {
	Kind   string `json:"kind"`
	Text   string `json:"text"`
	Reason string `json:"reason,omitempty"`
}

// HeldEarnings is the next earnings date and the days to it, or a gap with its reason.
type HeldEarnings struct {
	Kind   string `json:"kind"`
	Date   string `json:"date,omitempty"`
	DaysTo *int   `json:"days_to,omitempty"`
	Text   string `json:"text"`
	Reason string `json:"reason,omitempty"`
}

// HeldPageRow is one company.
type HeldPageRow struct {
	Company  string        `json:"company"`
	Accounts []HeldAccount `json:"accounts"`
	Value    HeldCell      `json:"value"`
	Share    HeldCell      `json:"share"`
	Earnings HeldEarnings  `json:"earnings"`
}

// heldColumns are the five columns: English labels, Chinese hints.
func heldColumns() []HeldColumn {
	return []HeldColumn{
		{ID: "company", Label: "Company", Hint: heldHintCompany},
		{ID: "accounts", Label: "Held in", Hint: heldHintAccounts},
		{ID: "value", Label: "Market value (USD)", Hint: heldHintValue},
		{ID: "share", Label: "Share of total", Hint: heldHintShare},
		{ID: "earnings", Label: "Next earnings", Hint: heldHintEarnings},
	}
}

// BuildHeld builds the held family page for day. Rows come from the accounts the family total counts; an
// account left out is a line of lead_notes. The first alias failing is an error, another alias failing is a
// note. A nil Account, Sessions, Family, Marks, Holdings or Held gives the gap page no_reader; a nil Underlying
// or Earnings gives every earnings cell the reason no_reader. generated_at is the start of day in Zone: the
// builder reads no clock.
func BuildHeld(ctx context.Context, c Config, day contracts.Day) (HeldPage, error) {
	if c.Zone == nil {
		return HeldPage{}, errors.New("household_page: no zone")
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return HeldPage{}, fmt.Errorf("household_page: bad day %q: %w", day, err)
	}
	p := heldBase(c, time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, c.Zone))
	if c.Account == nil || c.Sessions == nil || c.Family == nil || c.Marks == nil || c.Holdings == nil || c.Held == nil {
		p.RunStatus = "failed"
		p.Gaps = []HeldGap{{Kind: "gap", Reason: "no_reader", Text: heldTextNoRead}}
		p.Total = HeldTotal{Text: heldShareGap, Currency: currencyUSD, Day: string(day), AccountsText: heldShareGap}
		p.RowsSum = HeldCell{Kind: "gap", Text: heldShareGap, Reason: "no_reader"}
		return p, nil
	}
	distinct := distinctAliases(c.Aliases)
	sessions, err := c.sessionDays(ctx, d)
	if err != nil {
		return p, err
	}
	in, caps, err := c.readBook(ctx, distinct, sessions)
	if err != nil {
		return p, err
	}
	res := c.Family().Family(in)

	rate := c.lazyYenRate(ctx, res, caps)
	var lines []blockapi.HeldLineIn
	var notes []string
	for i, alias := range distinct {
		if !slices.Contains(res.Included, alias) {
			continue
		}
		got, err := c.heldLines(ctx, alias, caps[alias].day, rate)
		switch {
		case err != nil && i == 0:
			return p, err
		case err != nil:
			notes = append(notes, fmt.Sprintf(heldNoteReadFail, familyAccountLabel(alias)))
		default:
			lines = append(lines, got...)
		}
	}
	companies, err := c.heldCompanies(ctx, lines, day)
	if err != nil {
		return p, err
	}
	out := c.Held.HeldCompanies(blockapi.HeldIn{Lines: lines, Companies: companies, TotalUSD: res.Net, Today: day})
	c.assembleHeld(&p, res, caps, out, notes, day)
	return p, nil
}

func heldBase(c Config, at time.Time) HeldPage {
	return HeldPage{
		Schema: "held_family_page", SchemaVersion: 1, RunID: c.RunID, BuildID: c.BuildID, Scope: heldScope,
		GeneratedAt: at.Format(timeLayout), DataTimes: []HeldTime{}, RunStatus: "ok", Title: heldTitle,
		Notes: []string{heldIntro}, Gaps: []HeldGap{}, Columns: heldColumns(), Rows: []HeldPageRow{}, LeadNotes: []string{},
	}
}

// lazyYenRate gives the USD/JPY rate for the yen lines: the family rate when set, otherwise the stored rate at the
// latest capture day of the included accounts, as readBook reads it. It reads at most once and only when asked.
func (c Config) lazyYenRate(ctx context.Context, res blockapi.FamilyResult, caps map[string]capture) func() string {
	var rate string
	done := res.FxRate != ""
	if done {
		rate = res.FxRate
	}
	return func() string {
		if done {
			return rate
		}
		done = true
		latest := ""
		for _, a := range res.Included {
			latest = max(latest, caps[a].day)
		}
		if c.Fx == nil || latest == "" {
			return ""
		}
		if r, found, err := c.Fx.LatestFxRate(ctx, pairUsdJpy, contracts.Day(latest)); err == nil && found {
			rate = r.Rate
		}
		return rate
	}
}

// heldLines values the stock lines of one account at its capture day with the family rate, and names the
// mark day of each line. A line has a value only when its figure row is in USD; a yen row left in yen is a gap.
// rate is read only when a yen stock is held.
func (c Config) heldLines(ctx context.Context, alias, day string, rateOf func() string) ([]blockapi.HeldLineIn, error) {
	positions, err := c.Account.Positions(ctx, alias, day)
	if err != nil {
		return nil, fmt.Errorf("household_page: read positions: %w", err)
	}
	var stocks []store.Position
	for _, p := range positions {
		if p.SecType == secTypeStock && !isZeroQuantity(p.Quantity) {
			stocks = append(stocks, p)
		}
	}
	if len(stocks) == 0 {
		return nil, nil
	}
	price, markDay, err := c.stockMarks(ctx, alias, day, stocks)
	if err != nil {
		return nil, err
	}
	rate := ""
	for _, p := range stocks {
		if p.Currency == "JPY" {
			rate = rateOf()
			break
		}
	}
	in := blockapi.HoldingsIn{Currency: currencyUSD, UsdJpy: rate, Rows: make([]blockapi.HoldingIn, 0, len(stocks))}
	for _, p := range stocks {
		mult := p.Multiplier
		if mult == "" {
			mult = "1"
		}
		in.Rows = append(in.Rows, blockapi.HoldingIn{Underlying: p.Symbol, SecType: p.SecType, Currency: p.Currency,
			Quantity: p.Quantity, Multiplier: mult, Cost: "0", Mark: price[p.ConID]})
	}
	res := c.Holdings().Holdings(in)
	out := make([]blockapi.HeldLineIn, 0, len(stocks))
	for i, p := range stocks {
		v := res.Rows[i].Value
		if res.Rows[i].Currency != currencyUSD && v.Gap == "" {
			v = compute.FigureValue{Gap: "unsupported_currency"}
		}
		out = append(out, blockapi.HeldLineIn{Alias: alias, Underlying: p.Symbol, Currency: p.Currency,
			Value: v, MarkDay: contracts.Day(markDay[p.ConID])})
	}
	return out, nil
}

// isZeroQuantity is true for decimal text with no digit but zeros ("0", "0.0", "-0").
func isZeroQuantity(q string) bool {
	q = strings.TrimSpace(q)
	return q != "" && strings.Trim(q, "0.-+") == ""
}

// stockMarks prices the stocks from the marks of the capture day, then of each earlier calendar day up to
// markFallbackDays back; the newest mark of a position wins. It returns the price and the day of each con id.
func (c Config) stockMarks(ctx context.Context, alias, day string, stocks []store.Position) (price, markDay map[int64]string, err error) {
	price, markDay = map[int64]string{}, map[int64]string{}
	d, perr := time.Parse(dayLayout, day)
	if perr != nil {
		return price, markDay, nil
	}
	for i := 0; i <= markFallbackDays; i++ {
		if len(price) == len(stocks) {
			break
		}
		md := d.AddDate(0, 0, -i).Format(dayLayout)
		marks, err := c.Marks.Marks(ctx, alias, md)
		if err != nil {
			return nil, nil, fmt.Errorf("household_page: read marks: %w", err)
		}
		for _, m := range marks {
			if _, ok := price[m.ConID]; !ok && m.Basis != mark.BasisNone && m.Price != "" {
				price[m.ConID], markDay[m.ConID] = m.Price, md
			}
		}
	}
	return price, markDay, nil
}

// heldCompanies is one HeldCompanyIn per (symbol, currency) of the lines: its underlying row, its issuer, the
// owner's date and the stored earnings rows of the issuer for the 120 days up to day.
func (c Config) heldCompanies(ctx context.Context, lines []blockapi.HeldLineIn, day contracts.Day) ([]blockapi.HeldCompanyIn, error) {
	var rows []underlying.Row
	if c.Underlying != nil {
		var err error
		if rows, err = c.Underlying.Current(ctx); err != nil {
			return nil, fmt.Errorf("household_page: read underlyings: %w", err)
		}
	}
	d, _ := time.Parse(dayLayout, string(day))
	from := contracts.Day(d.AddDate(0, 0, -earningsDays).Format(dayLayout))
	read := c.Underlying != nil && c.Earnings != nil
	byIssuer := map[string][]blockapi.HeldEarningsRow{}
	var out []blockapi.HeldCompanyIn
	seen := map[[2]string]bool{}
	for _, l := range lines {
		key := [2]string{l.Underlying, l.Currency}
		if seen[key] {
			continue
		}
		seen[key] = true
		co := blockapi.HeldCompanyIn{Underlying: l.Underlying, Currency: l.Currency, Resolve: "ok", Earnings: blockapi.HeldEarningsIn{Read: read}}
		if c.Underlying != nil {
			row, found := underlyingOf(rows, l.Underlying, l.Currency)
			switch {
			case !found:
				co.Resolve = "no_underlying"
			case row.Issuer == "":
				co.Resolve = "no_issuer"
			default:
				co.Earnings.Override = contracts.Day(row.ReportDateOverride)
				if read {
					if _, ok := byIssuer[row.Issuer]; !ok {
						got, err := c.Earnings.EarningsDates(ctx, row.Issuer, from, day)
						if err != nil {
							return nil, fmt.Errorf("household_page: read earnings: %w", err)
						}
						byIssuer[row.Issuer] = earningsRows(got)
					}
					co.Earnings.Rows = byIssuer[row.Issuer]
				}
			}
		}
		out = append(out, co)
	}
	return out, nil
}

// underlyingOf is the row of a symbol, the one of the same currency first.
func underlyingOf(rows []underlying.Row, symbol, currency string) (underlying.Row, bool) {
	var first underlying.Row
	found := false
	for _, r := range rows {
		if r.Symbol != symbol {
			continue
		}
		if r.Currency == currency {
			return r, true
		}
		if !found {
			first, found = r, true
		}
	}
	return first, found
}

func earningsRows(rows []contracts.EarningsDate) []blockapi.HeldEarningsRow {
	out := make([]blockapi.HeldEarningsRow, 0, len(rows))
	for _, r := range rows {
		x := blockapi.HeldEarningsRow{Status: string(r.Status), ObservedOn: contracts.Day(r.ObservedOn.Format(dayLayout))}
		if r.ReportDate != nil {
			x.Report = contracts.Day(r.ReportDate.Format(dayLayout))
		}
		out = append(out, x)
	}
	return out
}

// assembleHeld writes the total, the rows, the notes and the gaps of p; every number and text of a row is copied
// from the figure result.
func (c Config) assembleHeld(p *HeldPage, res blockapi.FamilyResult, caps map[string]capture, out blockapi.HeldResult, notes []string, day contracts.Day) {
	dataDay := string(day)
	if len(res.Included) > 0 {
		dataDay = res.OldestDay
		for _, a := range res.Included {
			dataDay = max(dataDay, caps[a].day)
		}
	}
	p.Total = HeldTotal{Text: heldShareGap, Currency: currencyUSD, Day: dataDay, AccountsText: strings.Join(res.Included, " + ")}
	if p.Total.AccountsText == "" {
		p.Total.AccountsText = heldShareGap
	}
	if res.Net.Gap == "" && decimalText.MatchString(res.Net.Value) {
		p.Total.Text = moneyText("", res.Net.Value)
	}
	for _, alias := range res.Included {
		if t := capturedOf(caps[alias]); !t.IsZero() {
			p.DataTimes = append(p.DataTimes, HeldTime{Source: heldSourceTime, DataTime: t.In(c.Zone).Format(timeLayout)})
		}
	}

	old, noDate := heldRows(p, out, dataDay)
	p.RowsSum = heldCell(out.RowsSum, false)
	if p.RowsSum.Kind == "value" {
		p.RowsSum.Text += " USD"
	}

	p.LeadNotes = append(p.LeadNotes, heldNoteBasis, heldNoteShort, heldNoteOptions)
	for _, x := range res.Excluded {
		p.LeadNotes = append(p.LeadNotes, fmt.Sprintf(heldNoteLeftOut, familyAccountLabel(x.Alias), cellWords(x.Reason)))
		p.RunStatus = "partial"
	}
	p.LeadNotes = append(p.LeadNotes, notes...)
	if len(notes) > 0 {
		p.RunStatus = "partial"
	}
	if len(old) > 0 {
		p.LeadNotes = append(p.LeadNotes, fmt.Sprintf(heldNoteOldMark, len(old), strings.Join(old, ", ")))
	}
	if noDate > 0 {
		p.LeadNotes = append(p.LeadNotes, fmt.Sprintf(heldNoteNoDate, noDate))
	}
	if c.Underlying == nil || c.Earnings == nil {
		p.Gaps = append(p.Gaps, HeldGap{Kind: "gap", Reason: "no_reader", Text: heldTextNoEarn})
		p.RunStatus = "partial"
	}
}

// heldRows appends the rows of out to p and returns the companies of rows with an older mark and the count of rows without a date.
func heldRows(p *HeldPage, out blockapi.HeldResult, dataDay string) (old []string, noDate int) {
	for _, r := range out.Rows {
		row := HeldPageRow{Company: r.Underlying, Value: heldCell(r.Value, true), Share: heldShare(r.Share), Earnings: heldEarningsCell(r)}
		for _, a := range r.Aliases {
			row.Accounts = append(row.Accounts, HeldAccount{Alias: a, Label: a})
		}
		p.Rows = append(p.Rows, row)
		if r.OldestMark != "" && string(r.OldestMark) < dataDay {
			old = append(old, r.Underlying)
		}
		if r.EarnReason != "" {
			noDate++
		}
		if row.Value.Kind == "gap" {
			p.RunStatus = "partial"
		}
	}
	return old, noDate
}

// capturedOf is the capture time of the account values, zero when none.
func capturedOf(cp capture) time.Time {
	if len(cp.values) == 0 {
		return time.Time{}
	}
	return cp.values[0].CapturedAt
}

// heldCell is the cell of a figure: its text, or the words of its gap. forValue picks the words of a value cell.
func heldCell(f compute.FigureValue, forValue bool) HeldCell {
	if f.Gap == "" && decimalText.MatchString(f.Value) {
		return HeldCell{Kind: "value", Text: moneyText("", f.Value)}
	}
	reason := f.Gap
	if reason == "" {
		reason = "missing_input"
	}
	text := heldShareGap
	if forValue {
		text = heldValueGap
		if reason == "missing_quote" {
			text = heldValueNoMark
		}
	}
	return HeldCell{Kind: "gap", Text: text, Reason: reason}
}

// heldShare is the share cell: the percent with one decimal.
func heldShare(f compute.FigureValue) HeldCell {
	c := heldCell(f, false)
	if c.Kind == "value" {
		c.Text = strings.Replace(shareLabel(f.Value), "%", " %", 1)
	}
	return c
}

// heldEarningsCell is the date and the days to it, or the words of the reason.
func heldEarningsCell(r blockapi.HeldRow) HeldEarnings {
	if r.EarnReason != "" {
		text, ok := heldEarningsTexts[r.EarnReason]
		if !ok {
			text = heldEarningsTexts["no_row"]
		}
		return HeldEarnings{Kind: "gap", Text: text, Reason: r.EarnReason}
	}
	n := r.DaysTo
	words := fmt.Sprintf("%s (in %d days)", r.Next, n)
	switch n {
	case 0:
		words = string(r.Next) + " (today)"
	case 1:
		words = string(r.Next) + " (in 1 day)"
	}
	return HeldEarnings{Kind: "date", Date: string(r.Next), DaysTo: &n, Text: words}
}
