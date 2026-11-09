package report_blocks

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type tradeRow struct {
	Time       any `json:"time"`
	Account    any `json:"account"`
	Action     any `json:"action"`
	Instrument any `json:"instrument"`
	Quantity   any `json:"quantity"`
	Price      any `json:"price"`
}

type tradesJSON struct {
	Columns []string   `json:"columns"`
	Rows    []tradeRow `json:"rows"`
}

type actionsBlock struct {
	head
	Trades     any `json:"trades"`
	Candidates any `json:"candidates"`
}

// Actions is the actions block: the day's executions of the main alias, each with the kind compute
// gives it from the signed quantity of the contract in the newest earlier capture; candidates is null.
// No execution is state ok with no rows. A fill that is not the first of its contract that day, or
// that has no earlier capture, has no kind and shows the Gap missing_input in its action cell.
func Actions(c Config) BlockFunc {
	return func(ctx context.Context, day contracts.Day, generatedAt time.Time) ([]byte, error) {
		if err := c.ready(); err != nil {
			return nil, err
		}
		d, err := parseDay(day, generatedAt)
		if err != nil {
			return nil, err
		}
		if c.Account == nil || c.Sessions == nil || c.Figures == nil || len(c.Aliases) == 0 {
			return marshal(actionsBlock{head: gapHead(keyActions, titleActions, subActions, sayActionsGap, "no_reader"),
				Trades: gapOf("no_reader")})
		}
		alias := c.Aliases[0]
		execs, err := c.Account.Executions(ctx, alias, string(day), string(day))
		if err != nil {
			return nil, fmt.Errorf("report_blocks: read executions: %w", err)
		}
		prior, err := c.priorPositions(ctx, alias, d)
		if err != nil {
			return nil, err
		}
		return marshal(c.actionsBlock(alias, day, execs, prior))
	}
}

// priorPositions is the positions of the newest session day before d that has any, and whether one exists.
func (c Config) priorPositions(ctx context.Context, alias string, d time.Time) (*[]store.Position, error) {
	sessions, err := c.sessionDays(ctx, d.AddDate(0, 0, -1))
	if err != nil {
		return nil, err
	}
	for i := len(sessions) - 1; i >= 0; i-- {
		rows, err := c.Account.Positions(ctx, alias, string(sessions[i]))
		if err != nil {
			return nil, fmt.Errorf("report_blocks: read positions: %w", err)
		}
		if len(rows) > 0 {
			return &rows, nil
		}
	}
	return nil, nil
}

func contractKey(secType, symbol, expiry, strike, right string) string {
	return secType + "|" + symbol + "|" + expiry + "|" + strike + "|" + right
}

func (c Config) actionsBlock(alias string, day contracts.Day, execs []store.Execution, prior *[]store.Position) actionsBlock {
	slices.SortStableFunc(execs, func(a, b store.Execution) int { return a.ExecutedAt.Compare(b.ExecutedAt) })
	held := map[string]string{}
	if prior != nil {
		for _, p := range *prior {
			held[contractKey(p.SecType, p.Symbol, p.Expiry, p.Strike, p.Right)] = p.Quantity
		}
	}
	seen := map[string]bool{}
	in := make([]blockapi.ExecIn, 0, len(execs))
	for _, e := range execs {
		k := contractKey(e.SecType, e.Symbol, e.Expiry, e.Strike, e.Right)
		x := blockapi.ExecIn{
			Time: e.ExecutedAt.In(c.Zone).Format("15:04"), Symbol: e.Symbol, SecType: e.SecType, Right: e.Right,
			Strike: e.Strike, Expiry: e.Expiry, Side: e.Side, Quantity: e.Quantity, Price: e.Price, Currency: e.Currency,
		}
		// Only the first fill of a contract has a known prior; a later one would need the running quantity.
		if prior != nil && !seen[k] {
			x.PriorQuantity = "0"
			if q, ok := held[k]; ok {
				x.PriorQuantity = q
			}
		}
		seen[k] = true
		in = append(in, x)
	}
	lines := c.Figures().Trades(day, in)

	h := newHead(keyActions, titleActions, subActions, sayActionsNone, dataTimesExec+string(day))
	rows := make([]tradeRow, 0, len(lines))
	for i, l := range lines {
		rows = append(rows, c.tradeRow(&h, alias, l, in[i], prior != nil))
	}
	if len(rows) > 0 {
		h.Say = sayActionsSome + strconv.Itoa(len(rows))
	}
	return actionsBlock{head: h, Trades: tradesJSON{Columns: columnsTrades, Rows: rows}}
}

func (c Config) tradeRow(h *head, alias string, l blockapi.TradeLine, e blockapi.ExecIn, hasPrior bool) tradeRow {
	r := tradeRow{
		Time: textCell(l.Time), Account: textCell(alias), Instrument: textCell(l.Instrument),
		Quantity: textCell(l.Quantity), Price: textCell(l.Price),
	}
	switch l.Kind {
	case "open":
		r.Action = textCell(kindOpen)
	case "close":
		r.Action = textCell(kindClose)
	case "expire":
		r.Action = textCell(kindExpire)
	case "exercise":
		r.Action = textCell(kindExercise)
	default:
		text := textKindGap
		switch {
		case !hasPrior:
			text = textNoPrior
		case e.PriorQuantity == "":
			text = textManyFills
		}
		g := gapJSON{Reason: "missing_input", Text: text}
		r.Action = g
		h.addGap(g)
	}
	return r
}
