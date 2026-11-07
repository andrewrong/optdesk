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

	"github.com/andrewrong/optdesk/internal/pages"
)

func init() { register("serve", runServe) }

const shutdownGrace = 5 * time.Second

// serveOn serves the page on ln until ctx is done. The view model is built anew for each
// request; a build error answers 500 with a fixed text, so no error text reaches the client.
func serveOn(ctx context.Context, ln net.Listener, build func(context.Context) ([]byte, error), newHandler pages.NewHandlerFunc) error {
	return serveHandler(ctx, ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vm, err := build(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		newHandler(vm).ServeHTTP(w, r)
	}))
}

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

func runServe(a *app, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dayFlag := fs.String("day", "", "day YYYY-MM-DD (default: today in "+marketZone+")")
	name := fs.String("name", "macro_rates", "page name")
	addr := fs.String("addr", "127.0.0.1:8080", "listen address")
	site := fs.Bool("site", false, "serve the option data page per underlying")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *site {
		named := false
		fs.Visit(func(f *flag.Flag) { named = named || f.Name == "name" })
		if named {
			return errors.New("serve: --site and --name cannot be used together")
		}
		return runSite(a, *dayFlag, *addr)
	}
	page, err := pageNamed(a, *name)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	clk := a.deps.newClock()
	if _, err := dayOrDefault(clk, *dayFlag); err != nil {
		return err
	}
	r, closeFn, err := seriesReader(ctx, a)
	if err != nil {
		return err
	}
	defer closeFn()
	build := func(ctx context.Context) ([]byte, error) {
		day, err := dayOrDefault(clk, *dayFlag)
		if err != nil {
			return nil, err
		}
		b, err := page.Build(ctx, r, day, clk.Now().UTC())
		if err != nil {
			a.log.Error("serve build failed", "component", "ot", "status", "failed")
		}
		return b, err
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", *addr)
	if err != nil {
		return fmt.Errorf("serve: cannot listen on %s", *addr)
	}
	a.log.Info("serve listening", "component", "ot", "status", "ok")
	return serveOn(ctx, ln, build, a.deps.newHandler)
}

// runSite serves the option data page: the four stores are opened once and closed on exit.
func runSite(a *app, dayFlag, addr string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	clk := a.deps.newClock()
	if _, err := dayOrDefault(clk, dayFlag); err != nil {
		return err
	}
	rd, closeFn, err := openSiteReaders(ctx, a)
	if err != nil {
		return err
	}
	defer closeFn()
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("serve: cannot listen on %s", addr)
	}
	a.log.Info("serve listening", "component", "ot", "status", "ok")
	return serveHandler(ctx, ln, siteHandler(a, clk, dayFlag, rd, a.deps.newHandler))
}
