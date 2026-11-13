package pagefig

import (
	"math/big"
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// globalFigures is the global assets block: proxy changes, sector excess over the benchmark, the sort and the
// facts of the rotation sentence (docs/design/report-global-assets.md, Decisions 3 to 6, 8, 9).
func globalFigures(in blockapi.GlobalIn) blockapi.GlobalResult {
	out := blockapi.GlobalResult{}
	if len(in.Benchmark.Days) > 0 {
		out.DataDay = in.Benchmark.Days[0]
	}
	out.Proxies = make([]blockapi.MoveLine, 0, len(in.Proxies))
	for _, p := range in.Proxies {
		l := blockapi.MoveLine{Name: p.Name, Symbol: p.Symbol}
		if len(p.Days) > 0 {
			l.DataDay = p.Days[0]
		}
		l.D1, l.D5 = proxyMove(in.Benchmark, p, in.Day, 1), proxyMove(in.Benchmark, p, in.Day, 5)
		out.Proxies = append(out.Proxies, l)
	}
	listed := make([]blockapi.SectorLine, 0, len(in.Sectors))
	for _, s := range in.Sectors {
		listed = append(listed, blockapi.SectorLine{
			Name: s.Name, Symbol: s.Symbol,
			D1:  sectorExcess(in.Benchmark, s, in.Day, 1),
			D5:  sectorExcess(in.Benchmark, s, in.Day, 5),
			D20: sectorExcess(in.Benchmark, s, in.Day, 20),
		})
	}
	out.Sectors = sortByD5(listed)
	out.Rotation = rotationOf(listed, out.Sectors)
	return out
}

// benchGap is the gap every cell shares when the benchmark has no usable newest close: no_data without one,
// stale_input when it is more than staleWeekdays behind the page day; empty when the benchmark is usable.
func benchGap(spy blockapi.CloseIn, day contracts.Day) string {
	if len(spy.Closes) == 0 || len(spy.Days) != len(spy.Closes) {
		return "no_data"
	}
	if weekdaysBehind(spy.Days[0], day) > staleWeekdays {
		return "stale_input"
	}
	return ""
}

// seriesGap checks one symbol against the benchmark in the order of Decision 6: no close is no_data, a newest
// day other than the benchmark's is missing_input.
func seriesGap(spy, s blockapi.CloseIn, day contracts.Day) string {
	if g := benchGap(spy, day); g != "" {
		return g
	}
	if len(s.Closes) == 0 || len(s.Days) != len(s.Closes) {
		return "no_data"
	}
	if s.Days[0] != spy.Days[0] {
		return "missing_input"
	}
	return ""
}

// proxyMove is the figure report.proxy_move: the percent change of c0 over the close back sessions earlier (the
// formula of change), where c0 is the benchmark's newest close day.
func proxyMove(spy, s blockapi.CloseIn, day contracts.Day, back int) compute.FigureValue {
	if g := seriesGap(spy, s, day); g != "" {
		return gap(g)
	}
	return change(s.Closes, back)
}

// ratPercent is (now / then - 1) * 100 exactly; the caller has checked then is positive.
func ratPercent(now, then *big.Rat) *big.Rat {
	r := new(big.Rat).Quo(now, then)
	r.Sub(r, big.NewRat(1, 1))
	return r.Mul(r, big.NewRat(100, 1))
}

// sectorExcess is the figure report.sector_excess: the change of the fund minus the change of the benchmark over
// back sessions, each unrounded, the difference rounded half-even to 2 places.
func sectorExcess(spy, s blockapi.CloseIn, day contracts.Day, back int) compute.FigureValue {
	if g := seriesGap(spy, s, day); g != "" {
		return gap(g)
	}
	if len(s.Closes) <= back || len(spy.Closes) <= back {
		return gap("no_data")
	}
	if s.Days[back] != spy.Days[back] {
		return gap("missing_input")
	}
	e0, ok1 := parse(s.Closes[0])
	en, ok2 := parse(s.Closes[back])
	b0, ok3 := parse(spy.Closes[0])
	bn, ok4 := parse(spy.Closes[back])
	if !ok1 || !ok2 || !ok3 || !ok4 || en.Sign() <= 0 || bn.Sign() <= 0 {
		return gap("missing_input")
	}
	return compute.FigureValue{Value: money(new(big.Rat).Sub(ratPercent(e0, en), ratPercent(b0, bn)))}
}

// valueOf reads the number of a figure; ok is false for a gap.
func valueOf(f compute.FigureValue) (*big.Rat, bool) {
	if f.Gap != "" {
		return nil, false
	}
	return parse(f.Value)
}

// sortByD5 orders rows by d5 descending, rows with a d5 gap last; ties keep input order.
func sortByD5(rows []blockapi.SectorLine) []blockapi.SectorLine {
	out := append([]blockapi.SectorLine(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool {
		a, okA := valueOf(out[i].D5)
		b, okB := valueOf(out[j].D5)
		if okA != okB {
			return okA
		}
		return okA && a.Cmp(b) > 0
	})
	return out
}

// rotationOf is the facts of the rotation sentence from the rows in list order and sorted by d5: nil with fewer
// than 6 rows that have a d5 or when no row has a d20. The d20 leader is the greatest d20, a tie by list order.
func rotationOf(listed, sorted []blockapi.SectorLine) *blockapi.RotationLine {
	n := 0
	for _, r := range sorted {
		if _, ok := valueOf(r.D5); ok {
			n++
		}
	}
	if n < 6 {
		return nil
	}
	var best *big.Rat
	leader20 := ""
	for _, r := range listed {
		if v, ok := valueOf(r.D20); ok && (best == nil || v.Cmp(best) > 0) {
			best, leader20 = v, r.Name
		}
	}
	if best == nil {
		return nil
	}
	last := sorted[n-1]
	return &blockapi.RotationLine{
		Leaders5: []string{sorted[0].Name, sorted[1].Name, sorted[2].Name},
		Laggard:  last.Name, LaggardD5: last.D5.Value,
		Leader20: leader20,
	}
}
