package household_page

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

const (
	moversMaxGapDays = 5 // calendar days a mark day may lie before the page day (D1) or before D1 (D0)
	secTypeOptionRow = "option"
	netLiquidationTg = "NetLiquidation"
)

// moversPart is the movers block and the data times it adds to the page.
type moversPart struct {
	block moversBlock
	times []sourceTime
}

// markDay is one day that has a mark capture, with what that capture holds: the marks, the positions
// and the NetLiquidation of the same capture (a day with two captures never mixes them).
type markDay struct {
	day       string
	marks     []mark.PositionMark
	positions []store.Position
	net       blockapi.ValueIn
}

func noMovers(g gapJSON) moversPart { return moversPart{block: moversGap(g)} }

// movers builds the movers block from the two newest mark days on or before the page day. A nil Marks
// or Holdings keeps the block no_data; a reader error is an error of Build. sessions is ascending and
// ends on or before the page day; cp and h are the capture day the holdings blocks already read.
func (c Config) movers(ctx context.Context, alias string, tag accountTag, page time.Time,
	sessions []contracts.Day, cp capture, h held) (moversPart, error) {
	if c.Marks == nil || c.Holdings == nil {
		return noMovers(gapJSON{Reason: "no_data", Text: textNoMovers}), nil
	}
	var found []markDay
	for i := len(sessions) - 1; i >= 0 && len(found) < 2; i-- {
		rows, err := c.Marks.Marks(ctx, alias, string(sessions[i]))
		if err != nil {
			return moversPart{}, fmt.Errorf("household_page: read marks: %w", err)
		}
		if len(rows) > 0 {
			found = append(found, markDay{day: string(sessions[i]), marks: rows})
		}
	}
	if len(found) < 2 {
		return noMovers(gapJSON{Reason: "no_data", Text: textNoMovers}), nil
	}
	last, prev := found[0], found[1]
	if daysBetween(last.day, page) > moversMaxGapDays {
		return noMovers(gapJSON{Reason: "stale_input", Text: fmt.Sprintf(textMoversStaleLast, last.day)}), nil
	}
	if lastT, err := time.Parse(dayLayout, last.day); err != nil || daysBetween(prev.day, lastT) > moversMaxGapDays {
		return noMovers(gapJSON{Reason: "stale_input", Text: fmt.Sprintf(textMoversStalePrev, prev.day)}), nil
	}
	for _, d := range []*markDay{&last, &prev} {
		if err := c.readMarkDay(ctx, alias, d); err != nil {
			return moversPart{}, err
		}
	}
	var err error
	in := blockapi.MoversIn{
		Prev: contracts.Day(prev.day), Last: contracts.Day(last.day), NetPrev: prev.net, NetLast: last.net,
		Rows: moverRows(prev, last),
	}
	if in.UsdJpy, err = c.moversRate(ctx, in, cp, h); err != nil {
		return moversPart{}, err
	}
	res := c.Holdings().Movers(in)

	b := moversBlock{
		Title: titleMovers, Meaning: meaningMovers, Totals: []moneyCell{}, Rows: []moneyRow{},
		Columns: []string{headName, fmt.Sprintf(headPrevious, prev.day[5:]), fmt.Sprintf(headLast, last.day[5:]), headMoved},
		Stocks:  moverRowsOf(res.Stocks, alias, tag), Options: moverRowsOf(res.Options, alias, tag),
	}
	if g := moversTotalGap(res, in, last.day); g != nil {
		b.Gap = g
	} else {
		b.Rows = []moneyRow{
			{Name: nameMoversTotal, Amount: signedCell(res.Total.Value), Scope: alias},
			{Name: nameMoversOther, Amount: signedCell(res.Other.Value), Scope: alias},
		}
	}
	return moversPart{block: b, times: []sourceTime{
		{Source: "position_mark_prev", DataTime: newestMark(prev.marks).In(c.Zone).Format(timeLayout)},
		{Source: "position_mark_last", DataTime: newestMark(last.marks).In(c.Zone).Format(timeLayout)},
	}}, nil
}

// daysBetween is the whole calendar days from the day text to t; an unreadable day is far away.
func daysBetween(day string, t time.Time) int {
	d, err := time.Parse(dayLayout, day)
	if err != nil {
		return moversMaxGapDays + 1
	}
	return int(t.Sub(d).Hours() / 24)
}

func newestMark(rows []mark.PositionMark) time.Time {
	var newest time.Time
	for _, r := range rows {
		if r.MarkedAt.After(newest) {
			newest = r.MarkedAt
		}
	}
	return newest
}

// readMarkDay reads the positions and the NetLiquidation of the capture the marks of d belong to.
func (c Config) readMarkDay(ctx context.Context, alias string, d *markDay) error {
	capID := d.marks[0].CaptureID
	positions, err := c.Account.Positions(ctx, alias, d.day)
	if err != nil {
		return fmt.Errorf("household_page: read positions: %w", err)
	}
	for _, p := range positions {
		if p.CaptureID == capID {
			d.positions = append(d.positions, p)
		}
	}
	values, err := c.Account.Values(ctx, alias, d.day)
	if err != nil {
		return fmt.Errorf("household_page: read values: %w", err)
	}
	for _, v := range values {
		if v.CaptureID == capID && v.Tag == netLiquidationTg {
			d.net = blockapi.ValueIn{Tag: v.Tag, Value: v.Value, Currency: v.Currency}
			break
		}
	}
	return nil
}

// isZero is true for decimal text that is 0 (or empty).
func isZero(q string) bool { return strings.Trim(strings.TrimSpace(q), "+-0.") == "" }

// moverRows is one row per contract held on both days with q_prev not 0.
func moverRows(prev, last markDay) []blockapi.MoverIn {
	heldLast := map[int64]bool{}
	for _, p := range last.positions {
		if !isZero(p.Quantity) {
			heldLast[p.ConID] = true
		}
	}
	price := func(d markDay) map[int64]mark.PositionMark {
		out := map[int64]mark.PositionMark{}
		for _, m := range d.marks {
			out[m.ConID] = m
		}
		return out
	}
	m0, m1 := price(prev), price(last)
	rows := []blockapi.MoverIn{}
	for _, p := range prev.positions {
		if isZero(p.Quantity) || !heldLast[p.ConID] {
			continue
		}
		r := blockapi.MoverIn{
			Name: moverName(p), SecType: p.SecType, Right: p.Right, Strike: p.Strike, Expiry: p.Expiry,
			Currency: p.Currency, Quantity: p.Quantity, Multiplier: p.Multiplier,
		}
		r.PricePrev, r.BasisPrev = markPrice(m0[p.ConID])
		r.PriceLast, r.BasisLast = markPrice(m1[p.ConID])
		rows = append(rows, r)
	}
	return rows
}

// markPrice is the price and basis of a mark; a position without a mark or a basis none has no price.
func markPrice(m mark.PositionMark) (price, basis string) {
	if m.Basis == mark.BasisNone {
		return "", m.Basis
	}
	return m.Price, m.Basis
}

// moverName is the symbol of a stock; an option is named symbol, expiry month-day, right and strike.
func moverName(p store.Position) string {
	if p.SecType != secTypeOptionRow {
		return p.Symbol
	}
	strike := p.Strike
	if strings.Contains(strike, ".") {
		strike = strings.TrimSuffix(strings.TrimRight(strike, "0"), ".")
	}
	right := strings.ToUpper(p.Right[:min(1, len(p.Right))])
	expiry := p.Expiry
	if len(expiry) == len(dayLayout) {
		expiry = expiry[5:]
	}
	return strings.Join(strings.Fields(p.Symbol+" "+expiry+" "+right+strike), " ")
}

// holdsJPY is true when a row or a NetLiquidation of either day is in yen.
func holdsJPY(in blockapi.MoversIn) bool {
	jpy := in.NetPrev.Currency == currencyJPY || in.NetLast.Currency == currencyJPY
	for _, r := range in.Rows {
		jpy = jpy || r.Currency == currencyJPY
	}
	return jpy
}

// moversRate is the USDJPY rate of D1 when a row or a NetLiquidation of either day is in yen. The rate
// the holdings block already read is reused when D1 is the capture day; otherwise (or when that read
// found none) Fx is read here. Empty without a rate; every Fx read error is an error of Build.
func (c Config) moversRate(ctx context.Context, in blockapi.MoversIn, cp capture, h held) (string, error) {
	if !holdsJPY(in) {
		return "", nil
	}
	if string(in.Last) == cp.day && h.usdJpy != "" {
		return h.usdJpy, nil
	}
	if c.Fx == nil {
		return "", nil
	}
	r, found, err := c.Fx.LatestFxRate(ctx, pairUsdJpy, in.Last)
	if err != nil {
		return "", fmt.Errorf("household_page: read fx rate: %w", err)
	}
	if !found {
		return "", nil
	}
	return r.Rate, nil
}

// moversTotalGap is the block gap when the total and Other have no number, else nil.
func moversTotalGap(res blockapi.MoversResult, in blockapi.MoversIn, lastDay string) *gapJSON {
	g := res.Total.Gap
	if g == "" {
		g = res.Other.Gap
	}
	if g == "" {
		return nil
	}
	text := gapText(g)
	if g == "missing_input" && in.NetPrev.Value != "" && in.NetLast.Value != "" && holdsJPY(in) && in.UsdJpy == "" {
		text = fmt.Sprintf(textMoversNoRate, lastDay)
	}
	return &gapJSON{Reason: g, Text: text}
}

// signedCell is a USD change cell: always the signed unit.
func signedCell(value string) moneyCell {
	c, _ := makeCell(value, "USD")
	c.Unit = "usd_signed"
	return c
}

type priceCell struct {
	Value    json.Number `json:"value"`
	Text     string      `json:"text"`
	Currency string      `json:"currency"`
}

// priceOf is a price cell, or the gap that stands in its place.
func priceOf(f compute.FigureValue, currency string) any {
	if f.Gap != "" || !decimalText.MatchString(f.Value) {
		g := f.Gap
		if g == "" {
			g = "missing_input"
		}
		return gapJSON{Reason: g, Text: gapText(g)}
	}
	if currency != "USD" && currency != currencyJPY {
		return gapJSON{Reason: "unsupported_currency", Text: gapText("unsupported_currency")}
	}
	return priceCell{Value: json.Number(f.Value), Text: moneyText("", f.Value), Currency: currency}
}

// moverRowsOf copies the lines in the order the figure returns them.
func moverRowsOf(lines []blockapi.MoverLine, alias string, tag accountTag) []moverRow {
	out := make([]moverRow, 0, len(lines))
	for _, l := range lines {
		r := moverRow{
			Name: l.Name, Previous: priceOf(l.PricePrev, l.Currency), Current: priceOf(l.PriceLast, l.Currency),
			Note: l.Note, Scope: alias, Account: tag,
		}
		if l.Amount.Gap == "" && decimalText.MatchString(l.Amount.Value) {
			c := signedCell(l.Amount.Value)
			r.Amount = &c
		} else if l.Amount.Gap != "" && l.Amount.Gap != "missing_quote" {
			r.Note = strings.TrimSpace(r.Note + " (" + gapText(l.Amount.Gap) + ")")
		}
		out = append(out, r)
	}
	return out
}
