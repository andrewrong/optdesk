package main

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/underlyings"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// editTexts are the text fields of an edit form, in the order of the confirm table. exchange and currency are read
// only when they are not empty: the page does not show their stored value.
var editTexts = []string{"status", "name", "issuer", "exchange", "currency", "buy_price", "sale_price", "report_date_override", "note"}

// formError is the refusal of the route itself, read like an underlying.InvalidError.
type formError struct{ field, why string }

func (e formError) Error() string { return "underlyings: " + e.field + ": " + e.why }
func (e formError) Field() string { return e.field }
func (e formError) Why() string   { return e.why }

// underlyingsForm is a parsed post: the change the store writes and the preview of it.
type underlyingsForm struct {
	change  underlying.Change
	preview underlyings.Confirm
	confirm bool
}

// postOwnerUnderlyings serves POST /owner-underlyings: the guard of /owner-settings (guardOwnerPost), then the
// preview (no confirm field) or the save (confirm=1). Nothing is written before the guard passes or before confirm=1.
func postOwnerUnderlyings(a *app, clk clock.Clock, dayFlag string, rd siteReaders, newHandler pages.NewHandlerFunc, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	now := clk.Now().UTC()
	if !guardOwnerPost(w, r, rd.edit, now) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	fail := func(msg string) {
		a.log.Error(msg, "component", "ot", "status", "failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
	ctx := r.Context()
	if rd.underlying == nil {
		fail("underlyings post: no reader")
		return
	}
	page := func(code int, x underlyingsExtras) {
		vm, err := buildUnderlyingsPage(ctx, a, clk, dayFlag, rd, x)
		if err != nil {
			fail("serve build failed")
			return
		}
		newHandler(vm).ServeHTTP(&statusWriter{ResponseWriter: w, code: code}, r)
	}
	refused := func(err error) {
		var ie underlying.InvalidError
		if errors.As(err, &ie) {
			page(http.StatusUnprocessableEntity, underlyingsExtras{notice: "Refused " + ie.Field() + ": " + ie.Why()})
			return
		}
		fail("owner underlyings write failed")
	}
	f, err := readUnderlyingsForm(r, rd, now)
	if err != nil {
		refused(err)
		return
	}
	if len(f.change.Adds) == 0 && len(f.change.Edits) == 0 {
		if f.confirm {
			http.Redirect(w, r, "/"+siteUnderlyingsRoute+"?saved=0", http.StatusSeeOther)
			return
		}
		page(http.StatusOK, underlyingsExtras{notice: "Nothing changed."})
		return
	}
	if !f.confirm {
		page(http.StatusOK, underlyingsExtras{confirm: &f.preview})
		return
	}
	n, err := writeUnderlyings(a, r, f.change)
	if err != nil {
		refused(err)
		return
	}
	http.Redirect(w, r, "/"+siteUnderlyingsRoute+"?saved="+strconv.Itoa(n), http.StatusSeeOther)
}

// readUnderlyingsForm reads the list and the held names and parses the post against them. A read error is not an
// InvalidError, so the caller answers it as 500.
func readUnderlyingsForm(r *http.Request, rd siteReaders, now time.Time) (underlyingsForm, error) {
	rows, err := rd.underlying.Current(r.Context())
	if err != nil {
		return underlyingsForm{}, errors.New("underlyings post: read failed")
	}
	held, err := heldSymbols(r.Context(), rd, now)
	if err != nil {
		return underlyingsForm{}, errors.New("underlyings post: read failed")
	}
	return parseUnderlyingsForm(r.PostForm, rows, held)
}

// writeUnderlyings opens the writer with the OT_PG_DSN_SETTINGS line, applies c and closes it. An error that is
// not an underlying.InvalidError carries no field value.
func writeUnderlyings(a *app, r *http.Request, c underlying.Change) (int, error) {
	_, dsn, reason, err := a.ownerEditSecrets()
	if err != nil || reason != "" || a.deps.openUnderlying == nil {
		return 0, errors.New("underlyings: no writer")
	}
	st, err := a.deps.openUnderlying(r.Context(), dsn)
	if err != nil {
		return 0, errors.New("underlyings: cannot open the store")
	}
	defer func() { _ = st.Close() }()
	return st.Apply(r.Context(), c)
}

// parseUnderlyingsForm reads a post. A post with a key is an edit of that row: its fields are the whole form, so
// an absent switch is off. A post with no key and a symbol is an add. Only fields that differ from the row are
// written. A key that is not on the list, or a save that leaves no active name, is refused.
func parseUnderlyingsForm(v url.Values, rows []underlying.Row, held map[string]bool) (underlyingsForm, error) {
	f := underlyingsForm{confirm: v.Get("confirm") == "1"}
	if key := v.Get("key"); key != "" {
		i := slices.IndexFunc(rows, func(r underlying.Row) bool { return r.Key == key })
		if i < 0 {
			return f, formError{"key", "not on the list"}
		}
		e, old := editOf(v, rows[i])
		if e != nil {
			f.change.Edits = append(f.change.Edits, *e)
			f.preview = previewOf(*e, rows[i], old, held[rows[i].Symbol])
		}
		if lastActive(rows, f.change.Edits) {
			return f, formError{"status", "at least one name must stay active"}
		}
		return f, nil
	}
	if strings.TrimSpace(v.Get("symbol")) == "" {
		return f, nil
	}
	ad := addOf(v)
	f.change.Adds = append(f.change.Adds, ad)
	f.preview.Changes = append(f.preview.Changes, underlyings.Change{Label: "Add " + ad.Market + ":" + ad.Symbol, Old: "", New: "pending"})
	return f, nil
}

func addOf(v url.Values) underlying.Add {
	t := func(k string) string { return strings.TrimSpace(v.Get(k)) }
	return underlying.Add{
		Symbol: t("symbol"), Market: t("market"), Kind: t("kind"), Exchange: t("exchange"), Currency: t("currency"),
		Name: t("name"), Issuer: t("issuer"), Options: switchOn(v, "options"), Fundamentals: switchOn(v, "fundamentals"),
		BuyPrice: t("buy_price"), SalePrice: t("sale_price"), ReportDateOverride: t("report_date_override"), Note: t("note"),
	}
}

// switchOn reads a switch of a form: present and not off, 0 or false.
func switchOn(v url.Values, k string) bool {
	s := strings.ToLower(strings.TrimSpace(v.Get(k)))
	return s != "" && s != "off" && s != "0" && s != "false"
}

// editOf returns the edit that makes row r what the post says, nil when nothing differs, and the row as edited.
func editOf(v url.Values, r underlying.Row) (*underlying.Edit, underlying.Row) {
	e := underlying.Edit{Key: r.Key}
	n := r
	ptrs := map[string]**string{
		"status": &e.Status, "name": &e.Name, "issuer": &e.Issuer, "exchange": &e.Exchange, "currency": &e.Currency,
		"buy_price": &e.BuyPrice, "sale_price": &e.SalePrice, "report_date_override": &e.ReportDateOverride, "note": &e.Note,
	}
	cur := map[string]*string{
		"status": &n.Status, "name": &n.Name, "issuer": &n.Issuer, "exchange": &n.Exchange, "currency": &n.Currency,
		"buy_price": &n.BuyPrice, "sale_price": &n.SalePrice, "report_date_override": &n.ReportDateOverride, "note": &n.Note,
	}
	old := map[string]string{
		"status": r.Status, "name": r.Name, "issuer": r.Issuer, "exchange": r.Exchange, "currency": r.Currency,
		"buy_price": r.BuyPrice, "sale_price": r.SalePrice, "report_date_override": r.ReportDateOverride, "note": r.Note,
	}
	changed := false
	for _, k := range editTexts {
		if !v.Has(k) {
			continue
		}
		s := strings.TrimSpace(v.Get(k))
		was := old[k]
		if (k == "exchange" || k == "currency") && s == "" || k == "name" && was == "" && s == r.Symbol {
			continue
		}
		if s != was {
			text := s
			*ptrs[k] = &text
			*cur[k] = s
			changed = true
		}
	}
	if o := switchOn(v, "options"); o != r.Options {
		e.Options, n.Options, changed = &o, o, true
	}
	if o := switchOn(v, "fundamentals"); o != r.Fundamentals {
		e.Fundamentals, n.Fundamentals, changed = &o, o, true
	}
	if !changed {
		return nil, r
	}
	return &e, n
}

// previewOf is the old-to-new table of an edit and the groups that stop for the name. A group that a held name
// keeps is not named.
func previewOf(e underlying.Edit, was underlying.Row, now underlying.Row, held bool) underlyings.Confirm {
	var c underlyings.Confirm
	add := func(field, o, n string) {
		c.Changes = append(c.Changes, underlyings.Change{Label: was.Symbol + " " + field, Old: o, New: n})
	}
	text := func(field string, o, n string, p *string) {
		if p != nil {
			add(field, o, n)
		}
	}
	text("status", was.Status, now.Status, e.Status)
	text("name", was.Name, now.Name, e.Name)
	text("issuer", was.Issuer, now.Issuer, e.Issuer)
	text("exchange", was.Exchange, now.Exchange, e.Exchange)
	text("currency", was.Currency, now.Currency, e.Currency)
	text("buy_price", was.BuyPrice, now.BuyPrice, e.BuyPrice)
	text("sale_price", was.SalePrice, now.SalePrice, e.SalePrice)
	text("report_date_override", was.ReportDateOverride, now.ReportDateOverride, e.ReportDateOverride)
	text("note", was.Note, now.Note, e.Note)
	if e.Options != nil {
		add("options", onOff(was.Options), onOff(now.Options))
	}
	if e.Fundamentals != nil {
		add("fundamentals", onOff(was.Fundamentals), onOff(now.Fundamentals))
	}
	before := underlyings.Follows(was.Status, was.Options, was.Fundamentals, was.Issuer != "", held)
	after := underlyings.Follows(now.Status, now.Options, now.Fundamentals, now.Issuer != "", held)
	for _, g := range before {
		if !slices.Contains(after, g) {
			c.Stops = append(c.Stops, underlyings.Stop{Group: g, Symbol: was.Symbol, Stopped: true})
		}
	}
	return c
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// lastActive reports whether the edits leave the list with no active name while it had one.
func lastActive(rows []underlying.Row, edits []underlying.Edit) bool {
	had, has := false, false
	for _, r := range rows {
		status := r.Status
		had = had || status == underlying.StatusActive
		for _, e := range edits {
			if e.Key == r.Key && e.Status != nil {
				status = *e.Status
			}
		}
		has = has || status == underlying.StatusActive
	}
	return had && !has
}
