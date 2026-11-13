package household_page

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

const (
	secTypeStock = "stock"
	currencyJPY  = "JPY"
	pairUsdJpy   = "USDJPY"
	maxNamed     = 5 // symbols a gap text names before it ends "and <M> more"
)

// holdingBlocks are the three blocks built from the held stocks of the capture and their marks.
type holdingBlocks struct {
	composition, concentration moneyBlock
	allocation                 allocationBlock
}

// shareText is a share the schema accepts: 0 to 100, no sign.
var shareText = regexp.MustCompile(`^(100(\.0+)?|[0-9]{1,2}(\.[0-9]+)?)$`)

// held is what the capture day holds, read once: the stored positions, the price of each marked
// position by conid, and the stored USDJPY rate (empty without one) with its day.
type held struct {
	positions     []store.Position
	price         map[int64]string
	usdJpy, rateD string
}

// readHeld reads the positions once, the marks when Marks is set, and the USDJPY rate when a value,
// mark or position of the capture is JPY. A rate that is not found, a nil Fx and a rate read error
// leave the rate empty: the JPY rows then show the gap, never an error of Build.
func (c Config) readHeld(ctx context.Context, alias string, cp capture) (held, error) {
	h := held{price: map[int64]string{}}
	var err error
	if h.positions, err = c.Account.Positions(ctx, alias, cp.day); err != nil {
		return h, fmt.Errorf("household_page: read positions: %w", err)
	}
	jpy := false
	for _, v := range cp.values {
		jpy = jpy || v.Currency == currencyJPY
	}
	for _, p := range h.positions {
		jpy = jpy || p.Currency == currencyJPY
	}
	if c.Marks != nil {
		marks, err := c.Marks.Marks(ctx, alias, cp.day)
		if err != nil {
			return h, fmt.Errorf("household_page: read marks: %w", err)
		}
		for _, m := range marks {
			jpy = jpy || m.Currency == currencyJPY
			if m.Basis != mark.BasisNone && m.Price != "" {
				h.price[m.ConID] = m.Price
			}
		}
	}
	if jpy && c.Fx != nil {
		if r, found, err := c.Fx.LatestFxRate(ctx, pairUsdJpy, contracts.Day(cp.day)); err == nil && found {
			h.usdJpy, h.rateD = r.Rate, r.RateDay.Format(dayLayout)
		}
	}
	return h, nil
}

// positionsIn is the stored positions as the figures take them; an option is named with its right,
// strike and expiry.
func (h held) positionsIn() []blockapi.PositionIn {
	out := make([]blockapi.PositionIn, 0, len(h.positions))
	for _, p := range h.positions {
		mult := p.Multiplier
		if mult == "" {
			mult = "1"
		}
		name := p.Symbol
		if p.SecType == secTypeOption {
			name = strings.TrimSpace(p.Symbol + " " + strings.ToUpper(p.Right[:min(1, len(p.Right))]) + p.Strike + " " + p.Expiry)
		}
		out = append(out, blockapi.PositionIn{
			Underlying: name, SecType: p.SecType, Currency: p.Currency, Quantity: p.Quantity,
			Multiplier: mult, Mark: h.price[p.ConID],
		})
	}
	return out
}

// holdings builds the blocks from the held stocks; nil when Marks or Holdings is not set (the blocks
// stay missing_quote).
func (c Config) holdings(cp capture, h held) *holdingBlocks {
	if c.Marks == nil || c.Holdings == nil {
		return nil
	}
	in := blockapi.HoldingsIn{Rows: []blockapi.HoldingIn{}, UsdJpy: h.usdJpy}
	for _, v := range cp.values {
		if v.Tag == "NetLiquidation" {
			in.NetLiquidation, in.Currency = v.Value, v.Currency
		}
	}
	for _, p := range h.positions {
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
			Multiplier: mult, Cost: p.AvgCost, Mark: h.price[p.ConID],
		})
	}
	res := c.Holdings().Holdings(in)
	x := rowsCtx{net: in.Currency, day: cp.day}
	meaning := func(m string) string {
		for i, l := range res.Rows {
			if in.Rows[i].Currency == currencyJPY && l.Currency == "USD" && h.rateD != "" {
				return m + fmt.Sprintf(textRateDay, h.rateD)
			}
		}
		return m
	}
	return &holdingBlocks{
		composition:   holdingRows(titleComposition, meaning(meaningComposition), headsComposition, x, res),
		concentration: holdingRows(titleConcentration, meaning(meaningConcentration), headsConcentration, x, res),
		allocation:    allocationOf(meaning(meaningAllocation), x, res),
	}
}

// rowsCtx is what a holdings block needs besides the lines: the currency of the net value (the
// currency of a total) and the capture day (named in a rate gap).
type rowsCtx struct{ net, day string }

// named joins at most maxNamed symbols and ends "and <M> more" when more are left.
func named(names []string) string {
	if len(names) <= maxNamed {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:maxNamed], ", ") + " and " + strconv.Itoa(len(names)-maxNamed) + " more"
}

// rateGap is the text of the gap unsupported_currency of a holdings block: the symbols whose currency
// is not the net currency and the pair of the first (USDJPY for a JPY row).
func (x rowsCtx) rateGap(res blockapi.HoldingsResult) string {
	var names []string
	pair := ""
	for _, l := range res.Rows {
		if l.Currency == "" || l.Currency == x.net {
			continue
		}
		if pair == "" {
			pair = x.net + l.Currency
			if l.Currency == currencyJPY {
				pair = pairUsdJpy
			}
		}
		names = append(names, l.Underlying)
	}
	return fmt.Sprintf(textNoRate, named(names), pair, x.day)
}

func shareOf(f compute.FigureValue) *shareFigure {
	if f.Gap != "" || !shareText.MatchString(f.Value) {
		return nil
	}
	return &shareFigure{Value: json.Number(f.Value), Text: f.Value + "%", Unit: "share_percent"}
}

// holdingRows is one row per stock with its value and share, each cell labelled by the currency of
// its row. A stock without a value is left out and named in the gap of the block, with the reason of
// the first; a total that left one out is not shown.
func holdingRows(title, meaning string, heads []string, x rowsCtx, res blockapi.HoldingsResult) moneyBlock {
	b := moneyBlock{Title: title, Meaning: meaning, Heads: heads, Rows: []moneyRow{}, Totals: []moneyCell{}}
	var names []string
	reason := ""
	for _, l := range res.Rows {
		c, ok := cell(blockapi.MoneyLine{Currency: l.Currency, Amount: l.Value})
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
		text := gapText(reason)
		if reason == "unsupported_currency" {
			text = x.rateGap(res)
		} else {
			text = named(names) + ": " + text
		}
		b.Gap = &gapJSON{Reason: reason, Text: text}
		return b
	}
	if t, ok := cell(blockapi.MoneyLine{Currency: x.net, Amount: res.TotalValue}); ok {
		b.Totals = append(b.Totals, t)
	} else if len(res.Rows) > 0 {
		b.Gap = totalGap(res.TotalValue, x, res)
	}
	return b
}

// totalGap is the gap of a total: a rate gap names the symbols and the pair.
func totalGap(total compute.FigureValue, x rowsCtx, res blockapi.HoldingsResult) *gapJSON {
	reason := lineGap(blockapi.MoneyLine{Amount: total})
	if reason == "unsupported_currency" {
		return &gapJSON{Reason: reason, Text: x.rateGap(res)}
	}
	return &gapJSON{Reason: reason, Text: gapText(reason)}
}

// allocationOf is the one class held, stock, with the total value of the stocks.
func allocationOf(meaning string, x rowsCtx, res blockapi.HoldingsResult) allocationBlock {
	b := allocationBlock{Title: titleAllocation, Meaning: meaning, Rows: []allocationRow{}, Totals: []moneyCell{}}
	t, ok := cell(blockapi.MoneyLine{Currency: x.net, Amount: res.TotalValue})
	if !ok {
		if len(res.Rows) > 0 {
			g := totalGap(res.TotalValue, x, res)
			g.Text = nameStock + ": " + g.Text
			b.Gap = g
		}
		return b
	}
	b.Rows = append(b.Rows, allocationRow{Name: nameStock, Amount: t})
	b.Totals = append(b.Totals, t)
	return b
}
