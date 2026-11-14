package main

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// postOwnerPriceLines serves POST /plan/price-lines: the guard of the owner posts (guardOwnerPost), then the save
// or the delete of the buy_price of one name on the underlying list, written through writeUnderlyings with one
// underlying.Edit. A refusal re-renders the page with 422; a write answers 303 to the page with saved=1, a no-op
// saved=0. Nothing is written before the guard passes.
// Design: docs/design/price-lines.md#2 price-lines-route.
func postOwnerPriceLines(a *app, clk clock.Clock, rd siteReaders, newHandler pages.NewHandlerFunc, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !guardOwnerPost(w, r, rd.edit) {
		return
	}
	fail := func(msg string) {
		a.log.Error(msg, "component", "ot", "status", "failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
	if rd.underlying == nil {
		fail("price lines post: no reader")
		return
	}
	ctx := r.Context()
	refuse := func(text string) {
		vm, err := buildPriceLinesPage(ctx, a, clk, rd, "", text)
		if err != nil {
			fail("price lines build failed")
			return
		}
		newHandler(vm).ServeHTTP(&statusWriter{ResponseWriter: w, code: http.StatusUnprocessableEntity}, r)
	}
	rows, err := rd.underlying.Current(r.Context())
	if err != nil {
		fail("price lines post: read failed")
		return
	}
	e, refusal := priceLineEdit(r.PostForm.Get("action"), r.PostForm, rows)
	if refusal != "" {
		refuse(refusal)
		return
	}
	n := 0
	if e != nil {
		n, err = writeUnderlyings(a, r, underlying.Change{Edits: []underlying.Edit{*e}})
		if err != nil {
			var ie underlying.InvalidError
			if errors.As(err, &ie) {
				refuse(ie.Why())
				return
			}
			fail("price lines write failed")
			return
		}
	}
	http.Redirect(w, r, "/"+priceLinesRoute+"?saved="+strconv.Itoa(n), http.StatusSeeOther)
}

// priceLineEdit reads the post against the current rows: the edit to write (nil: nothing differs) or the text of a
// refusal. The symbol is trimmed and upper-cased; the key is market:SYMBOL.
func priceLineEdit(action string, v url.Values, rows []underlying.Row) (*underlying.Edit, string) {
	sym := strings.ToUpper(strings.TrimSpace(v.Get("symbol")))
	key := strings.TrimSpace(v.Get("market")) + ":" + sym
	i := slices.IndexFunc(rows, func(r underlying.Row) bool { return r.Key == key })
	if i < 0 {
		return nil, "symbol: not on the underlying list; add it on /underlyings"
	}
	row := rows[i]
	switch action {
	case "delete":
		if row.BuyPrice == "" {
			return nil, ""
		}
		empty := ""
		return &underlying.Edit{Key: row.Key, BuyPrice: &empty}, ""
	case "save":
		price := strings.TrimSpace(v.Get("price"))
		if price == "" {
			return nil, "price: required"
		}
		e := underlying.Edit{Key: row.Key}
		changed := false
		if price != row.BuyPrice {
			e.BuyPrice, changed = &price, true
		}
		if note := strings.TrimSpace(v.Get("note")); note != "" && note != row.Note {
			e.Note, changed = &note, true
		}
		if !changed {
			return nil, ""
		}
		return &e, ""
	}
	return nil, "action: unknown"
}
