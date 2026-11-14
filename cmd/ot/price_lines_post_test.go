package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// priceLineRows: AAA has no price (held), BBB has 50 and the held note, CCC has 7 and a note of its own.
func priceLineRows() []underlying.Row {
	rows := underlyingRows()
	rows[1].BuyPrice, rows[1].Note = "50", underlying.NoteHeld
	rows = append(rows, underlying.Row{Key: "US:CCC", Symbol: "CCC", Market: "US", Kind: underlying.KindStock,
		Status: underlying.StatusActive, BuyPrice: "7", Note: "mine", SetAt: rows[0].SetAt})
	return rows
}

func priceLinePost(t *testing.T, fields url.Values) *http.Request {
	t.Helper()
	r := postRequest(t, fields)
	r.URL.Path = "/plan/price-lines"
	return r
}

func priceLineSite(t *testing.T) (http.Handler, *fakeUnderlyingStore) {
	t.Helper()
	h, st, _ := underlyingsSite(t, true)
	st.rows = priceLineRows()
	return h, st
}

func TestPostPriceLineSymbolNormalised(t *testing.T) {
	t.Parallel()
	h, st := priceLineSite(t)
	rec := serve(h, priceLinePost(t, url.Values{"action": {"save"}, "symbol": {" aaa "}, "market": {"US"}, "price": {"10"}}))
	if rec.Code != http.StatusSeeOther || st.calls() != 1 || st.applied[0].Edits[0].Key != "US:AAA" {
		t.Fatalf("%d %+v", rec.Code, st.applied)
	}
}

func TestPostPriceLineSave(t *testing.T) {
	t.Parallel()
	h, st := priceLineSite(t)
	rec := serve(h, priceLinePost(t, url.Values{"action": {"save"}, "symbol": {"BBB"}, "market": {"US"}, "price": {"55"}, "note": {""}}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/plan/price-lines?saved=1" || st.calls() != 1 {
		t.Fatalf("%d %q %d", rec.Code, rec.Header().Get("Location"), st.calls())
	}
	e := st.applied[0].Edits
	if len(e) != 1 || e[0].BuyPrice == nil || *e[0].BuyPrice != "55" || e[0].Note != nil {
		t.Fatalf("%+v", e)
	}
}

func TestPostPriceLineSaveNote(t *testing.T) {
	t.Parallel()
	h, st := priceLineSite(t)
	serve(h, priceLinePost(t, url.Values{"action": {"save"}, "symbol": {"CCC"}, "market": {"US"}, "price": {"7"}, "note": {"new note"}}))
	if st.calls() != 1 {
		t.Fatalf("%d applies", st.calls())
	}
	e := st.applied[0].Edits[0]
	if e.Note == nil || *e.Note != "new note" || e.BuyPrice != nil {
		t.Fatalf("%+v", e)
	}
}

func TestPostPriceLineEmptyNoteKeepsNote(t *testing.T) {
	t.Parallel()
	h, st := priceLineSite(t)
	// the add form (a priced name) and the unset form (a name with no price; its note is the held one too).
	st.rows[0].Note = underlying.NoteHeld
	for _, sym := range []string{"BBB", "AAA"} {
		serve(h, priceLinePost(t, url.Values{"action": {"save"}, "symbol": {sym}, "market": {"US"}, "price": {"60"}, "note": {""}}))
	}
	if st.calls() != 2 {
		t.Fatalf("%d applies", st.calls())
	}
	for _, c := range st.applied {
		if c.Edits[0].Note != nil || c.Edits[0].BuyPrice == nil {
			t.Fatalf("%+v", c.Edits[0])
		}
	}
}

func TestPostPriceLineDelete(t *testing.T) {
	t.Parallel()
	h, st := priceLineSite(t)
	rec := serve(h, priceLinePost(t, url.Values{"action": {"delete"}, "symbol": {"BBB"}, "market": {"US"}}))
	if rec.Code != http.StatusSeeOther || st.calls() != 1 {
		t.Fatalf("%d %d", rec.Code, st.calls())
	}
	e := st.applied[0].Edits[0]
	want := underlying.Edit{Key: "US:BBB", BuyPrice: new(string)}
	if e.Key != want.Key || e.BuyPrice == nil || *e.BuyPrice != "" || e.Note != nil || e.Status != nil || e.SalePrice != nil {
		t.Fatalf("%+v", e)
	}
	// a name with no price writes nothing.
	rec = serve(h, priceLinePost(t, url.Values{"action": {"delete"}, "symbol": {"AAA"}, "market": {"US"}}))
	if rec.Header().Get("Location") != "/plan/price-lines?saved=0" || st.calls() != 1 {
		t.Fatalf("%q %d", rec.Header().Get("Location"), st.calls())
	}
}

func TestPostPriceLineEqualNoop(t *testing.T) {
	t.Parallel()
	h, st := priceLineSite(t)
	rec := serve(h, priceLinePost(t, url.Values{"action": {"save"}, "symbol": {"CCC"}, "market": {"US"}, "price": {"7"}, "note": {"mine"}}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/plan/price-lines?saved=0" || st.calls() != 0 {
		t.Fatalf("%d %q %d", rec.Code, rec.Header().Get("Location"), st.calls())
	}
}

func TestPostPriceLineUnknownKey(t *testing.T) {
	t.Parallel()
	h, st := priceLineSite(t)
	rec := serve(h, priceLinePost(t, url.Values{"action": {"save"}, "symbol": {"ZZZ"}, "market": {"US"}, "price": {"1"}}))
	if rec.Code != http.StatusUnprocessableEntity || st.calls() != 0 || !strings.Contains(rec.Body.String(), "not on the underlying list") {
		t.Fatalf("%d %d %.300q", rec.Code, st.calls(), rec.Body)
	}
}

func TestPostPriceLineEmptyPrice(t *testing.T) {
	t.Parallel()
	h, st := priceLineSite(t)
	rec := serve(h, priceLinePost(t, url.Values{"action": {"save"}, "symbol": {"BBB"}, "market": {"US"}, "price": {" "}}))
	if rec.Code != http.StatusUnprocessableEntity || st.calls() != 0 || !strings.Contains(rec.Body.String(), "required") {
		t.Fatalf("%d %d %.300q", rec.Code, st.calls(), rec.Body)
	}
}

func TestPostPriceLineRefusedByStore(t *testing.T) {
	t.Parallel()
	h, st := priceLineSite(t)
	st.err = invalidErr{field: "sale_price", why: "sale_price below buy_price"}
	rec := serve(h, priceLinePost(t, url.Values{"action": {"save"}, "symbol": {"BBB"}, "market": {"US"}, "price": {"99"}}))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "sale_price below buy_price") {
		t.Fatalf("%d %.300q", rec.Code, rec.Body)
	}
}

func TestPostPriceLineNoCookie(t *testing.T) {
	t.Parallel()
	h, st := priceLineSite(t)
	r := priceLinePost(t, url.Values{"action": {"delete"}, "symbol": {"BBB"}, "market": {"US"}})
	r.Header.Del("Cookie")
	if rec := serve(h, r); rec.Code != http.StatusForbidden || st.calls() != 0 {
		t.Fatalf("%d %d", rec.Code, st.calls())
	}
}

func TestPostPriceLineWrongMedia(t *testing.T) {
	t.Parallel()
	h, st := priceLineSite(t)
	r := priceLinePost(t, url.Values{"action": {"delete"}, "symbol": {"BBB"}, "market": {"US"}})
	r.Header.Set("Content-Type", "text/plain")
	if rec := serve(h, r); rec.Code != http.StatusForbidden || st.calls() != 0 {
		t.Fatalf("%d %d", rec.Code, st.calls())
	}
}
