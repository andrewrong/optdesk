package optionattr

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
)

func optionSide(r optionattrapi.Result) decimal.Decimal {
	s := dec(r.Entry)
	if r.Exit != "" {
		s = s.Add(dec(r.Exit))
	}
	for _, v := range []string{r.Direction, r.MoveCost, r.IVRemark, r.Time, r.Unsplit} {
		s = s.Add(dec(v))
	}
	return s
}

func runFirst(t *testing.T, name string) optionattrapi.Result {
	t.Helper()
	c := caseByName(t, name)
	res, err := New(stubFor(c)).Attribute(c.Runs[0].Input)
	require.NoError(t, err)
	return res
}

func TestIdentityExpiredGolden(t *testing.T) {
	t.Parallel()
	res := runFirst(t, "expired-three-quote-days")
	assert.Equal(t, "-2.6", res.Commission)
	assert.Empty(t, res.Exit)
	assert.Equal(t, "0", res.AssignedIntrinsic)
	assert.True(t, optionSide(res).Equal(dec("360.00")), optionSide(res).String())
	assert.Equal(t, "0", res.Difference)
}

func TestIdentityAssignedGolden(t *testing.T) {
	t.Parallel()
	res := runFirst(t, "assigned-three-quote-days")
	assert.Equal(t, "380", res.AssignedIntrinsic)
	assert.True(t, optionSide(res).Equal(dec("-20.00")), optionSide(res).String())
	assert.Equal(t, "0", res.Difference)
}

func TestDifferenceShowsLedgerGap(t *testing.T) {
	t.Parallel()
	c := caseByName(t, "expired-three-quote-days")
	in := c.Runs[0].Input
	in.LedgerPnL = "357.00"
	res, err := New(stubFor(c)).Attribute(in)
	require.NoError(t, err)
	assert.Equal(t, "-0.4", res.Difference) // ledger minus our sum
}
