package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/household_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/held-companies.md#Interface.

// siteHeldFamilyRoute is the address of the page held_family_page under the site root.
const siteHeldFamilyRoute = "held-family"

// heldFamilyName is the name of the page held_family_page, which pageNamed selects for ot page.
const heldFamilyName = "held_family_page"

func init() {
	registerPage(pageFamily{name: heldFamilyName, route: siteHeldFamilyRoute, order: 65, build: buildHeldFamily, named: heldFamilyPageNamed})
}

// heldFamilyConfig is the config of the page: the readers of the household page, the underlying reader and the
// earnings reader; a reader that is nil gives a gap.
func heldFamilyConfig(a *app, rd siteReaders) household_page.Config {
	c := householdConfig(a, rd)
	c.Underlying, c.Earnings, c.Held = rd.underlying, rd.earnings, pagefig.NewHeld()
	return c
}

// heldFamilyDemo is the page of the demo profile: fixed rows, invented names and numbers, no read.
func heldFamilyDemo(a *app, day contracts.Day) household_page.HeldPage {
	ptr := func(n int) *int { return &n }
	acct := func(aliases ...string) []household_page.HeldAccount {
		out := make([]household_page.HeldAccount, 0, len(aliases))
		for _, al := range aliases {
			out = append(out, household_page.HeldAccount{Alias: al, Label: al})
		}
		return out
	}
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if a.profile.Zone != nil {
		if d, err := time.ParseInLocation(dayLayout, string(day), a.profile.Zone); err == nil {
			at = d
		}
	}
	p := household_page.HeldPage{
		Schema: "held_family_page", SchemaVersion: 1, RunID: "ot-serve", BuildID: buildIDOf(version), Scope: "held-family",
		GeneratedAt: at.Format("2006-01-02T15:04:05-07:00"), DataTimes: []household_page.HeldTime{}, RunStatus: "ok",
		Title: "Held companies", Notes: []string{"Demo profile: invented names and numbers."}, Gaps: []household_page.HeldGap{},
		Total: household_page.HeldTotal{Text: "100,000.00", Currency: "USD", Day: string(day), AccountsText: "main + wife"},
		Columns: []household_page.HeldColumn{
			{ID: "company", Label: "Company", Hint: "Company"}, {ID: "accounts", Label: "Held in", Hint: "Accounts that hold it"},
			{ID: "value", Label: "Market value (USD)", Hint: "Quantity times the latest close"},
			{ID: "share", Label: "Share of total", Hint: "Value over the family total"},
			{ID: "earnings", Label: "Next earnings", Hint: "Date and days to it"},
		},
		Rows: []household_page.HeldPageRow{
			{Company: "DEMOA", Accounts: acct("main", "wife"), Value: household_page.HeldCell{Kind: "value", Text: "12,000.00"},
				Share:    household_page.HeldCell{Kind: "value", Text: "12.0 %"},
				Earnings: household_page.HeldEarnings{Kind: "date", Date: "2026-12-01", DaysTo: ptr(10), Text: "2026-12-01 (in 10 days)"}},
			{Company: "DEMOB", Accounts: acct("main"), Value: household_page.HeldCell{Kind: "value", Text: "8,000.00"},
				Share:    household_page.HeldCell{Kind: "value", Text: "8.0 %"},
				Earnings: household_page.HeldEarnings{Kind: "gap", Reason: "no_row", Text: "no date: the earnings job has no row for this company"}},
		},
		RowsSum:   household_page.HeldCell{Kind: "value", Text: "20,000.00 USD"},
		LeadNotes: []string{"Demo profile: these rows are fixed and nothing was read."},
	}
	return p
}

func marshalHeld(p household_page.HeldPage, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	return json.Marshal(p)
}

// buildHeldFamily builds held_family_page per request for the day of dayFlag or today; the demo profile answers
// fixed rows with no read.
func buildHeldFamily(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New(heldFamilyName + ": no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	if a.profile.Name == "demo" {
		return marshalHeld(heldFamilyDemo(a, day), nil)
	}
	return marshalHeld(household_page.BuildHeld(ctx, heldFamilyConfig(a, rd), day))
}

// heldFamilyPageNamed returns the page for ot page and a close function: the stores opened once as the site opens
// them (no reader with a missing line of .env, the gap no_reader).
func heldFamilyPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New(heldFamilyName + " needs the owner's zone: " + zoneEnvName + " is not set")
	}
	rd := siteReaders{}
	closeFn := func() {}
	demo := a.profile.Name == "demo"
	open := false
	if !demo {
		env, err := a.envFile()
		if err != nil {
			return pages.Page{}, nil, err
		}
		open = env[pagesDSNName] != ""
	}
	if open {
		var err error
		if rd, closeFn, err = openSiteReaders(ctx, a); err != nil {
			return pages.Page{}, nil, err
		}
	}
	cfg := heldFamilyConfig(a, rd)
	cfg.RunID = "ot-page"
	build := func(ctx context.Context, _ store.SeriesReader, day contracts.Day, _ time.Time) ([]byte, error) {
		if demo {
			return marshalHeld(heldFamilyDemo(a, day), nil)
		}
		return marshalHeld(household_page.BuildHeld(ctx, cfg, day))
	}
	return pages.Page{Meta: pages.Meta{Name: heldFamilyName, Schema: "page.held_family_page.v1"}, Build: build}, closeFn, nil
}
