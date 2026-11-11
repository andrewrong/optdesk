package household_page

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

const secTypeStock = "stock"

// holdingBlocks are the three blocks built from the held stocks of the capture and their marks.
type holdingBlocks struct {
	composition, concentration moneyBlock
	allocation                 allocationBlock
}

// shareText is a share the schema accepts: 0 to 100, no sign.
var shareText = regexp.MustCompile(`^(100(\.0+)?|[0-9]{1,2}(\.[0-9]+)?)$`)

// holdings reads the positions and the marks of the capture day and builds the blocks; nil when
// Marks or Holdings is not set (the blocks stay missing_quote).
func (c Config) holdings(ctx context.Context, alias string, cp capture, positions []store.Position) (*holdingBlocks, error) {
	if c.Marks == nil || c.Holdings == nil {
		return nil, nil
	}
	marks, err := c.Marks.Marks(ctx, alias, cp.day)
	if err != nil {
		return nil, fmt.Errorf("household_page: read marks: %w", err)
	}
	price := map[int64]string{}
	for _, m := range marks {
		if m.Basis != mark.BasisNone && m.Price != "" {
			price[m.ConID] = m.Price
		}
	}
	in := blockapi.HoldingsIn{Rows: []blockapi.HoldingIn{}}
	for _, v := range cp.values {
		if v.Tag == "NetLiquidation" {
			in.NetLiquidation, in.Currency = v.Value, v.Currency
		}
	}
	for _, p := range positions {
		if p.SecType != secTypeStock {
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
	res := c.Holdings().Holdings(in)
	return &holdingBlocks{
		composition:   holdingRows(titleComposition, meaningComposition, headsComposition, in.Currency, res),
		concentration: holdingRows(titleConcentration, meaningConcentration, headsConcentration, in.Currency, res),
		allocation:    allocationOf(in.Currency, res),
	}, nil
}

func shareOf(f compute.FigureValue) *shareFigure {
	if f.Gap != "" || !shareText.MatchString(f.Value) {
		return nil
	}
	return &shareFigure{Value: json.Number(f.Value), Text: f.Value + "%", Unit: "share_percent"}
}

// holdingRows is one row per stock with its value and share. A stock without a value is left out
// and named in the gap of the block, with the reason of the first; a total that left one out is
// not shown.
func holdingRows(title, meaning string, heads []string, currency string, res blockapi.HoldingsResult) moneyBlock {
	b := moneyBlock{Title: title, Meaning: meaning, Heads: heads, Rows: []moneyRow{}, Totals: []moneyCell{}}
	var names []string
	reason := ""
	for _, l := range res.Rows {
		c, ok := cell(blockapi.MoneyLine{Currency: currency, Amount: l.Value})
		if !ok {
			if reason == "" {
				reason = lineGap(blockapi.MoneyLine{Amount: l.Value})
			}
			names = append(names, l.Underlying)
			continue
		}
		b.Rows = append(b.Rows, moneyRow{Name: l.Underlying, Amount: c, Share: shareOf(l.Share)})
	}
	if reason != "" {
		b.Gap = &gapJSON{Reason: reason, Text: strings.Join(names, ", ") + ": " + gapText(reason)}
		return b
	}
	if t, ok := cell(blockapi.MoneyLine{Currency: currency, Amount: res.TotalValue}); ok {
		b.Totals = append(b.Totals, t)
	} else if len(res.Rows) > 0 {
		reason = lineGap(blockapi.MoneyLine{Amount: res.TotalValue})
		b.Gap = &gapJSON{Reason: reason, Text: gapText(reason)}
	}
	return b
}

// allocationOf is the one class held, stock, with the total value of the stocks.
func allocationOf(currency string, res blockapi.HoldingsResult) allocationBlock {
	b := allocationBlock{Title: titleAllocation, Meaning: meaningAllocation, Rows: []allocationRow{}, Totals: []moneyCell{}}
	t, ok := cell(blockapi.MoneyLine{Currency: currency, Amount: res.TotalValue})
	if !ok {
		if len(res.Rows) > 0 {
			reason := lineGap(blockapi.MoneyLine{Amount: res.TotalValue})
			b.Gap = &gapJSON{Reason: reason, Text: nameStock + ": " + gapText(reason)}
		}
		return b
	}
	b.Rows = append(b.Rows, allocationRow{Name: nameStock, Amount: t})
	b.Totals = append(b.Totals, t)
	return b
}
