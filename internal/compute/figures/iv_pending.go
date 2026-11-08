package figures

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// The three IV figures are not built yet: each is missing with bad_input until
// compute-figures-iv replaces this file.

func (c calc) Iv30(id string, _ []compute.ExpiryQuotes, _ compute.Point, asOf contracts.Day) compute.Figure {
	_, f, _ := c.begin(id, asOf)
	return missing(f, compute.BadInput)
}

func (c calc) IvPercentile(id string, _ compute.Series, asOf contracts.Day) compute.Figure {
	_, f, _ := c.begin(id, asOf)
	return missing(f, compute.BadInput)
}

func (c calc) IvRank(id string, _ compute.Series, asOf contracts.Day) compute.Figure {
	_, f, _ := c.begin(id, asOf)
	return missing(f, compute.BadInput)
}
