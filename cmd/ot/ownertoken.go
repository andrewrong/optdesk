package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"mime"
	"net/http"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
)

// Design: docs/design/owner-forms-remote.md#Issues (one owner token on every owner POST).

const (
	// ownerLoginRoute is the address of the login page and its post, under the site root.
	ownerLoginRoute = "owner-login"
	// ownerCookie is the name of the cookie that holds the owner token.
	ownerCookie = "ot_owner"
	// ownerCookieMaxAge is 30 days, in seconds.
	ownerCookieMaxAge = 2592000
	// minOwnerTokenBytes is the shortest OT_OWNER_TOKEN accepted.
	minOwnerTokenBytes = 32

	envOwnerToken       = "OT_OWNER_TOKEN"
	textOwnerTokenUnset = "owner token not set: set " + envOwnerToken
)

// tokenMatches compares got with want over equal-length SHA-256 sums in constant time. It is called by the login
// handler and by ownerCookieOK, and by nothing else.
func tokenMatches(got, want string) bool {
	g, w := sha256.Sum256([]byte(got)), sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(g[:], w[:]) == 1
}

// ownerTokenReady reports whether the token of e is set and long enough.
func ownerTokenReady(e *ownerEdit) bool { return e != nil && len(e.token) >= minOwnerTokenBytes }

// ownerCookieOK reports whether r carries the cookie ot_owner holding the owner token. A token that is not
// set (or short) matches nothing.
func ownerCookieOK(r *http.Request, e *ownerEdit) bool {
	if !ownerTokenReady(e) {
		return false
	}
	c, err := r.Cookie(ownerCookie)
	return err == nil && tokenMatches(c.Value, e.token)
}

// refuseOwner answers 403: the line naming OT_OWNER_TOKEN when the token is not usable, else "forbidden".
func refuseOwner(w http.ResponseWriter, e *ownerEdit) {
	if !ownerTokenReady(e) {
		http.Error(w, textOwnerTokenUnset, http.StatusForbidden)
		return
	}
	http.Error(w, "forbidden", http.StatusForbidden)
}

// guardOwnerPost applies the body limits (media type application/x-www-form-urlencoded, declared length within 64
// KiB, read under that cap), the cookie check and ParseForm. False means the 403 is written and nothing is to be
// written to the store.
func guardOwnerPost(w http.ResponseWriter, r *http.Request, e *ownerEdit) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/x-www-form-urlencoded" || r.ContentLength < 0 || r.ContentLength > ownerBodyMax {
		refuseOwner(w, e)
		return false
	}
	if !ownerCookieOK(r, e) {
		refuseOwner(w, e)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, ownerBodyMax)
	if err := r.ParseForm(); err != nil {
		refuseOwner(w, e)
		return false
	}
	return true
}

// ownerDummyToken is the constant expires and mac the pages carry where a form token stood: inert, the post
// ignores them.
func ownerDummyToken() settingsapi.FormToken {
	return settingsapi.FormToken{Expires: "0", MAC: strings.Repeat("0", 64)}
}

const ownerLoginPage = `<!doctype html><html><head><meta charset="utf-8"><title>owner login</title></head><body>
<form method="post" action="/` + ownerLoginRoute + `"><label>Owner token <input type="password" name="owner_token" autocomplete="current-password"></label>
<button type="submit">Log in</button></form></body></html>
`

// ownerLogin serves /owner-login and reports whether it handled r. GET shows one field; POST compares the field
// owner_token and sets the cookie ot_owner, then answers 303 to /. A wrong or unset token is 403.
func ownerLogin(rd siteReaders, w http.ResponseWriter, r *http.Request) bool {
	if strings.TrimPrefix(r.URL.Path, "/") != ownerLoginRoute {
		return false
	}
	e := rd.edit
	if !ownerTokenReady(e) {
		http.Error(w, textOwnerTokenUnset, http.StatusForbidden)
		return true
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(ownerLoginPage))
	case http.MethodPost:
		mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mt != "application/x-www-form-urlencoded" || r.ContentLength < 0 || r.ContentLength > ownerBodyMax {
			http.Error(w, "forbidden", http.StatusForbidden)
			return true
		}
		r.Body = http.MaxBytesReader(w, r.Body, ownerBodyMax)
		if r.ParseForm() != nil || !tokenMatches(r.PostForm.Get("owner_token"), e.token) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return true
		}
		http.SetCookie(w, &http.Cookie{Name: ownerCookie, Value: e.token, Path: "/", MaxAge: ownerCookieMaxAge,
			HttpOnly: true, SameSite: http.SameSiteStrictMode})
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
	return true
}
