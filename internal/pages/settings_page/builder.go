package settings_page

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func validate(c Config) error {
	if c.Zone == nil {
		return fmt.Errorf("settings_page: nil zone")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "scope": c.Scope} {
		if !idPattern.MatchString(v) {
			return fmt.Errorf("settings_page: bad %s %q", name, v)
		}
	}
	if c.Params == nil || c.Catalog == nil || c.Names == nil {
		return fmt.Errorf("settings_page: nil registry or reader")
	}
	if n := c.Notice; n != nil {
		switch n.Kind {
		case "saved", "nothing_changed", "error":
		default:
			return fmt.Errorf("settings_page: bad notice kind %q", n.Kind)
		}
		if n.Text == "" {
			return fmt.Errorf("settings_page: empty notice text")
		}
	}
	return nil
}

// Build returns the page.settings_page.v2 JSON. The settings are read as of at; r and day are unused. A nil Owner
// gives the page with the gap no_reader and no rows; a nil Form gives a read-only page. A reader error is returned
// and no page with it.
func Build(ctx context.Context, c Config, _ store.SeriesReader, _ contracts.Day, at time.Time) ([]byte, error) {
	if err := validate(c); err != nil {
		return nil, err
	}
	vm := viewModel{
		Schema: "settings_page", SchemaVersion: 2,
		RunID: c.RunID, BuildID: c.BuildID, Scope: c.Scope,
		GeneratedAt: at.In(c.Zone).Format(time.RFC3339),
		DataTimes:   []struct{}{}, RunStatus: "ok",
		Title: "Settings", AsOf: "Settings as of " + at.In(c.Zone).Format(time.RFC3339),
		Rows: []exclusionRow{}, History: []historyRow{}, Params: []paramRow{},
	}
	if n := c.Notice; n != nil {
		vm.Notice = &noticeView{Kind: n.Kind, Text: clip(n.Text)}
	}
	if c.Owner == nil {
		vm.RunStatus = "partial"
		vm.Gap = &gap{Reason: "no_reader", Text: "The owner settings are not readable here."}
		return json.Marshal(vm)
	}

	stored, err := c.Owner.Params(ctx, at)
	if err != nil {
		return nil, fmt.Errorf("settings_page: read params: %w", err)
	}
	flags, err := c.Owner.Exclusions(ctx, at)
	if err != nil {
		return nil, fmt.Errorf("settings_page: read exclusions: %w", err)
	}
	latest, err := c.Catalog.OptionEodLatest(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("settings_page: read catalog: %w", err)
	}
	var hist []ownerset.HistoryRow
	if c.HistoryN > 0 {
		h, err := c.Owner.History(ctx, min(c.HistoryN, maxHistory))
		if err != nil {
			return nil, fmt.Errorf("settings_page: read history: %w", err)
		}
		hist = h
	}

	ids, flagged := underlyingIDs(latest, flags)

	nameIDs := slices.Clone(ids)
	for _, h := range limit(hist, maxHistory) {
		if h.Kind != "param" {
			nameIDs = append(nameIDs, h.InstrumentID)
		}
	}
	slices.Sort(nameIDs)
	names, err := c.readNames(ctx, slices.Compact(nameIDs))
	if err != nil {
		return nil, err
	}

	specs := c.Params.Specs()
	vm.Params = newParamRows(specs, stored)
	for _, id := range ids {
		vm.Rows = append(vm.Rows, exclusionRow{InstrumentID: id, Symbol: symbolOf(names, id), Excluded: flagged[id]})
	}
	vm.History = newHistoryRows(hist, specs, names, c.Zone)
	vm.Confirm = newConfirm(c.Confirm)
	if c.Form != nil {
		vm.Form = newForm(*c.Form)
	} else {
		vm.Gap = &gap{Reason: "not_built", Text: "Editing is off: start ot serve with --owner-edit to change the settings."}
	}
	return json.Marshal(vm)
}

// underlyingIDs is the catalog's underlyings plus every instrument with an exclusion row, so a flag never
// disappears, in id order; flagged is the current flag per instrument.
func underlyingIDs(latest []store.OptionEodLatest, flags []ownerset.Exclusion) ([]int64, map[int64]bool) {
	flagged := make(map[int64]bool, len(flags))
	ids := make([]int64, 0, len(latest)+len(flags))
	for _, l := range latest {
		ids = append(ids, l.UnderlyingID)
	}
	for _, f := range flags {
		flagged[f.InstrumentID] = f.Excluded
		ids = append(ids, f.InstrumentID)
	}
	slices.Sort(ids)
	return limit(slices.Compact(ids), maxRows), flagged
}

// readNames reads the names of ids in calls of at most maxNames, the reader's limit.
func (c Config) readNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	names := make(map[int64]string, len(ids))
	for chunk := range slices.Chunk(ids, maxNames) {
		got, err := c.Names.InstrumentNames(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("settings_page: read names: %w", err)
		}
		for _, n := range got {
			names[n.InstrumentID] = n.Name
		}
	}
	return names, nil
}
