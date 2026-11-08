package main

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

func TestServeFlags(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
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
	addr := ln.Addr().String()
	for _, args := range [][]string{{"--site"}, {"--name", "x"}} {
		err := runServe(a, append(args, "--addr", addr))
		if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
			t.Fatalf("%v: err %v", args, err)
		}
	}
	err = runServe(a, []string{"--day", "x", "--addr", addr})
	if err == nil || strings.Contains(err.Error(), "cannot listen") || strings.Contains(err.Error(), "flag provided") {
		t.Fatalf("bad day: err %v", err)
	}
}
