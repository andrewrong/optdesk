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
	Notes         []string     `json:"notes"`
	Gaps          []gap        `json:"gaps"`
	ChainDay      *string      `json:"chain_day"`
	ComputedAt    *string      `json:"computed_at"`
	RuleVersion   *string      `json:"rule_version"`
	EstimateNote  string       `json:"estimate_note"`
	Groups        []group      `json:"groups"`
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
		DataTimes:   []sourceTime{}, RunStatus: "ok", Title: pageTitle,
		Notes: []string{jpNote}, Gaps: []gap{}, EstimateNote: estimateNote,
		Groups: make([]group, 0, len(groupSpecs)),
	}
	var chainDay, oldestStale contracts.Day
	var first *store.Candidate
	if c.Candidates == nil {
		for _, s := range groupSpecs {
			vm.Groups = append(vm.Groups, gapGroup(s.kind, mkGap("no_reader", gapNoReader)))
		}
		vm.Gaps = append(vm.Gaps, mkGap("no_reader", gapNoReader))
	} else {
		for _, s := range groupSpecs {
			gd, rows, rate, err := readGroup(ctx, c, series, s.kind, day)
			if err != nil {
				return nil, err
			}
			if len(rows) == 0 {
				vm.Groups = append(vm.Groups, gapGroup(s.kind, mkGap("no_rows", gapNoRows)))
				continue
			}
			g := buildGroup(s, gd, day, rows, c.Figures, rate)
			vm.Groups = append(vm.Groups, g)
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
	return json.Marshal(vm)
}

// readGroup reads the rows of one kind with their display names and the cash rate of the group's day.
func readGroup(ctx context.Context, c Config, series store.SeriesReader, kind string, day contracts.Day) (contracts.Day, []store.Candidate, rateRead, error) {
	gd, rows, err := readKind(ctx, c.Candidates, kind, day)
	if err != nil || len(rows) == 0 {
		return gd, nil, rateRead{}, err
	}
	if rows, err = withDisplayNames(ctx, c.Names, rows); err != nil {
		return gd, nil, rateRead{}, err
	}
	rate, err := readRate(ctx, series, gd)
	return gd, rows, rate, err
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
