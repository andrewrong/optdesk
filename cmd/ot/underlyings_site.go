package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/jobs/barsrefresh"
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
// (the page shows the gap no_reader). A failed open leaves it nil with one log line; the site still starts. A store
// that is not an underlying.HeldAdder is a start-up error: the result is false, with one Warn line and the store closed.
func openUnderlyingReader(ctx context.Context, a *app, dsn string, track func(io.Closer, error) bool) (underlying.Reader, bool) {
	if a.deps.openUnderlying == nil {
		return nil, true
	}
	st, err := a.deps.openUnderlying(ctx, dsn)
	if !track(st, err) {
		a.log.Warn("underlying store not opened", "component", "ot", "status", "failed")
		return nil, true
	}
	if _, ok := st.(underlying.HeldAdder); !ok {
		a.log.Warn("underlying store cannot check held names", "component", "ot", "status", "failed")
		_ = st.Close()
		return nil, false
	}
	return st, true
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
		positions, perr := heldPositions(ctx, rd, now)
		if perr != nil {
			in.HeldError = true
		} else {
			in.Held, in.HeldOptions = map[string]bool{}, map[string]bool{}
			addHeld(in.Held, positions)
			addHeldOptions(in.HeldOptions, positions)
			adder, _ := rd.underlying.(underlying.HeldAdder)
			in.HeldRefused = heldRefused(positions, in.Rows, adder)
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
	held, _, err := heldSets(ctx, rd, now)
	return held, err
}

// heldSets reads the newest positions of each alias once and returns the stock symbols held (shares) and the symbols
// with an option position of nonzero quantity.
func heldSets(ctx context.Context, rd siteReaders, now time.Time) (shares, options map[string]bool, err error) {
	rows, err := heldPositions(ctx, rd, now)
	if err != nil {
		return nil, nil, err
	}
	shares, options = map[string]bool{}, map[string]bool{}
	addHeld(shares, rows)
	addHeldOptions(options, rows)
	return shares, options, nil
}

// heldPositions is the newest positions of each alias of the site, searched back from now, with their currency. No
// account reader gives none.
func heldPositions(ctx context.Context, rd siteReaders, now time.Time) ([]store.Position, error) {
	var all []store.Position
	if rd.account == nil {
		return nil, nil
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
			all = append(all, rows...)
			break
		}
	}
	return all, nil
}

// heldRefused maps each held symbol (shares or options) with no row to the reason it cannot be added: the text of an
// index option for an option on an index symbol, else the Why of the InvalidError of adder.Check. A nil adder gives
// no reason for a name that is not an index option.
func heldRefused(positions []store.Position, rows []underlying.Row, adder underlying.HeldAdder) map[string]string {
	have := map[string]bool{}
	for _, r := range rows {
		have[strings.ToUpper(r.Symbol)] = true
	}
	indexes := map[string]bool{}
	if m, err := barsrefresh.DefaultManifest(); err == nil {
		for _, i := range m.Indexes {
			indexes[strings.ToUpper(i.Symbol)] = true
		}
	}
	out := map[string]string{}
	for _, p := range positions {
		if (p.SecType != heldSecStock && p.SecType != heldSecOption) || isZeroQuantity(p.Quantity) || have[strings.ToUpper(p.Symbol)] {
			continue
		}
		if p.SecType == heldSecOption && indexes[strings.ToUpper(p.Symbol)] {
			out[p.Symbol] = heldIndexText(p.Currency)
			continue
		}
		if adder == nil {
			continue
		}
		if _, err := adder.Check(p.Symbol, p.Currency); err != nil {
			var ie underlying.InvalidError
			if errors.As(err, &ie) {
				out[p.Symbol] = ie.Why()
			}
		}
	}
	return out
}

func addHeld(held map[string]bool, rows []store.Position) {
	for _, p := range rows {
		if p.SecType == "stock" && !isZeroQuantity(p.Quantity) {
			held[p.Symbol] = true
		}
	}
}

// addHeldOptions adds the underlying symbol of each option position with a nonzero quantity.
func addHeldOptions(held map[string]bool, rows []store.Position) {
	for _, p := range rows {
		if p.SecType == "option" && !isZeroQuantity(p.Quantity) {
			held[p.Symbol] = true
		}
	}
}
