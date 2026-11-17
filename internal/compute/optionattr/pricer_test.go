package optionattr

import (
	"errors"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
)

// quad is a stub pricer: Value = vol * f(spot) with f(s) = c0 + c1*s + c2*s*s, so it is linear in vol and
// quadratic in price, and Implied = price / f(spot) in closed form. Years and rate are ignored, except that
// years must be positive: a call with T = 0 is counted in zeroT and fails.
type quad struct {
	c0, c1, c2 decimal.Decimal
	noSol      []decimal.Decimal // prices for which no volatility exists

	mu    sync.Mutex
	zeroT int
}

type noSolutionErr struct{}

func (noSolutionErr) Error() string    { return "no volatility matches the price" }
func (noSolutionErr) NoSolution() bool { return true }

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// stubA has curvature 0.05 and no linear part.
func stubA() *quad { return &quad{c2: dec("0.05")} }

// stubB has another curvature (0.025), and f(32) = 64, f(40) = 100.
func stubB() *quad { return &quad{c0: dec("-48"), c1: dec("2.7"), c2: dec("0.025")} }

func (q *quad) f(s decimal.Decimal) decimal.Decimal {
	return q.c0.Add(q.c1.Mul(s)).Add(q.c2.Mul(s).Mul(s))
}

func (q *quad) checkYears(years string) error {
	y, err := decimal.NewFromString(years)
	if err != nil {
		return err
	}
	if !y.IsPositive() {
		q.mu.Lock()
		q.zeroT++
		q.mu.Unlock()
		return errors.New("expired")
	}
	return nil
}

func (q *quad) Value(_, spot, _, years, _, vol string) (string, error) {
	if err := q.checkYears(years); err != nil {
		return "", err
	}
	return dec(vol).Mul(q.f(dec(spot))).String(), nil
}

func (q *quad) Implied(_, spot, _, years, _, price string) (string, error) {
	if err := q.checkYears(years); err != nil {
		return "", err
	}
	p := dec(price)
	for _, n := range q.noSol {
		if n.Equal(p) {
			return "", noSolutionErr{}
		}
	}
	return p.Div(q.f(dec(spot))).String(), nil
}

var _ optionattrapi.Pricer = (*quad)(nil)

// ordinary runs the golden case "ordinary-two-day-step": the spot goes 40 -> 32, 2 contracts, multiplier 100,
// mids 1.80 -> 2.56.
func ordinary(t *testing.T, q *quad) optionattrapi.Result {
	t.Helper()
	c := caseByName(t, "ordinary-two-day-step")
	res, err := New(q).Attribute(c.Runs[0].Input)
	require.NoError(t, err)
	return res
}

func TestMoveCostIsCurvatureTimesMoveSquared(t *testing.T) {
	t.Parallel()
	// the quadratic coefficient of the price at the solved vol, times the squared move 8, times 2 x 100
	for name, tc := range map[string]struct {
		stub *quad
		want string
	}{
		"A": {stubA(), "-14.4"}, // vol 0.0225 * 0.05 = 0.001125; 0.001125 * 64 * 200
		"B": {stubB(), "-5.76"}, // vol 0.018 * 0.025 = 0.00045;  0.00045 * 64 * 200
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			res := ordinary(t, tc.stub)
			assert.Equal(t, tc.want, res.Steps[0].MoveCost)
		})
	}
	// the price did not move: exactly zero, not solver noise
	c := caseByName(t, "expired-three-quote-days")
	res, err := New(stubA()).Attribute(c.Runs[0].Input)
	require.NoError(t, err)
	for _, s := range res.Steps {
		assert.Equal(t, "0", s.MoveCost, s.From)
	}
}

func TestDirectionIsDeltaTimesMove(t *testing.T) {
	t.Parallel()
	// seller direction = -Delta0 * (S1 - S0) * 2 * 100 with Delta0 = vol * f'(40)
	assert.Equal(t, "144", ordinary(t, stubA()).Steps[0].Direction)    // 0.0225 * 4 = 0.09; -0.09 * -8 * 200
	assert.Equal(t, "135.36", ordinary(t, stubB()).Steps[0].Direction) // 0.018 * 4.7 = 0.0846; 0.0846 * 8 * 200
}

func TestDeltaIndependentOfStep(t *testing.T) {
	t.Parallel()
	c := caseByName(t, "three-days-exact")
	for name, mk := range map[string]func() *quad{"A": stubA, "B": stubB} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r1, err := newWithBump(mk(), deltaBump).Attribute(c.Runs[0].Input)
			require.NoError(t, err)
			r2, err := newWithBump(mk(), deltaBump.Div(two)).Attribute(c.Runs[0].Input)
			require.NoError(t, err)
			assert.Equal(t, r1, r2)
		})
	}
}

func TestTotalIndependentOfPricer(t *testing.T) {
	t.Parallel()
	a, b := ordinary(t, stubA()), ordinary(t, stubB())
	total := func(r optionattrapi.Result) decimal.Decimal {
		return dec(r.Direction).Add(dec(r.MoveCost)).Add(dec(r.IVRemark)).Add(dec(r.Time)).Add(dec(r.Unsplit))
	}
	assert.True(t, total(a).Equal(dec("-152")), total(a).String())
	assert.True(t, total(b).Equal(dec("-152")), total(b).String())
	assert.NotEqual(t, a.Direction, b.Direction)
	assert.NotEqual(t, a.MoveCost, b.MoveCost)
	assert.Equal(t, "0", a.Difference)
	assert.Equal(t, "0", b.Difference)
}
