package main

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/pages"
)

// marketName is the name of the page market_page, which ot page tests before pageNamed.
const marketName = "market_page"

// marketPage returns the page market_page built over the stored readings, and a close function
// the caller defers: the winner readings of the view opened once with the pages line of .env
// (reportReader), or the demo readings with the demo profile (no store). The series
// store is the argument of Build, which ot page opens with seriesReader. A nil zone is an error
// raised before any store is opened.
func marketPage(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New("market_page needs the owner's zone: " + zoneEnvName + " is not set")
	}
	if a.profile.Name == "demo" {
		return newMarketPage(a, demoMarketReadings), func() {}, nil
	}
	v, closeView, err := reportReader(ctx, a)
	if err != nil {
		return pages.Page{}, nil, err
	}
	return newMarketPage(a, v.WinnerReadings), closeView, nil
}

// newMarketPage calls deps.marketPage with the plain arguments, as newMacroPage fills its own.
func newMarketPage(a *app, readings compute.ReadingFunc) pages.Page {
	return a.deps.marketPage(a.profile.Zone, "ot-page", buildIDOf(version), "market", readings)
}
