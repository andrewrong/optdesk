package option_pnl_page

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
)

// cashDueLines is the "cash due if assigned" lines of one account: every open short put lot (any mark or
// none) goes to Booker.CashIfAssigned with its stored multiplier, and each CashDue becomes one line. A
// refused input is one muted line; no open put gives nil; any other error is returned. The page adds nothing itself.
// Design: docs/design/option-pnl-totals.md#Decisions (1, 3, 4).
func cashDueLines(c Config, idx int, lots []pnlLot) ([]line, error) {
	var puts []optionattrapi.OpenPut
	for _, l := range lots {
		if p := l.pos; isOpenShortPut(p) {
			puts = append(puts, optionattrapi.OpenPut{Currency: p.Currency, Strike: p.Strike, Multiplier: p.Multiplier, Quantity: p.Quantity})
		}
	}
	if len(puts) == 0 {
		return nil, nil
	}
	prefix := "a" + strconv.Itoa(idx) + "-cash"
	due, err := c.Booker.CashIfAssigned(puts)
	switch {
	case errors.Is(err, optionattrapi.ErrBadInput):
		return []line{mkLine(prefix, cashDueLabelZh, cashDueRefusedZh, "").tone("muted")}, nil
	case err != nil:
		return nil, fmt.Errorf("option_pnl_page: cash due: %w", err)
	}
	out := make([]line, 0, len(due))
	for n, d := range due {
		text := money(d.Cash, d.Currency) + " · " + d.Contracts + cashDueContrZh + strconv.Itoa(d.Lots) + cashDueLotsZh
		l := mkLine(prefix+strconv.Itoa(n), cashDueLabelZh, fit(text), "")
		h := cashDueHintZh
		l.Hint = &h
		out = append(out, l)
	}
	return out, nil
}

func isOpenShortPut(p compute.LedgerPosition) bool {
	return p.Status == statusOpen && strings.ToUpper(p.Right) == "P" && strings.HasPrefix(strings.TrimSpace(p.Quantity), "-")
}
