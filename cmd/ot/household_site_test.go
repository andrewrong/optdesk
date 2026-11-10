package main

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionledger"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/atombender/go-jsonschema/pkg/types"
)

var householdClock = fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}

// householdAccount is an account reader holding net liquidation for alias "main" on 2026-11-05, and noting
// the aliases asked for.
type householdAccount struct {
	store.AccountReader
	mu    *sync.Mutex
	asked *[]string
}

func (h householdAccount) Values(_ context.Context, alias, day string) ([]store.AccountValue, error) {
	h.mu.Lock()
	*h.asked = append(*h.asked, alias)
	h.mu.Unlock()
	if alias != "main" || day != "2026-11-05" {
		return []store.AccountValue{}, nil
	}
	return []store.AccountValue{{Alias: alias, SessionDay: day, Tag: "NetLiquidation", Value: "100000", Currency: "USD"}}, nil
}

func (householdAccount) Positions(context.Context, string, string) ([]store.Position, error) {
	return []store.Position{}, nil
}

func (householdAccount) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return []store.Execution{}, nil
}

type householdLedger struct{ store.OptionLedgerReader }

func (householdLedger) FlexTradeRows(context.Context, string) ([]store.FlexTradeRow, error) {
	return []store.FlexTradeRow{}, nil
}

type householdSessions struct{}

func (householdSessions) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	day := types.SerializableDate{Time: time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)}
	return []contracts.SessionDay{{CalendarID: "XNYS", SessionDay: day, IsSession: true}}, nil
}

func householdReaders(asked *[]string) siteReaders {
	return siteReaders{
		noticeAccount: householdAccount{mu: &sync.Mutex{}, asked: asked},
		optionLedger:  householdLedger{},
		sessions:      householdSessions{},
		ledgers:       optionledger.New,
	}
}

func TestHouseholdRouteServesStoredAccount(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	var asked []string
	h := siteHandler(a, householdClock, "2026-11-05", householdReaders(&asked), echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteHouseholdRoute)
	body := rec.Body.String()
	if rec.Code != 200 || strings.Contains(body, `"no_reader"`) || strings.Contains(body, `"no_data"`) || !strings.Contains(body, "household_page") {
		t.Fatalf("%d %q", rec.Code, body)
	}
	if len(asked) == 0 || asked[0] != "main" {
		t.Fatalf("default account: asked %v", asked)
	}
	asked = nil
	rec = companiesGet(t, h, http.MethodGet, "/"+siteHouseholdRoute+"?"+siteAccountQuery+"=stranger")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"no_data"`) || len(asked) != 0 {
		t.Fatalf("unknown account: %d asked %v %q", rec.Code, asked, rec.Body)
	}
}

func TestHouseholdRouteWithoutReaderIsGap(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	h := siteHandler(a, householdClock, "2026-11-05", siteReaders{}, echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteHouseholdRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"no_reader"`) {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
}

func TestHouseholdPageNamedDemoIsGap(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	a.profile.Name = "demo"
	page, closeFn, err := pageNamed(a, householdName)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	got, err := page.Build(t.Context(), nil, contracts.Day("2026-11-05"), householdClock.Now().UTC())
	if err != nil || !strings.Contains(string(got), `"no_reader"`) {
		t.Fatalf("%v %s", err, got)
	}
}
