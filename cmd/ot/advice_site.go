package main

import (
	"context"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/advice_page"
)

// Design: docs/design/pages-real-data.md#2 ot-advice-route.

// siteAdviceRoute is the address of the page advice_page under the site root.
const siteAdviceRoute = "advice"

// adviceName is the name of the page advice_page, which pageNamed selects for ot page.
const adviceName = "advice_page"

func init() {
	registerPage(pageFamily{name: adviceName, route: siteAdviceRoute, order: 70, build: buildAdvice, named: advicePageNamed})
}

// adviceConfig is the config of advice_page: the page shows its not_built gap and needs no reader, the
// session reader is passed only when the site has one.
func adviceConfig(a *app, runID string, rd siteReaders) advice_page.Config {
	return advice_page.Config{RunID: runID, BuildID: buildIDOf(version), Zone: a.profile.Zone, Sessions: rd.sessions}
}

// buildAdvice builds advice_page per request for the day of dayFlag or today.
func buildAdvice(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New(adviceName + ": no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	return advice_page.Build(ctx, adviceConfig(a, "ot-serve", rd), day, clk.Now().UTC())
}

// advicePageNamed returns advice_page for ot page; it opens nothing, so the close function does nothing.
func advicePageNamed(_ context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New(adviceName + " needs the owner's zone: " + zoneEnvName + " is not set")
	}
	return advice_page.NewPage(adviceConfig(a, "ot-page", siteReaders{})), func() {}, nil
}

var _ = time.Time{}
