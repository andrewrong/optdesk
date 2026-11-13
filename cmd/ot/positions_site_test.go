package main

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

// positionsAccount notes the alias of each positions read too (householdAccount notes values only).
type positionsAccount struct{ householdAccount }

func (p positionsAccount) Positions(_ context.Context, alias, _ string) ([]store.Position, error) {
	p.mu.Lock()
	*p.asked = append(*p.asked, alias)
	p.mu.Unlock()
	return []store.Position{}, nil
}

// positionsMarks is a mark reader with no marks.
type positionsMarks struct{ mark.Reader }

func (positionsMarks) Marks(context.Context, string, string) ([]mark.PositionMark, error) {
	return []mark.PositionMark{}, nil
}

// positionsSiteReaders are the household readers plus a mark reader, the readers positions_page needs.
func positionsSiteReaders(asked *[]string) siteReaders {
	rd := householdReaders(asked)
	rd.noticeAccount = positionsAccount{householdAccount: householdAccount{mu: &sync.Mutex{}, asked: asked}}
	rd.marks = positionsMarks{}
	return rd
}

func TestPositionsRouteServesStoredAccountPage(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	var asked []string
	h := siteHandler(a, householdClock, "2026-11-05", positionsSiteReaders(&asked), echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+sitePositionsRoute)
	body := rec.Body.String()
	if rec.Code != 200 || strings.Contains(body, `"no_reader"`) || !strings.Contains(body, "positions_page") {
		t.Fatalf("%d %q", rec.Code, body)
	}
	if len(asked) == 0 || asked[0] != "main" {
		t.Fatalf("default account: asked %v", asked)
	}
	asked = nil
	rec = companiesGet(t, h, http.MethodGet, "/"+sitePositionsRoute+"?"+siteAccountQuery+"=stranger")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"no_data"`) || len(asked) != 0 {
		t.Fatalf("unknown account: %d asked %v %q", rec.Code, asked, rec.Body)
	}
}

func TestPositionsRouteWithoutReaderIsGap(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	h := siteHandler(a, householdClock, "2026-11-05", siteReaders{}, echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+sitePositionsRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"no_reader"`) {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
}

func TestPositionsPageNamedDemoIsGap(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	a.profile.Name = "demo"
	page, closeFn, err := pageNamed(a, positionsName)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	got, err := page.Build(t.Context(), nil, contracts.Day("2026-11-05"), householdClock.Now().UTC())
	if err != nil || !strings.Contains(string(got), `"no_reader"`) {
		t.Fatalf("%v %s", err, got)
	}
}

func TestPositionsPageConfigGetsStatementAliases(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	writeBookText(t, a, bookText("a", "a", "b")+"statement = [\"b\"]\n")
	if got := positionsPageConfig(a, siteReaders{}).Statement; len(got) != 1 || got[0] != "b" {
		t.Fatalf("statement aliases %v, want [b]", got)
	}
	writeBookText(t, a, bookText("a", "a", "b"))
	if got := positionsPageConfig(a, siteReaders{}).Statement; got != nil {
		t.Fatalf("plain book gave %v", got)
	}
}
