package optionattr

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type goldenRun struct {
	Input optionattrapi.Input
	Want  map[string]any
}

type goldenRow struct {
	Day                                 contracts.Day
	Action, Quantity, Price, Commission string
}

type goldenCase struct {
	Name       string
	Stub       string
	NoSolution []string
	Rows       []goldenRow
	EndClose   string
	Runs       []goldenRun
}

func loadCases(t *testing.T) []goldenCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/golden.json")
	require.NoError(t, err)
	var f struct{ Cases []goldenCase }
	require.NoError(t, json.Unmarshal(raw, &f))
	return f.Cases
}

func caseByName(t *testing.T, name string) goldenCase {
	t.Helper()
	for _, c := range loadCases(t) {
		if c.Name == name {
			return c
		}
	}
	require.Failf(t, "no golden case", "%s", name)
	return goldenCase{}
}

// stubFor returns a new stub pricer for the case: its own instance, so tests share no state.
func stubFor(c goldenCase) *quad {
	q := stubA()
	if c.Stub == "B" {
		q = stubB()
	}
	for _, p := range c.NoSolution {
		q.noSol = append(q.noSol, dec(p))
	}
	return q
}

// subset checks that every field of want is in got: decimals compare by value, lists by length.
func subset(t *testing.T, path string, want, got any) {
	t.Helper()
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		require.True(t, ok, path)
		for k, v := range w {
			subset(t, path+"."+k, v, g[k])
		}
	case []any:
		g, ok := got.([]any)
		require.True(t, ok, path)
		require.Len(t, g, len(w), path)
		for i, v := range w {
			subset(t, path+"[]", v, g[i])
		}
	case string:
		gs, _ := got.(string)
		wd, e1 := decimal.NewFromString(w)
		gd, e2 := decimal.NewFromString(gs)
		if e1 == nil && e2 == nil {
			assert.True(t, wd.Equal(gd), "%s: want %s got %s", path, w, gs)
		} else {
			assert.Equal(t, w, gs, path)
		}
	default:
		assert.Equal(t, want, got, path)
	}
}

func asJSON(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	var out any
	require.NoError(t, json.Unmarshal(b, &out))
	return out
}

func TestGoldenCases(t *testing.T) {
	t.Parallel()
	for _, c := range loadCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			stub := stubFor(c)
			if c.Name == "fill-side-mapping" {
				t.Skip("run by TestFillSideMapping")
			}
			for i, r := range c.Runs {
				res, err := New(stub).Attribute(r.Input)
				require.NoError(t, err)
				subset(t, c.Name, r.Want, asJSON(t, res))
				for _, s := range res.Steps {
					if s.Reason == "" {
						assert.Equal(t, "0", s.Unsplit, "%s run %d: unsplit without a reason", c.Name, i)
					}
				}
			}
			assert.Zero(t, stub.zeroT, "the pricer was asked about T = 0")
		})
	}
}

func TestAttributeNoRounding(t *testing.T) {
	t.Parallel()
	res := runFirst(t, "three-days-exact")
	// exact decimal text: the sums of -14.4 + -32 and the like are not rounded or padded
	assert.Equal(t, "-112", res.Direction)
	assert.Equal(t, "-46.4", res.MoveCost)
	assert.Equal(t, "118.4", res.IVRemark)
	assert.Equal(t, "0", res.Time)
	assert.Equal(t, "-14.4", res.Steps[0].MoveCost)
	assert.Equal(t, "-281.6", res.Steps[0].IVRemark)
	assert.Equal(t, "-32", res.Steps[1].MoveCost)
	assert.Equal(t, "0", res.Difference)
}

func TestPartialCloseGolden(t *testing.T) {
	t.Parallel()
	res := runFirst(t, "partial-close")
	require.Len(t, res.Steps, 3)
	assert.Equal(t, "240", res.Steps[0].IVRemark) // 3 contracts
	assert.Equal(t, "180", res.Steps[1].IVRemark) // 3 contracts up to the close day
	assert.Equal(t, "80", res.Steps[2].Time)      // 2 contracts after it
	assert.Equal(t, "-5", res.Exit)
	assert.Equal(t, "0", res.Difference)
}

func TestFillOnDayWithoutQuote(t *testing.T) {
	t.Parallel()
	res := runFirst(t, "fill-on-day-without-quote")
	assert.True(t, res.LateEntry)
	assert.Equal(t, "60", res.Entry) // charged against the first quote day, 2026-11-10
	assert.Equal(t, "0", res.Difference)
}

func TestFillSideMapping(t *testing.T) {
	t.Parallel()
	c := caseByName(t, "fill-side-mapping")
	in := c.Runs[0].Input
	for _, r := range c.Rows {
		switch r.Action {
		case "open":
			in.Lot.Fills = append(in.Lot.Fills, optionattrapi.Fill{Day: r.Day, Side: "open", Quantity: r.Quantity, Price: r.Price, Commission: r.Commission})
		case "close":
			in.Lot.Fills = append(in.Lot.Fills, optionattrapi.Fill{Day: r.Day, Side: "close", Quantity: r.Quantity, Price: r.Price, Commission: r.Commission})
		case "expire":
			in.Lot.End, in.Lot.EndDay, in.Lot.EndClose = "expired", r.Day, c.EndClose
		}
	}
	require.Len(t, in.Lot.Fills, 2)
	assert.Equal(t, "expired", in.Lot.End)
	res, err := New(stubFor(c)).Attribute(in)
	require.NoError(t, err)
	subset(t, c.Name, c.Runs[0].Want, asJSON(t, res))
}
