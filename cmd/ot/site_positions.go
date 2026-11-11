package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strings"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/pages/option_result_page"
	"github.com/andrewrong/optdesk/internal/pages/structures_page"
)

// The addresses of the option positions pages under the site root (docs/design/option-positions.md#Interface).
const (
	siteStructuresRoute      = "options/structures"
	siteStructureRoutePrefix = "options/structures/"
	siteResultRoute          = "options/result"
)

const (
	siteAccountQuery = "account" // the query key naming the alias of a page
	unknownAccount   = "未知账户"
	demoNoFlexText   = "演示数据中没有 Flex 持仓"
)

// aliasList is the alias list of the account pages: the field aliases, or ["main"] when it is empty. The
// first is the default alias. It never returns an empty list.
func (rd siteReaders) aliasList() []string {
	if len(rd.aliases) == 0 {
		return []string{"main"}
	}
	return rd.aliases
}

// siteAliasList reads the alias list of the account pages from the gateways file named in .env, the way
// account_capture takes its aliases: main first (accountAliases), then the other `account` values, non-empty,
// without repeats, sorted. The demo profile, an unreadable .env, an unset key and a missing or invalid file
// give ["main"]; the failure logs one WARN line carrying no path and no account value. It is called once at
// serve start.
func siteAliasList(a *app) []string {
	fallback := []string{"main"}
	if a.profile.Name == "demo" {
		return fallback
	}
	warn := func(why string) []string {
		a.log.Warn("gateways file not usable for the account pages, alias main is used", "component", "ot", "reason", why)
		return fallback
	}
	env, err := a.envFile()
	if err != nil {
		return warn("env file cannot be read")
	}
	list, err := accountAliases(env[envIbkrGateways])
	if err != nil {
		return warn("gateways file missing or invalid")
	}
	return list
}

// positionsKind selects which of the three pages a request builds.
type positionsKind int

const (
	kindList positionsKind = iota
	kindDetail
	kindResult
)

// isPositionsPath tells whether path (without its leading slash) is a route of the option positions pages;
// the detail route needs a non-empty id, so "options/structures/" is not one.
func isPositionsPath(path string) bool {
	if path == siteStructuresRoute || path == siteResultRoute {
		return true
	}
	return strings.HasPrefix(path, siteStructureRoutePrefix) && len(path) > len(siteStructureRoutePrefix)
}

// positionsRoute is the page kind of a path accepted by isPositionsPath and, for the detail, the structure id
// (the server has already unescaped the path).
func positionsRoute(path string) (positionsKind, string) {
	switch path {
	case siteResultRoute:
		return kindResult, ""
	case siteStructuresRoute:
		return kindList, ""
	}
	return kindDetail, strings.TrimPrefix(path, siteStructureRoutePrefix)
}

// buildPositions builds one of the three option positions pages per request for the day of dayFlag or today
// and the alias of query ("account", main when absent). An alias not in the list, and the demo profile, give
// the page with its gap text instead of figures. A nil zone is an error.
func buildPositions(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders, kind positionsKind, id string, q url.Values) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New("option positions page: no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	alias := q.Get(siteAccountQuery)
	if alias == "" {
		alias = rd.aliasList()[0]
	}
	gapText := ""
	switch {
	case !slices.Contains(rd.aliasList(), alias):
		gapText = unknownAccount
	case a.profile.Name == "demo":
		gapText = demoNoFlexText
	}
	aliases := rd.aliasList()
	if gapText != "" {
		// the builders see no reader: they write their no_reader gap page, whose text the route replaces
		rd = siteReaders{}
		aliases = []string{alias}
	}
	buildID := buildIDOf(version)
	st := structures_page.Config{RunID: "ot-serve", BuildID: buildID, Zone: a.profile.Zone, Aliases: aliases,
		Ledger: rd.ledger, Account: rd.account, Eod: rd.eod, Sessions: rd.sessions, Terms: rd.terms,
		Ledgers: rd.ledgers, Structures: rd.structures}
	page := a.deps.structuresListPage(st, alias)
	switch kind {
	case kindList:
	case kindDetail:
		page = a.deps.structureDetailPage(st, alias, id)
	case kindResult:
		page = a.deps.optionResultPage(option_result_page.Config{RunID: "ot-serve", BuildID: buildID,
			Zone: a.profile.Zone, Aliases: aliases, Ledger: rd.ledger, Sessions: rd.sessions,
			Ledgers: rd.ledgers, Results: rd.results}, alias)
	}
	vm, err := page.Build(ctx, nil, day, clk.Now().UTC())
	if err != nil || gapText == "" {
		return vm, err
	}
	return withGapText(vm, gapText)
}

// withGapText writes text into the fields of a gap page view model that carry its sentence or note.
func withGapText(vm []byte, text string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(vm, &m); err != nil {
		return nil, err
	}
	t, err := json.Marshal(text)
	if err != nil {
		return nil, err
	}
	for _, k := range []string{"sentence", "note", "nothing"} {
		if v, ok := m[k]; ok && string(v) != "null" {
			m[k] = t
		}
	}
	return json.Marshal(m)
}
