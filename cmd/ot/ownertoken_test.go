package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/settingsform"
)

func loginRequest(t *testing.T, token string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/owner-login", strings.NewReader(url.Values{"owner_token": {token}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.RemoteAddr, r.Host = postPeer, postHost
	return r
}

// tokenSite is the settings site whose owner token is token.
func tokenSite(t *testing.T, token string) (http.Handler, *fakeOwnerStore) {
	t.Helper()
	a := settingsApp(t)
	st := &fakeOwnerStore{}
	rd := siteReaders{owner: st, catalog: demoOption{}, names: demoOption{},
		edit: &ownerEdit{form: settingsform.New(nil, ownerTokenTTL, a.deps.newRuleParams()), writer: st, token: token}}
	return siteHandler(a, fakeClock{postNow}, "", rd, echoHandler, http.NotFoundHandler()), st
}

func TestOwnerPostsAllGuarded(t *testing.T) {
	t.Parallel()
	s := newFISite(t, fiOpts{})
	for _, route := range []string{ownerPostRoute, jobConsolePostRoute, underlyingsPostRoute, flexImportPostRoute} {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/"+route, strings.NewReader("confirm=1"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if route == flexImportPostRoute {
			ct, body := fiBody(t, []fiPart{fiFile("a.xml", fiXML(fiAccount, "1"))})
			r = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/"+route, strings.NewReader(string(body)))
			r.Header.Set("Content-Type", ct)
		}
		r.RemoteAddr, r.Host = postPeer, postHost
		if rec := serve(s.h, r); rec.Code != http.StatusForbidden {
			t.Errorf("%s: %d, want 403", route, rec.Code)
		}
	}
	fiNothingKept(t, s)
}

func TestOwnerPostWithCookieAndOtherContentTypeIsRefused(t *testing.T) {
	t.Parallel()
	h, st := postSite(t, true)
	for _, ct := range []string{"application/json", "multipart/form-data; boundary=x", "text/plain", ""} {
		r := postRequest(t, url.Values{"param.min_dte": {"30"}, "confirm": {"1"}})
		r.Header.Set("Content-Type", ct)
		if rec := serve(h, r); rec.Code != http.StatusForbidden || st.calls() != 0 {
			t.Errorf("%q: %d", ct, rec.Code)
		}
	}
	// a parameter after the media type is still the right type
	r := postRequest(t, url.Values{"param.min_dte": {"30"}, "confirm": {"1"}})
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	if rec := serve(h, r); rec.Code != http.StatusSeeOther {
		t.Errorf("charset: %d", rec.Code)
	}
}

func TestOwnerTokenUnsetOrShortIs403NamingTheLine(t *testing.T) {
	t.Parallel()
	for name, token := range map[string]string{"unset": "", "short": "short"} {
		h, st := tokenSite(t, token)
		posts := []*http.Request{postRequest(t, url.Values{"param.min_dte": {"30"}, "confirm": {"1"}}), loginRequest(t, token)}
		withOwnerCookie(posts[0], token)
		for _, r := range posts {
			rec := serve(h, r)
			if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "OT_OWNER_TOKEN") || st.calls() != 0 {
				t.Errorf("%s %s: %d %q", name, r.URL.Path, rec.Code, rec.Body)
			}
		}
		g := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/owner-login", nil)
		if rec := serve(h, g); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "OT_OWNER_TOKEN") {
			t.Errorf("%s GET: %d", name, rec.Code)
		}
	}
}

func TestOwnerLoginRoutesAbsentWithoutEdit(t *testing.T) {
	t.Parallel()
	h, _ := postSite(t, false)
	for _, m := range []string{http.MethodGet, http.MethodPost} {
		r := httptest.NewRequestWithContext(t.Context(), m, "/owner-login", nil)
		if rec := serve(h, r); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", m, rec.Code)
		}
	}
}

func TestOwnerLoginSetsCookieAndRedirects(t *testing.T) {
	t.Parallel()
	h, _ := postSite(t, true)
	rec := serve(h, loginRequest(t, postKey))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
	cs := rec.Result().Cookies()
	if len(cs) != 1 {
		t.Fatalf("%d cookies", len(cs))
	}
	c := cs[0]
	if c.Name != "ot_owner" || c.Value != postKey || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge != 2592000 || c.Secure {
		t.Fatalf("cookie %+v", c)
	}
	if hdr := rec.Header().Get("Set-Cookie"); !strings.Contains(hdr, "Max-Age=2592000") || strings.Contains(hdr, "Secure") {
		t.Fatalf("Set-Cookie %q", hdr)
	}
	// the cookie it hands out is the proof of a post.
	r := postRequest(t, url.Values{"param.min_dte": {"30"}, "confirm": {"1"}})
	r.Header.Del("Cookie")
	r.AddCookie(c)
	if rec := serve(h, r); rec.Code != http.StatusSeeOther {
		t.Fatalf("post with the login cookie: %d", rec.Code)
	}
	// the login page has one field
	g := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/owner-login", nil)
	if rec := serve(h, g); rec.Code != 200 || !strings.Contains(rec.Body.String(), `name="owner_token"`) {
		t.Fatalf("GET: %d %q", rec.Code, rec.Body)
	}
}

func TestOwnerLoginWrongTokenIs403(t *testing.T) {
	t.Parallel()
	h, _ := postSite(t, true)
	for _, tok := range []string{"", postKey + "x", postKey[:len(postKey)-1]} {
		rec := serve(h, loginRequest(t, tok))
		if rec.Code != http.StatusForbidden || len(rec.Result().Cookies()) != 0 {
			t.Errorf("%q: %d", tok, rec.Code)
		}
	}
}

// TestOwnerLoginLeaksNoToken logs in once with a wrong token and once with a right one, both made at run time, and
// finds neither in the log, the body or the Location header.
func TestOwnerLoginLeaksNoToken(t *testing.T) {
	t.Parallel()
	mk := func() string {
		var b [32]byte
		if _, err := rand.Read(b[:]); err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(b[:])
	}
	right, wrong := mk(), mk()
	a, logs := testApp(t, "")
	a.deps.newRuleParams = settingsApp(t).deps.newRuleParams
	st := &fakeOwnerStore{}
	rd := siteReaders{owner: st, catalog: demoOption{}, names: demoOption{},
		edit: &ownerEdit{form: settingsform.New(nil, ownerTokenTTL, a.deps.newRuleParams()), writer: st, token: right}}
	h := siteHandler(a, fakeClock{postNow}, "", rd, echoHandler, http.NotFoundHandler())
	for _, tok := range []string{wrong, right} {
		rec := serve(h, loginRequest(t, tok))
		for name, text := range map[string]string{"body": rec.Body.String(), "location": rec.Header().Get("Location")} {
			if strings.Contains(text, right) || strings.Contains(text, wrong) {
				t.Errorf("%s holds a token", name)
			}
		}
	}
	if strings.Contains(logs.String(), right) || strings.Contains(logs.String(), wrong) {
		t.Fatalf("the log holds a token: %q", logs)
	}
}
