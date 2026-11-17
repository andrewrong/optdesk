package option_pnl_page

import (
	"strings"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
)

// gap is a common.v2 Gap: a figure that cannot be shown. Never a zero.
type gap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

func codeGap(code string) gap {
	return gap{Kind: "gap", Reason: codeReasons[code], Text: codeTexts[code]}
}

// line is one labelled line of final text.
type line struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Text  string  `json:"text"`
	Hint  *string `json:"hint"`
	Tone  string  `json:"tone"`
}

func mkLine(id, label, text, hintKey string) line {
	l := line{ID: id, Label: label, Text: text, Tone: "none"}
	if h, ok := hints[hintKey]; ok {
		l.Hint = &h
	}
	return l
}

func (l line) tone(t string) line { l.Tone = t; return l }

// toneOf is good for a positive text, bad for a negative one, none otherwise.
func toneOf(text string) string {
	d, err := decimal.NewFromString(text)
	switch {
	case err != nil || d.Sign() == 0:
		return "none"
	case d.Sign() > 0:
		return "good"
	}
	return "bad"
}

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

// signedText is d rounded half away from zero to places, grouped, with prefix after the sign; a
// negative gets the typographic minus unless it rounds to zero.
func signedText(d decimal.Decimal, places int32, prefix, suffix string) string {
	r := d.Round(places)
	whole, frac, _ := strings.Cut(r.Abs().StringFixed(places), ".")
	s := prefix + groupThousands(whole)
	if frac != "" {
		s += "." + frac
	}
	s += suffix
	if r.IsNegative() {
		s = minusSign + s
	}
	return s
}

// money is the text with the currency symbol: $ for USD, otherwise the code and a space; two
// decimals half away from zero. An empty text is "not stored"; an unreadable one is shown as stored.
func money(text, currency string) string {
	if text == "" {
		return notStored
	}
	d, err := decimal.NewFromString(text)
	if err != nil {
		return text
	}
	prefix := "$"
	if currency != "USD" {
		prefix = currency + " "
	}
	return signedText(d, 2, prefix, "")
}

// percent is a stored fraction as a percent with one decimal; an empty text is "not stored".
func percent(text string) string {
	if text == "" {
		return notStored
	}
	d, err := decimal.NewFromString(text)
	if err != nil {
		return text
	}
	return signedText(d.Shift(2), 1, "", "%")
}

// fit cuts text to the 400 characters the schema allows.
func fit(s string) string {
	if utf8.RuneCountInString(s) <= 400 {
		return s
	}
	r := []rune(s)
	return string(r[:399]) + "…"
}

// figures are the amounts of a card or a book: from a Book (Combine of the lots), or from one result
// whose lot the Booker did not add (Total empty: no sum is shown).
type figures struct {
	Time, MoveCost, Direction, IVRemark, Unsplit, Entry, Exit, Commission string
	Total, RentMinusCost, AssignedIntrinsic, LedgerPnL, Difference        string
}

func figuresOfBook(b optionattrapi.Book) figures {
	return figures{b.Time, b.MoveCost, b.Direction, b.IVRemark, b.Unsplit, b.Entry, b.Exit, b.Commission,
		b.Total, b.RentMinusCost, b.AssignedIntrinsic, b.LedgerPnL, b.Difference}
}

func figuresOfResult(r optionattrapi.Result) figures {
	return figures{Time: r.Time, MoveCost: r.MoveCost, Direction: r.Direction, IVRemark: r.IVRemark, Unsplit: r.Unsplit,
		Entry: r.Entry, Exit: r.Exit, Commission: r.Commission, AssignedIntrinsic: r.AssignedIntrinsic,
		LedgerPnL: r.LedgerPnL, Difference: r.Difference}
}

// splitLines is the line group of the split, in the order of the sample: time, move cost, direction,
// IV re-marking, not split, entry and exit against the mid, commissions, total, assignment intrinsic.
// An empty Exit or Total gives no line; the ledger line is added by the caller.
func splitLines(prefix string, f figures, cur string, assigned bool) []line {
	out := []line{
		mkLine(prefix+"-time", "Time (theta)", money(f.Time, cur), "time").tone(toneOf(f.Time)),
		mkLine(prefix+"-move", "Move cost (gamma)", money(f.MoveCost, cur), "move_cost").tone(toneOf(f.MoveCost)),
		mkLine(prefix+"-dir", "Direction (delta)", money(f.Direction, cur), "direction").tone(toneOf(f.Direction)),
		mkLine(prefix+"-iv", "IV re-marking (vega)", money(f.IVRemark, cur), "iv_remark").tone(toneOf(f.IVRemark)),
		mkLine(prefix+"-unsplit", "Not split", money(f.Unsplit, cur), "unsplit"),
		mkLine(prefix+"-entry", "Entry vs mid", money(f.Entry, cur), "entry").tone(toneOf(f.Entry)),
	}
	if f.Exit != "" {
		out = append(out, mkLine(prefix+"-exit", "Exit vs mid", money(f.Exit, cur), "exit").tone(toneOf(f.Exit)))
	}
	out = append(out, mkLine(prefix+"-comm", "Commissions", money(f.Commission, cur), "").tone(toneOf(f.Commission)))
	if f.Total != "" {
		out = append(out, mkLine(prefix+"-total", "Total options P&L", money(f.Total, cur), "total").tone(toneOf(f.Total)))
	}
	if assigned {
		out = append(out, mkLine(prefix+"-assigned", "Assignment intrinsic", money(f.AssignedIntrinsic, cur), "assigned"))
	}
	return out
}

// ledgerLine is the "Ledger says" line. With an assignment the text is total + assignment intrinsic =
// ledger; for a book it names the three parts and the difference.
func ledgerLine(prefix string, f figures, cur string, assigned, book bool) line {
	text := money(f.LedgerPnL, cur)
	switch {
	case book:
		text += " = total + assignment intrinsic + difference (" + money(f.Difference, cur) + ")"
	case assigned && f.Total != "":
		text = "total " + money(f.Total, cur) + " + assignment intrinsic " + money(f.AssignedIntrinsic, cur) + " = " + text
	}
	l := mkLine(prefix+"-ledger", "Ledger says", text, "ledger")
	if assigned && !book {
		h := assignmentText
		l.Hint = &h
	}
	return l
}
