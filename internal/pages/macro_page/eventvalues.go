package macro_page

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

// Design: docs/design/macro-event-values.md, Decisions 3 to 6, 9 and 10.
//
// The previous value and the actual of an event row, computed from stored points of FRED series. The observation is
// chosen by its reference date, never by position: a point that is absent or stored with no value is a gap that names
// it, and no value is taken from a neighbouring date.

const (
	// eventSeriesDays is how far before the page day the series of the event figures are read: a y/y figure of a
	// JOLTS-late observation needs sixteen months.
	eventSeriesDays = 560

	unitEventPercent = "percent"
	unitEventThou    = "thousands"
	unitEventMillion = "millions"
	unitEventTrill   = "usd_trillions"
	unitEventRange   = "percent_range"
)

// eventFigure is the Figure of an event: the number and unit are the data, the text is what the web prints.
type eventFigure struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
	Text  string  `json:"text"`
}

// eventCells are the three members of an event row that come from stored series. A cell is an eventFigure or a gap;
// Previous is the text of the figure, a gap, or "-" for a kind with no figure. Figures are nil for such a kind.
type eventCells struct {
	Previous       any
	PreviousFigure any
	Actual         any
}

// series is the stored points of one id by observation date. A nil value is a point stored with no value.
type series struct {
	id  string
	pts map[string]*float64
	// last is the newest observation date of any stored point.
	last time.Time
}

func (s *series) at(t time.Time) (float64, bool) {
	v, ok := s.pts[t.Format(dayLayout)]
	if !ok || v == nil {
		return 0, false
	}
	return *v, true
}

// kindSpec says how one event kind finds its observation and prints its figure.
type kindSpec struct {
	ids []string
	// expected is the observation an event dated d reports; back moves an observation n periods earlier.
	expected func(d time.Time) time.Time
	back     func(o time.Time, n int) time.Time
	// figure computes the figure of observation o; it reports the gap when a needed point is missing.
	figure func(s map[string]*series, o time.Time) (eventFigure, *gap)
	label  func(o time.Time) string
}

func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func monthBack(o time.Time, n int) time.Time { return monthStart(o).AddDate(0, -n, 0) }

func monthLabel(o time.Time) string { return o.Format("Jan 2006") }

func quarterStart(t time.Time) time.Time {
	return time.Date(t.Year(), time.Month((int(t.Month())-1)/3*3+1), 1, 0, 0, 0, 0, time.UTC)
}

func monthlyBefore(months int) func(time.Time) time.Time {
	return func(d time.Time) time.Time { return monthBack(d, months) }
}

func noPoint(id, label string, s *series, o time.Time) *gap {
	if _, stored := s.pts[o.Format(dayLayout)]; stored {
		return &gap{Reason: "no_data", Text: fmt.Sprintf(gapValueNoNumber, id, label)}
	}
	return &gap{Reason: "no_data", Text: fmt.Sprintf(gapValueNotStored, id, label)}
}

func round(v float64, places int) float64 {
	p := math.Pow10(places)
	return math.Round(v*p) / p
}

// yoy is the y/y percent figure of an index series.
func yoy(name, id string, label func(time.Time) string) func(map[string]*series, time.Time) (eventFigure, *gap) {
	return func(m map[string]*series, o time.Time) (eventFigure, *gap) {
		s := m[id]
		cur, ok := s.at(o)
		if !ok {
			return eventFigure{}, noPoint(id, name+" "+label(o), s, o)
		}
		ref := monthBack(o, 12)
		old, ok := s.at(ref)
		if !ok || old == 0 {
			return eventFigure{}, noPoint(id, name+" "+label(ref), s, ref)
		}
		v := round((cur/old-1)*100, 1)
		return eventFigure{Value: v, Unit: unitEventPercent, Text: fmt.Sprintf("%.1f%%", v)}, nil
	}
}

var specs = map[string]kindSpec{
	"cpi": {ids: []string{"CPIAUCNS"}, expected: monthlyBefore(1), back: monthBack, label: monthLabel,
		figure: yoy("CPI", "CPIAUCNS", monthLabel)},
	"pce": {ids: []string{"PCEPI"}, expected: monthlyBefore(1), back: monthBack, label: monthLabel,
		figure: yoy("PCE", "PCEPI", monthLabel)},
	"ppi": {ids: []string{"PPIFID"}, expected: monthlyBefore(1), back: monthBack, label: monthLabel,
		figure: yoy("PPI", "PPIFID", monthLabel)},
	"employment_situation": {ids: []string{"PAYEMS"}, expected: monthlyBefore(1), back: monthBack, label: monthLabel,
		figure: func(m map[string]*series, o time.Time) (eventFigure, *gap) {
			s := m["PAYEMS"]
			cur, ok := s.at(o)
			if !ok {
				return eventFigure{}, noPoint("PAYEMS", "payrolls "+monthLabel(o), s, o)
			}
			ref := monthBack(o, 1)
			old, ok := s.at(ref)
			if !ok {
				return eventFigure{}, noPoint("PAYEMS", "payrolls "+monthLabel(ref), s, ref)
			}
			v := round((cur-old)/1000, 0)
			return eventFigure{Value: v, Unit: unitEventThou, Text: fmt.Sprintf("%+.0fK", v)}, nil
		}},
	"retail_sales": {ids: []string{"RSAFS"}, expected: monthlyBefore(1), back: monthBack, label: monthLabel,
		figure: func(m map[string]*series, o time.Time) (eventFigure, *gap) {
			s := m["RSAFS"]
			cur, ok := s.at(o)
			if !ok {
				return eventFigure{}, noPoint("RSAFS", "retail sales "+monthLabel(o), s, o)
			}
			ref := monthBack(o, 1)
			old, ok := s.at(ref)
			if !ok || old == 0 {
				return eventFigure{}, noPoint("RSAFS", "retail sales "+monthLabel(ref), s, ref)
			}
			v := round((cur/old-1)*100, 1)
			return eventFigure{Value: v, Unit: unitEventPercent, Text: fmt.Sprintf("%+.1f%%", v)}, nil
		}},
	"jolts": {ids: []string{"JTSJOL"}, expected: monthlyBefore(2), back: monthBack, label: monthLabel,
		figure: func(m map[string]*series, o time.Time) (eventFigure, *gap) {
			s := m["JTSJOL"]
			cur, ok := s.at(o)
			if !ok {
				return eventFigure{}, noPoint("JTSJOL", "JOLTS "+monthLabel(o), s, o)
			}
			v := round(cur/1e6, 2)
			return eventFigure{Value: v, Unit: unitEventMillion, Text: fmt.Sprintf("%.2fM", v)}, nil
		}},
	"gdp": {ids: []string{"A191RL1Q225SBEA"}, back: func(o time.Time, n int) time.Time { return quarterStart(o).AddDate(0, -3*n, 0) },
		expected: func(d time.Time) time.Time { return quarterStart(d).AddDate(0, -3, 0) },
		label:    func(o time.Time) string { return fmt.Sprintf("Q%d %d", (int(o.Month())-1)/3+1, o.Year()) },
		figure: func(m map[string]*series, o time.Time) (eventFigure, *gap) {
			s := m["A191RL1Q225SBEA"]
			cur, ok := s.at(o)
			if !ok {
				return eventFigure{}, noPoint("A191RL1Q225SBEA", fmt.Sprintf("GDP Q%d %d", (int(o.Month())-1)/3+1, o.Year()), s, o)
			}
			v := round(cur, 1)
			return eventFigure{Value: v, Unit: unitEventPercent, Text: fmt.Sprintf("%.1f%%", v)}, nil
		}},
	"h41": {ids: []string{"WALCL"}, label: func(o time.Time) string { return o.Format(dayLayout) },
		back: func(o time.Time, n int) time.Time { return o.AddDate(0, 0, -7*n) },
		expected: func(d time.Time) time.Time {
			return d.AddDate(0, 0, -((int(d.Weekday())-int(time.Wednesday))+7)%7)
		},
		figure: func(m map[string]*series, o time.Time) (eventFigure, *gap) {
			s := m["WALCL"]
			cur, ok := s.at(o)
			if !ok {
				return eventFigure{}, noPoint("WALCL", "H.4.1 "+o.Format(dayLayout), s, o)
			}
			v := round(cur/1e12, 2)
			return eventFigure{Value: v, Unit: unitEventTrill, Text: fmt.Sprintf("$%.2fT", v)}, nil
		}},
	"fomc": {ids: []string{"DFEDTARU", "DFEDTARL"}, label: func(o time.Time) string { return o.Format(dayLayout) },
		back: func(o time.Time, n int) time.Time { return o.AddDate(0, 0, -n) },
		figure: func(m map[string]*series, o time.Time) (eventFigure, *gap) {
			up, ok := m["DFEDTARU"].at(o)
			if !ok {
				return eventFigure{}, noPoint("DFEDTARU", "target "+o.Format(dayLayout), m["DFEDTARU"], o)
			}
			lo, ok := m["DFEDTARL"].at(o)
			if !ok {
				return eventFigure{}, noPoint("DFEDTARL", "target "+o.Format(dayLayout), m["DFEDTARL"], o)
			}
			return eventFigure{Value: up, Unit: unitEventRange, Text: fmt.Sprintf("%.2f-%.2f%%", lo, up)}, nil
		}},
}

// loadSeries reads the series the kinds of events need. A read error is an error; an id with no point maps to an
// empty series.
func loadSeries(ctx context.Context, r store.SeriesReader, events []macroevent.Event, d time.Time) (map[string]*series, error) {
	out := map[string]*series{}
	from := contracts.Day(d.AddDate(0, 0, -eventSeriesDays).Format(dayLayout))
	to := contracts.Day(d.Format(dayLayout))
	for _, e := range events {
		for _, id := range specs[e.Kind].ids {
			if out[id] != nil {
				continue
			}
			rows, err := r.SeriesPoints(ctx, id, from, to)
			if err != nil {
				return nil, fmt.Errorf("macro_page: read %s: %w", id, err)
			}
			out[id] = newestVintage(id, rows, d)
		}
	}
	return out, nil
}

// newestVintage keeps, per observation date, the row with the greatest RealtimeStart not after d.
func newestVintage(id string, rows []contracts.SeriesPoint, d time.Time) *series {
	s := &series{id: id, pts: map[string]*float64{}}
	rt := map[string]time.Time{}
	for _, p := range rows {
		if p.RealtimeStart.After(d) {
			continue
		}
		k := p.ObservationDate.Format(dayLayout)
		if cur, ok := rt[k]; ok && !p.RealtimeStart.After(cur) {
			continue
		}
		rt[k] = p.RealtimeStart.Time
		s.pts[k] = p.Value
		if p.ObservationDate.After(s.last) {
			s.last = p.ObservationDate.Time
		}
	}
	return s
}

func missingSeries(spec kindSpec, m map[string]*series) *gap {
	for _, id := range spec.ids {
		if len(m[id].pts) == 0 {
			return &gap{Reason: "no_data", Text: id + " has no stored point"}
		}
	}
	return nil
}

func (c eventCells) withPrevious(f eventFigure, g *gap) eventCells {
	if g != nil {
		c.Previous, c.PreviousFigure = *g, *g
		return c
	}
	c.Previous, c.PreviousFigure = f.Text, f
	return c
}

func (c eventCells) withActual(f eventFigure, g *gap) eventCells {
	if g != nil {
		c.Actual = *g
		return c
	}
	c.Actual = f
	return c
}

// laterGdp reports whether a gdp event is a second or third estimate: another gdp event of the same calendar
// quarter has an earlier date.
func laterGdp(ctx context.Context, rd macroevent.Reader, e macroevent.Event, cache map[string][]string) (bool, error) {
	d, err := time.Parse(dayLayout, string(e.Date))
	if err != nil {
		return false, nil
	}
	qs := quarterStart(d)
	key := qs.Format(dayLayout)
	dates, ok := cache[key]
	if !ok {
		from := contracts.Day(key)
		to := contracts.Day(qs.AddDate(0, 3, -1).Format(dayLayout))
		rows, err := rd.Between(ctx, from, to)
		if err != nil {
			return false, fmt.Errorf("macro_page: read gdp events: %w", err)
		}
		for _, r := range rows {
			if r.Kind == "gdp" {
				dates = append(dates, string(r.Date))
			}
		}
		sort.Strings(dates)
		cache[key] = dates
	}
	return len(dates) > 0 && dates[0] < string(e.Date), nil
}

// eventValues returns the cells of each event in order. rd is read only for the earlier estimates of a quarter of GDP.
func eventValues(ctx context.Context, r store.SeriesReader, rd macroevent.Reader, events []macroevent.Event, d time.Time) ([]eventCells, error) {
	m, err := loadSeries(ctx, r, events, d)
	if err != nil {
		return nil, err
	}
	gdpCache := map[string][]string{}
	out := make([]eventCells, len(events))
	for i, e := range events {
		spec, ok := specs[e.Kind]
		if !ok {
			out[i] = eventCells{Previous: "-"}
			continue
		}
		ed, err := time.Parse(dayLayout, string(e.Date))
		if err != nil {
			out[i] = eventCells{Previous: gap{Reason: "no_data", Text: gapEventDateUnread}}
			continue
		}
		var cells eventCells
		if g := missingSeries(spec, m); g != nil {
			cells = eventCells{Previous: *g, PreviousFigure: *g, Actual: *g}
		} else if e.Kind == "fomc" {
			cells = fomcCells(spec, m, ed, d)
		} else {
			cells = periodCells(spec, m, ed, d)
		}
		if e.Kind == "gdp" {
			later, err := laterGdp(ctx, rd, e, gdpCache)
			if err != nil {
				return nil, err
			}
			if later {
				g := gap{Reason: "no_data", Text: gapEarlierEstimate}
				cells.Previous, cells.PreviousFigure = g, g
			}
		}
		out[i] = cells
	}
	return out, nil
}

// periodCells: a past event shows the figure of its observation as actual and the one before as previous; a later
// event shows the newest stored figure as previous when it is the period before the expected one.
func periodCells(spec kindSpec, m map[string]*series, ed, d time.Time) eventCells {
	var c eventCells
	o := spec.expected(ed)
	prev := spec.back(o, 1)
	if ed.Before(d) {
		f, g := spec.figure(m, prev)
		c = c.withPrevious(f, g)
		f, g = spec.figure(m, o)
		return c.withActual(f, g)
	}
	s := m[spec.ids[0]]
	if !s.last.Equal(prev) {
		g := gap{Reason: "stale", Text: "latest stored: " + s.last.Format(dayLayout)}
		c.Previous, c.PreviousFigure = g, g
	} else {
		f, g := spec.figure(m, prev)
		c = c.withPrevious(f, g)
	}
	c.Actual = gap{Reason: "no_data", Text: gapNotYetReleased}
	return c
}

// fomcCells: previous is the target on the day before the meeting, actual the target on the day after; a later
// event shows the newest stored target when it is recent.
func fomcCells(spec kindSpec, m map[string]*series, ed, d time.Time) eventCells {
	var c eventCells
	if ed.Before(d) {
		f, g := spec.figure(m, ed.AddDate(0, 0, -1))
		c = c.withPrevious(f, g)
		f, g = spec.figure(m, ed.AddDate(0, 0, 1))
		if g != nil {
			g = &gap{Reason: "no_data", Text: gapNotYetPublished}
		}
		return c.withActual(f, g)
	}
	last := m["DFEDTARU"].last
	if d.Sub(last) > staleFetchDays*24*time.Hour {
		g := gap{Reason: "stale", Text: "latest stored: " + last.Format(dayLayout)}
		c.Previous, c.PreviousFigure = g, g
	} else {
		f, g := spec.figure(m, last)
		c = c.withPrevious(f, g)
	}
	c.Actual = gap{Reason: "no_data", Text: gapNotYetReleased}
	return c
}
