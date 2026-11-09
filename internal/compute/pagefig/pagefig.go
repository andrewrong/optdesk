// Package pagefig implements blockapi.PageFigures: the figures of the household page, the
// report blocks and the companies blocks, as decimal text, with no I/O and no clock.
package pagefig

import (
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var _ blockapi.PageFigures = figures{}

type figures struct{}

// New returns the page figures.
func New() blockapi.PageFigures { return figures{} }

func (figures) Household(in blockapi.HouseholdInput) blockapi.HouseholdResult {
	return household(in)
}

func (figures) Trades(day contracts.Day, in []blockapi.ExecIn) []blockapi.TradeLine {
	return trades(day, in)
}

func (figures) AccountMove(in blockapi.AccountMoveIn) blockapi.AccountMoveOut { return accountMove(in) }

func (figures) Moves(in []blockapi.CloseIn) []blockapi.MoveLine { return moves(in) }

func (figures) Glance(rows []blockapi.CompanyState) []blockapi.MoneyLine { return glance(rows) }

func (figures) Changes(rows []blockapi.CompanyState) []blockapi.ChangeLine { return changes(rows) }
