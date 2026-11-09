package report_blocks

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type accountRow struct {
	Account   any `json:"account"`
	Net       any `json:"net"`
	DayAmount any `json:"day_amount"`
	DayPct    any `json:"day_pct"`
	Biggest   any `json:"biggest"`
}

type accountsJSON struct {
	Columns []string     `json:"columns"`
	Rows    []accountRow `json:"rows"`
}

type meterJSON struct {
	Account             string `json:"account"`
	MarginBuffer        any    `json:"margin_buffer"`
	CashAfterAssignment any    `json:"cash_after_assignment"`
}

type movesBlock struct {
	head
	Accounts any         `json:"accounts"`
	Movers   any         `json:"movers"`
	Meters   []meterJSON `json:"meters"`
}

// accountCapture is the newest capture of an alias with a value row, and the one before it.
type accountCapture struct {
	day          string
	values       []store.AccountValue
	prior        []store.AccountValue
	staleSession int // session days between the data day and the request day
}

// AccountMoves is the account_moves block: one row per configured alias, no total row, with net
// liquidation, the day amount and percent from two captures, and per account the two meters
// margin_buffer and cash_after_assignment. biggest and movers are the Gap missing_quote. An alias
// with no capture, or a capture older than 3 session days, shows its own gaps and the block is
// partial; it never touches the row of another alias.
func AccountMoves(c Config) BlockFunc {
	return func(ctx context.Context, day contracts.Day, generatedAt time.Time) ([]byte, error) {
		if err := c.ready(); err != nil {
			return nil, err
		}
		d, err := parseDay(day, generatedAt)
		if err != nil {
			return nil, err
		}
		if c.Account == nil || c.Ledger == nil || c.Sessions == nil || c.Ledgers == nil || c.Figures == nil || len(c.Aliases) == 0 {
			return marshal(movesBlock{head: gapHead(keyMoves, titleMoves, subMoves, sayMovesGap, "no_reader"),
				Accounts: gapOf("no_reader"), Movers: gapOf("no_reader")})
		}
		sessions, err := c.sessionDays(ctx, d)
		if err != nil {
			return nil, err
		}
		h := newHead(keyMoves, titleMoves, subMoves, sayMoves, noDataTimes)
		h.addGap(gapJSON{Reason: "missing_quote", Text: "最大变动与异动：" + textMissQuote})
		rows := make([]accountRow, 0, len(c.Aliases))
		meters := make([]meterJSON, 0, len(c.Aliases))
		var newest string
		for _, alias := range c.Aliases {
			cp, err := c.capture(ctx, alias, sessions)
			if err != nil {
				return nil, err
			}
			row, meter, reason, err := c.accountRow(ctx, alias, cp, sessions, day)
			if err != nil {
				return nil, err
			}
			if reason != "" {
				h.addGap(gapJSON{Reason: reason, Text: "账户 " + alias + "：" + gapText(reason)})
			}
			if reason != "no_data" && cp.day > newest {
				newest = cp.day
			}
			rows = append(rows, row)
			meters = append(meters, meter)
		}
		if newest != "" {
			h.DataTimes = dataTimesPre + newest
		}
		return marshal(movesBlock{head: h, Accounts: accountsJSON{Columns: columnsAccounts, Rows: rows}, Movers: gapOf("missing_quote"), Meters: meters})
	}
}

// capture reads the values of the session days newest first until it has the data day and the one before it.
func (c Config) capture(ctx context.Context, alias string, sessions []contracts.Day) (accountCapture, error) {
	var cp accountCapture
	for i := len(sessions) - 1; i >= 0; i-- {
		rows, err := c.Account.Values(ctx, alias, string(sessions[i]))
		if err != nil {
			return cp, fmt.Errorf("report_blocks: read values: %w", err)
		}
		if len(rows) == 0 {
			continue
		}
		if cp.day == "" {
			cp.day, cp.values, cp.staleSession = string(sessions[i]), rows, len(sessions)-1-i
			continue
		}
		cp.prior = rows
		break
	}
	return cp, nil
}

// accountRow is the row and the meter of one alias and the gap reason of the whole row, if any.
func (c Config) accountRow(ctx context.Context, alias string, cp accountCapture,
	sessions []contracts.Day, day contracts.Day) (accountRow, meterJSON, string, error) {
	r := accountRow{Account: textCell(alias), Biggest: gapOf("missing_quote")}
	m := meterJSON{Account: alias}
	whole := ""
	switch {
	case cp.day == "":
		whole = "no_data"
	case cp.staleSession > staleSessions:
		whole = "stale_input"
	}
	if whole != "" {
		g := gapOf(whole)
		r.Net, r.DayAmount, r.DayPct, m.MarginBuffer, m.CashAfterAssignment = g, g, g, g, g
		return r, m, whole, nil
	}
	obs, err := c.obligations(ctx, alias, sessions, day)
	if err != nil {
		return r, m, "", err
	}
	out := c.Figures().AccountMove(blockapi.AccountMoveIn{
		Alias: alias, Values: valuesIn(cp.values), Prior: valuesIn(cp.prior), Obligations: obs,
	})
	fillRow(&r, &m, out, cp)
	return r, m, "", nil
}

func fillRow(r *accountRow, m *meterJSON, out blockapi.AccountMoveOut, cp accountCapture) {
	cur := currencyOf(cp.values, "NetLiquidation")
	r.Net = figureCell(out.Net, cur, "USD", false)
	r.DayAmount = figureCell(out.DayAmount, cur, "usd_signed", true)
	r.DayPct = figureCell(out.DayPct, cur, "percent", true)
	m.MarginBuffer = figureCell(out.MarginBuffer, cur, "share_percent", false)
	m.CashAfterAssignment = figureCell(out.CashAfterAssignment, currencyOf(cp.values, "TotalCashValue"), "usd_signed", true)
}
