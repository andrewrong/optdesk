package structures_page

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	maxLegs      = 8
	maxFills     = 50
	maxGapsShown = 20
)

// kindGlosses is the gloss that ends the detail subline of a recognised kind; the old kinds have none.
var kindGlosses = map[string]string{
	"put_debit_spread":    glossPutDebitSpread,
	"call_debit_spread":   glossCallDebitSpread,
	"calendar_spread":     glossCalendarSpread,
	"diagonal_spread":     glossDiagonalSpread,
	"long_straddle":       glossLongStraddle,
	"short_straddle":      glossShortStraddle,
	"long_strangle":       glossLongStrangle,
	"short_strangle":      glossShortStrangle,
	"iron_condor":         glossIronCondor,
	"reverse_iron_condor": glossReverseIronCondor,
	"collar":              glossCollar,
}

type tile struct {
	Label   string `json:"label"`
	Figure  any    `json:"figure"`
	Caption string `json:"caption"`
}

type tiles struct {
	PnL       tile `json:"pnl"`
	MaxGain   tile `json:"max_gain"`
	MaxLoss   tile `json:"max_loss"`
	BreakEven tile `json:"break_even"`
}

type legsTotal struct {
	Label string `json:"label"`
	PnL   any    `json:"pnl"`
}

type level struct {
	Name     string `json:"name"`
	Price    usd    `json:"price"`
	Note     string `json:"note"`
	Distance any    `json:"distance"`
}

type detailModel struct {
	Schema        string       `json:"schema"`
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	BuildID       string       `json:"build_id"`
	Scope         string       `json:"scope"`
	GeneratedAt   string       `json:"generated_at"`
	DataTimes     []sourceTime `json:"data_times"`
	RunStatus     string       `json:"run_status"`
	Title         string       `json:"title"`
	Subline       string       `json:"subline"`
	Sentence      string       `json:"sentence"`
	LegsTitle     string       `json:"legs_title"`
	PayoffTitle   string       `json:"payoff_title"`
	GreeksTitle   string       `json:"greeks_title"`
	LevelsTitle   string       `json:"levels_title"`
	TimelineTitle string       `json:"timeline_title"`
	FillsTitle    string       `json:"fills_title"`
	Tiles         tiles        `json:"tiles"`
	Legs          []any        `json:"legs"`
	LegsTotal     legsTotal    `json:"legs_total"`
	Payoff        any          `json:"payoff"`
	Greeks        any          `json:"greeks"`
	KeyLevels     []level      `json:"key_levels"`
	Timeline      []stage      `json:"timeline"`
	Fills         []fill       `json:"fills"`
	Gaps          []gap        `json:"gaps"`
	Stale         staleLabel   `json:"stale"`
}

// newDetail is the page with its fixed text and empty lists; the caller fills the rest.
func newDetail(c Config, alias string, day contracts.Day, at time.Time) (*detailModel, error) {
	dayAt, err := midnight(day, c.Zone)
	if err != nil {
		return nil, fmt.Errorf("structures_page: bad day %q: %w", day, err)
	}
	next, err := shiftDay(day, 1)
	if err != nil {
		return nil, err
	}
	staleAt, err := midnight(next, c.Zone)
	if err != nil {
		return nil, err
	}
	return &detailModel{
		Schema: "structure_detail", SchemaVersion: 3, RunID: c.RunID, BuildID: c.BuildID, Scope: alias,
		GeneratedAt: at.In(c.Zone).Format(time.RFC3339), DataTimes: []sourceTime{{Source: "option_ledger", DataTime: dayAt}},
		RunStatus: "ok", Title: "Structure detail",
		LegsTitle: zhLegsTitle + " (Leg detail)", PayoffTitle: zhPayoffTitle + " (Payoff at expiry)",
		GreeksTitle: zhGreeksTitle + " (Greeks)", LevelsTitle: zhLevelsTitle + " (Key levels)",
		TimelineTitle: "Timeline", FillsTitle: "Fills",
		Legs: []any{}, KeyLevels: []level{}, Timeline: []stage{}, Fills: []fill{}, Gaps: []gap{},
		Stale: staleLabel{DataTime: dayAt, StaleAfter: staleAt, Text: "Ledger and closing prices as of " + string(day)},
	}, nil
}

// buildDetail writes the structure_detail page of structureID. It computes nothing: it reads, calls the two
// rules, finds the structure by id and copies their text.
func buildDetail(ctx context.Context, c Config, alias, structureID string, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, fmt.Errorf("structures_page: nil zone")
	}
	if alias == "" && len(c.Aliases) > 0 {
		alias = c.Aliases[0] // the main alias is the first
	}
	if !slices.Contains(c.Aliases, alias) {
		return nil, fmt.Errorf("structures_page: unknown alias %q", alias)
	}
	vm, err := newDetail(c, alias, day, at)
	if err != nil {
		return nil, err
	}
	if c.Ledger == nil || c.Sessions == nil {
		vm.gapPage("failed", mkGap(reasonNoReader), cannotReadText, structureID)
		return json.Marshal(vm)
	}
	in, _, err := c.load(ctx, alias, day)
	if err != nil {
		return nil, err
	}
	res := c.Structures().Evaluate(in, compute.StructureParams{MarkMaxAgeSessions: markMaxAgeSessions,
		CaptureTarget: captureTarget, ExitDTE: exitDTE, CommissionDays: commissionDays})
	s, ok := findByID(res, structureID)
	if !ok {
		vm.gapPage("partial", gap{Reason: reasonNoData, Text: notOpenText}, notOpenText, structureID)
		return json.Marshal(vm)
	}
	b := newBuilder(c, s, in)
	b.fill(vm)
	return json.Marshal(vm)
}

// findByID is the structure with the id among the recognised ones and the unrecognised ones.
func findByID(res compute.StructureResult, id string) (compute.Structure, bool) {
	for _, list := range [][]compute.Structure{res.Structures, res.Other} {
		if i := slices.IndexFunc(list, func(s compute.Structure) bool { return s.ID == id }); i >= 0 {
			return list[i], true
		}
	}
	return compute.Structure{}, false
}

// gapPage fills the page of a structure that cannot be shown: every block is the one gap.
func (vm *detailModel) gapPage(status string, g gap, sentence, id string) {
	vm.RunStatus, vm.Sentence, vm.Subline = status, sentence, clip("Structure "+id)
	label := func(l string, caption string) tile { return tile{Label: l, Figure: g, Caption: caption} }
	vm.Tiles = tiles{
		PnL: label("Current P&L", captionPnL), MaxGain: label("Max profit", captionAtExpiry),
		MaxLoss: label("Max loss", captionAtExpiry), BreakEven: label("Breakeven", captionExpiryPrice),
	}
	vm.Legs = []any{g}
	vm.LegsTotal = legsTotal{Label: "Total", PnL: g}
	vm.Payoff, vm.Greeks = g, g
	vm.Gaps = []gap{g}
}

// builder holds what the blocks of one structure share: the cells, the structure, the ledger pieces by
// position id, the quotes and the gaps met so far.
type builder struct {
	cells
	s      compute.Structure
	pos    map[string]compute.LedgerPosition
	quotes []compute.StructureQuote
	gaps   []gap
	zone   *time.Location
}

func newBuilder(c Config, s compute.Structure, in compute.StructureInput) *builder {
	b := &builder{noQuotes: c.Eod == nil || c.Terms == nil, s: s, quotes: in.Quotes, zone: c.Zone,
		pos: map[string]compute.LedgerPosition{}}
	for _, p := range in.Positions {
		b.pos[p.PositionID] = p
	}
	return b
}

// note records a gap cell once and passes the cell on.
func (b *builder) note(v any) any {
	if g, ok := v.(gap); ok {
		b.addGap(g)
	}
	return v
}

func (b *builder) addGap(g gap) {
	if len(b.gaps) < maxGapsShown && !slices.Contains(b.gaps, g) {
		b.gaps = append(b.gaps, g)
	}
}

func (b *builder) foreign() bool { return b.s.Currency != "" && b.s.Currency != "USD" }

// missingQuote is the gap of a figure that needs a quote.
func (b *builder) missingQuote() gap { return mkGap(b.reasonOf(compute.FigureValue{Gap: reasonQuote})) }

func (b *builder) fill(vm *detailModel) {
	s := b.s
	kind := kindTexts[s.Kind]
	if kind == "" {
		kind = kindOther
	}
	vm.Title = clip(s.Underlying + " " + kind)
	if s.Underlying == "" {
		vm.Title = kind
	}
	expiry := earliestExpiry(s)
	vm.Subline = kind + " · expiry " + expiry + " · account " + s.Alias
	if g := kindGlosses[s.Kind]; g != "" {
		vm.Subline += " · " + g
	}
	vm.Subline = clip(vm.Subline)
	vm.Tiles = tiles{
		PnL:       tile{Label: "Current P&L", Figure: b.note(b.money(s.Figures.UnrealizedPnL)), Caption: captionPnL},
		MaxGain:   tile{Label: "Max profit", Figure: b.note(b.money(s.Figures.MaxGain)), Caption: captionAtExpiry},
		MaxLoss:   tile{Label: "Max loss", Figure: b.note(b.maxLossCell(s)), Caption: captionAtExpiry},
		BreakEven: tile{Label: "Breakeven", Figure: b.note(b.money(s.Figures.BreakEven)), Caption: breakEvenCaption(s)},
	}
	vm.Legs = b.legs()
	vm.LegsTotal = legsTotal{Label: "Total", PnL: b.note(b.money(s.Figures.UnrealizedPnL))} // the one compute value, not a sum
	vm.Payoff = b.note(b.payoff())
	vm.Greeks = b.note(b.greeks())
	vm.KeyLevels = b.levels()
	vm.Timeline = b.timeline()
	vm.Fills = b.fills()
	vm.Sentence = b.sentence()
	if s.Greeks.Day != "" {
		if t, err := midnight(s.Greeks.Day, b.zone); err == nil {
			vm.DataTimes = append(vm.DataTimes, sourceTime{Source: "option_eod", DataTime: t})
		}
	}
	vm.Gaps = append(vm.Gaps, b.gaps...)
	if len(vm.Gaps) > 0 {
		vm.RunStatus = "partial"
	}
}

// sentence is the profit now in words, copied from the tile.
func (b *builder) sentence() string {
	switch v := b.money(b.s.Figures.UnrealizedPnL).(type) {
	case usd:
		return clip("Current P&L " + v.Text + " USD, valued at mid price")
	default:
		return "Current P&L: no data yet"
	}
}

// earliestExpiry is the first expiry among the legs, "-" without legs.
func earliestExpiry(s compute.Structure) string {
	out := "-"
	for i, l := range s.Legs {
		if i == 0 || l.Expiry < out {
			out = l.Expiry
		}
	}
	return out
}

var levelTexts = map[string][2]string{
	"strike":       {"Strike", hintStrike},
	"short_strike": {"Short strike", hintShortStrike},
	"long_strike":  {"Long strike", hintLongStrike},
	"break_even":   {"Breakeven", hintBreakEven},
}

// levels are the key levels of compute with their distance, moved two places; a missing quote is a gap.
func (b *builder) levels() []level {
	out := []level{}
	if b.foreign() {
		b.addGap(mkGap("unsupported_currency"))
		return out
	}
	for _, lv := range b.s.Levels {
		price, ok := usdCell(lv.Price)
		if !ok {
			b.addGap(mkGap(reasonMissing))
			continue
		}
		t, known := levelTexts[lv.Name]
		if !known {
			t = [2]string{lv.Name, hintOtherLevel}
		}
		var dist any = b.missingQuote()
		if lv.Distance != "" {
			if p, err := percentText(lv.Distance); err == nil {
				dist = p
			} else {
				dist = mkGap(reasonMissing)
			}
		}
		out = append(out, level{Name: clip(t[0]), Price: price, Note: t[1], Distance: b.note(dist)})
	}
	return out
}

// intText is a decimal text of a whole number within the schema's bound of a quantity.
func intText(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n > 1000 || n < -1000 {
		return 0, false
	}
	return n, true
}

// usdCell is the Usd cell of a stored decimal text that is a JSON number.
func usdCell(text string) (usd, bool) {
	n, ok := numberLiteral(text)
	return usd{Value: n, Text: text, Unit: "usd"}, ok
}
