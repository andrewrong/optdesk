package main

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/pages/price_lines_page"
)

// priceLinesRoute is the route of the price lines page. The form of the page posts to the same path (the schema fixes it).
// Design: docs/design/price-lines.md#2 price-lines-route.
const priceLinesRoute = "plan/price-lines"

func init() {
	registerPage(pageFamily{route: priceLinesRoute,
		build: func(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
			var saved string
			if n := savedNotice(queryOf(ctx)); n != nil {
				saved = n.Text
			}
			return buildPriceLinesPage(ctx, a, clk, rd, saved, "")
		}})
}

// buildPriceLinesPage builds page.price_lines_page.v1 per request over the underlying reader of the site (nil: the
// gap no_reader). With rd.edit the page carries the token of the form. saved and refusal are the texts of the result
// line and of a refused post.
func buildPriceLinesPage(ctx context.Context, a *app, clk clock.Clock, rd siteReaders, saved, refusal string) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New("price lines: no zone")
	}
	now := clk.Now().UTC()
	in := price_lines_page.Input{
		RunID: "ot-serve", BuildID: buildIDOf(version), Scope: "price_lines",
		Reader: rd.underlying != nil, Saved: saved, Error: refusal, Now: now,
	}
	if rd.underlying != nil {
		var err error
		if in.Rows, err = rd.underlying.Current(ctx); err != nil {
			return nil, err
		}
		positions, err := heldPositions(ctx, rd, now)
		if err != nil {
			return nil, err
		}
		in.Held = map[string]bool{}
		addHeld(in.Held, positions)
	}
	if rd.edit != nil {
		tok := ownerDummyToken()
		in.Form = &price_lines_page.Form{Expires: tok.Expires, MAC: tok.MAC}
	}
	return price_lines_page.Build(in)
}
