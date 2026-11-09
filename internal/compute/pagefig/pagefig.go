// Package pagefig implements blockapi.PageFigures: the figures of the household page, the
// report blocks and the companies blocks, as decimal text, with no I/O and no clock.
package pagefig

import (
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var _ blockapi.PageFigures = figures{}

type figures struct{}

// New returns the page figures. Entry 3 fills the report and companies methods.
func New() blockapi.PageFigures { return figures{} }

func (figures) Household(in blockapi.HouseholdInput) blockapi.HouseholdResult {
	return household(in)
}

func (figures) Trades(contracts.Day, []blockapi.ExecIn) []blockapi.TradeLine { return nil }

func (figures) AccountMove(blockapi.AccountMoveIn) blockapi.AccountMoveOut {
	return blockapi.AccountMoveOut{}
}

func (figures) Moves([]blockapi.CloseIn) []blockapi.MoveLine { return nil }

func (figures) Glance([]blockapi.CompanyState) []blockapi.MoneyLine { return nil }

func (figures) Changes([]blockapi.CompanyState) []blockapi.ChangeLine { return nil }
