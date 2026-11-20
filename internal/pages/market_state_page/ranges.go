package market_state_page

import (
	_ "embed" // ranges.toml
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/pelletier/go-toml/v2"
)

// ranges.toml holds every band number, direction, hint and source of the five market rows; this
// file holds none (docs/design/market-state-ranges.md, Decisions 2 to 6).
//
//go:embed ranges.toml
var rangesFile []byte

// minRangePoints is the shortest history that gives a percentile cut (Decision 6).
const minRangePoints = 250

const (
	rangeNotice      = "A notice only; nothing is blocked."
	rangeOwnHistory  = "own stored history"
	dirLowBad        = "low_bad"
	dirHighBad       = "high_bad"
	basisPublished   = "published"
	basisPercentile  = "percentile"
	levelCaution     = "caution"
	levelAlarm       = "alarm"
	stateNormal      = "normal"
	rangeTrialSuffix = ", trial"
)

type sourceDef struct {
	ID    string `toml:"id"`
	Short string `toml:"short"`
	Text  string `toml:"text"`
}

type cutDef struct {
	Level      string   `toml:"level"`
	Basis      string   `toml:"basis"`
	Value      *float64 `toml:"value"`
	Percentile *float64 `toml:"percentile"`
	Sources    []string `toml:"sources"`
}

type rowDef struct {
	ID        string   `toml:"id"`
	Direction string   `toml:"direction"`
	UnitText  string   `toml:"unit_text"`
	Decimals  int      `toml:"decimals"`
	Hint      string   `toml:"hint"`
	Cut       []cutDef `toml:"cut"`
}

type rangesDoc struct {
	Source []sourceDef `toml:"source"`
	Row    []rowDef    `toml:"row"`
}

// loadRanges reads the embedded file once; a file that does not parse gives no ranges at all.
var loadRanges = sync.OnceValue(func() *rangesDoc {
	var d rangesDoc
	if err := toml.Unmarshal(rangesFile, &d); err != nil {
		return &rangesDoc{}
	}
	return &d
})

func (d *rangesDoc) row(id string) *rowDef {
	for i := range d.Row {
		if d.Row[i].ID == id {
			return &d.Row[i]
		}
	}
	return nil
}

func (d *rangesDoc) source(id string) (sourceDef, bool) {
	for _, s := range d.Source {
		if s.ID == id {
			return s, true
		}
	}
	return sourceDef{}, false
}

// hintOf is the Chinese hint of a row, empty when the file has none.
func hintOf(id string) string {
	if r := loadRanges().row(id); r != nil {
		return r.Hint
	}
	return ""
}

type rangeCut struct {
	Level  string `json:"level"`
	Text   string `json:"text"`
	Basis  string `json:"basis"`
	Source string `json:"source"`
}

type rangeState struct {
	State     string     `json:"state"`
	StateText string     `json:"state_text"`
	Cuts      []rangeCut `json:"cuts"`
	Notice    string     `json:"notice"`
}

// rangeOf is the range of row id for the reading figure, in the unit of points (the row's history
// values). A NaN figure is a Gap figure and gives a Gap range. It returns nil, so the row has no
// range field, when the file has no such row or lacks a usable caution or alarm cut; it returns the
// Gap no_data when a percentile cut has fewer than minRangePoints points. It never changes a page.
func rangeOf(id string, figure float64, points []float64) any {
	doc := loadRanges()
	def := doc.row(id)
	if def == nil || (def.Direction != dirLowBad && def.Direction != dirHighBad) {
		return nil
	}
	var caution, alarm *cutDef
	for i := range def.Cut {
		c := &def.Cut[i]
		switch {
		case !c.usable():
		case c.Level == levelCaution && caution == nil:
			caution = c
		case c.Level == levelAlarm && alarm == nil:
			alarm = c
		}
	}
	if caution == nil || alarm == nil {
		return nil
	}
	if math.IsNaN(figure) {
		return mkGap(reasonNoData, "Range: no figure to compare")
	}
	sorted := slices.Clone(points)
	slices.Sort(sorted)
	var cuts []rangeCut
	var vals [2]float64
	for i, c := range []*cutDef{caution, alarm} {
		v, text, ok := c.resolve(doc, def, sorted)
		if !ok {
			return mkGap(reasonNoData, fmt.Sprintf("Range: fewer than %d daily points stored", minRangePoints))
		}
		vals[i] = v
		cuts = append(cuts, rangeCut{Level: c.Level, Text: text, Basis: c.Basis, Source: c.sourceText(doc)})
	}
	state := stateNormal
	switch {
	case beyond(def.Direction, figure, vals[1]):
		state = levelAlarm
	case beyond(def.Direction, figure, vals[0]):
		state = levelCaution
	}
	return rangeState{State: state, StateText: strings.ToUpper(state[:1]) + state[1:] + rangeTrialSuffix,
		Cuts: cuts, Notice: rangeNotice}
}

func (c *cutDef) usable() bool {
	switch c.Basis {
	case basisPublished:
		return c.Value != nil
	case basisPercentile:
		return c.Percentile != nil && *c.Percentile > 0 && *c.Percentile <= 100
	}
	return false
}

// beyond reports whether v is past cut in the bad direction.
func beyond(direction string, v, cut float64) bool {
	if direction == dirLowBad {
		return v < cut
	}
	return v > cut
}

// resolve gives the cut value and its text; ok is false when a percentile has too few points.
// sorted is ascending.
func (c *cutDef) resolve(doc *rangesDoc, def *rowDef, sorted []float64) (float64, string, bool) {
	way := "above"
	if def.Direction == dirLowBad {
		way = "below"
	}
	level := strings.ToUpper(c.Level[:1]) + c.Level[1:]
	if c.Basis == basisPublished {
		v := *c.Value
		return v, fmt.Sprintf("%s when %s %s; published trial reading", level, way, fmtCut(v, def)), true
	}
	n := len(sorted)
	if n < minRangePoints {
		return 0, "", false
	}
	rank := int(math.Ceil(*c.Percentile / 100 * float64(n)))
	rank = min(max(rank, 1), n)
	v := sorted[rank-1]
	return v, fmt.Sprintf("%s when %s %s, the %s percentile of %d days", level, way, fmtCut(v, def),
		ordinal(strconv.FormatFloat(*c.Percentile, 'f', -1, 64)), n), true
}

func fmtCut(v float64, def *rowDef) string {
	s := strconv.FormatFloat(v, 'f', def.Decimals, 64)
	if strings.HasPrefix(s, "-") {
		if strings.Trim(s, "-0.") == "" {
			s = s[1:]
		} else {
			s = "−" + s[1:]
		}
	}
	return s + def.UnitText
}

// sourceText says where the cut comes from: the short names of its sources, or the row's own history.
func (c *cutDef) sourceText(doc *rangesDoc) string {
	var names []string
	for _, id := range c.Sources {
		if s, ok := doc.source(id); ok {
			names = append(names, id+" "+s.Short)
		}
	}
	base := rangeOwnHistory
	if c.Basis == basisPublished {
		base = "published"
	}
	if len(names) == 0 {
		return base
	}
	return base + "; source " + strings.Join(names, ", ")
}
