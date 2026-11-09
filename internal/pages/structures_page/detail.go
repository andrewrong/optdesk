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
	notOpenText  = "结构已不在持仓中"
	maxLegs      = 8
	maxFills     = 50
	maxGapsShown = 20
)

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
		RunStatus: "ok", Title: "结构详情",
		LegsTitle: "逐腿明细", PayoffTitle: "到期盈亏", GreeksTitle: "希腊字母", LevelsTitle: "关键价位",
		TimelineTitle: "时间线", FillsTitle: "成交记录",
		Legs: []any{}, KeyLevels: []level{}, Timeline: []stage{}, Fills: []fill{}, Gaps: []gap{},
		Stale: staleLabel{DataTime: dayAt, StaleAfter: staleAt, Text: "按 " + string(day) + " 的账本与收盘价"},
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
		vm.gapPage("failed", mkGap(reasonNoReader), "无法读取持仓结构", structureID)
		return json.Marshal(vm)
	}
	in, err := c.load(ctx, alias, day)
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
	b.fill(vm, day)
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
	vm.RunStatus, vm.Sentence, vm.Subline = status, sentence, clip("结构 "+id)
	label := func(l string, caption string) tile { return tile{Label: l, Figure: g, Caption: caption} }
	vm.Tiles = tiles{
		PnL: label("当前盈亏", "按中间价估值"), MaxGain: label("最大盈利", "到期时"),
		MaxLoss: label("最大亏损", "到期时"), BreakEven: label("盈亏平衡", "到期价"),
	}
	vm.Legs = []any{g}
	vm.LegsTotal = legsTotal{Label: "合计", PnL: g}
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
	b := &builder{cells: cells{noQuotes: c.Eod == nil || c.Terms == nil}, s: s, quotes: in.Quotes, zone: c.Zone,
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

func (b *builder) fill(vm *detailModel, day contracts.Day) {
	s := b.s
	kind := kindTexts[s.Kind]
	if kind == "" {
		kind = "未识别"
	}
	vm.Title = clip(s.Underlying + " " + kind)
	if s.Underlying == "" {
		vm.Title = kind
	}
	expiry := earliestExpiry(s)
	vm.Subline = clip(kind + " · 到期 " + expiry + " · 账户 " + s.Alias)
	vm.Tiles = tiles{
		PnL:       tile{"当前盈亏", b.note(b.money(s.Figures.UnrealizedPnL)), "按中间价估值"},
		MaxGain:   tile{"最大盈利", b.note(b.money(s.Figures.MaxGain)), "到期时"},
		MaxLoss:   tile{"最大亏损", b.note(b.maxLossCell(s)), "到期时"},
		BreakEven: tile{"盈亏平衡", b.note(b.money(s.Figures.BreakEven)), breakEvenCaption(s)},
	}
	vm.Legs = b.legs()
	vm.LegsTotal = legsTotal{Label: "合计", PnL: b.note(b.money(s.Figures.UnrealizedPnL))} // the one compute value, not a sum
	vm.Payoff = b.note(b.payoff())
	vm.Greeks = b.note(b.greeks())
	vm.KeyLevels = b.levels()
	vm.Timeline = b.timeline(day)
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
		return clip("当前盈亏 " + v.Text + " 美元，按中间价估值")
	default:
		return "当前盈亏暂无数据"
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
	"strike":       {"行权价", "合约的行权价"},
	"short_strike": {"卖出行权价", "卖出腿的行权价"},
	"long_strike":  {"买入行权价", "买入腿的行权价"},
	"break_even":   {"盈亏平衡", "到期盈亏为零的标的价格"},
}

// levels are the key levels of compute with their distance, moved two places; a missing quote is a gap.
func (b *builder) levels() []level {
	out := []level{}
	if b.foreign() {
		b.addGap(mkGap("unsupported_currency"))
		return out
	}
	for _, lv := range b.s.Levels {
		n, ok := numberLiteral(lv.Price)
		if !ok {
			b.addGap(mkGap(reasonMissing))
			continue
		}
		t, known := levelTexts[lv.Name]
		if !known {
			t = [2]string{lv.Name, "关键价位"}
		}
		var dist any = b.missingQuote()
		if lv.Distance != "" {
			if p, err := percentText(lv.Distance); err == nil {
				dist = p
			} else {
				dist = mkGap(reasonMissing)
			}
		}
		out = append(out, level{Name: clip(t[0]), Price: usd{Value: n, Text: lv.Price, Unit: "usd"}, Note: t[1], Distance: b.note(dist)})
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
