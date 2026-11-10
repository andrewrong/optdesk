package main

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/pages"
)

// Design: docs/design/ot-wire-split.md#Decisions 8.

// pageFamily is one page of ot as the registry holds it: its site route, its name for ot page and how each is built.
type pageFamily struct {
	// name is the --name of ot page; empty means the page is a route only (settings_page stays unknown to ot page).
	name string
	// route is the path under the site root, unique.
	route string
	// build builds the page bytes of one request.
	build func(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error)
	// buildKey, when set, makes route a prefix: options/stock (key "") and options/stock/<key> both call it, with
	// build nil. A key outside pageKeyPattern or with a further slash is a 404.
	buildKey func(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders, key string) ([]byte, error)
	// named builds the page for ot page --name; nil with an empty name.
	named func(ctx context.Context, a *app) (pages.Page, func(), error)
	// order places the name in the known-pages text (explicit, since init order follows file names); 0 when unnamed.
	order int
}

var pageFamilies = map[string]pageFamily{}

// registerPage adds f; it panics on a clash of a non-empty name, of the route or of a non-zero order, at start.
func registerPage(f pageFamily) { addPage(pageFamilies, f) }

// addPage is registerPage over the table m, so that a test can run it on a copy.
func addPage(m map[string]pageFamily, f pageFamily) {
	if f.route == "" || (f.build == nil) == (f.buildKey == nil) {
		panic("registerPage: a page needs a route and exactly one of build and buildKey")
	}
	for _, g := range m {
		if g.route == f.route {
			panic(fmt.Sprintf("registerPage: duplicate route %q", f.route))
		}
		if f.name != "" && g.name == f.name {
			panic(fmt.Sprintf("registerPage: duplicate page name %q", f.name))
		}
		if f.order != 0 && g.order == f.order {
			panic(fmt.Sprintf("registerPage: pages %q and %q share order %d", f.route, g.route, f.order))
		}
	}
	m[f.route] = f
}

func pageFamilyOfRoute(route string) (pageFamily, bool) {
	f, ok := pageFamilies[route]
	return f, ok
}

// pageKeyPattern is a key of a keyed family.
var pageKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// pageFamilyOfKeyRoute finds the keyed family whose route is path or its prefix before one slash; key is "" for
// the bare route. ok is false for a path with a bad key or a further slash.
func pageFamilyOfKeyRoute(path string) (f pageFamily, key string, ok bool) {
	for _, g := range pageFamilies {
		if g.buildKey == nil {
			continue
		}
		if path == g.route {
			return g, "", true
		}
		if rest, found := strings.CutPrefix(path, g.route+"/"); found && pageKeyPattern.MatchString(rest) {
			return g, rest, true
		}
	}
	return pageFamily{}, "", false
}

// pageFamilyOfName finds the family with a non-empty name.
func pageFamilyOfName(name string) (pageFamily, bool) {
	if name == "" {
		return pageFamily{}, false
	}
	for _, f := range pageFamilies {
		if f.name == name {
			return f, true
		}
	}
	return pageFamily{}, false
}

// registeredPageNames is the non-empty names, by order.
func registeredPageNames() []string {
	var fs []pageFamily
	for _, f := range pageFamilies {
		if f.name != "" {
			fs = append(fs, f)
		}
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].order < fs[j].order })
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.name
	}
	return out
}

type queryKey struct{}

// withQuery carries the query string of the request to a family's build, which has no query parameter.
func withQuery(ctx context.Context, q url.Values) context.Context {
	return context.WithValue(ctx, queryKey{}, q)
}

// queryOf is the query string withQuery put in ctx, empty when none.
func queryOf(ctx context.Context) url.Values {
	q, _ := ctx.Value(queryKey{}).(url.Values)
	return q
}
