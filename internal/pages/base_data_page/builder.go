// builder.go reads the facts and the coverage and builds the document of the page (docs/design/base-data-page.md, entry 9).
package base_data_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/basefact"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

// ruleHints are the Chinese texts of the rules, by rule name.
var ruleHints = map[string]string{
	"rows": hintRows, "newest": hintNewest, "coverage_gap": hintCoverageGap, "coverage_latest": hintCoverageLatest,
	"age": hintAge, "ohlc_order": hintOhlcOrder, "bar_jump": hintBarJump, "crossed": hintCrossed, "empty": hintEmpty,
	"errors": hintErrors, "non_live": hintNonLive, "bad_iv": hintBadIV, "series_age": hintSeriesAge,
	"fx_band": hintFxBand, "fx_jump": hintFxJump, "earnings_past": hintEarningsPast, "dividend_past": hintDividendPast,
	"macro_exhausted": hintMacroExhausted, "calendar_horizon": hintCalendarHorizon, "period_order": hintPeriodOrder,
	"subject_stale": hintSubjectStale, "capture_age": hintCaptureAge, "no_netliq": hintNoNetliq,
	"position_expired": hintPositionExpired, "flex_age": hintFlexAge, "flex_section_absent": hintFlexSection,
	"history_lost": hintHistoryLost,
	"not_read":     hintNotRead,
}

// rowHints are the Chinese texts of the rows, by row id.
var rowHints = map[string]string{
	"bars": rowBars, "quotes": rowQuotes, "chains": rowChains, "open": rowOpen, "contracts": rowContracts,
	"rates": rowRates, "fx": rowFx, "events": rowEvents, "calendar": rowCalendar, "facts": rowFacts,
	"account": rowAccount, "flex": rowFlex,
}

// Hint returns the Chinese text of a row id (such as "bars") or of a rule id "<row>.<rule>" (such as "bars.bar_jump").
func Hint(id string) (string, bool) {
	row, rule, isRule := strings.Cut(id, ".")
	if !isRule {
		t, ok := rowHints[id]
		return t, ok
	}
	if _, ok := rowHints[row]; !ok {
		return "", false
	}
	if !ruleKnown(row, rule) {
		return "", false
	}
	t, ok := ruleHints[rule]
	return t, ok
}

// ruleKnown is true for a rule of the row in basecheckapi.Rules, and for not_read, which any row can show.
func ruleKnown(row, rule string) bool {
	if rule == "not_read" {
		return true
	}
	for _, r := range basecheckapi.Rules {
		if r.Row == row && r.Rule == rule {
			return true
		}
	}
	return false
}

var (
	errNoEval    = errors.New("base_data_page: no Eval")
	errNoVerdict = errors.New("base_data_page: Recon is set and Verdict is not")
)

// Build returns the JSON of page.base_data_page.v1. A failed view group, a failed fact read or a failed coverage
// read gives a gap and never an error.
func Build(ctx context.Context, in Input) ([]byte, error) {
	if in.Eval == nil {
		return nil, errNoEval
	}
	if in.Recon != nil && in.Verdict == nil {
		return nil, errNoVerdict
	}
	cal := readCalendars(ctx, in)
	vm := viewModel{Schema: pageName, Version: versionDoc, RunID: runID, BuiltAt: in.Now.Format(time.RFC3339),
		AsOf: in.Now.In(mustZone(zoneUS)).Format(dayFormat), Groups: []groupVM{}, Glossary: []GlossaryRow{},
		RepairPath: repairPath, Gaps: []Gap{},
		LastSessions: lastSessionsVM{XNYS: string(cal.days[calUS]), XTKS: string(cal.days[calJP])}}

	res, err := readFacts(ctx, in)
	if err != nil || len(res.Failed) >= len(viewWords) {
		vm.Gaps = append(vm.Gaps, Gap{gapKind, reasonFacts, gapFactsText})
		vm.Top = topVM{Level: string(basecheckapi.Unknown), TextZh: topTextNoFacts,
			Counts: countsVM{Unknown: len(basecheckapi.Rows)}}
		if cal.failed {
			vm.Gaps = append(vm.Gaps, Gap{gapKind, reasonCoverage, gapCoverageText})
		}
		return json.Marshal(vm)
	}
	notRead := notReadRows(res.Failed)
	if len(res.Failed) > 0 {
		vm.Gaps = append(vm.Gaps, Gap{gapKind, reasonView, viewGapText(res.Failed)})
	}
	if cal.failed {
		vm.Gaps = append(vm.Gaps, Gap{gapKind, reasonCoverage, gapCoverageText})
	}
	cc, pairs, err := readCrossCheck(ctx, in, cal.days[calUS])
	if err != nil {
		vm.Gaps = append(vm.Gaps, Gap{gapKind, reasonView, gapReconText})
	}
	out := in.Eval(basecheckapi.Input{Facts: convertFacts(res.Facts), NotRead: notRead, Cells: sumCells(cal.cells),
		Days: cal.days, Sessions: cal.sessions, Pairs: pairs})
	vm.Groups = groupsOf(out)
	attachCrossCheck(vm.Groups, barsRowID, cc)
	vm.Top = topVM{Level: string(out.Level), Counts: countsOf(out.Counts)}
	c := vm.Top.Counts
	vm.Top.TextZh = fmt.Sprintf(topTextFormat, c.Pass+c.Warn+c.Fail+c.Unknown, c.Pass, c.Warn, c.Fail, c.Unknown)
	vm.Glossary = glossary()
	vm.DerivedNote = derivedNoteText
	return json.Marshal(vm)
}

func readFacts(ctx context.Context, in Input) (basefact.Result, error) {
	if in.Facts == nil {
		return basefact.Result{}, errors.New("base_data_page: no fact reader")
	}
	return in.Facts.Facts(ctx)
}

// notReadRows are the row ids of the failed view groups, in the order of basecheckapi.Rows.
func notReadRows(failed []string) []string {
	bad := map[string]bool{}
	for _, g := range failed {
		bad[g] = true
	}
	ids := []string{}
	for _, r := range basecheckapi.Rows {
		if bad[r.ViewGroup] {
			ids = append(ids, r.ID)
		}
	}
	return ids
}

func viewGapText(failed []string) string {
	bad := map[string]bool{}
	for _, g := range failed {
		bad[g] = true
	}
	var words []string
	for _, g := range []string{basefact.GroupBars, basefact.GroupOption, basefact.GroupOther, basefact.GroupAccount} {
		if bad[g] {
			words = append(words, viewWords[g])
		}
	}
	return gapViewHead + strings.Join(words, "; ") + gapViewTail
}

// calendars is what the builder derives from the coverage reader.
type calendars struct {
	days     map[string]contracts.Day
	sessions map[string][]contracts.Day
	cells    []coverage.Cell
	failed   bool
}

// zones maps a calendar id to its zone.
var zones = map[string]string{calUS: zoneUS, calJP: zoneJP}

func mustZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

// readCalendars works out Days and Sessions of both calendars and the ledger cells of their window. Both calendars are
// always present; where the coverage cannot say, the day before today in the zone stands in and the read has failed.
func readCalendars(ctx context.Context, in Input) calendars {
	c := calendars{days: map[string]contracts.Day{}, sessions: map[string][]contracts.Day{}}
	from, to := contracts.Day(""), contracts.Day("")
	for cal, zone := range zones {
		loc := mustZone(zone)
		local := in.Now.In(loc)
		today := local.Format(dayFormat)
		c.days[cal] = contracts.Day(local.AddDate(0, 0, -1).Format(dayFormat))
		c.sessions[cal] = []contracts.Day{}
		if in.Cov == nil {
			c.failed = true
			continue
		}
		ss, err := in.Cov.Sessions(ctx, cal, contracts.Day(local.AddDate(0, 0, -coverage.LookbackDays).Format(dayFormat)), contracts.Day(today))
		if err != nil {
			c.failed = true
			continue
		}
		var days []contracts.Day
		for _, s := range ss {
			if s.IsSession && string(s.Day) < today {
				days = append(days, s.Day)
			}
		}
		sortDays(days)
		if len(days) == 0 {
			c.failed = true
			continue
		}
		c.sessions[cal] = days
		c.days[cal] = days[len(days)-1]
		first := days[0]
		if len(days) > coverage.WindowSessions {
			first = days[len(days)-coverage.WindowSessions]
		}
		if from == "" || first < from {
			from = first
		}
		if c.days[cal] > to {
			to = c.days[cal]
		}
	}
	if from != "" && in.Cov != nil {
		cells, err := in.Cov.Cells(ctx, from, to)
		if err != nil {
			c.failed = true
		} else {
			c.cells = cells
		}
	}
	return c
}
