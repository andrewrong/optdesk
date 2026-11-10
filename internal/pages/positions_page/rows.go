package positions_page

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	typeStock  = "stock"
	typeOption = "option"
	minus      = "−"
	maxInt     = 10 // digits of a quantity the schema accepts
)

var (
	decimalText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)
	wholeText   = regexp.MustCompile(`^(-?)0*([0-9]{1,10})(\.0*)?$`)
)

type gapJSON struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type textCell struct {
	Text  string `json:"text"`
	Value string `json:"value"`
}

type moneyCell struct {
	Value    json.Number `json:"value"`
	Text     string      `json:"text"`
	Unit     string      `json:"unit"`
	Currency string      `json:"currency"`
	Tone     string      `json:"tone,omitempty"`
}

type quantityCell struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

type daysCell struct {
	Value int    `json:"value"`
	Text  string `json:"text"`
	Unit  string `json:"unit"`
}

type stockRow struct {
	Code       textCell     `json:"code"`
	Name       textCell     `json:"name"`
	Role       textCell     `json:"role"`
	Quantity   quantityCell `json:"quantity"`
	Cost       any          `json:"cost"`
	Price      any          `json:"price"`
	Day        any          `json:"day"`
	Unrealised any          `json:"unrealised"`
}

type optionRow struct {
	Code        textCell     `json:"code"`
	Name        textCell     `json:"name"`
	Expiry      any          `json:"expiry"`
	DaysLeft    any          `json:"days_left"`
	Role        textCell     `json:"role"`
	Quantity    quantityCell `json:"quantity"`
	Cost        any          `json:"cost"`
	Price       any          `json:"price"`
	Day         any          `json:"day"`
	Unrealised  any          `json:"unrealised"`
	OpenPremium any          `json:"open_premium"`
}

type column struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Align string `json:"align"`
}

func columnsOf(heads []string) []column {
	out := make([]column, 0, len(heads))
	for _, h := range heads {
		id, label, _ := strings.Cut(h, ":")
		a := "left"
		if rightAligned[id] {
			a = "right"
		}
		out = append(out, column{ID: id, Label: label, Align: a})
	}
	return out
}

func gapOf(reason, text string) gapJSON { return gapJSON{Reason: reason, Text: text} }

func text2(s string) textCell { return textCell{Text: s, Value: s} }

// money2 is a decimal text with exactly two places (the schema's multipleOf 0.01), without a sign for
// zero; false when the text is not a decimal or has a non-zero digit beyond two places.
func money2(s string) (string, bool) {
	if !decimalText.MatchString(s) {
		return "", false
	}
	neg := strings.HasPrefix(s, "-")
	whole, frac, _ := strings.Cut(strings.TrimPrefix(s, "-"), ".")
	frac = strings.TrimRight(frac, "0")
	if len(frac) > 2 {
		return "", false
	}
	frac += strings.Repeat("0", 2-len(frac))
	whole = strings.TrimLeft(whole, "0")
	if whole == "" {
		whole = "0"
	}
	if whole == "0" && frac == "00" {
		neg = false
	}
	if neg {
		return "-" + whole + "." + frac, true
	}
	return whole + "." + frac, true
}

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

func symbolOf(currency string) (string, bool) {
	switch currency {
	case "USD":
		return "$", true
	case "JPY":
		return "¥", true
	}
	return "", false
}

// money builds a money cell from decimal text, or the gap that stops it. unit is usd, usd_per_share or
// usd_signed; the signed unit carries a sign in the text and the tone of the sign.
func money(s, currency, unit string) any {
	sym, ok := symbolOf(currency)
	if !ok {
		return gapOf("unsupported_currency", textCurrency)
	}
	if !decimalText.MatchString(s) {
		return gapOf("missing_input", textBadNumber)
	}
	v, ok := money2(s)
	if !ok {
		return gapOf("missing_input", textPrecision)
	}
	neg := strings.HasPrefix(v, "-")
	whole, frac, _ := strings.Cut(strings.TrimPrefix(v, "-"), ".")
	body := sym + groupThousands(whole) + "." + frac
	c := moneyCell{Value: json.Number(v), Unit: unit, Currency: currency, Tone: "none"}
	zero := v == "0.00"
	switch {
	case neg:
		c.Text = minus + body
	case unit == "usd_signed" && !zero:
		c.Text = "+" + body
	default:
		c.Text = body
	}
	if unit == "usd_signed" {
		switch {
		case zero:
			c.Tone = "flat"
		case neg:
			c.Tone = "down"
		default:
			c.Tone = "up"
		}
	}
	return c
}

func figure(f compute.FigureValue, currency, unit string) any {
	if f.Gap != "" {
		return gapOf(f.Gap, gapText(f.Gap))
	}
	return money(f.Value, currency, unit)
}

// quantity is the whole-number cell of a position; false for a fractional or oversized quantity.
func quantity(s, unit string) (quantityCell, bool) {
	m := wholeText.FindStringSubmatch(s)
	if m == nil {
		return quantityCell{}, false
	}
	digits := m[2]
	neg := m[1] == "-" && strings.Trim(digits, "0") != ""
	v, txt := digits, digits
	if neg {
		v, txt = "-"+digits, minus+digits
	}
	return quantityCell{Value: json.Number(v), Text: txt, Unit: unit}, true
}

// strikeKey is a strike text without trailing zeros, so that 100, 100.0 and 100.00 meet.
func strikeKey(s string) string {
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}

func rightLetter(right string) string {
	switch right {
	case "put":
		return "P"
	case "call":
		return "C"
	}
	return ""
}

func contractKey(underlying, right, expiry, strike string) string {
	return underlying + "|" + right + "|" + expiry + "|" + strikeKey(strike)
}

// premiums maps each open ledger contract to its remaining premium; a contract with two open
// positions maps to the empty string, which the row shows as a gap.
func premiums(ps []compute.LedgerPosition) map[string]string {
	out := map[string]string{}
	for _, p := range ps {
		if p.Status != "open" {
			continue
		}
		k := contractKey(p.Underlying, p.Right, p.Expiry, p.Strike)
		if _, dup := out[k]; dup {
			out[k] = ""
			continue
		}
		out[k] = p.PremiumIncome
	}
	return out
}

func optionName(p store.Position) string {
	sell := strings.HasPrefix(strings.TrimSpace(p.Quantity), "-")
	var w string
	switch {
	case p.Right == "put" && sell:
		w = sellPut
	case p.Right == "put":
		w = buyPut
	case sell:
		w = sellCall
	default:
		w = buyCall
	}
	return p.Symbol + " " + w
}

func optionCode(p store.Position) string {
	expiry := p.Expiry
	if len(expiry) == len("2006-01-02") {
		expiry = expiry[5:]
	}
	return p.Symbol + " " + expiry + " " + rightLetter(p.Right) + strikeKey(p.Strike)
}

// daysLeft is the cell of whole days from day to the expiry.
func daysLeft(expiry string, day contracts.Day) any {
	e, err1 := time.Parse("2006-01-02", expiry)
	d, err2 := time.Parse("2006-01-02", string(day))
	if err1 != nil || err2 != nil {
		return gapOf("missing_input", textBadExpiry)
	}
	n := int(e.Sub(d) / (24 * time.Hour))
	return daysCell{Value: n, Text: strconv.Itoa(n), Unit: "days"}
}

// holdingsOf is the input of Holdings: one row per stock or option position, in order, and the index
// of each position in it.
func holdingsOf(ps []store.Position, price map[int64]string) blockapi.HoldingsIn {
	in := blockapi.HoldingsIn{Rows: []blockapi.HoldingIn{}}
	for _, p := range ps {
		if p.SecType != typeStock && p.SecType != typeOption {
			continue
		}
		if in.Currency == "" {
			in.Currency = p.Currency
		}
		mult := p.Multiplier
		if mult == "" {
			mult = "1"
		}
		in.Rows = append(in.Rows, blockapi.HoldingIn{
			Underlying: p.Symbol, SecType: p.SecType, Currency: p.Currency, Quantity: p.Quantity,
			Multiplier: mult, Cost: p.AvgCost, Mark: price[p.ConID],
		})
	}
	return in
}

// rowsOf builds the stock and option rows; lines is the result of Holdings over holdingsOf(ps).
// Positions whose quantity is not a whole number are counted in skipped and not listed.
func rowsOf(ps []store.Position, price map[int64]string, lines []blockapi.HoldingLine, prem map[string]string,
	role textCell, day contracts.Day) (stocks []stockRow, options []optionRow, skipped int) {
	stocks, options = []stockRow{}, []optionRow{}
	i := 0
	for _, p := range ps {
		if p.SecType != typeStock && p.SecType != typeOption {
			continue
		}
		line := lines[i]
		i++
		dayGap := gapOf("no_data", textDayProfit)
		cost := money(p.AvgCost, p.Currency, "usd_per_share")
		var pr any = gapOf("missing_quote", textNoMark)
		if m := price[p.ConID]; m != "" {
			pr = money(m, p.Currency, "usd_per_share")
		}
		unreal := figure(line.Unrealised, p.Currency, "usd_signed")
		if p.SecType == typeStock {
			q, ok := quantity(p.Quantity, "shares")
			if !ok {
				skipped++
				continue
			}
			stocks = append(stocks, stockRow{
				Code: text2(p.Symbol), Name: text2(p.Symbol), Role: role, Quantity: q,
				Cost: cost, Price: pr, Day: dayGap, Unrealised: unreal,
			})
			continue
		}
		q, ok := quantity(p.Quantity, "contracts")
		if !ok {
			skipped++
			continue
		}
		var expiry any = gapOf("missing_input", textBadExpiry)
		if p.Expiry != "" {
			expiry = text2(p.Expiry)
		}
		var open any = gapOf("no_data", textNoLedger)
		if v, found := prem[contractKey(p.Symbol, rightLetter(p.Right), p.Expiry, p.Strike)]; found {
			if v == "" {
				open = gapOf("missing_input", textAmbiguous)
			} else {
				open = money(v, p.Currency, "usd")
			}
		}
		options = append(options, optionRow{
			Code: text2(optionCode(p)), Name: text2(optionName(p)), Expiry: expiry, DaysLeft: daysLeft(p.Expiry, day),
			Role: role, Quantity: q, Cost: cost, Price: pr, Day: dayGap, Unrealised: unreal, OpenPremium: open,
		})
	}
	return stocks, options, skipped
}
