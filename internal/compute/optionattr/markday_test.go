package optionattr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
)

// TestMarkDayFollowsPageRule: d1 passes; d2 has no underlying price, d3 no ask, d4 is after the
// expiry. The last row the rule passes is d1.
func TestMarkDayFollowsPageRule(t *testing.T) {
	t.Parallel()
	in := optionattrapi.Input{
		Lot: optionattrapi.Lot{
			Currency: "USD", Strike: "38.00", Multiplier: "100", Expiry: "2026-11-20", End: "open",
			Fills: []optionattrapi.Fill{{Day: "2026-11-09", Side: "open", Quantity: "2", Price: "1.80", Commission: "2.60"}},
		},
		Quotes: []optionattrapi.Quote{
			{Day: "2026-11-09", Bid: "1.75", Ask: "1.85", Spot: "40"},
			{Day: "2026-11-10", Bid: "1.75", Ask: "1.85", Spot: "0"},
			{Day: "2026-11-11", Bid: "1.75", Ask: "0", Spot: "40"},
			{Day: "2026-11-23", Bid: "1.75", Ask: "1.85", Spot: "40"},
		},
		Rates:     []optionattrapi.Rate{{Day: "2026-11-09", Percent: "4.5"}},
		LedgerPnL: "-2.60", LedgerMarkDay: "2026-11-09", MaxRateAgeDays: 7,
	}
	res, err := New(stubA()).Attribute(in)
	require.NoError(t, err)
	assert.EqualValues(t, "2026-11-09", res.MarkDay)
}
