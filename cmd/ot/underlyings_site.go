package main

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/pages/underlyings"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// siteUnderlyingsRoute is the address of the page underlyings under the site root. It is read-only unless the
// route /owner-underlyings is on (--owner-edit), when the page carries the token of the form.
const siteUnderlyingsRoute = "underlyings"

// underlyingsPostRoute is the address of the underlyings form post, under the site root. The page builder holds
// the same path as the post_path of the form.
const underlyingsPostRoute = "owner-underlyings"

func init() {
	registerPage(pageFamily{route: siteUnderlyingsRoute,
		build: func(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
			var notice string
			if rd.edit != nil {
				if n := savedNotice(queryOf(ctx)); n != nil {
					notice = n.Text
				}
			}
			return buildUnderlyingsPage(ctx, a, clk, dayFlag, rd, underlyingsExtras{notice: notice})
		}})
}

// underlyingsExtras is what a build of the page adds: the result line and the preview of a post.
type underlyingsExtras struct {
	notice  string
	confirm *underlyings.Confirm
}

// openUnderlyingReader opens the underlying reader with the pages line when deps has an opener, else it is nil
// (the page shows the gap no_reader). A failed open leaves it nil with one log line; the site still starts.
func openUnderlyingReader(ctx context.Context, a *app, dsn string, track func(io.Closer, error) bool) underlying.Reader {
	if a.deps.openUnderlying == nil {
		return nil
	}
	st, err := a.deps.openUnderlying(ctx, dsn)
	if !track(st, err) {
		a.log.Warn("underlying store not opened", "component", "ot", "status", "failed")
		return nil
	}
	return st
}

// buildUnderlyingsPage builds page.underlyings.v1 per request over the reader of the site (a nil reader gives no
// rows). With rd.edit the page carries the token of the form. A nil zone is an error.
func buildUnderlyingsPage(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders, x underlyingsExtras) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New("underlyings: no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	now := clk.Now().UTC()
	in := underlyings.Input{
		Confirm: x.confirm, Notice: x.notice, Now: now, SessionDay: string(day), Zone: a.profile.Zone,
	}
	if rd.underlying != nil {
		if in.Rows, err = rd.underlying.Current(ctx); err != nil {
			return nil, err
		}
		if in.History, err = rd.underlying.History(ctx, 20); err != nil {
			return nil, err
		}
		if in.Held, err = heldSymbols(ctx, rd, now); err != nil {
			return nil, err
		}
	}
	if rd.edit != nil {
		tok := rd.edit.form.Token(now)
		in.Form = &underlyings.Form{Expires: tok.Expires, MAC: tok.MAC}
	}
	return underlyings.Build(in)
}

// heldSymbols is the set of stock symbols with a nonzero quantity in the newest positions of each alias of the
// site, searched back from now. No account reader gives an empty set.
func heldSymbols(ctx context.Context, rd siteReaders, now time.Time) (map[string]bool, error) {
	held := map[string]bool{}
	if rd.account == nil {
		return held, nil
	}
	for _, alias := range rd.aliases {
		for back := 0; back <= heldLookbackDays; back++ {
			rows, err := rd.account.Positions(ctx, alias, now.AddDate(0, 0, -back).Format(time.DateOnly))
			if err != nil {
				return nil, err
			}
			if len(rows) == 0 {
				continue
			}
			addHeld(held, rows)
			break
		}
	}
	return held, nil
}

func addHeld(held map[string]bool, rows []store.Position) {
	for _, p := range rows {
		if p.SecType == "STK" && !isZeroQuantity(p.Quantity) {
			held[p.Symbol] = true
		}
	}
}
