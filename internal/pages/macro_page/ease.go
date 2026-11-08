package macro_page

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	easeName    = "macro.ease"
	easeVersion = 2
	easeKind    = "market"
	easeSubject = "WALCL"
	// easeWeekDays is how far back the reading of the week change lies.
	easeWeekDays = 7
	// The edges of the neutral band as drawn on the track: the band is taken on the value
	// rounded to a whole number, so 34 is drawn from 33.5 and 66 up to 66.5.
	easeBandLow  = "33.5"
	easeBandHigh = "66.5"
	minus        = "−"
)

// decimalText is a stored value: optional minus, digits, optional fraction. A fraction
// "1/2" or an exponent is not accepted.
var decimalText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// easeFigure is a figure whose value is the rounded decimal text written as a JSON number.
type easeFigure struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
	Tone  string      `json:"tone,omitempty"`
}

type easeBand struct {
	Low  easeFigure `json:"low"`
	High easeFigure `json:"high"`
}

type easeBlock struct {
	Figure        easeFigure `json:"figure"`
	WeekChange    any        `json:"week_change"`
	MarkerPercent easeFigure `json:"marker_percent"`
	NeutralBand   easeBand   `json:"neutral_band"`
	Ticks         []string   `json:"ticks"`
	ReadingText   string     `json:"reading_text"`
	AriaText      string     `json:"aria_text"`
}

// easeReading is the winner reading of one day that has a value.
type easeReading struct {
	value   *big.Rat
	band    string
	dataDay time.Time
}

var bandWords = map[string]string{"tighter": "偏紧", "steady": "中性", "looser": "偏松"}

// readEase reads macro.ease for day; nil with no error means no usable reading. An error
// of read is returned.
func readEase(ctx context.Context, read compute.ReadingFunc, day time.Time) (*easeReading, error) {
	d := contracts.Day(day.Format(dayLayout))
	rows, err := read(ctx, easeName, easeKind, easeSubject, d, d)
	if err != nil {
		return nil, fmt.Errorf("macro_page: read %s: %w", easeName, err)
	}
	var best *contracts.ReadingRecord
	for i := range rows {
		r := &rows[i]
		if r.Name != easeName || r.Version != easeVersion || r.Day.Format(dayLayout) != string(d) {
			continue
		}
		if best == nil || r.Rev > best.Rev {
			best = r
		}
	}
	if best == nil || string(best.State) != "ok" || best.Value == nil {
		return nil, nil
	}
	if !decimalText.MatchString(*best.Value) {
		return nil, fmt.Errorf("macro_page: %s value %q is not decimal text", easeName, *best.Value)
	}
	v, _ := new(big.Rat).SetString(*best.Value)
	if v.Sign() < 0 || v.Cmp(big.NewRat(100, 1)) > 0 {
		return nil, fmt.Errorf("macro_page: %s value %q outside 0 to 100", easeName, *best.Value)
	}
	out := &easeReading{value: v, dataDay: day}
	for _, p := range best.Parameters {
		if p.ValueTag == nil {
			continue
		}
		switch p.Param {
		case "band":
			out.band = *p.ValueTag
		case "data_day":
			if t, err := time.Parse(dayLayout, *p.ValueTag); err == nil {
				out.dataDay = t
			}
		}
	}
	if _, ok := bandWords[out.band]; !ok {
		return nil, nil
	}
	return out, nil
}

// roundHalfUp rounds r to places decimals, halves away from zero, and returns the exact
// decimal text.
func roundHalfUp(r *big.Rat, places int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	x := new(big.Rat).Mul(r, new(big.Rat).SetInt(scale))
	neg := x.Sign() < 0
	x.Abs(x)
	x.Add(x, big.NewRat(1, 2))
	n := new(big.Int).Quo(x.Num(), x.Denom())
	if neg {
		n.Neg(n)
	}
	return new(big.Rat).SetFrac(n, scale).FloatString(places)
}

// signedText writes the sign of a change: a plus, or U+2212 for a minus; zero has none.
func signedText(num string) string {
	switch {
	case strings.HasPrefix(num, "-"):
		return minus + num[1:]
	case strings.Trim(num, "0.") == "":
		return num
	}
	return "+" + num
}

func trackFigure(num, text string) easeFigure {
	return easeFigure{Value: json.Number(num), Text: text, Unit: "percent_of_track"}
}

// easeFor builds the ease block from the readings of day and of the day a week before. It
// returns nil when the day has no usable reading.
func easeFor(ctx context.Context, c Config, day time.Time) (*easeBlock, *easeReading, error) {
	cur, err := readEase(ctx, c.Read, day)
	if err != nil || cur == nil {
		return nil, nil, err
	}
	prev, err := readEase(ctx, c.Read, day.AddDate(0, 0, -easeWeekDays))
	if err != nil {
		return nil, nil, err
	}
	whole := roundHalfUp(cur.value, 0)
	word := bandWords[cur.band]
	b := &easeBlock{
		Figure:        easeFigure{Value: json.Number(roundHalfUp(cur.value, 1)), Text: whole, Unit: "index_points"},
		MarkerPercent: trackFigure(roundHalfUp(cur.value, 2), whole),
		NeutralBand: easeBand{
			Low:  trackFigure(easeBandLow, easeBandLow+"%"),
			High: trackFigure(easeBandHigh, easeBandHigh+"%"),
		},
		Ticks:      []string{"0", "25", "50", "75", "100"},
		WeekChange: gap{Reason: "no_data", Text: "较上周暂无数据"},
	}
	weekText := "较上周暂无数据"
	if prev != nil {
		num := roundHalfUp(new(big.Rat).Sub(cur.value, prev.value), 1)
		text := signedText(num)
		tone := "flat"
		if strings.HasPrefix(text, "+") {
			tone = "up"
		} else if strings.HasPrefix(text, minus) {
			tone = "down"
		}
		b.WeekChange = easeFigure{Value: json.Number(num), Text: text, Unit: "index_points", Tone: tone}
		weekText = "较上周 " + text
	}
	b.ReadingText = "当前 " + whole + "，" + word + "；固定刻度 0 到 100。"
	b.AriaText = "松紧度 " + whole + "，" + word + "，" + weekText +
		"；固定刻度 0 到 100，中性带 " + easeBandLow + " 到 " + easeBandHigh + "。"
	return b, cur, nil
}
