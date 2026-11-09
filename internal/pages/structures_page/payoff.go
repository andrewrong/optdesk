package structures_page

import (
	"encoding/json"

	"github.com/andrewrong/optdesk/internal/compute"
)

type payoffPoint struct {
	Price json.Number `json:"price"`
	PnL   json.Number `json:"pnl"`
}

type band struct {
	Low  json.Number `json:"low"`
	High json.Number `json:"high"`
	Text string      `json:"text"`
}

type payoffBlock struct {
	Points    []payoffPoint `json:"points"`
	PriceUnit string        `json:"price_unit"`
	PnLUnit   string        `json:"pnl_unit"`
	Spot      usd           `json:"spot"`
	BreakEven any           `json:"break_even"`
	MaxGain   any           `json:"max_gain"`
	MaxLoss   any           `json:"max_loss"`
	Band      band          `json:"band"`
	AriaText  string        `json:"aria_text"`
}

type greekFigure struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

type greekRow struct {
	Label string `json:"label"`
	Cells []any  `json:"cells"`
}

type greeksBlock struct {
	ModelText string     `json:"model_text"`
	Rows      []greekRow `json:"rows"`
}

// breakEvenCaption says which side of the break-even loses (or earns), by the kind of the structure.
func breakEvenCaption(s compute.Structure) string {
	if s.Figures.BreakEven.Gap != "" {
		return "无统一的盈亏平衡价"
	}
	switch s.Kind {
	case "short_put", "put_spread", "covered_call":
		return "到期价低于此价开始亏损"
	case "naked_call", "call_spread":
		return "到期价高于此价开始亏损"
	case "long_put":
		return "到期价低于此价开始盈利"
	case "long_call":
		return "到期价高于此价开始盈利"
	}
	return "逐腿显示, 无统一盈亏平衡价"
}

// spot is the underlying close of the newest quote of the underlying that has one, copied.
func (b *builder) spot() (usd, bool) {
	day, text := "", ""
	for _, q := range b.quotes {
		if q.Underlying == b.s.Underlying && q.UnderlyingClose != "" && string(q.Day) >= day {
			day, text = string(q.Day), q.UnderlyingClose
		}
	}
	n, ok := numberLiteral(text)
	return usd{Value: n, Text: text, Unit: "usd"}, ok
}

// payoff is the expiry payoff of compute with the break-even and the spot; when one of its numbers is a
// gap the block is that gap (the schema has no gap in its place).
func (b *builder) payoff() any {
	s := b.s
	if b.foreign() {
		return mkGap("unsupported_currency")
	}
	if len(s.Payoff) < 2 || len(s.Payoff) > 200 {
		if s.Kind == "other" {
			return mkGap(reasonNoRule)
		}
		return mkGap(reasonNoData)
	}
	be, gain, loss := b.money(s.Figures.BreakEven), b.money(s.Figures.MaxGain), b.money(s.Figures.MaxLoss)
	for _, v := range []any{be, gain, loss} {
		if g, ok := v.(gap); ok {
			return g
		}
	}
	spot, ok := b.spot()
	if !ok {
		return b.missingQuote()
	}
	pts := make([]payoffPoint, 0, len(s.Payoff))
	for _, p := range s.Payoff {
		price, okP := numberLiteral(p.Price)
		pnl, okL := numberLiteral(p.PnL)
		if !okP || !okL {
			return mkGap(reasonMissing)
		}
		pts = append(pts, payoffPoint{Price: price, PnL: pnl})
	}
	first, last := s.Payoff[0].Price, s.Payoff[len(s.Payoff)-1].Price
	return payoffBlock{
		Points: pts, PriceUnit: "usd", PnLUnit: "usd", Spot: spot, BreakEven: be, MaxGain: gain, MaxLoss: loss,
		// compute gives no one-standard-deviation band and optdesk solves no volatility: the band is the
		// range of the chart, and its text says so.
		Band: band{Low: pts[0].Price, High: pts[len(pts)-1].Price,
			Text: "图表价格范围 " + first + " 至 " + last + "，无波动率输入，不是一个标准差区间"},
		AriaText: clip("到期盈亏图，价格 " + first + " 至 " + last + "，盈亏平衡 " + s.Figures.BreakEven.Value + "，现价 " + spot.Text),
	}
}

func (b *builder) greekFig(f compute.FigureValue) any {
	if r := b.reasonOf(f); r != "" {
		return b.note(mkGap(r))
	}
	n, ok := numberLiteral(f.Value)
	if !ok {
		return b.note(mkGap(reasonMissing))
	}
	return greekFigure{Value: n, Text: f.Value, Unit: "greek"}
}

// quoteOfLeg is the newest quote of the contract of a leg, matched by terms.
func (b *builder) quoteOfLeg(l compute.StructureLeg) compute.StructureQuote {
	var out compute.StructureQuote
	found := false
	for _, q := range b.quotes {
		if q.Underlying == b.s.Underlying && q.Right == l.Right && q.Strike == l.Strike && q.Expiry == l.Expiry &&
			(!found || q.Day >= out.Day) {
			out, found = q, true
		}
	}
	return out
}

func quoteFigure(v string) compute.FigureValue {
	if v == "" {
		return compute.FigureValue{Gap: reasonQuote}
	}
	return compute.FigureValue{Value: v}
}

// greeks is the block of the IBKR model values: one row per leg (per share, as stored) and the total of
// compute (signed, with quantity and multiplier).
func (b *builder) greeks() any {
	s := b.s
	if b.noQuotes {
		return mkGap(reasonNoReader)
	}
	model := "IBKR 模型值，无报价日"
	if s.Greeks.Day != "" {
		model = "IBKR 模型值，取自 " + string(s.Greeks.Day)
	}
	rows := []greekRow{}
	for i, l := range s.Legs {
		if i == maxLegs {
			break
		}
		q := b.quoteOfLeg(l)
		rows = append(rows, greekRow{Label: clip(b.legLabel(l) + " 每股"), Cells: []any{
			b.greekFig(quoteFigure(q.Delta)), b.greekFig(quoteFigure(q.Gamma)),
			b.greekFig(quoteFigure(q.Theta)), b.greekFig(quoteFigure(q.Vega)),
		}})
	}
	g := s.Greeks
	rows = append(rows, greekRow{Label: "合计，含张数与乘数", Cells: []any{
		b.greekFig(g.Delta), b.greekFig(g.Gamma), b.greekFig(g.Theta), b.greekFig(g.Vega),
	}})
	if len(rows) < 2 {
		return mkGap(reasonNoData)
	}
	return greeksBlock{ModelText: model, Rows: rows}
}
