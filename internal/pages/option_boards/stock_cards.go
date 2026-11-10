package option_boards

import "fmt"

// The schema limit of the cards.
const maxStockCards = 10

// buildCards is one card per ok row in the stored order, at most maxStockCards.
func buildCards(rows []stockRow, chosen int, cells []matrixCell) ([]chosenCard, error) {
	inMatrix := map[string]bool{}
	for _, c := range cells {
		inMatrix[c.Key] = true
	}
	cards := []chosenCard{}
	for i, r := range rows {
		if !r.ok() {
			continue
		}
		if len(cards) == maxStockCards {
			break
		}
		c := r.C
		md, err := monthDay(c.Expiry)
		if err != nil {
			return nil, err
		}
		yt, err := yieldText(c.AnnualizedYield)
		if err != nil {
			return nil, fmt.Errorf("option_boards: yield of instrument %d: %w", c.InstrumentID, err)
		}
		ask := gapText
		if c.Ask != nil {
			ask = *c.Ask
		}
		key := cellKey(c.InstrumentID)
		cards = append(cards, chosenCard{
			Key: key, Expiry: string(c.Expiry), Strike: c.Strike, Chosen: i == chosen, InMatrix: inMatrix[key],
			Title: fmt.Sprintf("put %s %s", c.Strike, md), PassTag: true,
			YieldLine: fmt.Sprintf("%s · %d 天", yt, c.DTE),
			Ladder:    []pair{{Label: "买价", Text: c.BidEod}, {Label: "中间价", Text: gapText}, {Label: "卖价", Text: ask}},
			Advice:    "建议 卖出 1 张 put (收盘 bid 估,次日才卖)",
			Amounts:   []string{fmt.Sprintf("行权价 %s · 乘数 %d", c.Strike, c.Multiplier), "盈亏平衡 " + c.BreakEven},
			Risk:      fmt.Sprintf("风险 股价低于 %s 会被行权", c.Strike),
			Liquidity: fmt.Sprintf("流动性 成交量 %s · 持仓 %s", r.Eod.Volume, r.Eod.OpenInterest),
		})
	}
	return cards, nil
}

// buildMaps is one price map per matrix expiry: the stored spot and the break-even and strike of its ok rows.
func buildMaps(rows []stockRow, expiries []matrixExpiry) ([]priceMap, error) {
	maps := make([]priceMap, 0, len(expiries))
	for _, e := range expiries {
		pm := priceMap{Expiry: e.Expiry, Heading: e.Text, Description: "波动区间 缺 (compute 尚无)", Marks: []mapMark{}, Bands: []mapBand{}}
		spotDone := false
		for _, r := range rows {
			c := r.C
			if string(c.Expiry) != e.Expiry {
				continue
			}
			if !spotDone && c.Spot != nil {
				if _, ok := numberLiteral(*c.Spot); ok {
					pm.Marks = append(pm.Marks, mapMark{Kind: "spot", Price: *c.Spot, Text: "现价 " + *c.Spot})
					spotDone = true
				}
			}
			if !r.ok() {
				continue
			}
			pm.Marks = append(pm.Marks,
				mapMark{Kind: "breakeven", Price: c.BreakEven, Text: "盈亏平衡 " + c.BreakEven},
				mapMark{Kind: "strike", Price: c.Strike, Text: "行权价 " + c.Strike})
		}
		maps = append(maps, pm)
	}
	return maps, nil
}
