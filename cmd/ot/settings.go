package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/settings_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

// siteSettingsRoute is the address of the page settings_page under the site root. It is read-only unless the
// route /owner-settings is on (--owner-edit), when the page carries the token of the form.
const siteSettingsRoute = "settings"

// ownerPostRoute is the address of the settings form post, under the site root.
const ownerPostRoute = "owner-settings"

// settingsHistoryN is the number of history rows the page reads.
const settingsHistoryN = 20

const (
	ownerBodyMax  = 64 << 10         // bytes of a posted form
	ownerTokenTTL = 15 * time.Minute // life of a form token
)

// ownerLoopbackNames are the three names a request to /owner-settings may carry in Host: 127.0.0.1, localhost and
// [::1]. settingsform checks the same names with the port; this is the first of the two checks.
var ownerLoopbackNames = []string{"127.0.0.1", "localhost", "[::1]"}

// ownerEdit is the write side of the settings page: the form guard and the store, opened once with --owner-edit.
type ownerEdit struct {
	form   settingsapi.SettingsForm
	writer ownerset.Writer
	listen string // the address the listener holds, with its port
}

// newOwnerEdit builds the write side from the env file. It is nil, with one log line naming the reason, when the
// route is left out: a line unset, the key short, or no opener or form constructor in deps. The returned func
// closes the store.
func newOwnerEdit(ctx context.Context, a *app, listen string) (*ownerEdit, func(), error) {
	noop := func() {}
	key, dsn, reason, err := a.ownerEditSecrets()
	if err != nil {
		return nil, noop, errors.New("serve: cannot read the env file for --owner-edit")
	}
	if reason == "" && (a.deps.openOwnerSet == nil || a.deps.newSettingsForm == nil) {
		reason = "no settings store in this build"
	}
	if reason != "" {
		a.log.Info("owner edit route left out", "component", "ot", "status", reason)
		return nil, noop, nil
	}
	st, err := a.deps.openOwnerSet(ctx, dsn)
	if err != nil {
		return nil, noop, errors.New("serve: cannot open the settings store")
	}
	return &ownerEdit{
		form:   a.deps.newSettingsForm(key, ownerTokenTTL, a.deps.newRuleParams()),
		writer: st, listen: listen,
	}, func() { _ = st.Close() }, nil
}

// settingsExtras is what a build of the page adds: the result line and the preview of a post.
type settingsExtras struct {
	notice  *settings_page.Notice
	confirm *settings_page.Confirm
}

// buildSettings builds settings_page per request for the day of dayFlag or today, over the readers of the site
// (rd.catalog, rd.names, and rd.owner, opened once per process; a nil owner gives the gap no_reader) and the
// registry of deps.newRuleParams. With rd.edit the page carries the token of the form and the notice of a save.
// A nil zone is an error.
func buildSettings(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders, notice *settings_page.Notice) ([]byte, error) {
	if rd.edit == nil {
		notice = nil
	}
	return buildSettingsPage(ctx, a, clk, dayFlag, rd, settingsExtras{notice: notice})
}

func buildSettingsPage(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders, x settingsExtras) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New("settings_page: no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	now := clk.Now().UTC()
	c := settings_page.Config{
		RunID: "ot-serve", BuildID: buildIDOf(version), Scope: "settings",
		Zone:     a.profile.Zone,
		Params:   a.deps.newRuleParams(),
		Owner:    rd.owner,
		Catalog:  rd.catalog,
		Names:    rd.names,
		HistoryN: settingsHistoryN,
		Notice:   x.notice,
		Confirm:  x.confirm,
	}
	if rd.edit != nil {
		tok := rd.edit.form.Token(now)
		c.Form = &tok
	}
	return a.deps.settingsPage(c).Build(ctx, nil, day, now)
}

// savedNotice is the result line of GET /settings?saved=<n>: n is digits only, 0 is "nothing changed"; any other
// text gives no line.
func savedNotice(q url.Values) *settings_page.Notice {
	s := q.Get("saved")
	n, err := strconv.Atoi(s)
	if err != nil || len(s) > 6 || strconv.Itoa(n) != s {
		return nil
	}
	if n == 0 {
		return &settings_page.Notice{Kind: "nothing_changed", Text: "Nothing changed."}
	}
	return &settings_page.Notice{Kind: "saved", Text: fmt.Sprintf("Saved %d row(s).", n)}
}

// statusWriter answers code where the page handler writes its implicit 200.
type statusWriter struct {
	http.ResponseWriter
	code    int
	written bool
}

func (w *statusWriter) WriteHeader(code int) {
	if w.written {
		return
	}
	w.written = true
	if code == http.StatusOK {
		code = w.code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return w.ResponseWriter.Write(b)
}

// hostNameOK reports whether the Host header carries one of ownerLoopbackNames.
func hostNameOK(hostport string) bool {
	h, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return false
	}
	if ip := net.ParseIP(h); ip != nil && ip.To4() == nil {
		h = "[" + h + "]"
	}
	return slices.Contains(ownerLoopbackNames, h)
}

// postOwnerSettings serves POST /owner-settings: the guard, then the preview (no confirm field) or the save
// (confirm=1). The token sits in the body, so the form is read, under a 64 KiB cap, before Check sees it; nothing
// is written before Check passes.
func postOwnerSettings(a *app, clk clock.Clock, dayFlag string, rd siteReaders, newHandler pages.NewHandlerFunc, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	e := rd.edit
	now := clk.Now().UTC()
	if !guardOwnerPost(w, r, e, now) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	fail := func() {
		a.log.Error("serve build failed", "component", "ot", "status", "failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
	ctx := r.Context()
	cur, shown, err := currentSettings(ctx, a, rd, now)
	if err != nil {
		fail()
		return
	}
	page := func(code int, x settingsExtras) {
		vm, err := buildSettingsPage(ctx, a, clk, dayFlag, rd, x)
		if err != nil {
			fail()
			return
		}
		newHandler(vm).ServeHTTP(&statusWriter{ResponseWriter: w, code: code}, r)
	}
	if unknownField(r.PostForm, a.deps.newRuleParams().Specs(), shown) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ch, err := e.form.Parse(r.PostForm, shown, cur)
	if err != nil {
		page(http.StatusUnprocessableEntity, settingsExtras{notice: &settings_page.Notice{Kind: "error", Text: err.Error()}})
		return
	}
	if len(ch.Params) == 0 && len(ch.Exclusions) == 0 {
		if ch.Confirm {
			http.Redirect(w, r, "/"+siteSettingsRoute+"?saved=0", http.StatusSeeOther)
			return
		}
		page(http.StatusOK, settingsExtras{notice: &settings_page.Notice{Kind: "nothing_changed", Text: "Nothing changed."}})
		return
	}
	if !ch.Confirm {
		rows, err := confirmRows(ctx, a, rd, ch, cur)
		if err != nil {
			fail()
			return
		}
		page(http.StatusOK, settingsExtras{confirm: &settings_page.Confirm{Rows: rows, Form: e.form.Token(now)}})
		return
	}
	n, err := e.writer.Apply(ctx, ownerChange(ch))
	if err != nil {
		a.log.Error("owner settings write failed", "component", "ot", "status", "failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/"+siteSettingsRoute+"?saved="+strconv.Itoa(n), http.StatusSeeOther)
}

// guardOwnerPost reads the form of r under the 64 KiB cap and applies the guard; false means 403 and no write.
func guardOwnerPost(w http.ResponseWriter, r *http.Request, e *ownerEdit, now time.Time) bool {
	if !hostNameOK(r.Host) || r.ContentLength < 0 || r.ContentLength > ownerBodyMax {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, ownerBodyMax)
	if err := r.ParseForm(); err != nil {
		return false
	}
	req := settingsapi.FormRequest{
		RemoteAddr: r.RemoteAddr, Host: r.Host, Origin: r.Header.Get("Origin"), Listen: e.listen,
		ContentType: r.Header.Get("Content-Type"), BodySize: r.ContentLength,
		Token: settingsapi.FormToken{Expires: r.PostForm.Get("expires"), MAC: r.PostForm.Get("mac")},
	}
	return e.form.Check(req, now) == nil
}

// unknownField reports a posted param.<key> that is not in the registry, or a row or excluded id that is not
// one of the shown ids: the post names something not on the page, which is 400 and no write. A refused value of
// a known key is not this; Parse answers it and it is 422.
func unknownField(values url.Values, specs []settingsapi.RuleParamSpec, shown []int64) bool {
	known := make(map[string]bool, len(specs))
	for _, s := range specs {
		known[s.Key] = true
	}
	for name := range values {
		if k, ok := strings.CutPrefix(name, "param."); ok && !known[k] {
			return true
		}
	}
	for _, name := range []string{"row", "excluded"} {
		for _, v := range values[name] {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil || !slices.Contains(shown, id) {
				return true
			}
		}
	}
	return false
}

// currentSettings reads the settings in force at now and the instrument ids the page shows (the catalog's
// underlyings and every instrument with an exclusion row). No owner reader is an error.
func currentSettings(ctx context.Context, a *app, rd siteReaders, now time.Time) (settingsapi.FormState, []int64, error) {
	var cur settingsapi.FormState
	if rd.owner == nil || rd.catalog == nil {
		return cur, nil, errors.New("settings: no reader")
	}
	params, err := rd.owner.Params(ctx, now)
	if err != nil {
		return cur, nil, err
	}
	flags, err := rd.owner.Exclusions(ctx, now)
	if err != nil {
		return cur, nil, err
	}
	latest, err := rd.catalog.OptionEodLatest(ctx, "")
	if err != nil {
		return cur, nil, err
	}
	stored := make(map[string]string, len(params))
	for _, p := range params {
		stored[p.Key] = p.Value
	}
	cur.Params = a.deps.newRuleParams().Resolve(stored)
	cur.Exclusions = make(map[int64]bool, len(flags))
	shown := make([]int64, 0, len(latest)+len(flags))
	for _, l := range latest {
		shown = append(shown, l.UnderlyingID)
	}
	for _, f := range flags {
		cur.Exclusions[f.InstrumentID] = f.Excluded
		shown = append(shown, f.InstrumentID)
	}
	slices.Sort(shown)
	return cur, slices.Compact(shown), nil
}

// confirmRows is the old-to-new table of a change, texts only.
func confirmRows(ctx context.Context, a *app, rd siteReaders, ch settingsapi.FormChange, cur settingsapi.FormState) ([]settings_page.ConfirmRow, error) {
	labels := map[string]string{}
	for _, s := range a.deps.newRuleParams().Specs() {
		labels[s.Key] = s.Label
	}
	var rows []settings_page.ConfirmRow
	for _, f := range ch.Params {
		rows = append(rows, settings_page.ConfirmRow{Label: labels[f.Key], Old: cur.Params[f.Key], New: f.Text})
	}
	ids := slices.Sorted(maps.Keys(ch.Exclusions))
	var names []store.InstrumentName
	if len(ids) > 0 && rd.names != nil {
		var err error
		if names, err = rd.names.InstrumentNames(ctx, ids); err != nil {
			return nil, err
		}
	}
	for _, id := range ids {
		label := "Underlying " + strconv.FormatInt(id, 10)
		for _, n := range names {
			if n.InstrumentID == id && n.Name != "" {
				label = n.Name
			}
		}
		rows = append(rows, settings_page.ConfirmRow{Label: "Exclude " + label, Old: flagText(cur.Exclusions[id]), New: flagText(ch.Exclusions[id])})
	}
	return rows, nil
}

func flagText(excluded bool) string {
	if excluded {
		return "excluded"
	}
	return "included"
}

// ownerChange maps the parsed form to the change the store writes.
func ownerChange(ch settingsapi.FormChange) ownerset.Change {
	var c ownerset.Change
	for _, f := range ch.Params {
		c.Params = append(c.Params, ownerset.ParamValue{Key: f.Key, Value: f.Text})
	}
	for _, id := range slices.Sorted(maps.Keys(ch.Exclusions)) {
		c.Exclusions = append(c.Exclusions, ownerset.ExclusionValue{InstrumentID: id, Excluded: ch.Exclusions[id]})
	}
	return c
}
