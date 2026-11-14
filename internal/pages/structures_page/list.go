package structures_page

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	pageTitle      = "Position structures"
	decideTitle    = "To decide this week"
	othersTitle    = "Other structures"
	otherNote      = "Unidentified structures, shown leg by leg"
	noReaderNote   = "Ledger or trading day reader not connected (no_reader)"
	noCaptureNote  = "No position snapshot, not compared with Flex"
	oldCaptureNote = "Position snapshot is older than 3 trading days, not compared with Flex"
	noAccountNote  = "Position snapshot reader not connected (no_reader), not compared with Flex"
	shownContracts = 2
)

var kindTexts = map[string]string{
	"short_put": "Short put", "covered_call": "Covered call", "naked_call": "Naked call",
	"put_spread": "Put spread", "call_spread": "Call spread", "long_put": "Long put", "long_call": "Long call",
	"other": "Unidentified",
}

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type staleLabel struct {
	DataTime   string `json:"data_time"`
	StaleAfter string `json:"stale_after"`
	Text       string `json:"text"`
}

type row struct {
	Symbol     string   `json:"symbol"`
	Href       string   `json:"href"`
	KindText   string   `json:"kind_text"`
	ExpiryText string   `json:"expiry_text"`
	Moneyness  any      `json:"moneyness"`
	Distance   any      `json:"distance"`
	Earned     any      `json:"earned"`
	ExDividend any      `json:"ex_dividend"`
	Compare    *compare `json:"compare"`
	MaxLoss    any      `json:"max_loss"`
	MaxGain    any      `json:"max_gain"`
	PnL        any      `json:"pnl"`
}

type group struct {
	Title string `json:"title"`
	Rows  []row  `json:"rows"`
}

type viewModel struct {
	Schema        string       `json:"schema"`
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	BuildID       string       `json:"build_id"`
	Scope         string       `json:"scope"`
	GeneratedAt   string       `json:"generated_at"`
	DataTimes     []sourceTime `json:"data_times"`
	RunStatus     string       `json:"run_status"`
	Title         string       `json:"title"`
	Sentence      string       `json:"sentence"`
	Note          *string      `json:"note"`
	Groups        []group      `json:"groups"`
	Stale         staleLabel   `json:"stale"`
}

// buildList writes the structures_list page of alias for day. It computes nothing: it reads, calls the two
// rules and copies their text.
func buildList(ctx context.Context, c Config, alias string, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, fmt.Errorf("structures_page: nil zone")
	}
	if alias == "" && len(c.Aliases) > 0 {
		alias = c.Aliases[0] // the main alias is the first
	}
	if !slices.Contains(c.Aliases, alias) {
		return nil, fmt.Errorf("structures_page: unknown alias %q", alias)
	}
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
	vm := viewModel{
		Schema: "structures_list", SchemaVersion: 1, RunID: c.RunID, BuildID: c.BuildID, Scope: alias,
		GeneratedAt: at.In(c.Zone).Format(time.RFC3339), DataTimes: []sourceTime{},
		RunStatus: "ok", Title: pageTitle,
		Groups: []group{{Title: decideTitle, Rows: []row{}}, {Title: othersTitle, Rows: []row{}}},
		Stale:  staleLabel{DataTime: dayAt, StaleAfter: staleAt, Text: "Ledger and closing prices as of " + string(day)},
	}
	if c.Ledger == nil || c.Sessions == nil {
		vm.RunStatus, vm.Sentence, vm.Note = "failed", "Cannot read the position structures", ptr(noReaderNote)
		return json.Marshal(vm)
	}
	in, err := c.load(ctx, alias, day)
	if err != nil {
		return nil, err
	}
	res := c.Structures().Evaluate(in, compute.StructureParams{MarkMaxAgeSessions: markMaxAgeSessions,
		CaptureTarget: captureTarget, ExitDTE: exitDTE, CommissionDays: commissionDays})
	gaps := fillRows(&vm, res, cells{noQuotes: c.Eod == nil || c.Terms == nil})
	vm.DataTimes = append(vm.DataTimes, sourceTime{Source: "option_ledger", DataTime: dayAt})
	if in.HeldDay != "" {
		held, err := midnight(in.HeldDay, c.Zone)
		if err != nil {
			return nil, err
		}
		vm.DataTimes = append(vm.DataTimes, sourceTime{Source: "account_capture", DataTime: held})
	}
	total := len(res.Structures) + len(res.Other)
	if total == 0 {
		vm.Sentence = "Account " + alias + " has no structures held"
	} else {
		vm.Sentence = "Total " + strconv.Itoa(total) + " position structures, to decide this week: " + strconv.Itoa(len(vm.Groups[0].Rows))
	}
	if note := c.noteOf(res); note != "" {
		vm.Note = &note
	}
	if vm.Note != nil || gaps > 0 {
		vm.RunStatus = "partial"
	}
	return json.Marshal(vm)
}

// fillRows puts each structure in its group, in the order Evaluate gave, and counts the gap cells.
func fillRows(vm *viewModel, res compute.StructureResult, cs cells) int {
	gaps := 0
	for _, s := range res.Structures {
		r := cs.rowOf(s)
		gaps += r.gapCount()
		g := &vm.Groups[1]
		if s.Decide {
			g = &vm.Groups[0]
		}
		g.Rows = append(g.Rows, r)
	}
	for _, s := range res.Other {
		r := cs.rowOf(s)
		gaps += r.gapCount()
		vm.Groups[1].Rows = append(vm.Groups[1].Rows, r)
	}
	return gaps
}

func ptr(s string) *string { return &s }

// noteOf joins the note of Decision 4 and the capture note of Decision 2.
func (c Config) noteOf(res compute.StructureResult) string {
	var parts []string
	if len(res.Other) > 0 {
		parts = append(parts, otherNote)
	}
	d := res.Diff
	switch {
	case c.Account == nil:
		parts = append(parts, noAccountNote)
	case d.Skipped == "no_capture":
		parts = append(parts, noCaptureNote)
	case d.Skipped != "":
		parts = append(parts, oldCaptureNote)
	}
	if len(d.CaptureOnly) > 0 {
		parts = append(parts, "Positions not yet in Flex: "+contractList(d.CaptureOnly))
	}
	if len(d.LedgerOnly) > 0 {
		parts = append(parts, "Flex positions missing from the position snapshot: "+contractList(d.LedgerOnly))
	}
	return clip(strings.Join(parts, "; "))
}

func contractList(ids []string) string {
	shown := ids[:min(len(ids), shownContracts)]
	out := strings.Join(shown, ", ")
	if len(ids) > len(shown) {
		out += " etc., " + strconv.Itoa(len(ids)) + " in all"
	}
	return out
}

// rowOf is the row of one structure; every figure is copied from the compute output.
func (c cells) rowOf(s compute.Structure) row {
	kind, ok := kindTexts[s.Kind]
	if !ok {
		kind = s.Kind
	}
	if kind == "" {
		kind = "Unidentified"
	}
	expiry := "-"
	for i, l := range s.Legs {
		if i == 0 || l.Expiry < expiry {
			expiry = l.Expiry
		}
	}
	symbol := s.Underlying
	if symbol == "" {
		symbol = "-"
	}
	return row{
		Symbol: symbol, Href: hrefOf(s.ID), KindText: kind, ExpiryText: expiry,
		Moneyness: c.moneyness(s), Distance: c.percent(s.Figures.ExpiryDistance), Earned: c.percent(s.Figures.EarnedShare),
		Compare: c.compare(s),
		MaxLoss: c.listMaxLossCell(s),
		MaxGain: c.listMoney(s.Figures.MaxGain), PnL: c.listMoney(s.Figures.UnrealizedPnL),
	}
}

// gapCount is the number of gap cells of the row, compare included.
func (r row) gapCount() int {
	n := 0
	cellsOf := []any{r.Moneyness, r.Distance, r.Earned, r.MaxLoss, r.MaxGain, r.PnL}
	if r.Compare != nil {
		cellsOf = append(cellsOf, r.Compare.CloseCost, r.Compare.RollNet, r.Compare.CashAfter, r.Compare.RecentCommission)
	}
	for _, v := range cellsOf {
		if _, ok := v.(gap); ok {
			n++
		}
	}
	return n
}
