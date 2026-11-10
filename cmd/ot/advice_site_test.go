package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAdviceRouteServesNotBuiltGapWithNoReader(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	a.profile.Name = "demo"
	h := siteHandler(a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", siteReaders{}, echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteAdviceRoute)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `"reason":"not_built"`) || strings.Contains(body, "no_reader") || strings.Contains(body, "sample_page") {
		t.Fatalf("%d %q", rec.Code, body)
	}
}

func TestAdvicePageNamedNeedsNoReader(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	p, closeFn, err := advicePageNamed(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	if p.Meta.Name != adviceName || p.Meta.Schema != "page.advice_page.v1" {
		t.Fatalf("meta %+v", p.Meta)
	}
	_ = context.Background
}
