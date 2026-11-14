package marketstate_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/figures"
	"github.com/andrewrong/optdesk/internal/compute/gonumstats"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/compute/marketstate"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/compute/rules"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type fundSpec struct {
	Symbol, Name string
	StartCents   int `json:"start_cents"`
	StepCents    int `json:"step_cents"`
}

type fixture struct {
	Day         contracts.Day `json:"day"`
	Sessions    int           `json:"sessions"`
	WiggleCents []int         `json:"wiggle_cents"`
	Benchmark   fundSpec      `json:"benchmark"`
	Funds       []fundSpec    `json:"funds"`
	Industry    []fundSpec    `json:"industry"`
}

func load(t *testing.T) fixture {
	t.Helper()
	b, err := os.ReadFile("testdata/marketstate.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// sessionDays is n weekdays ending on day, newest first.
func sessionDays(day contracts.Day, n int) []contracts.Day {
	d, _ := time.Parse(time.DateOnly, string(day))
	var out []contracts.Day
	for len(out) < n {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append(out, contracts.Day(d.Format(time.DateOnly)))
		}
		d = d.AddDate(0, 0, -1)
	}
	return out
}

func (f fixture) closes(s fundSpec) blockapi.CloseIn {
	days := sessionDays(f.Day, f.Sessions)
	c := blockapi.CloseIn{Name: s.Name, Symbol: s.Symbol, Days: days}
	for i := range days {
		sess := f.Sessions - 1 - i
		cents := s.StartCents + s.StepCents*sess + f.WiggleCents[sess%len(f.WiggleCents)]
		c.Closes = append(c.Closes, fmt.Sprintf("%d.%02d", cents/100, cents%100))
	}
	return c
}

func (f fixture) input() marketstateapi.Input {
	in := marketstateapi.Input{Day: f.Day, Benchmark: f.closes(f.Benchmark), Params: marketstate.TrialParams()}
	for _, s := range f.Funds {
		in.Funds = append(in.Funds, f.closes(s))
	}
	for _, s := range f.Industry {
		in.Industry = append(in.Industry, f.closes(s))
	}
	return in
}

func realFigures() (compute.Figures, compute.Units, compute.Rules) {
	stats := gonumstats.New()
	r := rules.New()
	return figures.New(stats, own.NewRolling(stats), own.NewSeriesOps(), r), own.NewUnits(), r
}

// dayN is the calendar day n days after 2024-01-01.
func dayN(n int) contracts.Day {
	return contracts.Day(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, n).Format(time.DateOnly))
}

func series(n int, value func(i int) float64) compute.Series {
	var s compute.Series
	for i := 0; i < n; i++ {
		s = append(s, compute.Point{Day: dayN(i), Value: value(i)})
	}
	return s
}

func TestTrialParams(t *testing.T) {
	t.Parallel()
	want := marketstateapi.Params{HighSessions: 60, ShortSessions: 21, LongSessions: 63, MeanSessions: 200,
		StaleWeekdays: 3, MaxAgeDays: 4, ChartYears: 3, ChartMax: 800, TailStep: 5, TailPoints: 4}
	if got := marketstate.TrialParams(); got != want {
		t.Fatalf("TrialParams = %+v", got)
	}
}

func TestSectorRankDropsGapFund(t *testing.T) {
	t.Parallel()
	f := load(t)
	in := f.input()
	gapSym := in.Funds[3].Symbol
	in.Funds[3].Closes, in.Funds[3].Days = in.Funds[3].Closes[1:], in.Funds[3].Days[1:]
	r := marketstate.Compute(in)
	if len(r.Funds) != 11 {
		t.Fatalf("funds %d", len(r.Funds))
	}
	last := r.Funds[10]
	if last.Symbol != gapSym || last.Gap63.Reason != liquidityapi.MissingInput || last.Rank63.Reason != liquidityapi.MissingInput ||
		last.Rank63.At != 0 || last.Rank21.Reason != liquidityapi.MissingInput || last.Gap63.Value != "" {
		t.Fatalf("gap fund %+v", last)
	}
	for i, fd := range r.Funds[:10] {
		if fd.Rank63.Of != 10 || fd.Rank21.Of != 10 || fd.Rank63.At != i+1 {
			t.Fatalf("fund %d %+v", i, fd)
		}
	}
	if r.Funds[0].Symbol != "XLK" {
		t.Fatalf("greatest gap is %s", r.Funds[0].Symbol)
	}
}

func TestSectorMatchesReportRule(t *testing.T) {
	t.Parallel()
	f := load(t)
	in := f.input()
	in.Params.ShortSessions = 20
	in.Funds[0].Closes, in.Funds[0].Days = in.Funds[0].Closes[1:], in.Funds[0].Days[1:] // no close on SPY's newest day
	in.Funds[1].Days = append(append([]contracts.Day(nil), in.Funds[1].Days[:20]...), in.Funds[1].Days[21:]...)
	in.Funds[1].Closes = append(append([]string(nil), in.Funds[1].Closes[:20]...), in.Funds[1].Closes[21:]...) // none on the window start
	in.Funds[2].Closes, in.Funds[2].Days = in.Funds[2].Closes[:10], in.Funds[2].Days[:10]                      // short history
	in.Funds[3].Closes, in.Funds[3].Days = nil, nil                                                            // no closes
	got := marketstate.Compute(in)
	want := pagefig.NewGlobal().Global(blockapi.GlobalIn{Day: in.Day, Benchmark: in.Benchmark, Sectors: in.Funds})
	if len(want.Sectors) != len(got.Funds) {
		t.Fatalf("rows %d %d", len(want.Sectors), len(got.Funds))
	}
	byName := map[string]compute.FigureValue{}
	for _, s := range want.Sectors {
		byName[s.Symbol] = s.D20
	}
	reasons := map[string]compute.Reason{"no_data": compute.NoInput, "missing_input": liquidityapi.MissingInput, "stale_input": compute.StaleInput}
	gaps := 0
	for _, fd := range got.Funds {
		w := byName[fd.Symbol]
		if fd.Gap21.Value != w.Value || (w.Gap != "" && fd.Gap21.Reason != reasons[w.Gap]) || (w.Gap == "" && fd.Gap21.Reason != "") {
			t.Fatalf("%s: got %+v want %+v", fd.Symbol, fd.Gap21, w)
		}
		if w.Gap != "" {
			gaps++
		}
	}
	if gaps != 4 {
		t.Fatalf("gap funds %d", gaps)
	}
}

func TestDrawdownFromSixtyHigh(t *testing.T) {
	t.Parallel()
	s := series(100, func(i int) float64 {
		switch {
		case i == 5:
			return 2000 // outside the newest 60 closes
		case i <= 70:
			return 1000 + float64(i)
		}
		return 1070 - float64(i-70)
	})
	in := marketstateapi.Input{Day: dayN(99), SP500: s, Params: marketstate.TrialParams()}
	d := marketstate.Compute(in).Drawdown
	if d.Value != "-2.71" || d.Aux != dayN(70) || d.Day != dayN(99) || d.Reason != "" {
		t.Fatalf("drawdown %+v", d)
	}
	in.Day = dayN(120)
	if d := marketstate.Compute(in).Drawdown; d.Reason != compute.StaleInput || d.Value != "" {
		t.Fatalf("stale drawdown %+v", d)
	}
	in.SP500 = s[:30]
	in.Day = dayN(29)
	if d := marketstate.Compute(in).Drawdown; d.Reason != compute.ShortHistory {
		t.Fatalf("short drawdown %+v", d)
	}
	in.SP500 = nil
	if d := marketstate.Compute(in).Drawdown; d.Reason != compute.NoInput {
		t.Fatalf("no drawdown %+v", d)
	}
}

func TestHistoryEqualsRegisterFigs(t *testing.T) {
	t.Parallel()
	figs, units, rl := realFigures()
	sp := series(320, func(i int) float64 { return 4000 + float64(i*7%53) + float64(i)/3 })
	vix := series(320, func(i int) float64 { return 15 + float64(i*5%11)/2 })
	vxv := series(320, func(i int) float64 { return 18 + float64(i*3%7)/2 })
	in := marketstateapi.Input{Day: dayN(319), SP500: sp, VIXCLS: vix, VXVCLS: vxv, Figures: figs, Units: units, Rules: rl, Params: marketstate.TrialParams()}
	h := marketstate.Compute(in).History
	check := func(name string, got []marketstateapi.Point, want map[contracts.Day]string) {
		t.Helper()
		if len(want) < 100 {
			t.Fatalf("%s: figures give only %d points", name, len(want))
		}
		if len(got) != len(want) {
			t.Fatalf("%s: %d points, figures %d", name, len(got), len(want))
		}
		for _, p := range got {
			if want[p.Day] != p.Value {
				t.Fatalf("%s %s: %q, figures %q", name, p.Day, p.Value, want[p.Day])
			}
		}
	}
	wantMA, wantTerm := map[contracts.Day]string{}, map[contracts.Day]string{}
	maRule, _ := rl.Rule("market.sp500_vs_200d")
	trRule, _ := rl.Rule("market.vix_term_ratio")
	for i, p := range sp {
		if f := figs.MovingAverageGap("market.sp500_vs_200d", sp[:i+1], p.Day); f.Status == compute.StatusOK {
			wantMA[p.Day], _ = units.RoundHalfEven(f.Value, maRule.Scale)
		}
		if f := figs.Ratio("market.vix_term_ratio", vix[:i+1], vxv[:i+1], p.Day); f.Status == compute.StatusOK {
			wantTerm[p.Day], _ = units.RoundHalfEven(f.Value, trRule.Scale)
		}
	}
	check("sp500_vs_200d", h.Sp500Vs200d.Points, wantMA)
	check("vix_term", h.VIXTerm.Points, wantTerm)
}

func TestHistoryDropsOneCalendarGap(t *testing.T) {
	t.Parallel()
	figs, units, rl := realFigures()
	vix := series(8, func(i int) float64 { return 20 })
	vix[5].Value = 18
	var vxv compute.Series
	for i := 0; i < 8; i++ {
		if i != 4 {
			v := 25.0
			if i == 5 {
				v = 20
			}
			vxv = append(vxv, compute.Point{Day: dayN(i), Value: v})
		}
	}
	in := marketstateapi.Input{Day: dayN(7), VIXCLS: vix, VXVCLS: vxv, Figures: figs, Units: units, Rules: rl, Params: marketstate.TrialParams()}
	pts := marketstate.Compute(in).History.VIXTerm.Points
	if len(pts) != 7 {
		t.Fatalf("points %+v", pts)
	}
	for _, p := range pts {
		if p.Day == dayN(4) {
			t.Fatalf("day with one leg has a point %+v", p)
		}
		if p.Day == dayN(5) && p.Value != "0.900000" {
			t.Fatalf("after the gap %+v", p)
		}
	}
}

func TestHistoryTotalBeforeCap(t *testing.T) {
	t.Parallel()
	vix := series(900, func(i int) float64 { return float64(10 + i%7) })
	in := marketstateapi.Input{Day: dayN(899), VIXCLS: vix, Params: marketstate.TrialParams()}
	l := marketstate.Compute(in).History.VIX
	if len(l.Points) != 800 || l.Total != 900 {
		t.Fatalf("points %d total %d", len(l.Points), l.Total)
	}
	if l.Points[0].Day != dayN(100) || l.Points[799].Day != dayN(899) || l.Points[799].Value != "13" {
		t.Fatalf("ends %+v %+v", l.Points[0], l.Points[799])
	}
}

func TestHistoryLastPointIsExample(t *testing.T) {
	t.Parallel()
	figs, units, rl := realFigures()
	sp := series(200, func(i int) float64 { return 100 })
	sp[198].Value, sp[199].Value = 96.88, 103.12
	vix := series(3, func(i int) float64 { return 18 })
	vxv := series(3, func(i int) float64 { return 20 })
	in := marketstateapi.Input{Day: dayN(199), SP500: sp, Figures: figs, Units: units, Rules: rl, Params: marketstate.TrialParams()}
	pts := marketstate.Compute(in).History.Sp500Vs200d.Points
	if len(pts) != 1 || pts[0].Value != "0.031200" || pts[0].Day != dayN(199) {
		t.Fatalf("ma points %+v", pts)
	}
	in = marketstateapi.Input{Day: dayN(2), VIXCLS: vix, VXVCLS: vxv, Figures: figs, Units: units, Rules: rl, Params: marketstate.TrialParams()}
	pts = marketstate.Compute(in).History.VIXTerm.Points
	if len(pts) != 3 || pts[2].Value != "0.900000" {
		t.Fatalf("term points %+v", pts)
	}
}

func TestHistoryDrawdownAndLevels(t *testing.T) {
	t.Parallel()
	sp := series(61, func(i int) float64 { return 100 })
	sp[60].Value = 97.5
	hy := series(3, func(i int) float64 { return 3.12 })
	in := marketstateapi.Input{Day: dayN(60), SP500: sp, BAMLH0A0HYM2: hy, Params: marketstate.TrialParams()}
	h := marketstate.Compute(in).History
	if n := len(h.Drawdown.Points); n != 2 || h.Drawdown.Points[0].Value != "0.00" || h.Drawdown.Points[1].Value != "-2.50" {
		t.Fatalf("drawdown %+v", h.Drawdown)
	}
	if len(h.Sp500Vs200d.Points) != 0 || len(h.HighYield.Points) != 3 || h.HighYield.Points[0].Value != "3.12" {
		t.Fatalf("history %+v", h)
	}
}

func TestBenchmarkStale(t *testing.T) {
	t.Parallel()
	f := load(t)
	in := f.input()
	in.Day = "2026-11-19" // four weekdays after SPY's newest close
	r := marketstate.Compute(in)
	for _, fd := range append(append([]marketstateapi.Fund(nil), r.Funds...), r.Industry...) {
		if fd.Gap21.Reason != compute.StaleInput || fd.Gap63.Reason != compute.StaleInput || len(fd.Rotation) != 0 {
			t.Fatalf("%s %+v", fd.Symbol, fd)
		}
	}
	if r.EndDay != f.Day {
		t.Fatalf("end day %q", r.EndDay)
	}
	in.Day = "2026-11-18" // three weekdays: still fresh
	if g := marketstate.Compute(in).Funds[0].Gap63; g.Reason != "" {
		t.Fatalf("fresh %+v", g)
	}
	in.Benchmark = blockapi.CloseIn{Name: "SPY"}
	r = marketstate.Compute(in)
	if r.EndDay != "" || r.Funds[0].Gap63.Reason != compute.NoInput || r.Funds[0].Rank63.Reason != compute.NoInput {
		t.Fatalf("no benchmark %+v", r.Funds[0])
	}
}

func TestIndustryNeverRanked(t *testing.T) {
	t.Parallel()
	f := load(t)
	r := marketstate.Compute(f.input())
	if len(r.Industry) != 1 || r.Industry[0].Symbol != "SMH" {
		t.Fatalf("industry %+v", r.Industry)
	}
	smh := r.Industry[0]
	if smh.Gap63.Value == "" || smh.Rank21 != (marketstateapi.Position{}) || smh.Rank63 != (marketstateapi.Position{}) {
		t.Fatalf("smh %+v", smh)
	}
	for _, fd := range r.Funds {
		if fd.Rank63.Of != 11 || fd.Symbol == "SMH" {
			t.Fatalf("funds %+v", fd)
		}
	}
}

// exactSpot is the independent exact calculation of a spot from the closes of a fund and SPY newest first.
func exactSpot(t *testing.T, spy, s blockapi.CloseIn, short, long int) (x, y string) {
	t.Helper()
	rat := func(v string) *big.Rat { r, _ := new(big.Rat).SetString(v); return r }
	gap := func(back int) *big.Rat {
		pc := func(c blockapi.CloseIn) *big.Rat {
			r := new(big.Rat).Quo(rat(c.Closes[0]), rat(c.Closes[back]))
			r.Sub(r, big.NewRat(1, 1))
			return r.Mul(r, big.NewRat(100, 1))
		}
		return new(big.Rat).Sub(pc(s), pc(spy))
	}
	g21, g63 := gap(short), gap(long)
	yy := new(big.Rat).Sub(g21, new(big.Rat).Quo(g63, big.NewRat(3, 1)))
	// 2 places half-even by exact integer arithmetic.
	round := func(r *big.Rat) string {
		n := new(big.Int).Mul(r.Num(), big.NewInt(100))
		q, m := new(big.Int).DivMod(n, r.Denom(), new(big.Int))
		switch c := new(big.Int).Lsh(m, 1).Cmp(r.Denom()); {
		case c > 0, c == 0 && q.Bit(0) == 1:
			q.Add(q, big.NewInt(1))
		}
		return new(big.Rat).SetFrac(q, big.NewInt(100)).FloatString(2)
	}
	return round(g63), round(yy)
}

func cut(c blockapi.CloseIn, n int) blockapi.CloseIn {
	c.Closes, c.Days = c.Closes[n:], c.Days[n:]
	return c
}

func TestRotationSpotEqualsGapAsOf(t *testing.T) {
	t.Parallel()
	f := load(t)
	in := f.input()
	r := marketstate.Compute(in)
	all := append(append([]marketstateapi.Fund(nil), r.Funds...), r.Industry...)
	inputs := append(append([]blockapi.CloseIn(nil), in.Funds...), in.Industry...)
	bySym := map[string]blockapi.CloseIn{}
	for _, c := range inputs {
		bySym[c.Symbol] = c
	}
	for _, fd := range all {
		if len(fd.Rotation) != 5 {
			t.Fatalf("%s: %d spots", fd.Symbol, len(fd.Rotation))
		}
		for j, sp := range fd.Rotation {
			k := (4 - j) * 5
			cin := in
			cin.Benchmark = cut(in.Benchmark, k)
			cin.Day = cin.Benchmark.Days[0]
			cin.Funds, cin.Industry = []blockapi.CloseIn{cut(bySym[fd.Symbol], k)}, nil
			g := marketstate.Compute(cin).Funds[0]
			wx, wy := exactSpot(t, cin.Benchmark, cin.Funds[0], 21, 63)
			if sp.X != g.Gap63.Value || sp.Y != wy || sp.X != wx || sp.Day != cin.Benchmark.Days[0] {
				t.Fatalf("%s k=%d: spot %+v, gap63 %s, exact %s %s", fd.Symbol, k, sp, g.Gap63.Value, wx, wy)
			}
		}
	}
}

func TestRotationDropsTailNoClose(t *testing.T) {
	t.Parallel()
	f := load(t)
	in := f.input()
	// XLB has no close on the tail session 10 sessions back; a Saturday stands in its place so that the
	// sessions of the two windows still line up and today keeps its figure.
	x := in.Funds[0]
	sat, _ := time.Parse(time.DateOnly, string(x.Days[10]))
	x.Days = append([]contracts.Day(nil), x.Days...)
	x.Days[10] = contracts.Day(sat.AddDate(0, 0, 1).Format(time.DateOnly))
	in.Funds = append([]blockapi.CloseIn{x}, in.Funds[1:]...)
	r := marketstate.Compute(in)
	var got marketstateapi.Fund
	for _, fd := range r.Funds {
		if fd.Symbol == x.Symbol {
			got = fd
		}
	}
	if len(got.Rotation) != 4 {
		t.Fatalf("spots %+v", got.Rotation)
	}
	days := map[contracts.Day]bool{}
	for _, sp := range got.Rotation {
		days[sp.Day] = true
	}
	if days[in.Benchmark.Days[10]] || !days[in.Benchmark.Days[5]] || !days[in.Benchmark.Days[15]] || !days[in.Benchmark.Days[20]] || !days[in.Benchmark.Days[0]] {
		t.Fatalf("days %v", days)
	}
	// Today has no figure: the rotation is empty.
	y := in.Funds[1]
	y.Closes, y.Days = y.Closes[1:], y.Days[1:]
	in.Funds = []blockapi.CloseIn{in.Funds[0], y}
	if rot := marketstate.Compute(in).Funds[1].Rotation; len(rot) != 0 {
		t.Fatalf("rotation %+v", rot)
	}
}
