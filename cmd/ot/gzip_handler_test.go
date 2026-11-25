package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func gzipDo(t *testing.T, h http.Handler, method, ae string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, "/health", nil)
	if ae != "" {
		req.Header.Set("Accept-Encoding", ae)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func typed(ct, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Length", "99")
		_, _ = io.WriteString(w, body)
	})
}

func TestGzipHandlerCompresses(t *testing.T) {
	t.Parallel()
	body := strings.Repeat("<p>hello</p>", (1<<20)/12+1)[:1<<20]
	rec := gzipDo(t, gzipHandler(typed("text/html; charset=utf-8", body)), http.MethodGet, "gzip, deflate", nil)
	if rec.Header().Get("Content-Encoding") != "gzip" || rec.Header().Get("Content-Length") != "" {
		t.Fatalf("headers %v", rec.Header())
	}
	zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(zr)
	if err != nil || string(got) != body {
		t.Fatalf("roundtrip err=%v len=%d", err, len(got))
	}
	if rec.Body.Len() >= len(body) {
		t.Fatalf("not smaller: %d", rec.Body.Len())
	}
}

func TestGzipHandlerPlain(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, method, ae, ct string
		hdr                  map[string]string
	}{
		{"no header", http.MethodGet, "", "text/html", nil},
		{"q0", http.MethodGet, "gzip;q=0", "text/html", nil},
		{"head", http.MethodHead, "gzip", "text/html", nil},
		{"png", http.MethodGet, "gzip", "image/png", nil},
		{"range", http.MethodGet, "gzip", "text/html", map[string]string{"Range": "bytes=0-1"}},
	}
	for _, c := range cases {
		rec := gzipDo(t, gzipHandler(typed(c.ct, "plain body")), c.method, c.ae, c.hdr)
		if rec.Header().Get("Content-Encoding") != "" || rec.Header().Get("Vary") != "Accept-Encoding" {
			t.Fatalf("%s: headers %v", c.name, rec.Header())
		}
		if c.method != http.MethodHead && rec.Body.String() != "plain body" {
			t.Fatalf("%s: body %q", c.name, rec.Body.String())
		}
	}
}

func TestGzipHandlerSkipsEncodedAndNoContent(t *testing.T) {
	t.Parallel()
	enc := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Encoding", "br")
		_, _ = io.WriteString(w, "x")
	})
	if rec := gzipDo(t, gzipHandler(enc), http.MethodGet, "gzip", nil); rec.Header().Get("Content-Encoding") != "br" || rec.Body.String() != "x" {
		t.Fatalf("%v %q", rec.Header(), rec.Body.String())
	}
	nm := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotModified)
	})
	if rec := gzipDo(t, gzipHandler(nm), http.MethodGet, "gzip", nil); rec.Header().Get("Content-Encoding") != "" || rec.Code != 304 {
		t.Fatalf("%v %d", rec.Header(), rec.Code)
	}
}

func TestGzipHandlerFlush(t *testing.T) {
	t.Parallel()
	h := gzipHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{}")
		w.(http.Flusher).Flush()
	}))
	rec := gzipDo(t, h, http.MethodGet, "gzip", nil)
	if !rec.Flushed || rec.Body.Len() == 0 {
		t.Fatalf("flushed=%v len=%d", rec.Flushed, rec.Body.Len())
	}
}

func TestGzipHandlerHealth(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	clk := fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	h := gzipHandler(siteHandler(a, clk, "2026-01-01", siteReaders{series: &healthSeries{}}, echoHandler, http.NotFoundHandler()))
	for _, ae := range []string{"", "gzip"} {
		rec := gzipDo(t, h, http.MethodGet, ae, nil)
		body := rec.Body.Bytes()
		if ae != "" {
			zr, err := gzip.NewReader(bytes.NewReader(body))
			if err != nil {
				t.Fatalf("ae=%q: %v", ae, err)
			}
			body, _ = io.ReadAll(zr)
		}
		if rec.Code != 200 || strings.TrimSpace(string(body)) != `{"status":"ok"}` {
			t.Fatalf("ae=%q: %d %q", ae, rec.Code, body)
		}
	}
}

func TestGzipHandlerFlushBeforeWrite(t *testing.T) {
	t.Parallel()
	h := gzipHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.(http.Flusher).Flush()
		_, _ = io.WriteString(w, `{"a":1}`)
	}))
	rec := gzipDo(t, h, http.MethodGet, "gzip", nil)
	res := rec.Result() // headers as sent at the first flush
	body, _ := io.ReadAll(res.Body)
	if res.Header.Get("Content-Encoding") == "" && string(body) != `{"a":1}` {
		t.Fatalf("body is gzip but sent headers have no Content-Encoding: %q", body)
	}
}

func TestGzipHandlerNoContentStaysPlain(t *testing.T) {
	t.Parallel()
	h := gzipHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}))
	if rec := gzipDo(t, h, http.MethodGet, "gzip", nil); rec.Code != 204 || rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
}

func TestGzipHandlerSniffedTypeKept(t *testing.T) {
	t.Parallel()
	h := gzipHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "<!DOCTYPE html><html><body>hi</body></html>")
	}))
	rec := gzipDo(t, h, http.MethodGet, "gzip", nil)
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("not compressed: %v", rec.Header())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}
}
