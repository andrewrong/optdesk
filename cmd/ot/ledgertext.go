package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// ledgerSections is the order the positions are listed in: open first, then closed, assigned, expired
// and exercised; `unresolved` and `review` follow as lists apart (Decision 16).
var ledgerSections = []string{"open", "closed", "assigned", "expired", "exercised"}

// ledgerText writes the replay of one alias as text. It adds nothing up: every amount is printed as the rule
// gave it with its currency beside it, so no figure mixes two currencies (Decision 9). The rule decides the
// order of the positions and cycles; this function groups and prints.
func ledgerText(w io.Writer, alias string, day contracts.Day, res compute.LedgerResult, o ledgerOpts) {
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(w, format+"\n", a...) }
	p("ledger alias=%s as_of=%s rule_version=%d", alias, day, res.RuleVersion)
	for _, status := range ledgerSections {
		if o.status != "" && o.status != status {
			continue
		}
		var rows []compute.LedgerPosition
		for _, pos := range res.Positions {
			if pos.Status == status {
				rows = append(rows, pos)
			}
		}
		p("%s (%d)", status, len(rows))
		for _, pos := range rows {
			p("  %s", positionLine(pos))
		}
	}
	if o.status == "" || o.status == "unresolved" {
		listApart(p, "unresolved", res.Unresolved)
	}
	if o.status == "" || o.status == "review" {
		listApart(p, "review", res.Review)
	}
	if o.cycles && o.status == "" {
		p("cycles (%d)", len(res.Cycles))
		for _, c := range res.Cycles {
			cycleBlock(p, c)
		}
	}
}

// listApart prints a list of transaction ids with its count.
func listApart(p func(string, ...any), name string, ids []string) {
	p("%s (%d)", name, len(ids))
	for _, id := range ids {
		p("  transaction %s", id)
	}
}

// money is an amount with its currency beside it; an empty amount stays empty.
func money(amount, currency string) string {
	if amount == "" {
		return "-"
	}
	return amount + " " + currency
}

// positionLine is one position on one line: the contract, the quantity and the amounts of the rule.
func positionLine(pos compute.LedgerPosition) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %s strike=%s qty=%s", pos.Underlying, pos.Right, pos.Expiry, pos.Strike, pos.Quantity)
	if pos.Early {
		b.WriteString(" early")
	}
	fmt.Fprintf(&b, " premium=%s commissions=%s", money(pos.PremiumIncome, pos.Currency), money(pos.Commissions, pos.Currency))
	if pos.Status == "open" {
		if pos.MarkStatus == "ok" {
			fmt.Fprintf(&b, " mark=%s (%s) unrealized=%s", money(pos.Mark, pos.Currency), pos.MarkDay, money(pos.UnrealizedPnL, pos.Currency))
		} else {
			fmt.Fprintf(&b, " mark=%s", pos.MarkStatus)
		}
		return b.String()
	}
	fmt.Fprintf(&b, " realized=%s closed_on=%s", money(pos.RealizedPnL, pos.Currency), pos.ClosedOn)
	return b.String()
}

// cycleBlock prints one wheel cycle; the shares, the cost per share (the purchase price, no premium taken off),
// the premium income and the stock figures are separate lines and are never totalled.
func cycleBlock(p func(string, ...any), c compute.LedgerCycle) {
	p("  cycle %s %s (%s)", c.CycleID, c.Underlying, c.Currency)
	p("    shares: %s", c.Shares)
	p("    cost per share (purchase price): %s", money(c.CostBasisPerShare, c.Currency))
	p("    premium income: %s", money(c.PremiumIncome, c.Currency))
	p("    stock realized: %s", money(c.StockRealizedPnL, c.Currency))
	if c.UnrealizedStockPnL == "" {
		p("    stock unrealized: no_mark")
	} else {
		p("    stock unrealized: %s", money(c.UnrealizedStockPnL, c.Currency))
	}
	p("    stock commissions: %s", money(c.StockCommissions, c.Currency))
	p("    premium outside cycles: %s", money(c.PremiumOutsideCycles, c.Currency))
}
