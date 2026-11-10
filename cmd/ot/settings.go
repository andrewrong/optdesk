package main

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/pages/settings_page"
)

// siteSettingsRoute is the address of the page settings_page under the site root. It is read-only here: Form
// and Confirm are nil, so the page answers form null (the POST route is a later issue).
const siteSettingsRoute = "settings"

// settingsHistoryN is the number of history rows the page reads.
const settingsHistoryN = 20

// buildSettings builds settings_page per request for the day of dayFlag or today, over the readers of the site
// (rd.catalog, rd.names, and rd.owner, opened once per process; a nil owner gives the gap no_reader) and the
// registry of deps.newRuleParams. A nil zone is an error.
func buildSettings(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New("settings_page: no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	page := a.deps.settingsPage(settings_page.Config{
		RunID: "ot-serve", BuildID: buildIDOf(version), Scope: "settings",
		Zone:     a.profile.Zone,
		Params:   a.deps.newRuleParams(),
		Owner:    rd.owner,
		Catalog:  rd.catalog,
		Names:    rd.names,
		HistoryN: settingsHistoryN,
	})
	return page.Build(ctx, nil, day, clk.Now().UTC())
}
