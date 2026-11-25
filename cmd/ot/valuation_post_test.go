package main

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// vpost posts fields to path with the owner cookie; the path of postRequest is replaced.
func vpost(t *testing.T, h http.Handler, path string, fields url.Values) *httptest.ResponseRecorder {
	t.Helper()
	r := postRequest(t, fields)
	r.URL.Path = path
	return serve(h, r)
}

func TestValuationPostRoutesNeedOwnerEdit(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/valuation-post", "/valuation-params-post"} {
		off, fo := valuationSite(t, false)
		vpost(t, off, path, url.Values{"action": {"preview"}})
		if fo.opened != 0 || len(fo.written()) != 0 {
			t.Fatalf("%s with owner edit off reached the handler: opened %d", path, fo.opened)
		}
		on, fn := valuationSite(t, true)
		vpost(t, on, path, url.Values{"action": {"preview"}})
		if fn.opened != 1 {
			t.Fatalf("%s with owner edit on: opened %d, want 1", path, fn.opened)
		}
	}
}

func TestPostsNeedGuard(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/valuation-post", "/valuation-params-post"} {
		h, f := valuationSite(t, true)
		r := postRequest(t, url.Values{"action": {"draft"}, "company": {"US:AAA"}, "effective_date": {"2026-11-06"}})
		r.URL.Path = path
		r.Header.Del("Cookie") // no token
		rec := serve(h, r)
		if rec.Code != http.StatusForbidden && rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status %d", path, rec.Code)
		}
		if f.opened != 0 || len(f.written()) != 0 {
			t.Fatalf("%s: store touched before the guard: opened %d, %v", path, f.opened, f.written())
		}
		// a wrong content type is refused the same way
		r = postRequest(t, url.Values{"action": {"draft"}})
		r.URL.Path = path
		r.Header.Set("Content-Type", "text/plain")
		if rec := serve(h, r); rec.Code < 400 || f.opened != 0 {
			t.Fatalf("%s: text/plain gave %d, opened %d", path, rec.Code, f.opened)
		}
	}
}

func TestOpenValuationWired(t *testing.T) {
	t.Parallel()
	if newDeps().openValuation == nil {
		t.Fatal("openValuation is nil in the real deps")
	}
}

func paramsFields(extra url.Values) url.Values {
	v := url.Values{"effective_date": {"2026-11-06"}}
	maps.Copy(v, extra)
	return v
}

func TestAcceptRRecomputesOnServer(t *testing.T) {
	t.Parallel()
	h, f := valuationSite(t, true)
	rec := vpost(t, h, "/valuation-params-post", paramsFields(url.Values{
		"action": {"accept_r"}, "yield": {"4.2"}, "r_us": {"9"}, "r_jp": {"9"}, "r_hkcn": {"9"},
	}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/valuation-params?saved=1" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
	got := f.rvers[len(f.rvers)-1]
	if got.RUS != "0.85" || got.RJP != "0.75" || got.RHKCN != "0.65" || got.YieldUsed != "4.2" || got.EffectiveDate != "2026-11-06" {
		t.Fatalf("saved %+v: the posted R values must not be used", got)
	}
}

func TestStepAboveLimitSavesWithWarning(t *testing.T) {
	t.Parallel()
	h, f := valuationSite(t, true)
	rec := vpost(t, h, "/valuation-params-post", paramsFields(url.Values{"action": {"accept_r"}, "yield": {"10"}}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/valuation-params?saved=1&warned=1" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
	if !slices.Equal(f.written(), []string{"rversion"}) || f.rvers[len(f.rvers)-1].RUS != "0.7" {
		t.Fatalf("%v %+v", f.written(), f.rvers)
	}
}

func TestAISwitchNeedsExpiry(t *testing.T) {
	t.Parallel()
	h, f := valuationSite(t, true)
	typed := url.Values{"action": {"r_version"}, "r_us": {"0.9"}, "r_jp": {"0.8"}, "r_hkcn": {"0.7"}, "ai_switch": {"on"}}
	rec := vpost(t, h, "/valuation-params-post", paramsFields(typed))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "ai_expiry") || len(f.written()) != 0 {
		t.Fatalf("%d %.300q %v", rec.Code, rec.Body, f.written())
	}
	typed.Set("ai_expiry", "until the next report")
	rec = vpost(t, h, "/valuation-params-post", paramsFields(typed))
	got := f.rvers[len(f.rvers)-1]
	if rec.Code != http.StatusSeeOther || !got.AISwitch || got.AIExpiry != "until the next report" {
		t.Fatalf("%d %+v", rec.Code, got)
	}
}

func TestTableRowsMustAscend(t *testing.T) {
	t.Parallel()
	h, f := valuationSite(t, true)
	rec := vpost(t, h, "/valuation-params-post", paramsFields(url.Values{
		"action": {"r_table"}, "table": {"4.0 0.9 0.8 0.7\n3.0 0.8 0.7 0.6\n0.7 0.6 0.5"},
	}))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "line 2") || len(f.written()) != 0 {
		t.Fatalf("%d %.300q %v", rec.Code, rec.Body, f.written())
	}
	rec = vpost(t, h, "/valuation-params-post", paramsFields(url.Values{
		"action": {"r_table"}, "table": {"3.0 0.9 0.8 0.7\n4.0 0.8 0.7 0.6\n0.7 0.6 0.5"},
	}))
	if rec.Code != http.StatusSeeOther || len(f.rtables) != 2 || len(f.rtables[1].Rows) != 3 {
		t.Fatalf("%d %+v", rec.Code, f.rtables)
	}
	// the margin table writes one version, and the date is required
	rec = vpost(t, h, "/valuation-params-post", url.Values{"action": {"margin_table"}, "table": {"1 0.1 0.1 0.1 0.1"}})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "effective_date") {
		t.Fatalf("%d %.300q", rec.Code, rec.Body)
	}
	rec = vpost(t, h, "/valuation-params-post", paramsFields(url.Values{"action": {"margin_table"}, "table": {"1 0.1 0.1 0.1 0.1\n5 0.2 0.2 0.2 0.2"}}))
	if rec.Code != http.StatusSeeOther || len(f.margins) != 2 || len(f.margins[1].Rows) != 2 {
		t.Fatalf("%d %+v", rec.Code, f.margins)
	}
}

var _ = valuation.ErrClosed
