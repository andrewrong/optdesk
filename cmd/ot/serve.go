package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func init() { register("serve", runServe) }

const shutdownGrace = 5 * time.Second

// serveHandler serves h on ln until ctx is done, then shuts down within shutdownGrace.
func serveHandler(ctx context.Context, ln net.Listener, h http.Handler) error {
	srv := &http.Server{ReadHeaderTimeout: shutdownGrace, Handler: h}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		return err
	}
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// runServe serves the whole site: the six stores are opened once and closed on exit.
func runServe(a *app, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dayFlag := fs.String("day", "", "day YYYY-MM-DD (default: today in "+marketZone+")")
	addr := fs.String("addr", "127.0.0.1:8080", "listen address")
	edit := fs.Bool("owner-edit", false, "accepted and ignored: the owner forms are on whenever the settings DSN line exists")
	live := fs.Bool("live-quotes", false, "serve POST /options/refresh, live quotes through pyside (loopback address only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *live && !loopbackListen(*addr) {
		return errors.New("serve: --live-quotes needs a loopback --addr")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	clk := a.deps.newClock()
	if _, err := dayOrDefault(clk, *dayFlag); err != nil {
		return err
	}
	rd, closeFn, err := openSiteReaders(ctx, a)
	if err != nil {
		return err
	}
	defer closeFn()
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", *addr)
	if err != nil {
		return fmt.Errorf("serve: cannot listen on %s", *addr)
	}
	if *edit {
		a.log.Info("--owner-edit is ignored", "component", "ot", "status", "ignored")
	}
	oe, closeEdit, err := newOwnerEdit(ctx, a)
	if err != nil {
		_ = ln.Close()
		return err
	}
	defer closeEdit()
	rd.edit = oe
	if *live && a.profile.Name != "demo" {
		lq, closeLive, err := newLiveQuotes(ctx, a, clk, rd)
		if err != nil {
			_ = ln.Close()
			return err
		}
		defer closeLive()
		rd.live = lq
	}
	a.log.Info("serve listening", "component", "ot", "status", "ok")
	return serveHandler(ctx, ln, siteHandler(a, clk, *dayFlag, rd, a.deps.newHandler, newAssets()))
}

// loopbackListen reports whether addr is "host:port" with a loopback IP literal as its host.
func loopbackListen(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
