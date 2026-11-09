package option_notices_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
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
	Day           string       `json:"day"`
	Accounts      []aliasBlock `json:"accounts"`
}

// order is the aliases to show: Main first, then Aliases in their order without repeats.
func order(c Config) []string {
	var out []string
	seen := map[string]bool{}
	for _, a := range append([]string{c.Main}, c.Aliases...) {
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	return out
}

// Build writes the page.option_notices_page.v1 JSON for day from the stored rows of c.Account,
// c.Candidates and c.Earnings, through c.Rule (Decisions 7 to 10). A listed alias with no capture is
// a no_capture block, never an error; a missing capture of c.Main is the page gap no_capture. A
// reader error of Main or of the guard or earnings readers is an error of Build. generatedAt is
// written in c.Zone.
func Build(ctx context.Context, c Config, _ store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error) {
	if err := checkConfig(c, day); err != nil {
		return nil, err
	}
	vm := viewModel{Schema: "option_notices_page", SchemaVersion: 1, RunID: c.RunID, BuildID: c.BuildID, Scope: c.Scope,
		GeneratedAt: generatedAt.In(c.Zone).Format(timeLayout), DataTimes: []sourceTime{}, RunStatus: "ok",
		Title: pageTitle, Notes: []string{}, Gaps: []gap{}, Day: string(day), Accounts: []aliasBlock{}}
	if c.Account == nil {
		vm.Gaps = append(vm.Gaps, mkGap("no_reader", gapNoReader))
		vm.RunStatus = "partial"
		return json.Marshal(vm)
	}
	if c.Candidates == nil || c.Earnings == nil {
		vm.Gaps = append(vm.Gaps, mkGap("no_reader", gapNoReader))
	}
	var oldest contracts.Day
	for _, alias := range order(c) {
		main := alias == c.Main
		cp, ok, err := findCapture(ctx, c.Account, alias, day)
		if err != nil && main {
			return nil, err
		}
		if err != nil || !ok {
			vm.Accounts = append(vm.Accounts, noCaptureBlock(alias, main))
			if main {
				vm.Gaps = append(vm.Gaps, mkGap("no_capture", gapNoCapture))
			}
			continue
		}
		block, err := aliasOf(ctx, c, alias, main, day, cp)
		if err != nil {
			return nil, err
		}
		vm.Accounts = append(vm.Accounts, block)
		at, err := midnight(cp.Day, c.Zone)
		if err != nil {
			return nil, fmt.Errorf("option_notices_page: bad capture day %q: %w", cp.Day, err)
		}
		vm.DataTimes = append(vm.DataTimes, sourceTime{Source: sourceID(alias), DataTime: at})
		if block.Status == "stale" && (oldest == "" || cp.Day < oldest) {
			oldest = cp.Day
		}
	}
	if err := finish(&vm, c.Zone, oldest); err != nil {
		return nil, err
	}
	return json.Marshal(vm)
}

// checkConfig is the error of Build for a Config or day it cannot build from.
func checkConfig(c Config, day contracts.Day) error {
	if c.Zone == nil {
		return errors.New("option_notices_page: nil zone")
	}
	if c.Rule == nil {
		return errors.New("option_notices_page: nil rule")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "scope": c.Scope} {
		if !idPattern.MatchString(v) {
			return fmt.Errorf("option_notices_page: bad %s %q", name, v)
		}
	}
	if _, err := daysBack(day, 0); err != nil {
		return fmt.Errorf("option_notices_page: bad day %q: %w", day, err)
	}
	return nil
}

// finish writes the stale label of the oldest stale data day and the run status.
func finish(vm *viewModel, zone *time.Location, oldest contracts.Day) error {
	if oldest != "" {
		at, err := midnight(oldest, zone)
		if err != nil {
			return fmt.Errorf("option_notices_page: bad day %q: %w", oldest, err)
		}
		vm.Stale = &staleLabel{DataTime: at, IsStale: true, Text: staleHead + string(oldest) + staleTail}
	}
	for _, b := range vm.Accounts {
		if b.Status == "no_capture" {
			vm.RunStatus = "partial"
		}
	}
	if len(vm.Gaps) > 0 {
		vm.RunStatus = "partial"
	}
	return nil
}

// aliasOf converts the stored rows of one alias to the rule input, evaluates it and writes the block.
func aliasOf(ctx context.Context, c Config, alias string, main bool, day contracts.Day, cp capture) (aliasBlock, error) {
	in := compute.NoticeInput{Day: day, Alias: alias, CaptureDay: cp.Day, Positions: noticePositions(cp.Positions),
		PrevDay: cp.PrevDay, PrevPositions: noticePositions(cp.Prev), Guards: []compute.NoticeGuard{},
		Earnings: []compute.NoticeEarnings{}, Values: noticeValues(cp.Values)}
	note := ""
	if c.Candidates != nil {
		guards, found, err := guardInputs(ctx, c.Candidates, alias, cp.Day)
		if err != nil {
			return aliasBlock{}, err
		}
		in.Guards = guards
		if !found {
			note = noteNoGuard
		}
	}
	if c.Earnings != nil {
		e, err := earningsInputs(ctx, c.Earnings, cp.Positions, day)
		if err != nil {
			return aliasBlock{}, err
		}
		in.Earnings = e
	}
	return okBlock(alias, main, cp.Day, day, c.Rule.Evaluate(in, c.Params), note), nil
}
