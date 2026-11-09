package option_result_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type viewModel struct {
	Schema        string        `json:"schema"`
	SchemaVersion int           `json:"schema_version"`
	RunID         string        `json:"run_id"`
	BuildID       string        `json:"build_id"`
	Scope         string        `json:"scope"`
	GeneratedAt   string        `json:"generated_at"`
	DataTimes     []struct{}    `json:"data_times"`
	RunStatus     string        `json:"run_status"`
	Title         string        `json:"title"`
	Stale         *struct{}     `json:"stale"`
	Nothing       *string       `json:"nothing"`
	Sections      []section     `json:"sections"`
	Reliability   []reliability `json:"reliability"`
	Facts         []pair        `json:"facts"`
	Notes         []string      `json:"notes"`
}

// sessionSpan is how many days back from the build day the session days start: 366 days up to the build day.
const sessionSpan = 365

// calendar is the calendar of the session days.
const calendar = "XNYS"

// build writes the page for alias on day; see Config for the errors.
func build(ctx context.Context, c Config, alias string, day contracts.Day, at time.Time) ([]byte, error) {
	d, err := checkConfig(c, alias, day)
	if err != nil {
		return nil, err
	}
	vm := viewModel{Schema: "option_result_page", SchemaVersion: 1, RunID: c.RunID, BuildID: c.BuildID, Scope: alias,
		GeneratedAt: stamp(at, c.Zone), DataTimes: []struct{}{}, RunStatus: "ok", Title: pageTitle,
		Sections: []section{}, Reliability: []reliability{}, Facts: []pair{}, Notes: []string{}}
	var ledger compute.OptionLedger
	var results compute.OptionResults
	if c.Ledger != nil && c.Sessions != nil && c.Ledgers != nil && c.Results != nil {
		ledger, results = c.Ledgers(), c.Results()
	}
	if ledger == nil || results == nil {
		vm.RunStatus = "partial"
		vm.Nothing = ptr(noReader)
		return json.Marshal(vm)
	}
	rows, err := c.Ledger.FlexTradeRows(ctx, alias)
	if err != nil {
		return nil, fmt.Errorf("option_result_page: read trades of %s: %w", alias, err)
	}
	trades := make([]compute.LedgerTrade, 0, len(rows))
	for _, r := range rows {
		t, err := tradeOf(r.TransactionID, r.Revision, r.Attributes)
		if err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	from := d.AddDate(0, 0, -sessionSpan).Format(dayLayout)
	sd, err := c.Sessions.SessionDays(ctx, calendar, contracts.Day(from), day)
	if err != nil {
		return nil, fmt.Errorf("option_result_page: read session days: %w", err)
	}
	var sessions []contracts.Day
	for _, s := range sd {
		if s.IsSession {
			sessions = append(sessions, contracts.Day(s.SessionDay.Format(dayLayout)))
		}
	}
	rep := ledger.Replay(compute.LedgerInput{Alias: alias, Trades: trades, SessionDays: sessions, AsOf: day}, compute.LedgerParams{})
	sum := results.Summarise(compute.ResultInput{Alias: alias, Positions: rep.Positions, Unresolved: rep.Unresolved, Review: rep.Review, AsOf: day})
	for _, s := range sum.Sections {
		sec, err := sectionOf(alias, s)
		if err != nil {
			return nil, err
		}
		vm.Sections = append(vm.Sections, sec)
		vm.Reliability = append(vm.Reliability, reliabilityOf(sec.Key, sec.Heading, s.Reliability))
	}
	if len(vm.Sections) == 0 {
		vm.Nothing = ptr(nothingText)
	}
	return json.Marshal(vm)
}

// checkConfig is the error of build for a Config, alias or day it cannot build from; it returns the build day.
func checkConfig(c Config, alias string, day contracts.Day) (time.Time, error) {
	if c.Zone == nil {
		return time.Time{}, errors.New("option_result_page: nil zone")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "alias": alias} {
		if !idPattern.MatchString(v) {
			return time.Time{}, fmt.Errorf("option_result_page: bad %s %q", name, v)
		}
	}
	if !slices.Contains(c.Aliases, alias) {
		return time.Time{}, fmt.Errorf("option_result_page: unknown alias %q", alias)
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return time.Time{}, fmt.Errorf("option_result_page: bad day %q: %w", day, err)
	}
	return d, nil
}

// tradeOf reads the Attributes JSON text of a stored row into a LedgerTrade.
func tradeOf(id string, rev int, attrs string) (compute.LedgerTrade, error) {
	m := map[string]string{}
	if err := json.Unmarshal([]byte(attrs), &m); err != nil {
		return compute.LedgerTrade{}, fmt.Errorf("option_result_page: attributes of %s: %w", id, err)
	}
	return compute.LedgerTrade{TransactionID: id, Revision: rev, Attributes: m}, nil
}

func ptr(s string) *string { return &s }
