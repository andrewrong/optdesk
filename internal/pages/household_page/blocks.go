package household_page

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

// The JSON shapes of page.household_page.v1. Numbers are json.Number: the decimal text the figures
// carry, written as it is; nothing here parses, rounds or adds a figure.

type gapJSON struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type moneyCell struct {
	Value    json.Number `json:"value"`
	Text     string      `json:"text"`
	Unit     string      `json:"unit"`
	Currency string      `json:"currency"`
}

type moneyRow struct {
	Name   string       `json:"name"`
	Amount moneyCell    `json:"amount"`
	Note   string       `json:"note,omitempty"`
	Share  *shareFigure `json:"share,omitempty"`
}

type shareFigure struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

type moneyBlock struct {
	Title   string      `json:"title"`
	Meaning string      `json:"meaning"`
	Heads   []string    `json:"heads,omitempty"`
	Rows    []moneyRow  `json:"rows"`
	Totals  []moneyCell `json:"totals"`
	Gap     *gapJSON    `json:"gap,omitempty"`
}

type emptyRow struct{}

type allocationRow struct {
	Name   string    `json:"name"`
	Amount moneyCell `json:"amount"`
}

type allocationBlock struct {
	Title   string          `json:"title"`
	Meaning string          `json:"meaning"`
	Rows    []allocationRow `json:"rows"`
	Totals  []moneyCell     `json:"totals"`
	Gap     *gapJSON        `json:"gap,omitempty"`
}

type moversBlock struct {
	Title   string      `json:"title"`
	Meaning string      `json:"meaning"`
	Rows    []moneyRow  `json:"rows"`
	Totals  []moneyCell `json:"totals"`
	Stocks  []emptyRow  `json:"stocks"`
	Options []emptyRow  `json:"options"`
	Gap     *gapJSON    `json:"gap,omitempty"`
}

type usdFigure struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

type trendPoint struct {
	Day   string    `json:"day"`
	Value usdFigure `json:"value"`
}

type trendSeries struct {
	Scope  string       `json:"scope"`
	Label  string       `json:"label"`
	Rows   []trendPoint `json:"rows"`
	Window string       `json:"window,omitempty"`
}

type trendBlock struct {
	Title   string        `json:"title"`
	Meaning string        `json:"meaning"`
	Rows    []trendPoint  `json:"rows"`
	Series  []trendSeries `json:"series"`
	Window  string        `json:"window,omitempty"`
	Gap     *gapJSON      `json:"gap,omitempty"`
}

type waterfallStep struct {
	Name     string     `json:"name"`
	Kind     string     `json:"kind"`
	Change   *moneyCell `json:"change"`
	Level    moneyCell  `json:"level"`
	Sentence string     `json:"sentence"`
	Base     moneyCell  `json:"base"`
	Height   moneyCell  `json:"height"`
}

type waterfallBlock struct {
	Title   string          `json:"title"`
	Meaning string          `json:"meaning"`
	Rows    []waterfallStep `json:"rows"`
	Total   *moneyCell      `json:"total,omitempty"`
	Gap     *gapJSON        `json:"gap,omitempty"`
}

type accountFigure struct {
	Cell *moneyCell `json:"cell"`
	Gap  string     `json:"gap,omitempty"`
}

type accountRow struct {
	Name           string        `json:"name"`
	NetLiquidation accountFigure `json:"net_liquidation"`
	Cash           accountFigure `json:"cash"`
	Stock          accountFigure `json:"stock"`
	Options        accountFigure `json:"options"`
	Scope          string        `json:"scope"`
	Whose          string        `json:"whose"`
	DataDay        string        `json:"data_day,omitempty"`
}

type accountsBlock struct {
	Title   string       `json:"title"`
	Meaning string       `json:"meaning"`
	Columns []string     `json:"columns"`
	Rows    []accountRow `json:"rows"`
}

type accountTag struct {
	Label string `json:"label"`
	Whose string `json:"whose"`
}

type accountScope struct {
	Key      string       `json:"key"`
	Label    string       `json:"label"`
	Accounts []accountTag `json:"accounts"`
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

type pageJSON struct {
	Schema        string          `json:"schema"`
	SchemaVersion int             `json:"schema_version"`
	RunID         string          `json:"run_id"`
	BuildID       string          `json:"build_id"`
	Scope         string          `json:"scope"`
	GeneratedAt   string          `json:"generated_at"`
	DataTimes     []sourceTime    `json:"data_times"`
	RunStatus     string          `json:"run_status"`
	Title         string          `json:"title"`
	Sentence      string          `json:"sentence"`
	Overview      moneyBlock      `json:"overview"`
	Composition   moneyBlock      `json:"composition"`
	Movers        moversBlock     `json:"movers"`
	Trend         trendBlock      `json:"trend"`
	Stress        moneyBlock      `json:"stress"`
	Income        moneyBlock      `json:"income"`
	MonthChange   waterfallBlock  `json:"month_change"`
	CashAfter     moneyBlock      `json:"cash_after"`
	Obligations   moneyBlock      `json:"obligations"`
	Concentration moneyBlock      `json:"concentration"`
	Allocation    allocationBlock `json:"allocation"`
	LongTerm      moneyBlock      `json:"long_term"`
	Accounts      *accountsBlock  `json:"accounts"`
	Notes         []string        `json:"notes"`
	Gaps          []gapJSON       `json:"gaps"`
	Scopes        []accountScope  `json:"scopes"`
	Stale         staleLabel      `json:"stale"`
}

var decimalText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// cell turns an amount into a money cell; ok is false for a gap, a currency the schema has no
// place for, or text that is no decimal.
func cell(f blockapi.MoneyLine) (c moneyCell, ok bool) {
	if f.Amount.Gap != "" || !decimalText.MatchString(f.Amount.Value) {
		return c, false
	}
	return makeCell(f.Amount.Value, f.Currency)
}

func makeCell(value, currency string) (moneyCell, bool) {
	var sym string
	switch currency {
	case "USD":
		sym = "$"
	case "JPY":
		sym = "¥"
	default:
		return moneyCell{}, false
	}
	unit := "usd"
	if strings.HasPrefix(value, "-") {
		unit = "usd_signed"
	}
	return moneyCell{Value: json.Number(value), Text: moneyText(sym, value), Unit: unit, Currency: currency}, true
}

// moneyText writes decimal text as money: U+2212 for a negative, the symbol, the integer part in
// groups of three and the decimals as given.
func moneyText(sym, value string) string {
	neg := strings.HasPrefix(value, "-")
	whole, frac, hasFrac := strings.Cut(strings.TrimPrefix(value, "-"), ".")
	var b strings.Builder
	if neg {
		b.WriteString("\u2212")
	}
	b.WriteString(sym)
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

// lineGap is the reason a line has no cell.
func lineGap(f blockapi.MoneyLine) string {
	switch {
	case f.Amount.Gap != "":
		return f.Amount.Gap
	case !decimalText.MatchString(f.Amount.Value):
		return "missing_input"
	}
	return "unsupported_currency"
}

// moneyOf builds a block of money rows. A line without a cell is left out of the rows and named in
// the gap of the block, with the reason of the first such line.
func moneyOf(title, meaning string, lines []blockapi.MoneyLine) moneyBlock {
	b := moneyBlock{Title: title, Meaning: meaning, Rows: []moneyRow{}, Totals: []moneyCell{}}
	var names []string
	reason := ""
	for _, l := range lines {
		c, ok := cell(l)
		if !ok {
			if reason == "" {
				reason = lineGap(l)
			}
			names = append(names, l.Name)
			continue
		}
		b.Rows = append(b.Rows, moneyRow{Name: l.Name, Amount: c, Note: l.Note})
	}
	if reason != "" {
		b.Gap = &gapJSON{Reason: reason, Text: strings.Join(names, ", ") + ": " + gapText(reason)}
	}
	return b
}

func gapBlock(title, meaning string, g gapJSON) moneyBlock {
	return moneyBlock{Title: title, Meaning: meaning, Rows: []moneyRow{}, Totals: []moneyCell{}, Gap: &g}
}

func missingQuote() gapJSON { return gapJSON{Reason: "missing_quote", Text: textMissingQuote} }

// trendOf turns the net liquidation history into the trend block of the one scope.
func trendOf(alias, label string, days []blockapi.DayValue) trendBlock {
	b := trendBlock{Title: titleTrend, Meaning: meaningTrend, Rows: []trendPoint{}, Series: []trendSeries{}}
	for _, d := range days {
		if d.Currency != "USD" {
			b.Gap = &gapJSON{Reason: "unsupported_currency", Text: gapText("unsupported_currency")}
			b.Rows = []trendPoint{}
			return b
		}
		if !decimalText.MatchString(d.NetLiquidation) || strings.HasPrefix(d.NetLiquidation, "-") {
			continue
		}
		b.Rows = append(b.Rows, trendPoint{Day: string(d.Day), Value: usdFigure{
			Value: json.Number(d.NetLiquidation), Text: moneyText("$", d.NetLiquidation), Unit: "USD"}})
	}
	if len(b.Rows) == 0 {
		b.Gap = &gapJSON{Reason: "no_data", Text: gapText("no_data")}
		return b
	}
	b.Window = labelDay + strconv.Itoa(len(b.Rows)) + labelDays
	b.Series = []trendSeries{{Scope: alias, Label: label, Rows: b.Rows, Window: b.Window}}
	return b
}

// waterfallOf lists the month start and the month end. The steps between them need a running level
// that no line of the figures carries, and a builder adds nothing, so the block says so in its gap.
func waterfallOf(lines []blockapi.MoneyLine) waterfallBlock {
	b := waterfallBlock{Title: titleMonthChange, Meaning: meaningMonthChange, Rows: []waterfallStep{}}
	if len(lines) < 2 {
		reason := "missing_input"
		if len(lines) == 1 && lines[0].Amount.Gap != "" {
			reason = lines[0].Amount.Gap
		}
		b.Gap = &gapJSON{Reason: reason, Text: gapText(reason)}
		return b
	}
	first, last := lines[0], lines[len(lines)-1]
	start, ok1 := cell(first)
	end, ok2 := cell(last)
	if !ok1 || !ok2 {
		reason := lineGap(first)
		if ok1 {
			reason = lineGap(last)
		}
		b.Gap = &gapJSON{Reason: reason, Text: gapText(reason)}
		return b
	}
	zero, _ := makeCell("0", start.Currency)
	b.Rows = []waterfallStep{
		{Name: nameMonthStart, Kind: "start", Level: start, Sentence: nameMonthStart + " " + start.Text, Base: zero, Height: start},
		{Name: nameMonthEnd, Kind: "end", Level: end, Sentence: nameMonthEnd + " " + end.Text, Base: zero, Height: end},
	}
	b.Total = &end
	b.Gap = &gapJSON{Reason: "missing_quote", Text: textMonthSteps}
	return b
}

// accountsOf is the one row of the accounts block: net liquidation and cash from the summary, the
// stock and option cells are gaps (they need quotes).
func accountsOf(alias, label, whose, dataDay string, lines []blockapi.MoneyLine) *accountsBlock {
	b := &accountsBlock{Title: titleAccounts, Meaning: meaningAccounts, Columns: accountColumns, Rows: []accountRow{}}
	if len(lines) < 4 {
		return b
	}
	fig := func(l blockapi.MoneyLine) accountFigure {
		if c, ok := cell(l); ok {
			return accountFigure{Cell: &c}
		}
		return accountFigure{Gap: gapText(lineGap(l))}
	}
	b.Rows = append(b.Rows, accountRow{
		Name: label, NetLiquidation: fig(lines[0]), Cash: fig(lines[1]), Stock: fig(lines[2]), Options: fig(lines[3]),
		Scope: alias, Whose: whose, DataDay: dataDay,
	})
	return b
}
