package positions_page

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	tradeDays    = 30  // session days of executions
	maxTrades    = 200 // rows the schema accepts; the newest are kept
	priorLookup  = 5   // session days looked back for the capture before a fill
	timeLayout   = "01-02 15:04"
	sideSold     = "SLD"
	kindOpen     = "open"
	kindClose    = "close"
	kindExpire   = "expire"
	kindExercise = "exercise"
)

type cell struct {
	Text string `json:"text"`
	Tone string `json:"tone"`
}

type tradeRow struct {
	Time       any `json:"time"`
	Account    any `json:"account"`
	Action     any `json:"action"`
	Instrument any `json:"instrument"`
	Quantity   any `json:"quantity"`
	Price      any `json:"price"`
}

type tradesTable struct {
	Title   string     `json:"title"`
	Columns []column   `json:"columns"`
	Rows    []tradeRow `json:"rows"`
	Gap     *gapJSON   `json:"gap"`
}

func plain(s string) any {
	if s == "" {
		return gapOf("missing_input", textBadNumber)
	}
	return cell{Text: s, Tone: "none"}
}

func contractOf(secType, symbol, expiry, strike, right string) string {
	return secType + "|" + symbol + "|" + expiry + "|" + strike + "|" + right
}

// priorQuantities returns the quantity of every contract in the newest capture before day d, looking
// back at most priorLookup session days; nil when there is none.
func (c Config) priorQuantities(ctx context.Context, alias string, sessions []contracts.Day, d string,
	cache map[string]map[string]string) (map[string]string, error) {
	idx := slices.Index(sessions, contracts.Day(d))
	for i := idx - 1; i >= 0 && i >= idx-priorLookup; i-- {
		day := string(sessions[i])
		held, ok := cache[day]
		if !ok {
			rows, err := c.Account.Positions(ctx, alias, day)
			if err != nil {
				return nil, fmt.Errorf("positions_page: read positions: %w", err)
			}
			if len(rows) > 0 {
				held = map[string]string{}
				for _, p := range rows {
					held[contractOf(p.SecType, p.Symbol, p.Expiry, p.Strike, p.Right)] = p.Quantity
				}
			}
			cache[day] = held
		}
		if held != nil {
			return held, nil
		}
	}
	return nil, nil
}

// trades is the table of the executions of the last tradeDays session days, newest first.
func (c Config) trades(ctx context.Context, alias, label string, sessions []contracts.Day, day contracts.Day) (tradesTable, error) {
	t := tradesTable{Title: titleTrades, Columns: columnsOf(tradeHeads), Rows: []tradeRow{}}
	from := sessions[max(0, len(sessions)-tradeDays)]
	execs, err := c.Account.Executions(ctx, alias, string(from), string(day))
	if err != nil {
		return t, fmt.Errorf("positions_page: read executions: %w", err)
	}
	if len(execs) == 0 {
		t.Gap = &gapJSON{Reason: "no_data", Text: textNoTrades}
		return t, nil
	}
	slices.SortStableFunc(execs, func(a, b store.Execution) int { return a.ExecutedAt.Compare(b.ExecutedAt) })
	cache := map[string]map[string]string{}
	seen := map[string]bool{}
	in := make([]blockapi.ExecIn, 0, len(execs))
	for _, e := range execs {
		x := blockapi.ExecIn{
			Symbol: e.Symbol, SecType: e.SecType, Right: e.Right, Strike: e.Strike, Expiry: e.Expiry,
			Side: e.Side, Quantity: e.Quantity, Price: e.Price, Currency: e.Currency,
		}
		if !e.ExecutedAt.IsZero() {
			x.Time = e.ExecutedAt.In(c.Zone).Format(timeLayout)
		}
		k := contractOf(e.SecType, e.Symbol, e.Expiry, e.Strike, e.Right)
		// Only the first fill of a contract on a day has a known prior; a later one would need the running quantity.
		if !seen[e.SessionDay+k] {
			held, err := c.priorQuantities(ctx, alias, sessions, e.SessionDay, cache)
			if err != nil {
				return t, err
			}
			if held != nil {
				x.PriorQuantity = "0"
				if q, ok := held[k]; ok {
					x.PriorQuantity = q
				}
			}
		}
		seen[e.SessionDay+k] = true
		in = append(in, x)
	}
	lines := c.Figures().Trades(day, in)
	for i := len(lines) - 1; i >= 0 && len(t.Rows) < maxTrades; i-- {
		t.Rows = append(t.Rows, tradeRowOf(label, lines[i], in[i]))
	}
	return t, nil
}

func tradeRowOf(label string, l blockapi.TradeLine, e blockapi.ExecIn) tradeRow {
	r := tradeRow{Account: plain(label), Instrument: plain(l.Instrument)}
	if l.Time == "" {
		r.Time = gapOf("flex_time_unknown", textNoTime)
	} else {
		r.Time = plain(l.Time)
	}
	r.Action = actionOf(l.Kind, e)
	r.Quantity = tradeQuantity(e)
	r.Price = tradePrice(e)
	return r
}

func actionOf(kind string, e blockapi.ExecIn) any {
	sold := strings.EqualFold(e.Side, sideSold)
	switch kind {
	case kindOpen:
		if sold {
			return plain(actionSellOpen)
		}
		return plain(actionBuyOpen)
	case kindClose:
		if sold {
			return plain(actionSellClose)
		}
		return plain(actionBuyClose)
	case kindExpire:
		return plain(actionExpire)
	case kindExercise:
		return plain(actionExercise)
	}
	text := textNoPrior
	if e.PriorQuantity == "" {
		text = textManyFills
	}
	return gapOf("missing_input", text)
}

func tradeQuantity(e blockapi.ExecIn) any {
	unit, word := "shares", unitShare
	if e.SecType == typeOption {
		unit, word = "contracts", unitContract
	}
	q, ok := quantity(strings.TrimPrefix(e.Quantity, "-"), unit)
	if !ok {
		return gapOf("missing_input", textBadNumber)
	}
	return cell{Text: q.Text + word, Tone: "none"}
}

func tradePrice(e blockapi.ExecIn) any {
	if !decimalText.MatchString(e.Price) {
		return gapOf("missing_input", textBadNumber)
	}
	sym, ok := symbolOf(e.Currency)
	if !ok {
		return plain(e.Price + " " + e.Currency + perShare)
	}
	return cell{Text: sym + e.Price + perShare, Tone: "none"}
}
