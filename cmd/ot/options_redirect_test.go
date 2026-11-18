package main

import (
	"net/http"
	"testing"
)

func TestOptionsRouteRedirectsToCandidates(t *testing.T) {
	t.Parallel()
	h := newSiteFixture(t).h
	for _, m := range []string{http.MethodGet, http.MethodHead} {
		rec := candidatesGet(t, h, m, "/options")
		if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/options/candidates" {
			t.Errorf("%s: %d %q", m, rec.Code, rec.Header().Get("Location"))
		}
	}
	if rec := candidatesGet(t, h, http.MethodPost, "/options"); rec.Code == http.StatusFound {
		t.Errorf("POST redirected")
	}
	if rec := candidatesGet(t, h, http.MethodGet, "/options/data"); rec.Code == http.StatusFound {
		t.Errorf("options/data redirected")
	}
}
