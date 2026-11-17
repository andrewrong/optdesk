package optionevidence

import "github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"

// Label names; a label changes no figure and no gate.
const (
	labelNoEdge = "no_edge_at_bid"
	labelDelta  = "delta_outside_band"
	labelExDiv  = "ex_div_inside_life"
)

// labels lists the soft labels of a row, never nil.
func (e *evaluator) labels(r optionevidenceapi.Row, ev optionevidenceapi.Evidence) []string {
	out := []string{}
	if edge, ok := parse(ev.EdgePerShare); ok && !edge.IsPositive() {
		out = append(out, labelNoEdge)
	}
	if d, ok := parse(r.Delta); ok {
		lo, okLo := parse(e.p.DeltaLow)
		hi, okHi := parse(e.p.DeltaHigh)
		if a := d.Abs(); okLo && okHi && (a.LessThan(lo) || a.GreaterThan(hi)) {
			out = append(out, labelDelta)
		}
	}
	if r.ExDiv != "" && r.ExDiv > r.ChainDay && r.ExDiv <= r.Expiry {
		out = append(out, labelExDiv)
	}
	return out
}
