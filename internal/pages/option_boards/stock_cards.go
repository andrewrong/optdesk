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
			YieldLine: fmt.Sprintf("%s · %d days", yt, c.DTE),
			Ladder:    []pair{{Label: "Bid", Text: c.BidEod}, {Label: "Mid", Text: gapText}, {Label: "Ask", Text: ask}},
			Advice:    "Advice: sell 1 put contract (close bid est., sold next day)",
			Amounts:   []string{fmt.Sprintf("Strike %s · Multiplier %d", c.Strike, c.Multiplier), "Breakeven " + c.BreakEven},
			Risk:      fmt.Sprintf("Risk: assigned if the stock is below %s", c.Strike),
			Liquidity: fmt.Sprintf("Liquidity: volume %s · open interest %s", r.Eod.Volume, r.Eod.OpenInterest),
		})
	}
	return cards, nil
}

// buildMaps is one price map per matrix expiry: the stored spot and the break-even and strike of its ok rows.
func buildMaps(rows []stockRow, expiries []matrixExpiry) ([]priceMap, error) {
	maps := make([]priceMap, 0, len(expiries))
	for _, e := range expiries {
		pm := priceMap{Expiry: e.Expiry, Heading: e.Text, Description: "Move range n/a (compute has none yet)", Marks: []mapMark{}, Bands: []mapBand{}}
		spotDone := false
		for _, r := range rows {
			c := r.C
			if string(c.Expiry) != e.Expiry {
				continue
			}
			if !spotDone && c.Spot != nil {
				if _, ok := numberLiteral(*c.Spot); ok {
					pm.Marks = append(pm.Marks, mapMark{Kind: "spot", Price: *c.Spot, Text: "Last price " + *c.Spot})
					spotDone = true
				}
			}
			if !r.ok() {
				continue
			}
			pm.Marks = append(pm.Marks,
				mapMark{Kind: "breakeven", Price: c.BreakEven, Text: "Breakeven " + c.BreakEven},
				mapMark{Kind: "strike", Price: c.Strike, Text: "Strike " + c.Strike})
		}
		maps = append(maps, pm)
	}
	return maps, nil
}
