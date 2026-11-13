package pagefig

import "github.com/andrewrong/optdesk/internal/compute/blockapi"

var _ blockapi.HoldingFigures = holdingFigures{}

type holdingFigures struct{}

// NewHoldings returns the figures of held positions: value, unrealised and share per position,
// and delta by underlying and vega by expiry.
func NewHoldings() blockapi.HoldingFigures { return holdingFigures{} }

func (holdingFigures) Movers(in blockapi.MoversIn) blockapi.MoversResult { return buildMovers(in) }
