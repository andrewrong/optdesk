package option_tracking_page

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var numberPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)

// gap is a common.v2 Gap: a figure that cannot be shown. Never a zero.
type gap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

// codeGap is the gap of a tracking code: the reason is a common.v2 reason and the text is the
// plain sentence of the code, never the code word.
func codeGap(code string) gap {
	reason, ok := reasonOf[code]
	if !ok {
		reason = reasonInput
	}
	text, ok := codeTexts[code]
	if !ok {
		text = codeTexts[codeNoValue]
	}
	return gap{Kind: "gap", Reason: reason, Text: text}
}

// readerGap is the gap of a missing reader.
func readerGap() gap {
	g := codeGap(reasonNoReader)
	g.Reason = reasonNoReader
	return g
}

type valueCell struct {
	Text   string       `json:"text"`
	Value  *json.Number `json:"value"`
	Badges []string     `json:"badges"`
	Tone   string       `json:"tone"`
}

func textCell(text string) valueCell {
	return valueCell{Text: text, Badges: []string{}, Tone: "none"}
}

// numberLiteral is the stored decimal text as a JSON number literal; a leading plus is dropped.
func numberLiteral(text string) (json.Number, bool) {
	t := strings.TrimPrefix(text, "+")
	if !numberPattern.MatchString(t) {
		return "", false
	}
	return json.Number(t), true
}

// numValue shows the stored decimal text as it is, with the text as its value.
func numValue(text string) (valueCell, bool) {
	v, ok := numberLiteral(text)
	if !ok {
		return valueCell{}, false
	}
	c := textCell(text)
	c.Value = &v
	return c, true
}

// parseNumber is the value cell of the stored decimal text with the decimal it holds.
func parseNumber(text string) (valueCell, decimal.Decimal, bool) {
	c, ok := numValue(text)
	if !ok {
		return valueCell{}, decimal.Decimal{}, false
	}
	d, err := decimal.NewFromString(strings.TrimPrefix(text, "+"))
	if err != nil {
		return valueCell{}, decimal.Decimal{}, false
	}
	return c, d, true
}

// numCell is a numeric cell; the empty text is a gap with code, never a zero.
func numCell(text, code string) any {
	if text == "" {
		return codeGap(code)
	}
	c, ok := numValue(text)
	if !ok {
		return codeGap(codeUnreadable)
	}
	return c
}

// decimalCell is a numeric cell whose text is the stored decimal rounded half away from zero to places
// (moneyCell rounds half to even; the two differ on exact halves);
// the value stays the stored number. The empty text is the gap of code, an unreadable text the gap
// unreadable, never a zero.
func decimalCell(text, code string, places int32) any {
	if text == "" {
		return codeGap(code)
	}
	c, d, ok := parseNumber(text)
	if !ok {
		return codeGap(codeUnreadable)
	}
	c.Text = d.Round(places).StringFixed(places)
	return c
}

// currencySymbols are the symbols of the currencies a leg can hold.
var currencySymbols = map[string]string{"USD": "$", "JPY": "¥"}

// groupThousands puts a comma between groups of three digits of a whole-number text.
func groupThousands(whole string) string {
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// moneyCell is the premium text with the currency symbol, grouped thousands and two decimals; the
// value stays the stored number. An empty text is the gap no_value, a currency with no symbol or a
// text that is not a number is the gap unreadable, never a bare number.
func moneyCell(text, currency string) any {
	if text == "" {
		return codeGap(codeNoValue)
	}
	c, d, ok := parseNumber(text)
	sym, known := currencySymbols[currency]
	if !ok || !known {
		return codeGap(codeUnreadable)
	}
	neg := d.IsNegative()
	whole, frac, _ := strings.Cut(d.Abs().RoundBank(2).StringFixed(2), ".")
	c.Text = sym + groupThousands(whole) + "." + frac
	if neg && c.Text != sym+"0.00" {
		c.Text = "-" + c.Text
	}
	return c
}

// moneyHalfUp is moneyCell rounded half away from zero, with the typographic minus; the value stays
// the stored number. The empty text is the gap of code, an unreadable text or a currency with no
// symbol the gap unreadable.
func moneyHalfUp(text, currency, code string) any {
	if text == "" {
		return codeGap(code)
	}
	c, d, ok := parseNumber(text)
	sym, known := currencySymbols[currency]
	if !ok || !known {
		return codeGap(codeUnreadable)
	}
	c.Text = signedText(d, 2, sym)
	return c
}

// signedText is d rounded half away from zero to places, grouped, with prefix after the sign; a
// negative gets the typographic minus unless it rounds to zero.
func signedText(d decimal.Decimal, places int32, prefix string) string {
	r := d.Round(places)
	whole, frac, _ := strings.Cut(r.Abs().StringFixed(places), ".")
	s := prefix + groupThousands(whole)
	if frac != "" {
		s += "." + frac
	}
	if r.IsNegative() {
		s = "−" + s
	}
	return s
}

// deltaCell is the delta with two decimals and the typographic minus.
func deltaCell(text, code string) any {
	if text == "" {
		return codeGap(code)
	}
	c, d, ok := parseNumber(text)
	if !ok {
		return codeGap(codeUnreadable)
	}
	c.Text = signedText(d, 2, "")
	return c
}

// ivPercentCell is the stored IV fraction as a percent with one decimal, half away from zero.
func ivPercentCell(text, code string) any {
	if text == "" {
		return codeGap(code)
	}
	c, d, ok := parseNumber(text)
	if !ok {
		return codeGap(codeUnreadable)
	}
	c.Text = signedText(d.Mul(decimal.NewFromInt(100)), 1, "") + "%"
	return c
}

func plainCell(s string) any {
	if s == "" {
		return textCell(dash)
	}
	return textCell(s)
}

func dayCell(d contracts.Day) any { return plainCell(string(d)) }

// markCode is the cause of an empty figure that comes from the mark.
func markCode(o trackapi.TrackedOption) string {
	if o.MarkStatus == codeNoMark {
		return codeNoMark
	}
	return codeNoValue
}

func isPut(right string) bool {
	return strings.EqualFold(right, "P") || strings.EqualFold(right, "put")
}

func termsText(right, strike string, expiry contracts.Day) string {
	return right + termSep + strike + termSep + string(expiry)
}

func openContract(o trackapi.TrackedOption) any {
	return textCell(termsText(o.Right, o.Strike, contracts.Day(o.Expiry)))
}

func company(o trackapi.TrackedOption) any { return plainCell(o.Underlying) }

func qty(o trackapi.TrackedOption) any { return numCell(o.Quantity, codeNoValue) }

func dteCell(o trackapi.TrackedOption) any { return numCell(strconv.Itoa(o.DTE), codeNoValue) }

func moneynessCell(o trackapi.TrackedOption) any {
	badge, known := badgeMoneyness[o.Moneyness]
	if !known || o.OtmPercent == "" {
		return codeGap(markCode(o))
	}
	c, ok := numValue(o.OtmPercent)
	if !ok {
		return codeGap(codeUnreadable)
	}
	c.Text = o.OtmPercent + "%"
	c.Badges = []string{badge}
	return c
}

// eventGap is the gap of the event cells when the tracker found no event row.
func eventGap(o trackapi.TrackedOption) (gap, bool) {
	if o.EventStatus == "" || o.EventStatus == "ok" {
		return gap{}, false
	}
	return codeGap(o.EventStatus), true
}

func resultsCell(o trackapi.TrackedOption) any {
	if g, bad := eventGap(o); bad {
		return g
	}
	c := textCell(dash)
	if o.NextEarnings != "" {
		c.Text = string(o.NextEarnings)
	}
	if o.EarningsInLife {
		c.Badges = append(c.Badges, inLife)
	}
	return c
}

func exCell(o trackapi.TrackedOption) any {
	if isPut(o.Right) {
		return textCell(notApply)
	}
	if g, bad := eventGap(o); bad {
		return g
	}
	c := textCell(dash)
	if o.NextExDate != "" {
		c.Text = string(o.NextExDate)
		if o.ExAmount != "" {
			c.Text += termSep + o.ExAmount
		}
	}
	if o.ExInLife {
		c.Badges = append(c.Badges, inLife)
	}
	return c
}

// guardCell is the stored verdict as a badge and its reason once, with its basis.
func guardCell(o trackapi.TrackedOption) any {
	if o.Guard == "n/a" || (o.Guard == "" && isPut(o.Right)) {
		c := textCell(notApply)
		c.Badges = []string{notApply}
		return c
	}
	if o.Guard == "" || (o.Guard == "unknown" && o.GuardReason == codeNoGuardRow) {
		return codeGap(codeNoGuardRow)
	}
	badge, ok := badgeGuard[o.Guard]
	if !ok {
		badge = o.Guard
	}
	text := ""
	if o.GuardReason != "" {
		text = o.GuardReason
		if t, known := reasonTexts[o.GuardReason]; known {
			text = t
		}
	}
	if o.GuardBasis != "" {
		if text != "" {
			text += reasonSep
		}
		text += basisHead + o.GuardBasis
	}
	if text == "" {
		text = dash
	}
	c := textCell(text)
	c.Badges = []string{badge}
	if tone, ok := guardTone[o.Guard]; ok {
		c.Tone = tone
	}
	return c
}

func statusCell(o trackapi.TrackedOption) any {
	c := textCell(o.Status)
	c.Badges = []string{o.Status}
	if o.Early {
		c.Badges = append(c.Badges, earlyBadge)
	}
	return c
}

func closedDayCell(o trackapi.TrackedOption) any { return dayCell(o.ClosedOn) }

func reconContract(r trackapi.TrackRecon) any {
	return textCell(termsText(r.Right, r.Strike, r.Expiry))
}

func reconCompany(r trackapi.TrackRecon) any { return plainCell(r.Underlying) }

func reconQty(r trackapi.TrackRecon) any { return numCell(r.Quantity, codeNoValue) }

func reconCode(r trackapi.TrackRecon) any {
	c := textCell(r.Code)
	c.Badges = []string{r.Code}
	return c
}

func reconNote(r trackapi.TrackRecon) any { return plainCell(checkNotes[r.Code]) }
