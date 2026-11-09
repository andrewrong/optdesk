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
	"open": "开仓", "close": "平仓", "expire": "到期", "assign": "被指派", "exercise": "行权",
}

func rightText(r string) string {
	if r == "P" {
		return "看跌"
	}
	if r == "C" {
		return "看涨"
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
			b.addGap(gap{Reason: reasonNoData, Text: "仅显示前 8 条腿"})
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
	strike, okS := numberLiteral(l.Strike)
	price, okP := numberLiteral(l.OpenPrice)
	if !okQ || !okS || !okP || l.Expiry == "" {
		return mkGap(reasonMissing)
	}
	side := "买入"
	if qty < 0 {
		side = "卖出"
	}
	return leg{
		SideText: side, RightText: rightText(l.Right), Strike: usd{Value: strike, Text: l.Strike, Unit: "usd"},
		ExpiryText: l.Expiry, Quantity: qty, Price: usd{Value: price, Text: l.OpenPrice, Unit: "usd"},
		Label: b.legLabel(l), Mark: b.note(b.legMoney(l, l.Mid)), PnL: b.note(b.legMoney(l, l.LegPnL)),
	}
}

// legMoney is a leg's mid or P&L as a Usd cell; a leg with no mark has the gap of a missing quote.
func (b *builder) legMoney(l compute.StructureLeg, text string) any {
	if l.MarkStatus != "ok" || text == "" {
		return b.missingQuote()
	}
	n, ok := numberLiteral(text)
	if !ok {
		return mkGap(reasonMissing)
	}
	return usd{Value: n, Text: text, Unit: "usd"}
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
				b.addGap(gap{Reason: reasonNoData, Text: "仅显示前 50 笔成交"})
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
		b.addGap(gap{Reason: reasonMissing, Text: "部分成交记录无法显示"})
	}
	return out
}

func (b *builder) fillRow(l compute.StructureLeg, f compute.LedgerFill) (fill, bool) {
	qty, okQ := intText(f.Quantity)
	price, okP := numberLiteral(f.Price)
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
		day = "时间未知"
	}
	var fee any
	if n, ok := numberLiteral(f.Commission); ok {
		fee = usd{Value: n, Text: f.Commission, Unit: "usd"}
	} else {
		fee = b.note(mkGap(reasonNoData))
	}
	return fill{TimeText: day, ActionText: action, LegText: b.legLabel(l), Quantity: qty,
		Price: usd{Value: price, Text: f.Price, Unit: "usd"}, Fee: fee}, true
}
