package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/ruleparam"
	"github.com/andrewrong/optdesk/internal/compute/settingsform"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
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

// TestServeOwnerEditFlagIsIgnored starts serve with and without --owner-edit over a settings line whose store
// cannot be opened: both reach newOwnerEdit (so the write forms are wired on every start, a non-loopback --addr
// included), and only the flag adds the one log line saying it is ignored.
func TestServeOwnerEditFlagIsIgnored(t *testing.T) {
	t.Parallel()
	for _, flag := range []bool{false, true} {
		a, logs := testApp(t, "OT_PG_DSN_SETTINGS=settings-dsn\n")
		a.profile.Name = "demo"
		a.deps.newRuleParams = ruleparam.New
		a.deps.newSettingsForm = settingsform.New
		opened := 0
		a.deps.openOwnerSet = func(context.Context, string) (ownerset.Store, error) {
			opened++
			return nil, errors.New("down")
		}
		args := []string{"--addr", "0.0.0.0:0"}
		if flag {
			args = append(args, "--owner-edit")
		}
		err := runServe(a, args)
		if err == nil || !strings.Contains(err.Error(), "cannot open the settings store") || opened != 1 {
			t.Fatalf("flag %v: err %v, %d opens", flag, err, opened)
		}
		if got := strings.Count(logs.String(), "--owner-edit is ignored"); got != map[bool]int{false: 0, true: 1}[flag] {
			t.Fatalf("flag %v: %d ignore lines in %q", flag, got, logs)
		}
	}
}
