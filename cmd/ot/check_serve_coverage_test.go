package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

// otherRoute is the reason of a page that has its own route or handler in siteHandler and no block table in
// check_blocks.json, so `ot check serve` does not call it.
const otherRoute = "served by its own handler of siteHandler, outside the check serve table: no block table in check_blocks.json"

// serveRouteAllow holds the pages a web view registers that serveRoutes does not list, each with its reason.
var serveRouteAllow = map[string]string{
	"advice_page":           otherRoute,
	"base_data_page":        otherRoute,
	"companies_page":        otherRoute,
	"company_data_page":     otherRoute,
	"company_quarters_page": otherRoute,
	"data_health_page":      otherRoute,
	"home_page":             otherRoute,
	"household_page":        otherRoute,
	"market_state_page":     otherRoute,
	"ops_today_page":        otherRoute,
	"option_result_page":    otherRoute,
	"positions_page":        otherRoute,
	"price_lines_page":      otherRoute,
	"strategy_replay_page":  otherRoute,
	"valuation_import_page": otherRoute,
	"valuation_page":        otherRoute,
	"valuation_params_page": otherRoute,
	"macro_rates":           "kept as a package, not routed: nothing links to it, /macro serves macro_page",
}

var webSchemaName = regexp.MustCompile(`schema:\s*'([a-z0-9_]+)'`)

// TestEveryServedPageHasARoute fails when a page that a web view registers and that has a producer package
// under internal/pages has no row in serveRoutes.
func TestEveryServedPageHasARoute(t *testing.T) {
	t.Parallel()
	names := map[string]bool{}
	err := filepath.WalkDir("../../web/src/pages", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p) //nolint:gosec // a path under the repository's web/ tree
		if err != nil {
			return err
		}
		for _, m := range webSchemaName.FindAllSubmatch(b, -1) {
			names[string(m[1])] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	routed := map[string]bool{}
	for _, r := range serveRoutes {
		routed[r.page] = true
	}
	var list []string
	for n := range names {
		list = append(list, n)
	}
	sort.Strings(list)
	for _, n := range list {
		if st, err := os.Stat(filepath.Join("../../internal/pages", n)); err != nil || !st.IsDir() {
			continue
		}
		if _, ok := serveRouteAllow[n]; ok || routed[n] {
			continue
		}
		t.Errorf("page %s is registered by a web view and has no row in serveRoutes", n)
	}
}
