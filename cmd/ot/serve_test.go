package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

type result struct {
	status int
	body   string
}

// serveAndGet runs serveOn on a loopback listener, GETs once, stops it and returns the answer.
func serveAndGet(t *testing.T, build func(context.Context) ([]byte, error)) result {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- serveOn(ctx, ln, build, func(vm []byte) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(vm) })
		})
	}()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+ln.Addr().String()+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serveOn returned %v", err)
	}
	return result{resp.StatusCode, string(b)}
}

func TestServeOnAnswersBuilderBytes(t *testing.T) {
	t.Parallel()
	n := 0
	got := serveAndGet(t, func(context.Context) ([]byte, error) { n++; return []byte("vm"), nil })
	if got.status != 200 || got.body != "vm" || n != 1 {
		t.Fatalf("got %+v, builds %d", got, n)
	}
}

func TestServeOnBuildError500(t *testing.T) {
	t.Parallel()
	got := serveAndGet(t, func(context.Context) ([]byte, error) { return nil, errors.New("dsn sentinel-secret") })
	if got.status != 500 || strings.Contains(got.body, "sentinel") {
		t.Fatalf("got %+v", got)
	}
}

func TestServeUnknownNameBindsNoPort(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
	var call pageCall
	pageApp(a, &call)
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		t.Error("a store was opened")
		return nil, nil
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	_, want := pageNamed(a, "nosuch")
	err = runServe(a, []string{"--name", "nosuch", "--addr", ln.Addr().String()})
	if err == nil || err.Error() != want.Error() {
		t.Fatalf("err %v want %v", err, want)
	}
}

func TestServeMacroPage(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	a.deps.pageList = []pages.Page{fakePage("macro_rates", "RATES")}
	var gotZone *time.Location
	a.deps.macroPage = func(z *time.Location, _ string) pages.Page {
		gotZone = z
		return fakePage("macro_page", "MACRO")
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	// The port is taken, so serve stops at the listen, after it has built the page by name.
	err = runServe(a, []string{"--name", "macro_page", "--addr", ln.Addr().String()})
	if err == nil || !strings.Contains(err.Error(), "cannot listen") || gotZone != a.profile.Zone || gotZone == nil {
		t.Fatalf("err %v zone %v", err, gotZone)
	}
	err = runServe(a, []string{"--name", "nosuch", "--addr", ln.Addr().String()})
	if err == nil || !strings.Contains(err.Error(), "known pages: macro_rates, macro_page") {
		t.Fatalf("unknown name: err %v", err)
	}
}
