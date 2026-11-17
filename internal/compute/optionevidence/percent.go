package optionevidence

import "github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"

// pctText is the fraction times 100 at four decimals; "" for an empty or non-number input.
func pctText(fraction string) string {
	d, ok := parse(fraction)
	if !ok {
		return ""
	}
	return d.Mul(hundred).StringFixed(ratioPlaces)
}

// fillPercents writes the six percent fields of ev from its Skew and the fractions of r.
func fillPercents(ev *optionevidenceapi.Evidence, r optionevidenceapi.Row) {
	ev.SkewPct = pctText(ev.Skew)
	ev.ATMIVPct = pctText(r.ATMIV)
	ev.ForecastPct = pctText(r.Forecast)
	ev.ForecastErrPct = pctText(r.ForecastErr)
	ev.IVPPct = pctText(r.IVP)
	ev.VolEdgePct = pctText(r.VolEdge)
}
