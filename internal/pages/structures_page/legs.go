package structures_page

import (
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
)

type leg struct {
	SideText   string `json:"side_text"`
	RightText  string `json:"right_text"`
	Strike     usd    `json:"strike"`
	ExpiryText string `json:"expiry_text"`
	Quantity   int    `json:"quantity"`
	Price      usd    `json:"price"`
	Label      string `json:"label"`
	Mark       any    `json:"mark"`
	PnL        any    `json:"pnl"`
}

type fill struct {
	TimeText   string `json:"time_text"`
	ActionText string `json:"action_text"`
	LegText    string `json:"leg_text"`
	Quantity   int    `json:"quantity"`
	Price      usd    `json:"price"`
	Fee        any    `json:"fee"`
}

var actionTexts = map[string]string{
	"open": "Open", "close": "Close", "expire": "Expire", "assign": "Assigned", "exercise": "Exercise",
}

func rightText(r string) string {
	if r == "P" {
		return "Put"
	}
	if r == "C" {
		return "Call"
	}
	return "-"
}

// legLabel names a leg by its terms.
func (b *builder) legLabel(l compute.StructureLeg) string {
	return clip(strings.TrimSpace(b.s.Underlying + " " + l.Expiry + " " + l.Strike + " " + rightText(l.Right)))
}

// legs is the table of the legs: the mid and the P&L are copied from compute, a leg that cannot be
// shown is a gap row.
func (b *builder) legs() []any {
	out := []any{}
	for i, l := range b.s.Legs {
		if i == maxLegs {
			b.addGap(gap{Reason: reasonNoData, Text: "Showing only the first 8 legs"})
			break
		}
		out = append(out, b.note(b.legRow(l)))
	}
	if len(out) == 0 {
		g := mkGap(reasonNoData)
		b.addGap(g)
		out = append(out, g)
	}
	return out
}

func (b *builder) legRow(l compute.StructureLeg) any {
	if b.foreign() {
		return mkGap("unsupported_currency")
	}
	qty, okQ := intText(l.Quantity)
	strike, okS := usdCell(l.Strike)
	price, okP := usdCell(l.OpenPrice)
	if !okQ || !okS || !okP || l.Expiry == "" {
		return mkGap(reasonMissing)
	}
	side := "Buy"
	if qty < 0 {
		side = "Sell"
	}
	return leg{
		SideText: side, RightText: rightText(l.Right), Strike: strike,
		ExpiryText: l.Expiry, Quantity: qty, Price: price,
		Label: b.legLabel(l), Mark: b.note(b.legMoney(l, l.Mid)), PnL: b.note(b.legMoney(l, l.LegPnL)),
	}
}

// legMoney is a leg's mid or P&L as a Usd cell; a leg with no mark has the gap of a missing quote.
func (b *builder) legMoney(l compute.StructureLeg, text string) any {
	if l.MarkStatus != "ok" || text == "" {
		return b.missingQuote()
	}
	u, ok := usdCell(text)
	if !ok {
		return mkGap(reasonMissing)
	}
	return u
}

// fills lists the rows of the pieces of the legs, in the order of the legs and of the rows.
func (b *builder) fills() []fill {
	out := []fill{}
	if b.foreign() {
		return out
	}
	bad := false
	for _, l := range b.s.Legs {
		for _, f := range b.pos[l.PositionID].Fills {
			if len(out) == maxFills {
				b.addGap(gap{Reason: reasonNoData, Text: "Showing only the first 50 fills"})
				return out
			}
			row, ok := b.fillRow(l, f)
			if !ok {
				bad = true
				continue
			}
			out = append(out, row)
		}
	}
	if bad {
		b.addGap(gap{Reason: reasonMissing, Text: "Some fills cannot be shown"})
	}
	return out
}

func (b *builder) fillRow(l compute.StructureLeg, f compute.LedgerFill) (fill, bool) {
	qty, okQ := intText(f.Quantity)
	price, okP := usdCell(f.Price)
	if !okQ || !okP {
		return fill{}, false
	}
	action := actionTexts[f.Action]
	if action == "" {
		action = clip(f.Action)
	}
	if action == "" {
		action = "-"
	}
	day := string(f.Day)
	if day == "" {
		day = "Time unknown"
	}
	var fee any
	if u, ok := usdCell(f.Commission); ok {
		fee = u
	} else {
		fee = b.note(mkGap(reasonNoData))
	}
	return fill{TimeText: day, ActionText: action, LegText: b.legLabel(l), Quantity: qty,
		Price: price, Fee: fee}, true
}
