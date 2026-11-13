package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionledger"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
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

func TestHouseholdRouteReadsAliasesOfGatewaysFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "gateways.toml")
	body := "[[gateway]]\naccount = \"acct-a\"\nrequired = true\n\n[[gateway]]\naccount = \"acct-b\"\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	a, _ := testApp(t, envIbkrGateways+"="+path+"\n")
	var asked []string
	rd := householdReaders(&asked)
	rd.aliases = siteAliasList(a)
	h := siteHandler(a, householdClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteHouseholdRoute)
	if rec.Code != 200 || len(asked) == 0 {
		t.Fatalf("%d %v %q", rec.Code, asked, rec.Body)
	}
	for _, al := range asked {
		if al != "acct-a" {
			t.Fatalf("asked %v", asked)
		}
	}
	asked = nil
	rec = companiesGet(t, h, http.MethodGet, "/"+siteHouseholdRoute+"?"+siteAccountQuery+"=main")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"no_data"`) || len(asked) != 0 {
		t.Fatalf("main: %d asked %v %q", rec.Code, asked, rec.Body)
	}
}

func TestHouseholdRouteServesStoredAccount(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	var asked []string
	h := siteHandler(a, householdClock, "2026-11-05", householdReaders(&asked), echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteHouseholdRoute)
	body := rec.Body.String()
	if rec.Code != 200 || strings.Contains(body, `"no_reader"`) || !strings.Contains(body, "household_page") {
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

// householdMarkStore is a mark store that notes its close.
type householdMarkStore struct {
	mark.Store
	closed *bool
}

func (f householdMarkStore) Close() error { *f.closed = true; return nil }

func TestOpenMarkReaderUsesPagesLineAndTracksClose(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	var gotDSN string
	closed := false
	a.deps.openMark = func(_ context.Context, dsn string) (mark.Store, error) {
		gotDSN = dsn
		return householdMarkStore{closed: &closed}, nil
	}
	var tracked []io.Closer
	track := func(c io.Closer, err error) bool {
		if err == nil {
			tracked = append(tracked, c)
		}
		return err == nil
	}
	got := openMarkReader(t.Context(), a, "pages-dsn", track)
	if got == nil || gotDSN != "pages-dsn" || len(tracked) != 1 {
		t.Fatalf("reader %v dsn %q tracked %d", got, gotDSN, len(tracked))
	}
	_ = tracked[0].Close()
	if !closed {
		t.Fatal("store not closed through track")
	}
	cfg := householdConfig(a, siteReaders{marks: got})
	if cfg.Marks != got || cfg.Holdings == nil {
		t.Fatalf("config marks %v holdings %v", cfg.Marks, cfg.Holdings != nil)
	}
}

func TestOpenMarkReaderFailedOrMissingOpenerIsNil(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	track := func(_ io.Closer, err error) bool { return err == nil }
	a.deps.openMark = nil
	if got := openMarkReader(t.Context(), a, "d", track); got != nil {
		t.Fatalf("nil opener gave %v", got)
	}
	a.deps.openMark = func(context.Context, string) (mark.Store, error) { return nil, errors.New("down") }
	if got := openMarkReader(t.Context(), a, "d", track); got != nil {
		t.Fatalf("failed open gave %v", got)
	}
	if cfg := householdConfig(a, siteReaders{}); cfg.Marks != nil {
		t.Fatal("Marks must stay nil")
	}
}
