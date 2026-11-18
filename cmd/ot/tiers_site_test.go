package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

type tierAccount struct{ store.AccountReader }

type tierOwner struct{ ownerset.Reader }

func TestTiererIsRealRule(t *testing.T) {
	t.Parallel()
	tr := newTierer()
	if tr == nil {
		t.Fatal("nil tierer")
	}
	res, err := tr.Judge(optiontierapi.Input{
		Day:    "2026-11-17",
		Params: optiontierapi.Params{DeltaMax: "0.3", ThinDelta: "0.15", MaxCashAgeDays: 3, DaysInYear: 365},
		Rows: []optiontierapi.Row{{
			Key: "1", Company: "AAA", ChainDay: "2026-11-17", Expiry: "2026-12-18", DTE: 31,
			Spot: "96.50", Strike: "90.00", Bid: "0.94", BuyPrice: "90.00", IV: "0.312", Delta: "-0.2", Multiplier: "100",
		}},
	})
	if err != nil || len(res.Verdicts) != 1 || res.Verdicts[0].Tier != "fits" {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestCandidatesWiresTiers(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, "")
	f.a.profile.Name = "real"
	acc, own := tierAccount{}, tierOwner{}
	rd := siteReaders{cands: demoCandidates{}, account: acc, owner: own, aliases: []string{"main", "ira"}}
	h := siteHandler(f.a, candidatesClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/"+siteCandidatesRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"schema":"option_candidates_page"`) {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	g := f.gotAll
	if g.Tiers == nil || g.Account != acc || g.Owner != own || g.RuleParams == nil ||
		len(g.Aliases) != 2 || g.Aliases[1] != "ira" {
		t.Fatalf("readers %+v", g)
	}
}

func TestCandidatesNamedNoTiers(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, store.DSNEnvPages+"=pages-dsn\n")
	f.a.profile.Name = "real"
	cc := closeCounter{new(atomic.Int32)}
	f.a.deps.openCandidate = func(context.Context, string) (candidateStore, error) {
		return cCandsReader{demoCandidates: demoCandidates{}, closeCounter: cc}, nil
	}
	f.a.deps.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) {
		return cNames{closeCounter: cc}, nil
	}
	f.a.deps.openAccount = func(context.Context, string) (store.AccountStore, error) {
		t.Error("account store opened")
		return nil, errors.New("opened")
	}
	p, closeFn, err := candidatesPageNamed(t.Context(), f.a)
	if err != nil || p.Meta.Name != candidatesName {
		t.Fatalf("%v %+v", err, p.Meta)
	}
	closeFn()
	g := f.gotAll
	if g.Tiers != nil || g.Account != nil || g.Aliases != nil || g.Owner != nil || g.RuleParams != nil {
		t.Fatalf("readers %+v", g)
	}
}

func TestDemoProfileBuildsNoTiers(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, "")
	f.a.profile.Name = "demo"
	rd := siteReaders{cands: demoCandidates{}, account: tierAccount{}, owner: tierOwner{}}
	if _, err := buildCandidates(t.Context(), f.a, candidatesClock, "2026-11-05", rd); err != nil {
		t.Fatal(err)
	}
	g := f.gotAll
	if g.Tiers != nil || g.Account != nil || g.Owner != nil || g.RuleParams != nil || g.Aliases != nil {
		t.Fatalf("readers %+v", g)
	}
	if _, closeFn, err := candidatesPageNamed(t.Context(), f.a); err != nil {
		t.Fatal(err)
	} else {
		closeFn()
	}
	if g := f.gotAll; g.Tiers != nil || g.Account != nil {
		t.Fatalf("named readers %+v", g)
	}
}
