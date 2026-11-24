package option_result_page

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
)

type point struct {
	Label string      `json:"label"`
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Muted bool        `json:"muted"`
}

type series struct {
	Title   string  `json:"title"`
	Points  []point `json:"points"`
	Summary *string `json:"summary"`
	Empty   *string `json:"empty"`
}

// mkSeries is a series of pts in the given order: summary is the points' "label text" joined by "; ", empty is
// the sentence when there is no point.
func mkSeries(title, empty string, pts []point) series {
	s := series{Title: title, Points: pts}
	if len(pts) == 0 {
		s.Points = []point{}
		s.Empty = &empty
		return s
	}
	parts := make([]string, len(pts))
	for i, p := range pts {
		parts[i] = p.Label + " " + p.Text
	}
	sum := strings.Join(parts, "; ")
	s.Summary = &sum
	return s
}

// monthSeries are the month and the running series, in the order of the rule; a month that is not over is muted.
func monthSeries(months []compute.ResultMonth, cur string) (m, r series, err error) {
	var mp, rp []point
	for _, x := range months {
		v, err := number(x.Realized)
		if err != nil {
			return m, r, err
		}
		run, err := number(x.Running)
		if err != nil {
			return m, r, err
		}
		mp = append(mp, point{Label: x.Month, Value: v, Text: moneyText(x.Realized, cur), Muted: !x.Over})
		rp = append(rp, point{Label: x.Month, Value: run, Text: moneyText(x.Running, cur), Muted: !x.Over})
	}
	return mkSeries("Monthly realized", emptyMonths, mp), mkSeries("Cumulative realized", emptyRunning, rp), nil
}

// endingSeries is one point per ending, value its realized result; endingLines are the same as lines.
func endingSeries(ends []compute.ResultEnding, cur string) (series, []string, error) {
	var pts []point
	lines := []string{}
	for _, e := range ends {
		v, err := number(e.Realized)
		if err != nil {
			return series{}, nil, err
		}
		text := strconv.Itoa(e.Count) + tradeUnit + endingRealized + moneyText(e.Realized, cur)
		pts = append(pts, point{Label: e.Status, Value: v, Text: text})
		lines = append(lines, e.Status+": "+text)
	}
	return mkSeries("Endings", emptyEndings, pts), lines, nil
}
