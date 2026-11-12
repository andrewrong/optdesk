package contracttest

import (
	"bytes"
	"encoding/json"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// ruleIn is the rule object of a figure case.
type ruleIn struct {
	ID          string          `json:"id"`
	Version     int             `json:"version"`
	Convention  string          `json:"convention"`
	Unit        string          `json:"unit"`
	Scale       int             `json:"scale"`
	Compare     compute.Compare `json:"compare"`
	N           int             `json:"n"`
	Window      int             `json:"window"`
	MinValid    int             `json:"min_valid"`
	Years       int             `json:"years"`
	TradingDays int             `json:"trading_days"`
	MaxAgeDays  int             `json:"max_age_days"`
	Hash        string          `json:"hash"`
}

// seriesIn is a series of a figure case: explicit points, or values on the consecutive
// calendar days from first_day.
type seriesIn struct {
	FirstDay string     `json:"first_day"`
	Values   []float64  `json:"values"`
	Points   []dayPoint `json:"points"`
}

func (s seriesIn) series(t T) compute.Series {
	t.Helper()
	if s.FirstDay == "" {
		return toSeries(s.Points)
	}
	first := date(t, s.FirstDay).Time
	out := make(compute.Series, len(s.Values))
	for i, v := range s.Values {
		out[i] = compute.Point{Day: contracts.Day(first.AddDate(0, 0, i).Format(time.DateOnly)), Value: v}
	}
	return out
}

// figureIn is the input of a figure case; a case sets the series its method takes.
type figureIn struct {
	Rule   ruleIn      `json:"rule"`
	AsOf   string      `json:"as_of"`
	Closes seriesIn    `json:"closes"`
	Iv30   seriesIn    `json:"iv30"`
	Series seriesIn    `json:"series"`
	Num    seriesIn    `json:"num"`
	Den    seriesIn    `json:"den"`
	Rate   *dayPoint   `json:"rate"`
	Chain  []chainItem `json:"chain"`
	Parts  []partIn    `json:"parts"`
}

// partIn is one input Figure of a Combine case.
type partIn struct {
	Status  compute.Status `json:"status"`
	Reason  compute.Reason `json:"reason"`
	Value   float64        `json:"value"`
	DataDay string         `json:"data_day"`
}

func (p partIn) figure() compute.Figure {
	return compute.Figure{ID: "part", Version: 1, Day: contracts.Day(p.DataDay), DataDay: contracts.Day(p.DataDay),
		Value: p.Value, Status: p.Status, Reason: p.Reason, Observations: 1}
}

type chainItem struct {
	MinutesToExpiry int `json:"minutes_to_expiry"`
	Strikes         []struct {
		Strike  float64 `json:"strike"`
		CallIV  float64 `json:"call_iv"`
		PutIV   float64 `json:"put_iv"`
		CallMid float64 `json:"call_mid"`
		PutMid  float64 `json:"put_mid"`
		HasCall bool    `json:"has_call"`
		HasPut  bool    `json:"has_put"`
	} `json:"strikes"`
}

func (c chainItem) quotes() compute.ExpiryQuotes {
	e := compute.ExpiryQuotes{MinutesToExpiry: c.MinutesToExpiry}
	for _, s := range c.Strikes {
		e.Strikes = append(e.Strikes, compute.StrikeQuote(s))
	}
	return e
}

// figureWant is the expected Figure of a case; stale_rate is checked only when it is present.
type figureWant struct {
	Status       compute.Status `json:"status"`
	Reason       compute.Reason `json:"reason"`
	Value        float64        `json:"value"`
	DataDay      string         `json:"data_day"`
	Observations int            `json:"observations"`
	StaleRate    *bool          `json:"stale_rate"`
}

// fixedRules is the Rules of one case: the one rule of its rule object.
type fixedRules struct{ rule compute.Rule }

func (r fixedRules) Rule(id string) (compute.Rule, bool) { return r.rule, id == r.rule.ID }
func (r fixedRules) IDs() []string                       { return []string{r.rule.ID} }

// Figures runs the cases of figures.json against the Figures that newFigures builds for each
// case from a Rules holding the rule of the case. methods selects cases by group, the name of
// a Figures method: no name runs every case, and a name that no case carries fails the suite.
func Figures(t T, newFigures compute.NewFiguresFunc, stats compute.Stats, rolling compute.Rolling, ops compute.SeriesOps, methods ...string) {
	t.Helper()
	file := load(t, "figures.json")
	for _, m := range methods {
		if !slices.ContainsFunc(file.Cases, func(c Case) bool { return c.Group == m }) {
			t.Fatalf("no case of figures.json has the group %q", m)
		}
	}
	call := func(fn func(f compute.Figures, in figureIn, asOf contracts.Day, t T) compute.Figure) func(T, Case) {
		return func(t T, c Case) {
			var in figureIn
			decode(t, c.Input, &in)
			var raw struct {
				Rule json.RawMessage `json:"rule"`
			}
			decode(t, c.Input, &raw)
			dec := json.NewDecoder(bytes.NewReader(raw.Rule))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&in.Rule); err != nil {
				t.Fatalf("rule of %s: %v", c.Name, err)
			}
			var want figureWant
			decode(t, c.Expected, &want)
			r := in.Rule
			rule := compute.Rule{ID: r.ID, Version: r.Version, Convention: r.Convention, Unit: r.Unit, Scale: r.Scale,
				Compare: r.Compare, N: r.N, Window: r.Window, MinValid: r.MinValid, Years: r.Years,
				TradingDays: r.TradingDays, MaxAgeDays: r.MaxAgeDays, Hash: r.Hash}
			asOf := contracts.Day(in.AsOf)
			got := fn(newFigures(stats, rolling, ops, fixedRules{rule}), in, asOf, t)
			checkFigure(t, c, rule, asOf, got, want)
		}
	}
	handlers := map[string]func(T, Case){
		"HistoricalVol": call(func(f compute.Figures, in figureIn, asOf contracts.Day, t T) compute.Figure {
			return f.HistoricalVol(in.Rule.ID, in.Closes.series(t), asOf)
		}),
		"Iv30": call(func(f compute.Figures, in figureIn, asOf contracts.Day, t T) compute.Figure {
			if in.Rate == nil {
				t.Fatalf("Iv30 case has no rate")
			}
			chain := make([]compute.ExpiryQuotes, len(in.Chain))
			for i, e := range in.Chain {
				chain[i] = e.quotes()
			}
			rate := compute.Point{Day: contracts.Day(in.Rate.Day), Value: in.Rate.Value}
			return f.Iv30(in.Rule.ID, chain, rate, asOf)
		}),
		"IvPercentile": call(func(f compute.Figures, in figureIn, asOf contracts.Day, t T) compute.Figure {
			return f.IvPercentile(in.Rule.ID, in.Iv30.series(t), asOf)
		}),
		"IvRank": call(func(f compute.Figures, in figureIn, asOf contracts.Day, t T) compute.Figure {
			return f.IvRank(in.Rule.ID, in.Iv30.series(t), asOf)
		}),
		"MovingAverageGap": call(func(f compute.Figures, in figureIn, asOf contracts.Day, t T) compute.Figure {
			return f.MovingAverageGap(in.Rule.ID, in.Closes.series(t), asOf)
		}),
		"PercentileInYears": call(func(f compute.Figures, in figureIn, asOf contracts.Day, t T) compute.Figure {
			return f.PercentileInYears(in.Rule.ID, in.Series.series(t), asOf)
		}),
		"Combine": call(func(f compute.Figures, in figureIn, asOf contracts.Day, t T) compute.Figure {
			parts := make([]compute.Figure, len(in.Parts))
			for i, p := range in.Parts {
				parts[i] = p.figure()
			}
			return f.Combine(in.Rule.ID, parts, asOf)
		}),
		"Ratio": call(func(f compute.Figures, in figureIn, asOf contracts.Day, t T) compute.Figure {
			return f.Ratio(in.Rule.ID, in.Num.series(t), in.Den.series(t), asOf)
		}),
	}
	if len(methods) > 0 {
		for g := range handlers {
			if !slices.Contains(methods, g) {
				delete(handlers, g)
			}
		}
	}
	runCases(t, file, handlers)
}

// checkFigure compares a Figure with the case's expectation and with the rule and day it came from.
func checkFigure(t T, c Case, rule compute.Rule, asOf contracts.Day, got compute.Figure, want figureWant) {
	t.Helper()
	if got.ID != rule.ID || got.Version != rule.Version || got.Day != asOf {
		t.Errorf("%s: figure is %s v%d on %s, want %s v%d on %s", c.Name, got.ID, got.Version, got.Day, rule.ID, rule.Version, asOf)
	}
	if got.Status != want.Status || got.Reason != want.Reason {
		t.Errorf("%s: status %q reason %q, want %q %q", c.Name, got.Status, got.Reason, want.Status, want.Reason)
	}
	if !near(got.Value, want.Value, c.Tolerance) {
		t.Errorf("%s: value %v, want %v (tolerance %v)", c.Name, got.Value, want.Value, c.Tolerance)
	}
	if string(got.DataDay) != want.DataDay {
		t.Errorf("%s: data day %q, want %q", c.Name, got.DataDay, want.DataDay)
	}
	if got.Observations != want.Observations {
		t.Errorf("%s: observations %d, want %d", c.Name, got.Observations, want.Observations)
	}
	if want.StaleRate != nil && got.StaleRate != *want.StaleRate {
		t.Errorf("%s: stale rate %v, want %v", c.Name, got.StaleRate, *want.StaleRate)
	}
}
