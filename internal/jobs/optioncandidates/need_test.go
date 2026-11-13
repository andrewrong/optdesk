package optioncandidates

import (
	"reflect"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/strategy"
)

func TestNeedsAreTheTwoRows(t *testing.T) {
	t.Parallel()
	got, err := Needs()
	if err != nil {
		t.Fatal(err)
	}
	want := []jobs.Need{
		{Strategy: "short_put", Group: "watch:options", Rights: []string{"P"}, MinDTE: 21, MaxDTE: 45, Ref: "buy_price", Low: "0.80", High: "1.05", Expiry: "all"},
		{Strategy: "covered_call", Group: "watch:options", Rights: []string{"C"}, MinDTE: 21, MaxDTE: 45, Ref: "spot", Low: "1.00", High: "1.05", Expiry: "all"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

const needBase = `
version = 1
id = "x"
[schedule]
calendar = "XNYS"
anchor = "session_close"
offset = "30m"
run_bound = "10m"
[params]
min_dte = 7
max_dte = 12
max_spread = "0.10"
div_guard_factor = "1.0"
`

func TestNeedsTakeDTEFromParams(t *testing.T) {
	t.Parallel()
	got, err := needsOf([]byte(needBase + `
[[need]]
strategy = "a"
group = "watch:options"
rights = ["P", "C"]
ref = "spot"
low = "0.9"
high = "1.1"
expiry = "monthly"
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MinDTE != 7 || got[0].MaxDTE != 12 || got[0].TradingClass != "" || got[0].Expiry != "monthly" {
		t.Fatalf("got %+v", got)
	}
}

func TestNeedsRefuseABadRow(t *testing.T) {
	t.Parallel()
	row := func(f func(r *needRow)) string {
		r := needRow{Strategy: "a", Group: "g", Rights: []string{"P"}, Ref: "spot", Low: "0.9", High: "1.1", Expiry: "all"}
		f(&r)
		return `[[need]]
strategy = "` + r.Strategy + `"
group = "` + r.Group + `"
rights = ["` + strings.Join(r.Rights, `","`) + `"]
ref = "` + r.Ref + `"
low = "` + r.Low + `"
high = "` + r.High + `"
expiry = "` + r.Expiry + `"
`
	}
	cases := []struct {
		name, key string
		f         func(r *needRow)
	}{
		{"strategy", "strategy", func(r *needRow) { r.Strategy = "" }},
		{"group", "group", func(r *needRow) { r.Group = "" }},
		{"ref", "ref", func(r *needRow) { r.Ref = "secret-ref" }},
		{"rights", "rights", func(r *needRow) { r.Rights = []string{"X"} }},
		{"low", "low", func(r *needRow) { r.Low = "1.2" }},
		{"expiry", "expiry", func(r *needRow) { r.Expiry = "weekly-secret" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := needsOf([]byte(needBase + row(c.f)))
			if err == nil || !strings.Contains(err.Error(), c.key) {
				t.Fatalf("err %v", err)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error carries a value: %v", err)
			}
		})
	}
}

type ghostStrategy struct{ strategy.Strategy }

func (ghostStrategy) ID() string { return "ghost" }

func TestEveryStrategyHasANeedRow(t *testing.T) {
	t.Parallel()
	needs, err := Needs()
	if err != nil {
		t.Fatal(err)
	}
	// the registered strategy of today has its row; covered_call has a row and no registered strategy: not an error
	if err := checkStrategyNeeds([]string{"short_put"}, needs); err != nil {
		t.Fatal(err)
	}
	if err := checkStrategyNeeds([]string{"short_put", "ghost"}, needs); err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("err %v", err)
	}
	// the constructor refuses a strategy with no row
	f := newFakes()
	st := f.stores()
	st.Strategy, st.Results = ghostStrategy{}, &fakeResults{}
	if _, err := New(st, f, nil, fixedClock{}); err == nil {
		t.Fatal("a strategy without a need row was accepted")
	}
}
