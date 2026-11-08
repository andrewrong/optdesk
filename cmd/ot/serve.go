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
	if err := fs.Parse(args); err != nil {
		return err
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
	a.log.Info("serve listening", "component", "ot", "status", "ok")
	return serveHandler(ctx, ln, siteHandler(a, clk, *dayFlag, rd, a.deps.newHandler, newAssets()))
}
