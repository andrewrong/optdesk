package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

func init() { register("page", runPage) }

// seriesReader returns the reader a page is built from and a close function: the demo points
// with the demo profile, else the series store opened with the .env line DSNEnvPages.
func seriesReader(ctx context.Context, a *app) (store.SeriesReader, func(), error) {
	if a.profile.Name == "demo" {
		return demoReader{}, func() {}, nil
	}
	env, err := a.envFile()
	if err != nil {
		return nil, nil, err
	}
	dsn := env[store.DSNEnvPages]
	if dsn == "" {
		return nil, nil, fmt.Errorf("%s is missing in .env", store.DSNEnvPages)
	}
	s, err := a.deps.openSeries(ctx, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open series store with %s failed", store.DSNEnvPages)
	}
	return s, func() { _ = s.Close() }, nil
}

// findPage returns the entry of list whose Meta.Name is name; an unknown name is an error
// that lists every known name in list order.
func findPage(list []pages.Page, name string) (pages.Page, error) {
	known := make([]string, 0, len(list))
	for _, p := range list {
		if p.Meta.Name == name {
			return p, nil
		}
		known = append(known, p.Meta.Name)
	}
	return pages.Page{}, fmt.Errorf("unknown page %q; known pages: %s", name, strings.Join(known, ", "))
}

func runPage(a *app, args []string) error {
	fs := flag.NewFlagSet("page", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dayFlag := fs.String("day", "", "day YYYY-MM-DD (default: today in "+marketZone+")")
	name := fs.String("name", "macro_rates", "page name")
	out := fs.String("out", "", "file to write (default: standard output)")
	market := fs.String("market", "", "market us or jp (report_page only)")
	slot := fs.String("slot", "", "slot open or close (report_page only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx := context.Background()
	var page pages.Page
	var closePage func()
	var err error
	withSeries := true
	switch *name {
	case reportName:
		page, closePage, err = reportPage(ctx, a, *market, *slot)
		withSeries = false
	case marketName:
		page, closePage, err = marketPage(ctx, a)
	default:
		page, closePage, err = pageNamed(a, *name)
	}
	if err != nil {
		return err
	}
	defer closePage()
	open := func() (store.SeriesReader, func(), error) { return nil, func() {}, nil }
	if withSeries {
		open = func() (store.SeriesReader, func(), error) { return seriesReader(ctx, a) }
	}
	clk := a.deps.newClock()
	day, err := dayOrDefault(clk, *dayFlag)
	if err != nil {
		return err
	}
	r, closeFn, err := open()
	if err != nil {
		return err
	}
	defer closeFn()
	b, err := page.Build(ctx, r, day, clk.Now().UTC())
	if err != nil {
		return fmt.Errorf("page %s: %w", day, err)
	}
	if *out == "" {
		_, err = a.stdout.Write(b)
		return err
	}
	return os.WriteFile(*out, b, 0o644)
}
