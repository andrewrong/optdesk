package optionopen

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// usWindowChain holds one chain with expiries 14, 15, 45, 46, 49 and 50 days after testDay.
func usWindowChain() []Contract {
	var cs []Contract
	for _, d := range []int{14, 15, 45, 46, 49, 50} {
		cs = append(cs, chain(d, "P", 100, 101)...)
	}
	return cs
}

func TestUSWindowIs15To45(t *testing.T) {
	t.Parallel()
	m, err := parseManifest(planManifest)
	require.NoError(t, err)
	n := NameInput{UnderlyingID: 1, Symbol: "AAA", Spot: 100.5, Candidates: usWindowChain()}
	got := map[int]bool{}
	for _, it := range BuildPlan(testDay, m.params, []NameInput{n}).Items {
		got[dteOf(testDay, it.Contract.Expiry)] = true
	}
	require.Equal(t, map[int]bool{15: true, 45: true}, got)
}

func TestUSManifestsShareOneWindow(t *testing.T) {
	t.Parallel()
	plan, err := parseManifest(planManifest)
	require.NoError(t, err)
	gap, err := parseManifest(gapManifest)
	require.NoError(t, err)
	resolve, err := parseManifest(resolveManifest)
	require.NoError(t, err)
	list, err := parseList(listManifest)
	require.NoError(t, err)
	zone, err := parseList(listManifestZone)
	require.NoError(t, err)
	require.Equal(t, 4, resolve.LookaheadDays)

	day := contracts.Day("2026-11-10")
	var expiries []contracts.Day
	for _, d := range []int{14, 15, 45, 46, 49, 50} {
		expiries = append(expiries, contracts.Day(testDay.AddDate(0, 0, d).Format("2006-01-02")))
	}
	offsets := func(p manifest, extra int) []int {
		var out []int
		for _, e := range ChooseExpiries(day, expiries, p.params.MinDTE, p.params.MaxDTE+extra) {
			t0, _ := dayTime(e)
			out = append(out, int(t0.Sub(testDay).Hours()/24))
		}
		return out
	}
	want := []int{15, 45}
	require.ElementsMatch(t, want, offsets(plan, 0))
	require.ElementsMatch(t, want, offsets(gap, 0))
	require.ElementsMatch(t, []int{15, 45, 46, 49}, offsets(resolve, resolve.LookaheadDays))
	for _, lm := range []listTable{list, zone} {
		require.Len(t, lm.Windows, 1)
		var got []int
		for _, e := range ChooseExpiries(day, expiries, lm.Windows[0].MinDTE, lm.Windows[0].MaxDTE) {
			t0, _ := dayTime(e)
			got = append(got, int(t0.Sub(testDay).Hours()/24))
		}
		require.ElementsMatch(t, want, got)
	}
}
