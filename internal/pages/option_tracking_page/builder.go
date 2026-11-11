package option_tracking_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	timeLayout  = "2006-01-02T15:04:05-07:00"
	maxAccounts = 4
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type staleLabel struct {
	DataTime string `json:"data_time"`
	IsStale  bool   `json:"is_stale"`
	Text     string `json:"text"`
}

type account struct {
	Alias           string  `json:"alias"`
	Title           string  `json:"title"`
	MarkDay         *string `json:"mark_day"`
	GuardDay        *string `json:"guard_day"`
	LongLegsLeftOut int     `json:"long_legs_left_out"`
	Open            *table  `json:"open"`
	Recent          *table  `json:"recent"`
	Recon           *table  `json:"recon"`
	Gap             *gap    `json:"gap"`
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
	AsOf          string       `json:"as_of"`
	MarkNote      string       `json:"mark_note"`
	Accounts      []account    `json:"accounts"`
}

// Build returns the page.option_tracking_page.v1 JSON for day. For each alias of the ledger (at
// most four, in the order the reader gives) it gathers the inputs, asks the tracker and formats
// the result. It adds, compares and sorts nothing: every figure is the tracker's text and the
// order is the tracker's. at is written in c.Zone. A store error is an error of Build.
func Build(ctx context.Context, c Config, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, errors.New("option_tracking_page: nil zone")
	}
	if c.Tracker == nil {
		return nil, errors.New("option_tracking_page: nil tracker")
	}
	if c.Replay == nil {
		return nil, errors.New("option_tracking_page: nil replay")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "scope": c.Scope} {
		if !idPattern.MatchString(v) {
			return nil, fmt.Errorf("option_tracking_page: bad %s %q", name, v)
		}
	}
	if _, err := time.Parse(dayLayout, string(day)); err != nil {
		return nil, fmt.Errorf("option_tracking_page: bad day %q: %w", day, err)
	}
	vm := &viewModel{
		Schema: "option_tracking_page", SchemaVersion: 1,
		RunID: c.RunID, BuildID: c.BuildID, Scope: c.Scope,
		GeneratedAt: at.In(c.Zone).Format(timeLayout),
		DataTimes:   []sourceTime{}, RunStatus: "ok", Title: pageTitle,
		Notes: []string{flexNote}, Gaps: []gap{}, AsOf: string(day), MarkNote: markNote,
		Accounts: []account{},
	}
	if c.Ledger == nil {
		g := readerGap()
		vm.Gaps = append(vm.Gaps, g)
		vm.Accounts = append(vm.Accounts, gapAccount(defaultName, g))
	} else if err := buildAccounts(ctx, c, day, vm); err != nil {
		return nil, err
	}
	if len(vm.Gaps) > 0 {
		vm.RunStatus = "partial"
	}
	for _, a := range vm.Accounts {
		if a.Gap != nil {
			vm.RunStatus = "partial"
		}
	}
	return json.Marshal(vm)
}

func gapAccount(alias string, g gap) account {
	return account{Alias: alias, Title: accountTitle(alias), Gap: &g}
}

func buildAccounts(ctx context.Context, c Config, day contracts.Day, vm *viewModel) error {
	aliases, err := c.Ledger.StoredAliases(ctx)
	if err != nil {
		return fmt.Errorf("option_tracking_page: read aliases: %w", err)
	}
	if len(aliases) == 0 {
		vm.Accounts = append(vm.Accounts, gapAccount(defaultName, codeGap(codeNoFlexRows)))
		return nil
	}
	if len(aliases) > maxAccounts {
		aliases = aliases[:maxAccounts]
	}
	seenGap := map[string]bool{}
	for _, alias := range aliases {
		in, gs, err := gather(ctx, c, alias, day)
		if err != nil {
			return err
		}
		skip := false
		for _, name := range gs {
			if name == gapNoFlexRows {
				vm.Accounts = append(vm.Accounts, gapAccount(alias, codeGap(codeNoFlexRows)))
				skip = true
				continue
			}
			code := gatherCodes[name]
			if code != "" && !seenGap[code] {
				seenGap[code] = true
				g := codeGap(code)
				if code != codeNoAccount {
					g.Reason = reasonNoReader
				}
				vm.Gaps = append(vm.Gaps, g)
			}
		}
		if skip {
			continue
		}
		if !in.HeldKnown && !seenGap[codeNoAccount] {
			seenGap[codeNoAccount] = true
			vm.Gaps = append(vm.Gaps, codeGap(codeNoAccount))
		}
		res := c.Tracker.Track(in, c.Params)
		a, stale := formatAccount(alias, res, in.HeldKnown)
		vm.Accounts = append(vm.Accounts, a)
		if res.LongLegsLeftOut > 0 {
			vm.Notes = append(vm.Notes, alias+noteSep+strconv.Itoa(res.LongLegsLeftOut)+leftOutTail)
		}
		if stale && vm.Stale == nil {
			at, err := midnight(day, c.Zone)
			if err != nil {
				return err
			}
			vm.Stale = &staleLabel{DataTime: at, IsStale: true, Text: staleText}
		}
	}
	return nil
}

func midnight(day contracts.Day, zone *time.Location) (string, error) {
	t, err := time.ParseInLocation(dayLayout, string(day), zone)
	if err != nil {
		return "", fmt.Errorf("option_tracking_page: bad day %q: %w", day, err)
	}
	return t.Format(timeLayout), nil
}
