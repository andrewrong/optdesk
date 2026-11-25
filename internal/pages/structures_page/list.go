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
	"github.com/andrewrong/optdesk/internal/pages/shared"
)

const (
	pageTitle          = zhPageTitle + " (Position structures)"
	decideTitle        = zhDecideTitle + " (To decide this week)"
	othersTitle        = zhOthersTitle + " (Other structures)"
	shownContracts     = 2
	exDividendLookback = 10 // days before the page day of the dividend rows read
)

var kindTexts = map[string]string{
	"short_put": "Short put", "covered_call": "Covered call", "naked_call": "Naked call",
	"put_spread": "Put spread", "call_spread": "Call spread", "long_put": "Long put", "long_call": "Long call",
	"put_debit_spread": "Put debit spread", "call_debit_spread": "Call debit spread",
	"calendar_spread": "Calendar spread", "diagonal_spread": "Diagonal spread",
	"long_straddle": "Long straddle", "short_straddle": "Short straddle",
	"long_strangle": "Long strangle", "short_strangle": "Short strangle",
	"iron_condor": "Iron condor", "reverse_iron_condor": "Reverse iron condor", "collar": "Collar",
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
		vm.RunStatus, vm.Sentence, vm.Note = "failed", cannotReadText, ptr(noReaderNote)
		return json.Marshal(vm)
	}
	in, closes, ids, err := c.loadIDs(ctx, alias, day)
	if err != nil {
		return nil, err
	}
	res := c.Structures().Evaluate(in, compute.StructureParams{MarkMaxAgeSessions: markMaxAgeSessions,
		CaptureTarget: captureTarget, ExitDTE: exitDTE, CommissionDays: commissionDays})
	exDiv, err := c.exDividends(ctx, res, ids, day)
	if err != nil {
		return nil, err
	}
	gaps := fillRows(&vm, res, cells{noQuotes: c.Eod == nil || c.Terms == nil, exDiv: exDiv})
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
		vm.Sentence = emptySentenceHead + alias + emptySentenceTail
	} else {
		vm.Sentence = totalSentenceHead + strconv.Itoa(total) + totalSentenceMid + strconv.Itoa(len(vm.Groups[0].Rows))
	}
	if note := c.noteOf(res); note != "" {
		vm.Note = &note
	}
	if vm.Note != nil || gaps > 0 {
		vm.RunStatus = "partial"
	}
	if line := closeNote(res, closes); line != "" {
		vm.Note = ptr(joinNote(vm.Note, line))
	}
	return json.Marshal(vm)
}

// closeNote is the line that names the price and day behind each distance of the page: one entry per
// underlying whose structure shows a distance, in the order of the rows. It is empty when none is shown.
func closeNote(res compute.StructureResult, closes map[string]shared.Close) string {
	var parts []string
	seen := map[string]bool{}
	for _, s := range append(append([]compute.Structure{}, res.Structures...), res.Other...) {
		cl, ok := closes[s.Underlying]
		if !ok || seen[s.Underlying] || s.Figures.ExpiryDistance.Value == "" || s.Figures.ExpiryDistance.Gap != "" {
			continue
		}
		seen[s.Underlying] = true
		word := closeWord
		if cl.Via != "bar" {
			word = printWord
		}
		parts = append(parts, s.Underlying+" "+word+" "+string(cl.Day))
	}
	if len(parts) == 0 {
		return ""
	}
	return closeNotePrefix + strings.Join(parts, closeNoteSep)
}

func joinNote(note *string, line string) string {
	if note == nil {
		return clip(line)
	}
	return clip(*note + "; " + line)
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
		for _, s := range res.Other {
			if r := otherReason(s.Legs); r != "" {
				parts = append(parts, s.Underlying+": "+r)
			}
		}
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
		parts = append(parts, captureOnlyPrefix+contractList(d.CaptureOnly))
	}
	if len(d.LedgerOnly) > 0 {
		parts = append(parts, ledgerOnlyPrefix+contractList(d.LedgerOnly))
	}
	return clip(strings.Join(parts, "; "))
}

// otherReason names the tie that made a multi-leg unit of kind other, from the stored Right and Expiry of its
// legs; a unit that matches neither has no reason.
func otherReason(legs []compute.StructureLeg) string {
	if len(legs) < 2 {
		return ""
	}
	rights, expiries := map[string]bool{}, map[string]bool{}
	for _, l := range legs {
		rights[l.Right], expiries[l.Expiry] = true, true
	}
	if len(rights) == 2 && rights["P"] && rights["C"] && len(expiries) == 1 {
		return reasonPutCallOneExpiry
	}
	return ""
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
		ExDividend: c.exDiv[s.ID], Compare: c.compare(s),
		MaxLoss: c.listMaxLossCell(s),
		MaxGain: c.listMoney(s.Figures.MaxGain), PnL: c.listMoney(s.Figures.UnrealizedPnL),
	}
}

// exDividends is the ex_dividend cell of each structure of res by structure id (Decision 7), never nil.
// ids is the instrument id of each underlying.
func (c Config) exDividends(ctx context.Context, res compute.StructureResult, ids map[string]int64,
	day contracts.Day) (map[string]any, error) {
	out := map[string]any{}
	from, err := shiftDay(day, -exDividendLookback)
	if err != nil {
		return nil, err
	}
	for _, s := range append(append([]compute.Structure{}, res.Structures...), res.Other...) {
		switch {
		case !hasSoldCall(s):
			// the rule is about a sold call: it does not apply, so the cell is empty, not a gap
		case c.Dividends == nil:
			out[s.ID] = gap{Reason: reasonNoReader, Text: exNoReaderText}
		default:
			cell, err := c.exDividendOf(ctx, ids[s.Underlying], from, day)
			if err != nil {
				return nil, err
			}
			out[s.ID] = cell
		}
	}
	return out, nil
}

// exDividendOf is the cell of a structure with a sold call: the newest stored row of the underlying id.
func (c Config) exDividendOf(ctx context.Context, id int64, from, day contracts.Day) (any, error) {
	noRow := gap{Reason: reasonNoData, Text: exNoRowText}
	if id == 0 {
		return noRow, nil
	}
	rows, err := c.Dividends.Dividends(ctx, int(id), from, day)
	if err != nil {
		return nil, fmt.Errorf("structures_page: read dividends: %w", err)
	}
	if len(rows) == 0 {
		return noRow, nil
	}
	r := rows[len(rows)-1] // ascending by day: the newest
	if string(r.Status) != "present" || r.NextExDate == nil {
		return noRow, nil
	}
	date := r.NextExDate.Format(dayLayout)
	if date < string(day) {
		return gap{Reason: reasonNoData, Text: exPassedText}, nil
	}
	if r.NextAmount != nil && *r.NextAmount != "" {
		date += " · " + *r.NextAmount
	}
	return date, nil
}

// hasSoldCall reports whether a leg of s is a call held short.
func hasSoldCall(s compute.Structure) bool {
	for _, l := range s.Legs {
		if l.Right == "C" && strings.HasPrefix(l.Quantity, "-") {
			return true
		}
	}
	return false
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
