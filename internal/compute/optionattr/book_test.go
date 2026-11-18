package optionattr

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
)

type bookCase struct {
	Name    string
	Results []optionattrapi.Result
	Want    []map[string]any
	// Months and Companies key each result for the partition test; parallel to Results.
	Months, Companies []string
}

func bookCaseByName(t *testing.T, name string) bookCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/book.json")
	require.NoError(t, err)
	var f struct{ Cases []bookCase }
	require.NoError(t, json.Unmarshal(raw, &f))
	for _, c := range f.Cases {
		if c.Name == name {
			return c
		}
	}
	require.Failf(t, "no book case", "%s", name)
	return bookCase{}
}

// combineCase runs the case and checks every field of its want, by value for decimals; zero-valued
// sum fields the case leaves out are checked as "0" or "" through subset only when named.
func combineCase(t *testing.T, name string) []optionattrapi.Book {
	t.Helper()
	c := bookCaseByName(t, name)
	got := NewBooker().Combine(c.Results)
	require.Len(t, got, len(c.Want))
	for i, w := range c.Want {
		subset(t, name, w, asJSON(t, got[i]))
	}
	return got
}

func TestCombineSumsOneCurrency(t *testing.T) { t.Parallel(); combineCase(t, "sample") }

func TestCombineTwoCurrencies(t *testing.T) {
	t.Parallel()
	got := combineCase(t, "two-currencies")
	assert.Equal(t, "USD", got[0].Currency)
	assert.Equal(t, "JPY", got[1].Currency)
}

func TestCombineNoQuotesCounted(t *testing.T) { t.Parallel(); combineCase(t, "no-quotes") }

func TestCombineEmptyDiffCounted(t *testing.T) { t.Parallel(); combineCase(t, "empty-diff") }

func TestCombineRentMinusCost(t *testing.T) {
	t.Parallel()
	got := combineCase(t, "rent")
	assert.Equal(t, "226", got[0].RentMinusCost)
}

func TestCombineOfOneEqualsLot(t *testing.T) {
	t.Parallel()
	for _, r := range bookCaseByName(t, "sample").Results {
		sum := dec(r.Entry).Add(dec(zeroIfEmpty(r.Exit))).Add(dec(r.Commission))
		for _, s := range r.Steps {
			for _, f := range []string{s.Direction, s.MoveCost, s.IVRemark, s.Time, s.Unsplit} {
				sum = sum.Add(dec(f))
			}
		}
		got := NewBooker().Combine([]optionattrapi.Result{r})
		require.Len(t, got, 1)
		assert.True(t, sum.Equal(dec(got[0].Total)), "want %s got %s", sum, got[0].Total)
	}
}

func zeroIfEmpty(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

func TestCombineIdentityAssigned(t *testing.T) {
	t.Parallel()
	b := combineCase(t, "sample")[0]
	assert.True(t, dec(b.AssignedIntrinsic).IsPositive())
	rhs := dec(b.Total).Add(dec(b.AssignedIntrinsic)).Add(dec(b.Difference))
	assert.True(t, dec(b.LedgerPnL).Equal(rhs), "ledger %s, total+assigned+difference %s", b.LedgerPnL, rhs)
}

func TestLotTotalsSumToBook(t *testing.T) {
	t.Parallel()
	rs := bookCaseByName(t, "sample").Results
	all := NewBooker().Combine(rs)[0]
	var total, ledger, diff decimal.Decimal
	for _, r := range rs {
		one := NewBooker().Combine([]optionattrapi.Result{r})[0]
		total = total.Add(dec(one.Total))
		ledger = ledger.Add(dec(one.LedgerPnL))
		diff = diff.Add(dec(one.Difference))
	}
	assert.True(t, total.Equal(dec(all.Total)))
	assert.True(t, ledger.Equal(dec(all.LedgerPnL)))
	assert.True(t, diff.Equal(dec(all.Difference)))
}

// TestBookerContractReal runs the cases of TestBookerContractFake and TestBookerEmptyIsNonNil of
// optionattrapi against the real Booker, with the values the contract names.
func TestBookerContractReal(t *testing.T) {
	t.Parallel()
	b := NewBooker()
	usd := optionattrapi.Result{Currency: "USD", Time: "412", MoveCost: "-186", LedgerPnL: "226", Difference: "0"}
	jpy := optionattrapi.Result{Currency: "JPY", Time: "50", MoveCost: "-20", LedgerPnL: "30", Difference: "0"}
	noQuotes := optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "9", Difference: "0"}
	got := b.Combine([]optionattrapi.Result{usd, jpy, noQuotes})
	require.Len(t, got, 2)
	assert.Equal(t, "USD", got[0].Currency)
	assert.Equal(t, "226", got[0].RentMinusCost)
	assert.Equal(t, "226", got[0].Total)
	assert.Equal(t, "0", got[0].Difference)
	assert.Equal(t, 1, got[0].NoQuoteLots)
	assert.Equal(t, "9", got[0].NoQuoteLedgerPnL)
	assert.Equal(t, "JPY", got[1].Currency)
	assert.Equal(t, "30", got[1].RentMinusCost)
	assert.Equal(t, 0, got[1].NoQuoteLots)
	empty := b.Combine(nil)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestCombineAllLotsCounted(t *testing.T) {
	t.Parallel()
	c := bookCaseByName(t, "all-lots")
	got := NewBooker().Combine(c.Results)
	require.Len(t, got, 1)
	assert.Equal(t, 5, got[0].AllLots)
	want := dec(got[0].LedgerPnL).Add(dec(got[0].NoQuoteLedgerPnL)).Add(dec(got[0].UnreconciledLedgerPnL))
	assert.True(t, want.Equal(dec(got[0].LedgerAllPnL)), "want %s got %s", want, got[0].LedgerAllPnL)
	assert.Equal(t, "538.55", got[0].LedgerAllPnL)
}

func TestCombineLedgerAllExact(t *testing.T) {
	t.Parallel()
	got := NewBooker().Combine(bookCaseByName(t, "ledger-exact").Results)
	require.Len(t, got, 1)
	assert.Equal(t, "0.3", got[0].LedgerAllPnL)
}

func TestCombineBucketsSumToAll(t *testing.T) {
	t.Parallel()
	c := bookCaseByName(t, "all-lots")
	all := NewBooker().Combine(c.Results)[0]
	for name, keys := range map[string][]string{"month": c.Months, "company": c.Companies} {
		buckets := map[string][]optionattrapi.Result{}
		for i, r := range c.Results {
			buckets[keys[i]] = append(buckets[keys[i]], r)
		}
		require.Greater(t, len(buckets), 1, name)
		var total, tm, move, ledger, diff, allLedger decimal.Decimal
		var allLots int
		for _, rs := range buckets {
			b := NewBooker().Combine(rs)[0]
			total = total.Add(dec(b.Total))
			tm = tm.Add(dec(b.Time))
			move = move.Add(dec(b.MoveCost))
			ledger = ledger.Add(dec(b.LedgerPnL))
			diff = diff.Add(dec(b.Difference))
			allLedger = allLedger.Add(dec(b.LedgerAllPnL))
			allLots += b.AllLots
		}
		assert.True(t, total.Equal(dec(all.Total)), name)
		assert.True(t, tm.Equal(dec(all.Time)), name)
		assert.True(t, move.Equal(dec(all.MoveCost)), name)
		assert.True(t, ledger.Equal(dec(all.LedgerPnL)), name)
		assert.True(t, diff.Equal(dec(all.Difference)), name)
		assert.True(t, allLedger.Equal(dec(all.LedgerAllPnL)), name)
		assert.Equal(t, all.AllLots, allLots, name)
	}
}
