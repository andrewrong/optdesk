package main

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func reviewFields(extra url.Values) url.Values {
	v := url.Values{"company": {"US:AAA"}, "type": {"first"}, "version_date": {"2026-11-05"}, "v": {"100"}, "p": {"3"}}
	for k, s := range extra {
		v[k] = s
	}
	return v
}

func TestPreviewWritesNothing(t *testing.T) {
	t.Parallel()
	h, f := valuationSite(t, true)
	r := postRequest(t, reviewFields(url.Values{"action": {"preview"}}))
	r.URL.Path = "/valuation-post"
	r.Header.Set("Accept", "application/json")
	rec := serve(h, r)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") ||
		!strings.Contains(rec.Body.String(), `"schema":"valuation_page"`) || len(f.written()) != 0 {
		t.Fatalf("%d %q %.200q %v", rec.Code, rec.Header().Get("Content-Type"), rec.Body, f.written())
	}
	rec = vpost(t, h, "/valuation-post", reviewFields(url.Values{"action": {"preview"}}))
	if rec.Code != http.StatusOK || len(f.written()) != 0 {
		t.Fatalf("page answer: %d %v", rec.Code, f.written())
	}
}

func TestFailedCheckStaysDraft(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, v string }{{"zero", "0"}, {"empty", ""}} {
		h, f := valuationSite(t, true)
		rec := vpost(t, h, "/valuation-post", reviewFields(url.Values{"action": {"effective"}, "v": {c.v}}))
		if rec.Code != http.StatusUnprocessableEntity || !slices.Equal(f.written(), []string{"profile", "review:draft"}) {
			t.Fatalf("%s: %d %v %.300q", c.name, rec.Code, f.written(), rec.Body)
		}
		if !strings.Contains(rec.Body.String(), "draft") {
			t.Fatalf("%s: the problems are not listed: %.300q", c.name, rec.Body)
		}
	}
	h, f := valuationSite(t, true)
	rec := vpost(t, h, "/valuation-post", reviewFields(url.Values{"action": {"draft"}, "v": {""}, "p": {""}}))
	if rec.Code != http.StatusSeeOther || !slices.Contains(f.written(), "review:draft") || strings.Contains(rec.Body.String(), "problem") {
		t.Fatalf("draft: %d %v", rec.Code, f.written())
	}
}

func TestFirstSaveInsertsProfile(t *testing.T) {
	t.Parallel()
	h, f := valuationSite(t, true)
	vpost(t, h, "/valuation-post", reviewFields(url.Values{"action": {"draft"}, "review_id": {"r1"}}))
	if !slices.Equal(f.written(), []string{"profile", "review:draft"}) {
		t.Fatalf("calls %v", f.written())
	}
	p := f.profiles[0]
	if p.CompanyKey != "US:AAA" || p.RMarket != "US" || p.Pool != "in_pool" || p.Currency != "USD" || p.Name != "AAA" {
		t.Fatalf("profile %+v", p)
	}
	vpost(t, h, "/valuation-post", reviewFields(url.Values{"action": {"draft"}, "review_id": {"r1"}}))
	if !slices.Equal(f.written(), []string{"profile", "review:draft", "review:draft"}) {
		t.Fatalf("a second save wrote a profile: %v", f.written())
	}
}

func TestEffectiveWritesOnce(t *testing.T) {
	t.Parallel()
	h, f := valuationSite(t, true)
	rec := vpost(t, h, "/valuation-post", reviewFields(url.Values{"action": {"effective"}}))
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/valuation-company?company=US%3AAAA&") {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
	n := 0
	for _, c := range f.written() {
		if strings.HasPrefix(c, "review:") {
			n++
		}
	}
	if n != 1 || len(f.reviews) != 1 || f.reviews[0].Status != "effective" || f.reviews[0].Source != "owner" || f.reviews[0].V != "100" {
		t.Fatalf("%v %+v", f.written(), f.reviews)
	}
}

func TestReviewIdKeptAcrossDrafts(t *testing.T) {
	t.Parallel()
	h, f := valuationSite(t, true)
	vpost(t, h, "/valuation-post", reviewFields(url.Values{"action": {"draft"}, "review_id": {"rec-1"}}))
	vpost(t, h, "/valuation-post", reviewFields(url.Values{"action": {"draft"}, "review_id": {"rec-1"}, "v": {"110"}}))
	if len(f.reviews) != 2 || f.reviews[0].ReviewID != "rec-1" || f.reviews[1].ReviewID != "rec-1" {
		t.Fatalf("%+v", f.reviews)
	}
	rec := vpost(t, h, "/valuation-post", reviewFields(url.Values{"action": {"effective"}, "review_id": {"rec-1"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("effective: %d", rec.Code)
	}
	rec = vpost(t, h, "/valuation-post", reviewFields(url.Values{"action": {"draft"}, "review_id": {"rec-1"}}))
	if rec.Code != http.StatusUnprocessableEntity || len(f.reviews) != 3 {
		t.Fatalf("a save after effective: %d, %d rows", rec.Code, len(f.reviews))
	}
	// with no review_id the id is made and carried in the redirect
	rec = vpost(t, h, "/valuation-post", reviewFields(url.Values{"action": {"draft"}}))
	if !strings.Contains(rec.Header().Get("Location"), "review=r-") {
		t.Fatalf("%q", rec.Header().Get("Location"))
	}
}

func TestVoidNeedsReason(t *testing.T) {
	t.Parallel()
	h, f := valuationSite(t, true)
	vpost(t, h, "/valuation-post", reviewFields(url.Values{"action": {"effective"}, "review_id": {"rec-1"}}))
	before := len(f.reviews)
	rec := vpost(t, h, "/valuation-post", url.Values{"action": {"void"}, "company": {"US:AAA"}, "review_id": {"rec-1"}})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "void_reason") || len(f.reviews) != before {
		t.Fatalf("%d %.300q", rec.Code, rec.Body)
	}
	rec = vpost(t, h, "/valuation-post", url.Values{"action": {"void"}, "company": {"US:AAA"}, "review_id": {"rec-1"}, "void_reason": {"typed wrong"}})
	last := f.reviews[len(f.reviews)-1]
	if rec.Code != http.StatusSeeOther || last.Status != "void" || last.VoidReason != "typed wrong" || last.Source != "owner" || last.V != "100" {
		t.Fatalf("%d %+v", rec.Code, last)
	}
}

func TestProfileKeyRefusedWhenOnList(t *testing.T) {
	t.Parallel()
	h, f := valuationSite(t, true)
	rec := vpost(t, h, "/valuation-post", url.Values{"action": {"profile"}, "key": {"US:AAA"}, "name": {"x"}, "r_market": {"US"}})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "underlying list") || len(f.written()) != 0 {
		t.Fatalf("%d %.300q %v", rec.Code, rec.Body, f.written())
	}
	rec = vpost(t, h, "/valuation-post", url.Values{"action": {"profile"}, "key": {"US:ZZZ"}, "name": {"Zed"}, "symbol": {"ZZZ"}, "r_market": {"US"}})
	if rec.Code != http.StatusSeeOther || !slices.Equal(f.written(), []string{"profile"}) || f.profiles[0].CompanyKey != "US:ZZZ" {
		t.Fatalf("%d %v", rec.Code, f.written())
	}
}
