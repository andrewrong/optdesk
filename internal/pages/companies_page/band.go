package companies_page

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	dayLayout = "2006-01-02"

	bandName     = "valuation.buy_band"
	approvalName = "valuation.band_approval"

	// approvalValidDays is how long an approval holds, counted from its day.
	approvalValidDays = 90
	// approvalLookbackDays is how far before today the approval is looked for.
	approvalLookbackDays = 400

	// farAboveBand is the multiple of the band top over which the last price earns a mention in
	// the page notes: the band is a no-growth value, and a reader should not take a growth name's
	// distance from it for a broken figure. Trial value.
	farAboveBand = 3
	// farNamesMax is how many subjects the note names; the rest are "and N more".
	farNamesMax = 6
	// noteMax is the schema limit of a page note, in characters.
	noteMax = 200

	// The price_state values of page.companies_page.v2.
	stateBand     = "band"
	stateOverride = "owner override"
	stateAwaiting = "awaiting approval"
	stateExpired  = "expired"
)

// bandReady reports whether every reader the band needs is set.
func (c Config) bandReady() bool {
	return c.Read != nil && c.Facts != nil && c.StandardOf != nil && c.BuildFilings != nil && c.Band != nil
}

// priceCell builds the price line cell of one subject. The override wins; then the band as the
// compute functions give it, qualified by the owner's approval. The cell holds no formula: every
// figure in it is text the compute functions or the owner wrote. top is the 8% value of the band
// as the compute functions gave it, empty for an override or a subject with no band.
func (c Config) priceCell(ctx context.Context, s Subject, today contracts.Day) (out cell, top, reason string, err error) {
	out = cell{HandPrice: s.HandPrice}
	if c.Override != nil {
		line, ok, oerr := c.Override.Line(ctx, s.Symbol)
		if oerr != nil {
			return cell{}, "", "", fmt.Errorf("companies_page: override of %s: %w", s.Symbol, oerr)
		}
		if ok && line != "" {
			out.Text, out.PriceState = line, stateOverride
			return out, "", "", nil
		}
	}
	if !c.bandReady() {
		out.Text = noReaderText
		return out, "", "", nil
	}
	source, ok := factSource(s.Issuer)
	if !ok {
		out.Text = unvaluedCell
		return out, "", "no issuer id", nil
	}
	facts, ferr := c.Facts.FactsFiledBy(ctx, source, s.Issuer, today)
	if ferr != nil {
		return cell{}, "", "", fmt.Errorf("companies_page: facts of %s: %w", s.Symbol, ferr)
	}
	fr := c.BuildFilings(facts, today, c.StandardOf(facts))
	if fr.State != valuationapi.StateOK {
		out.Text = unvaluedCell
		return out, "", reasonText(fr.Reasons...), nil
	}
	band := c.Band(bandName, fr.Filings, today)
	if band.State != valuationapi.StateOK {
		out.Text = unvaluedCell
		return out, "", reasonText(band.Reasons...), nil
	}
	state, aerr := c.approvalState(ctx, s.Symbol, band.Value, today)
	if aerr != nil {
		return cell{}, "", "", aerr
	}
	out.PriceState = state
	figure := bandText(band)
	switch state {
	case stateBand:
		out.Text = figure
	case stateAwaiting:
		// The owner sees the figure he is asked to approve.
		out.Text = figure
	default:
		out.Text = state
	}
	return out, band.Value, "", nil
}

// approvalState is the price state of a band whose 8% value is value: the band while the latest
// approval is for that value and today is within 90 days of its day, expired after that,
// awaiting approval without such an approval.
func (c Config) approvalState(ctx context.Context, symbol, value string, today contracts.Day) (string, error) {
	t, err := time.Parse(dayLayout, string(today))
	if err != nil {
		return "", fmt.Errorf("companies_page: bad day %q: %w", today, err)
	}
	from := contracts.Day(t.AddDate(0, 0, -approvalLookbackDays).Format(dayLayout))
	rows, err := c.Read.WinnerReadings(ctx, approvalName, "underlying", symbol, from, today)
	if err != nil {
		return "", fmt.Errorf("companies_page: approval of %s: %w", symbol, err)
	}
	for _, r := range slices.Backward(rows) {

		if r.State != "ok" || r.Value == nil {
			continue
		}
		if *r.Value != value {
			return stateAwaiting, nil
		}
		if t.After(r.Day.AddDate(0, 0, approvalValidDays)) {
			return stateExpired, nil
		}
		return stateBand, nil
	}
	return stateAwaiting, nil
}

// factSource names the fact source of an issuer text: cik:<digits> is read from sec, edinet:<code>
// from edinet.
func factSource(issuer string) (string, bool) {
	switch {
	case strings.HasPrefix(issuer, "cik:"):
		return "sec", true
	case strings.HasPrefix(issuer, "edinet:"):
		return "edinet", true
	}
	return "", false
}

// unvaluedCell is the price cell of a company that cannot be valued; the reason is stated once, in
// the unvalued line of the page.
const unvaluedCell = "unvalued"

// currencySigns are the currencies the page writes a sign for.
var currencySigns = map[string]string{"USD": "$", "JPY": "¥"}

// bandText is the figure of a band: low and value to two decimals, half away from zero, with
// thousands separators and the currency sign. A currency with no sign, or a figure that is not a
// decimal, gives the figures as stored, so none is hidden.
func bandText(band valuationapi.BandFigure) string {
	stored := band.Low + " to " + band.Value
	sign, ok := currencySigns[band.Currency]
	if !ok {
		return stored
	}
	low, lerr := decimal.NewFromString(band.Low)
	value, verr := decimal.NewFromString(band.Value)
	if lerr != nil || verr != nil {
		return stored
	}
	return signed(sign, low) + " to " + signed(sign, value)
}

// lastText is the "Last" cell: the price with the sign of the market's currency, then the close
// day. A price that is not a decimal is written as stored.
func lastText(market string, q Spot) string {
	currency := "USD"
	if market == "JP" {
		currency = "JPY"
	}
	price := q.Price
	if d, err := decimal.NewFromString(q.Price); err == nil {
		price = signed(currencySigns[currency], d)
	}
	return price + " · close " + string(q.Day)
}

// signed writes d with the sign, thousands separators and two decimals, half away from zero.
func signed(sign string, d decimal.Decimal) string {
	text := d.Round(2).Abs().StringFixed(2)
	whole, frac, _ := strings.Cut(text, ".")
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	out := sign + b.String() + "." + frac
	if d.Round(2).IsNegative() {
		out = "-" + out
	}
	return out
}

// farAboveNotes is the page note, as a list of none or one, naming the subjects whose last price
// is over farAboveBand times the top of their band, built once for all of them: at most farNamesMax names, then "and N more",
// fewer names when the note would pass the schema limit of a note. It is empty when there are none.
func farAboveNotes(symbols []string) []string {
	n := min(len(symbols), farNamesMax)
	for ; n > 0; n-- {
		note := farAboveText(symbols, n)
		if utf8.RuneCountInString(note) <= noteMax {
			return []string{note}
		}
	}
	return nil
}

// farAboveText names the first n symbols and counts the rest.
func farAboveText(symbols []string, n int) string {
	more := ""
	if len(symbols) > n {
		more = fmt.Sprintf(" and %d more", len(symbols)-n)
	}
	return fmt.Sprintf("Last is over %d times the top of the price line for: %s%s. "+
		"The line is the price at an 8%% free-cash-flow yield on the 5-year median (after stock pay), a no-growth value.",
		farAboveBand, strings.Join(symbols[:n], ", "), more)
}

// isFarAbove reports whether last is more than farAboveBand times top. A figure that is not a
// decimal, or a top that is not positive, is not far above.
func isFarAbove(last, top string) bool {
	l, lerr := decimal.NewFromString(last)
	t, terr := decimal.NewFromString(top)
	if lerr != nil || terr != nil || !t.IsPositive() {
		return false
	}
	return l.GreaterThan(t.Mul(decimal.NewFromInt(farAboveBand)))
}
