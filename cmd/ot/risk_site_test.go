package main

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionstruct"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// riskAccount holds one USD stock for alias "main" in the capture of 2026-11-05 and notes the aliases asked for.
type riskAccount struct {
	store.AccountReader
	mu    *sync.Mutex
	asked *[]string
}

func (r riskAccount) Positions(_ context.Context, alias, day string) ([]store.Position, error) {
	r.mu.Lock()
	*r.asked = append(*r.asked, alias)
	r.mu.Unlock()
	if alias != "main" || day != "2026-11-05" {
		return []store.Position{}, nil
	}
	return []store.Position{{Symbol: "AAA", SecType: "STK", Currency: "USD", ConID: 1, Quantity: "10",
		CapturedAt: time.Date(2026, 11, 5, 21, 0, 0, 0, time.UTC)}}, nil
}

type riskEod struct{}

func (riskEod) OptionEod(context.Context, int64, contracts.Day, contracts.Day) ([]contracts.OptionEod, error) {
	return nil, nil
}

type riskTerms struct{}

func (riskTerms) OptionTerms(context.Context, []int64) ([]store.OptionTerms, error) { return nil, nil }

func riskReaders(asked *[]string) siteReaders {
	rd := householdReaders(asked)
	rd.noticeAccount = riskAccount{mu: &sync.Mutex{}, asked: asked}
	rd.structures = optionstruct.New
	rd.eod, rd.terms = riskEod{}, riskTerms{}
	return rd
}

func TestRiskRouteServesFromReaders(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	var asked []string
	h := siteHandler(a, householdClock, "2026-11-05", riskReaders(&asked), echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteRiskRoute)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, riskName) || strings.Contains(body, `"no_reader"`) ||
		strings.Contains(body, `"no_data"`) || !strings.Contains(body, `"run_status":"partial"`) {
		t.Fatalf("%d %q", rec.Code, body)
	}
	if len(asked) == 0 || asked[0] != "main" {
		t.Fatalf("default account: asked %v", asked)
	}
	asked = nil
	rec = companiesGet(t, h, http.MethodGet, "/"+siteRiskRoute+"?"+siteAccountQuery+"=main")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"run_status":"partial"`) || len(asked) == 0 || asked[0] != "main" {
		t.Fatalf("account main: %d asked %v", rec.Code, asked)
	}
	asked = nil
	rec = companiesGet(t, h, http.MethodGet, "/"+siteRiskRoute+"?"+siteAccountQuery+"=stranger")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"no_data"`) || len(asked) != 0 {
		t.Fatalf("unknown account: %d asked %v %q", rec.Code, asked, rec.Body)
	}
}

func TestRiskRouteWithoutReaderIsGap(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	h := siteHandler(a, householdClock, "2026-11-05", siteReaders{}, echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteRiskRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"no_reader"`) {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
}

func TestRiskPageNamedDemoIsGap(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	a.profile.Name = "demo"
	page, closeFn, err := pageNamed(a, riskName)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	got, err := page.Build(t.Context(), nil, contracts.Day("2026-11-05"), householdClock.Now().UTC())
	if err != nil || !strings.Contains(string(got), `"no_reader"`) {
		t.Fatalf("%v %s", err, got)
	}
}
