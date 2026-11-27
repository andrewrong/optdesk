package optionattr

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
)

type cashCase struct {
	Name string
	Puts []optionattrapi.OpenPut
	Want []optionattrapi.CashDue
}

func cashCaseByName(t *testing.T, name string) cashCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/cashdue.json")
	require.NoError(t, err)
	var f struct{ Cases []cashCase }
	require.NoError(t, json.Unmarshal(raw, &f))
	for _, c := range f.Cases {
		if c.Name == name {
			return c
		}
	}
	require.Failf(t, "no cash case", "%s", name)
	return cashCase{}
}

func runCashCase(t *testing.T, name string) {
	t.Helper()
	c := cashCaseByName(t, name)
	got, err := NewBooker().CashIfAssigned(c.Puts)
	require.NoError(t, err)
	assert.Equal(t, c.Want, got)
}

func TestCashIfAssignedGolden(t *testing.T) {
	t.Parallel()
	runCashCase(t, "golden")
}

func TestCashIfAssignedTwoCurrencies(t *testing.T) {
	t.Parallel()
	runCashCase(t, "two-currencies")
}

func TestCashIfAssignedExactDecimals(t *testing.T) {
	t.Parallel()
	runCashCase(t, "exact-decimals")
}

func TestCashIfAssignedShortSignDropped(t *testing.T) {
	t.Parallel()
	b := NewBooker()
	neg, err := b.CashIfAssigned([]optionattrapi.OpenPut{{Currency: "USD", Strike: "40", Multiplier: "100", Quantity: "-3"}})
	require.NoError(t, err)
	pos, err := b.CashIfAssigned([]optionattrapi.OpenPut{{Currency: "USD", Strike: "40", Multiplier: "100", Quantity: "3"}})
	require.NoError(t, err)
	assert.Equal(t, pos, neg)
	assert.Equal(t, "12000", neg[0].Cash)
}

func TestCashIfAssignedNoPuts(t *testing.T) {
	t.Parallel()
	got, err := NewBooker().CashIfAssigned(nil)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestCashIfAssignedBadInput(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"Strike", "Multiplier", "Quantity"} {
		for _, bad := range []string{"abc", "0", ""} {
			p := optionattrapi.OpenPut{Currency: "USD", Strike: "40", Multiplier: "100", Quantity: "-1"}
			switch field {
			case "Strike":
				p.Strike = bad
			case "Multiplier":
				p.Multiplier = bad
			default:
				p.Quantity = bad
			}
			good := optionattrapi.OpenPut{Currency: "USD", Strike: "40", Multiplier: "100", Quantity: "-1"}
			got, err := NewBooker().CashIfAssigned([]optionattrapi.OpenPut{good, p})
			require.ErrorIs(t, err, optionattrapi.ErrBadInput, "%s %q", field, bad)
			assert.Nil(t, got)
		}
	}
}
