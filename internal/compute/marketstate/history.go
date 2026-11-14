package marketstate

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	idSp500Vs200d = "market.sp500_vs_200d"
	idVIXTerm     = "market.vix_term_ratio"
)

// history is the daily line of each of the five market rows over the newest ChartYears calendar years of its
// own series.
func history(in marketstateapi.Input) marketstateapi.History {
	p := in.Params
	return marketstateapi.History{
		Sp500Vs200d: line(in.SP500, p, func(i int) (string, bool) {
			return registered(in, idSp500Vs200d, func(d contracts.Day) compute.Figure {
				return in.Figures.MovingAverageGap(idSp500Vs200d, in.SP500[:i+1], d)
			}, in.SP500[i].Day)
		}),
		Drawdown: line(in.SP500, p, func(i int) (string, bool) {
			text, _, why := drawdownAt(in.SP500, i, p.HighSessions)
			return text, why == ""
		}),
		VIX:       line(in.VIXCLS, p, func(i int) (string, bool) { return levelText(in.VIXCLS[i].Value), true }),
		VIXTerm:   termLine(in),
		HighYield: line(in.BAMLH0A0HYM2, p, func(i int) (string, bool) { return levelText(in.BAMLH0A0HYM2[i].Value), true }),
	}
}

// registered runs one registered figure and rounds it at the scale of its rule; ok is false when there is no
// Figures, Units or Rules, when the figure is not ok or the value cannot be rounded.
func registered(in marketstateapi.Input, id string, run func(contracts.Day) compute.Figure, day contracts.Day) (string, bool) {
	if in.Figures == nil || in.Units == nil || in.Rules == nil {
		return "", false
	}
	rule, ok := in.Rules.Rule(id)
	if !ok {
		return "", false
	}
	f := run(day)
	if f.Status != compute.StatusOK {
		return "", false
	}
	text, err := in.Units.RoundHalfEven(f.Value, rule.Scale)
	return text, err == nil
}

// termLine is VIX over VIX3M on each day both have a point; a day with one leg only is no point.
func termLine(in marketstateapi.Input) marketstateapi.Line {
	at := make(map[contracts.Day]int, len(in.VXVCLS))
	for j, pt := range in.VXVCLS {
		at[pt.Day] = j
	}
	return line(in.VIXCLS, in.Params, func(i int) (string, bool) {
		j, ok := at[in.VIXCLS[i].Day]
		if !ok {
			return "", false
		}
		return registered(in, idVIXTerm, func(d contracts.Day) compute.Figure {
			return in.Figures.Ratio(idVIXTerm, in.VIXCLS[:i+1], in.VXVCLS[:j+1], d)
		}, in.VIXCLS[i].Day)
	})
}

// line collects the points of base's days inside the newest ChartYears years of base, ascending; value gives the
// text at index i of base. Total counts the points before the ChartMax cut.
func line(base compute.Series, p marketstateapi.Params, value func(i int) (string, bool)) marketstateapi.Line {
	if len(base) == 0 {
		return marketstateapi.Line{}
	}
	var cutoff contracts.Day
	if t, ok := parseDay(base[len(base)-1].Day); ok {
		cutoff = contracts.Day(t.AddDate(-p.ChartYears, 0, 0).Format("2006-01-02"))
	}
	var pts []marketstateapi.Point
	for i, pt := range base {
		if pt.Day <= cutoff {
			continue
		}
		if text, ok := value(i); ok {
			pts = append(pts, marketstateapi.Point{Day: pt.Day, Value: text})
		}
	}
	out := marketstateapi.Line{Total: len(pts)}
	if len(pts) > p.ChartMax {
		pts = pts[len(pts)-p.ChartMax:]
	}
	out.Points = pts
	return out
}
