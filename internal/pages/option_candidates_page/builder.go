package option_candidates_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type staleLabel struct {
	DataTime string `json:"data_time"`
	IsStale  bool   `json:"is_stale"`
	Text     string `json:"text"`
}

// headLabels are the words of the page head and the text of the run status.
type headLabels struct {
	DataDay       string `json:"data_day"`
	RunStatus     string `json:"run_status"`
	RunStatusText string `json:"run_status_text"`
	RuleVersion   string `json:"rule_version"`
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
	Stale         *staleLabel  `json:"stale"`
	Title         string       `json:"title"`
	Subtitle      string       `json:"subtitle"`
	Notes         []string     `json:"notes"`
	Gaps          []gap        `json:"gaps"`
	ChainDay      *string      `json:"chain_day"`
	ComputedAt    *string      `json:"computed_at"`
	RuleVersion   *string      `json:"rule_version"`
	HeadLabels    headLabels   `json:"head_labels"`
	EstimateNote  string       `json:"estimate_note"`
	Groups        []group      `json:"groups"`
	// ShortPutList is the grouped short list, omitted without a Shortlist reader or short-put rows.
	ShortPutList *shortListView `json:"short_put_list,omitempty"`
	// ShortPutTiers is the tier view of the short-put rows, null without a Tiers judge or rows.
	ShortPutTiers *tiersView `json:"short_put_tiers"`
	// ShortPutListJPY and ShortPutTiersJPY are the same views of the yen group (candidates-jp.md Decision 24),
	// omitted without yen rows.
	ShortPutListJPY  *shortListView `json:"short_put_list_jpy,omitempty"`
	ShortPutTiersJPY *tiersView     `json:"short_put_tiers_jpy,omitempty"`
}

// Build returns the page.option_candidates_page.v1 JSON for day: the stored rows of each kind
// from c.Candidates, found by the lookback of Decision 3. The two figures of the short-put rows come
// from c.Figures, with the cash rate read from the series reader once per group; the rest is stored
// text. generatedAt is written in c.Zone. A reader error is an error of Build.
func Build(ctx context.Context, c Config, series store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, errors.New("option_candidates_page: nil zone")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "scope": c.Scope} {
		if !idPattern.MatchString(v) {
			return nil, fmt.Errorf("option_candidates_page: bad %s %q", name, v)
		}
	}
	if _, err := daysBack(day, 0); err != nil {
		return nil, fmt.Errorf("option_candidates_page: bad day %q: %w", day, err)
	}
	vm := viewModel{
		Schema: "option_candidates_page", SchemaVersion: 1,
		RunID: c.RunID, BuildID: c.BuildID, Scope: c.Scope,
		GeneratedAt: generatedAt.In(c.Zone).Format(timeLayout),
		DataTimes:   []sourceTime{}, RunStatus: "ok", Title: pageTitle, Subtitle: zhSubtitle,
		Notes: []string{jpNote}, Gaps: []gap{}, EstimateNote: estimateNote,
		Groups: make([]group, 0, len(groupSpecs)),
	}
	var chainDay, oldestStale contracts.Day
	var first *store.Candidate
	if c.Candidates == nil {
		vm.noReader()
	} else {
		for _, s := range specsOf(c) {
			bday := buildDayOf(s, day, generatedAt)
			gr, err := readGroup(ctx, c, series, s, bday)
			if err != nil {
				return nil, err
			}
			gd, rows := gr.day, gr.rows
			if len(rows) == 0 {
				vm.Groups = append(vm.Groups, gapGroup(s, mkGap("no_rows", gapNoRows)))
				continue
			}
			g := buildGroup(s, gd, bday, rows, c.Figures, gr.rate, gr.ev)
			vm.Groups = append(vm.Groups, g)
			if err = fillShortPut(ctx, c, &vm, s, gr, bday, generatedAt); err != nil {
				return nil, err
			}
			vm.Notes = withTrialNote(vm.Notes, g)
			if gd > chainDay {
				chainDay, first = gd, &rows[0]
			}
			if g.Stale && (oldestStale == "" || gd < oldestStale) {
				oldestStale = gd
			}
		}
		if chainDay == "" {
			vm.Gaps = append(vm.Gaps, mkGap("no_run", gapNoRun))
		}
	}
	if chainDay != "" {
		if err := fillRun(&vm, c.Zone, chainDay, oldestStale, first); err != nil {
			return nil, err
		}
	}
	for _, g := range vm.Groups {
		if g.Gap != nil {
			vm.RunStatus = "partial"
		}
	}
	vm.HeadLabels = headLabels{DataDay: zhHeadDataDay, RunStatus: zhHeadRunStatus, RunStatusText: zhRunStatusOK, RuleVersion: zhHeadRuleVer}
	if vm.RunStatus == "partial" {
		vm.HeadLabels.RunStatusText = zhRunStatusPart
	}
	return json.Marshal(vm)
}

// noReader is the page of a nil Candidates reader: every group a gap.
func (vm *viewModel) noReader() {
	for _, s := range groupSpecs {
		vm.Groups = append(vm.Groups, gapGroup(s, mkGap("no_reader", gapNoReader)))
	}
	vm.Gaps = append(vm.Gaps, mkGap("no_reader", gapNoReader))
}

// buildDayOf is the day a group is read for: the build day, and for the yen group the Tokyo date of the build time.
func buildDayOf(s groupSpec, day contracts.Day, at time.Time) contracts.Day {
	if s.currency == currencyJPY {
		return tokyoDay(at)
	}
	return day
}

// groupRead is what one group read: its day, rows with their display names, the cash rate of the day, the
// tickers of the names and, for the short-put kind, the evidence of its ok rows.
type groupRead struct {
	day     contracts.Day
	rows    []store.Candidate
	cur     string // the currency of the rows: "" for the US groups, JPY for the yen group
	rate    rateRead
	ev      evidenceSet
	tickers map[int]string
}

// specsOf are the groups in page order: the yen short-put group follows the US one when a yen reader is set.
func specsOf(c Config) []groupSpec {
	out := make([]groupSpec, 0, len(groupSpecs)+1)
	for _, s := range groupSpecs {
		out = append(out, s)
		if s.kind == kindShortPut && c.CandCur != nil {
			out = append(out, jpSpec)
		}
	}
	return out
}

// readGroup reads the rows of one group with their display names, the cash rate of the group's day and,
// for the short-put kind, the evidence of its ok rows (the stored main table); the tier view asks for the
// evidence of its own rows. day is the build day of the group's market.
func readGroup(ctx context.Context, c Config, series store.SeriesReader, s groupSpec, day contracts.Day) (groupRead, error) {
	gd, rows, err := readKind(ctx, c, s.kind, s.currency, day)
	if err != nil || len(rows) == 0 {
		return groupRead{day: gd, cur: s.currency}, err
	}
	tickers, err := tickersOf(ctx, c.Names, rows)
	if err != nil {
		return groupRead{day: gd, cur: s.currency}, err
	}
	rows = namedRows(rows, tickers)
	rate, err := readRate(ctx, series, gd)
	gr := groupRead{day: gd, rows: rows, rate: rate, tickers: tickers, cur: s.currency}
	if err != nil || s.kind != kindShortPut {
		return gr, err
	}
	var ok []store.Candidate
	for _, r := range rows {
		if r.Status == "ok" {
			ok = append(ok, r)
		}
	}
	gr.ev, err = evidenceOf(ctx, inputsOf(c, s.currency), tickers, gd, rate, ok)
	return gr, err
}

// fillShortPut writes the tier view and then the short list of a short-put group, into the US properties or,
// for the yen group, the yen ones; any other kind leaves vm as it is.
func fillShortPut(ctx context.Context, c Config, vm *viewModel, s groupSpec, gr groupRead, day contracts.Day, at time.Time) error {
	if s.kind != kindShortPut {
		// As before this change: the groups after the short put, read last, leave the US list unset (the
		// command and the site build the same bytes, tested in cmd/ot); the tier view is kept.
		vm.ShortPutList = nil
		return nil
	}
	tiers, err := buildTiers(ctx, c, gr, day, at)
	if err != nil {
		return err
	}
	list, err := shortListOfKind(c, s.kind, gr, tiers)
	if err != nil {
		return err
	}
	if s.currency == currencyJPY {
		vm.ShortPutTiersJPY, vm.ShortPutListJPY = tiers, list
	} else {
		vm.ShortPutTiers, vm.ShortPutList = tiers, list
	}
	return nil
}

// fillRun writes the fields that exist only when some group has rows.
func fillRun(vm *viewModel, zone *time.Location, chainDay, oldestStale contracts.Day, first *store.Candidate) error {
	cd := string(chainDay)
	vm.ChainDay = &cd
	at, err := midnight(chainDay, zone)
	if err != nil {
		return fmt.Errorf("option_candidates_page: bad chain day %q: %w", chainDay, err)
	}
	vm.DataTimes = append(vm.DataTimes, sourceTime{Source: "option_candidate", DataTime: at})
	if first.RuleVersion != "" {
		vm.RuleVersion = &first.RuleVersion
	}
	if !first.ComputedAt.IsZero() {
		ca := first.ComputedAt.In(zone).Format(timeLayout)
		vm.ComputedAt = &ca
	}
	if oldestStale != "" {
		st, err := midnight(oldestStale, zone)
		if err != nil {
			return fmt.Errorf("option_candidates_page: bad day %q: %w", oldestStale, err)
		}
		vm.Stale = &staleLabel{DataTime: st, IsStale: true, Text: staleTextHead + string(oldestStale) + staleTextTail}
	}
	return nil
}

// withTrialNote adds the trial-rate sentence once a short-put group shows the evidence columns.
func withTrialNote(notes []string, g group) []string {
	if g.Kind == kindShortPut && g.Main != nil && g.Currency == nil {
		return append(notes, trialRateNote)
	}
	return notes
}
