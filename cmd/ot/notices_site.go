package main

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// siteNoticesRoute is the address of the page option_notices_page under the site root. The link to it in the
// bar belongs to web (docs/design/option-notices.md, Decision 10).
const siteNoticesRoute = "options/notices"

// noticesName is the name of the page option_notices_page, which pageNamed selects for ot page.
const noticesName = "option_notices_page"

// noticesReaders are the three stores of the page; a nil one gives the gap no_reader.
type noticesReaders struct {
	account store.AccountReader
	cands   store.CandidateReader
	earn    store.EarningsDateReader
}

// noticeAliases reads the main alias and the aliases to show as the account job reads them: the main label
// of the gateways file (accountMain) and the labels of OT_ACCOUNT_SOCKETS in sorted order. No account id leaves
// this function. The demo profile has its own two aliases.
func noticeAliases(a *app) (string, []string, error) {
	if a.profile.Name == "demo" {
		return demoNoticesMain, []string{demoNoticesMain, demoNoticesSecond}, nil
	}
	env, err := a.envFile()
	if err != nil {
		return "", nil, err
	}
	if env[envAccountSockets] == "" {
		return "", nil, fmt.Errorf("%s is not set", envAccountSockets)
	}
	sockets, err := parseAccountSockets(env[envAccountSockets])
	if err != nil {
		return "", nil, err
	}
	main, err := accountMain(env[envIbkrGateways])
	if err != nil {
		return "", nil, err
	}
	if _, ok := sockets[main]; !ok {
		return "", nil, fmt.Errorf("the main label of %s is not in %s", envIbkrGateways, envAccountSockets)
	}
	labels := make([]string, 0, len(sockets))
	for l := range sockets {
		labels = append(labels, l)
	}
	slices.Sort(labels)
	return main, labels, nil
}

func init() {
	registerPage(pageFamily{name: noticesName, route: siteNoticesRoute, order: 20, build: buildNotices, named: noticesPageNamed})
}

// buildNotices builds option_notices_page per request for the day of dayFlag or today, from the readers of
// the site (opened once per process); the demo profile serves a fixed result and opens nothing. A nil zone is
// an error.
func buildNotices(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New(noticesName + ": no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	p, err := newNoticesPage(a, "ot-serve", noticesReaders{account: rd.noticeAccount, cands: rd.cands, earn: rd.earnings})
	if err != nil {
		return nil, err
	}
	return p.Build(ctx, nil, day, clk.Now().UTC())
}

// newNoticesPage calls deps.optionNoticesPage with the plain arguments (scope "notices"); the demo profile
// gets the fixed demo rule.
func newNoticesPage(a *app, runID string, r noticesReaders) (pages.Page, error) {
	main, aliases, err := noticeAliases(a)
	if err != nil {
		return pages.Page{}, err
	}
	var rule compute.OptionNotices = demoNoticeRule{}
	params := compute.NoticeParams{}
	if a.profile.Name != "demo" {
		rule, params = a.deps.newOptionNotices(), a.deps.noticeParams()
	}
	return a.deps.optionNoticesPage(a.profile.Zone, runID, buildIDOf(version), "notices", main, aliases,
		r.account, r.cands, r.earn, rule, params), nil
}

// noticesPageNamed returns option_notices_page for ot page and a close function: the account, candidate and
// earnings stores opened once with the pages line of .env, none with a missing line (the page shows its gap
// no_reader), or the demo readers with the demo profile (no store). A nil zone is an error raised before any
// store is opened.
func noticesPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New(noticesName + " needs the owner's zone: " + zoneEnvName + " is not set")
	}
	if a.profile.Name == "demo" {
		p, err := newNoticesPage(a, "ot-page", noticesReaders{account: demoNoticeAccount{}, cands: demoCandidates{}})
		return p, func() {}, err
	}
	if _, _, err := noticeAliases(a); err != nil {
		return pages.Page{}, nil, err
	}
	env, err := a.envFile()
	if err != nil {
		return pages.Page{}, nil, err
	}
	dsn := env[pagesDSNName]
	if dsn == "" {
		p, err := newNoticesPage(a, "ot-page", noticesReaders{})
		return p, func() {}, err
	}
	var r noticesReaders
	var closers []func() error
	closeAll := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			_ = closers[i]()
		}
	}
	fail := func() (pages.Page, func(), error) {
		closeAll()
		return pages.Page{}, nil, fmt.Errorf("open stores with %s failed", pagesDSNName)
	}
	if a.deps.openAccount != nil {
		s, err := a.deps.openAccount(ctx, dsn)
		if err != nil {
			return fail()
		}
		closers, r.account = append(closers, s.Close), s
	}
	if a.deps.openCandidate != nil {
		s, err := a.deps.openCandidate(ctx, dsn)
		if err != nil {
			return fail()
		}
		closers, r.cands = append(closers, s.Close), s
	}
	if a.deps.openEarningsDates != nil {
		s, err := a.deps.openEarningsDates(ctx, dsn)
		if err != nil {
			return fail()
		}
		closers, r.earn = append(closers, s.Close), s
	}
	p, err := newNoticesPage(a, "ot-page", r)
	if err != nil {
		closeAll()
		return pages.Page{}, nil, err
	}
	return p, closeAll, nil
}

var _ compute.OptionNotices = demoNoticeRule{}
