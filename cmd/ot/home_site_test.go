package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute/optionledger"
	"github.com/andrewrong/optdesk/internal/compute/optionresult"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
)

func homeFixture(t *testing.T, demo bool) http.Handler {
	t.Helper()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	rd := siteReaders{}
	if demo {
		a.profile.Name = "demo"
		a.deps.newOptionLedger, a.deps.newOptionResults = optionledger.New, optionresult.New
		rd, _, _ = openSiteReaders(context.Background(), a)
	}
	return siteHandler(a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
}

func TestHomeRouteServesFiveMembers(t *testing.T) {
	t.Parallel()
	for name, demo := range map[string]bool{"no reader": false, "demo": true} {
		h := homeFixture(t, demo)
		rec := companiesGet(t, h, http.MethodGet, "/"+siteHomeRoute)
		if rec.Code != 200 {
			t.Fatalf("%s: %d %q", name, rec.Code, rec.Body.String())
		}
		var p map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var schema string
		_ = json.Unmarshal(p["schema"], &schema)
		if schema != "home_page" {
			t.Fatalf("%s: schema %q", name, schema)
		}
		for _, m := range []string{"household", "companies", "macro", "market", "options"} {
			if len(p[m]) == 0 {
				t.Errorf("%s: member %s missing", name, m)
			}
		}
		if string(p["gap_line"]) == `""` || len(p["gap_line"]) == 0 {
			t.Errorf("%s: no gap_line", name)
		}
	}
}

func TestHomeRouteNoReaderMembersAreGaps(t *testing.T) {
	t.Parallel()
	rec := companiesGet(t, homeFixture(t, false), http.MethodGet, "/home")
	var p struct {
		Overview struct {
			Gaps json.RawMessage `json:"gaps"`
		} `json:"overview"`
		GapLine string `json:"gap_line"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(p.Overview.Gaps), "[") {
		t.Fatalf("overview.gaps is not a JSON array: %q", p.Overview.Gaps)
	}
	if p.GapLine == "" || p.GapLine == "No gaps today" {
		t.Fatalf("gap_line %q: five members without a reader are gaps", p.GapLine)
	}
}

func TestRootRedirectsToHome(t *testing.T) {
	t.Parallel()
	rec := companiesGet(t, homeFixture(t, false), http.MethodGet, "/")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/home" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestHomePageNamed(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	p, closeFn, err := homePageNamed(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	if p.Meta.Name != homeName || p.Meta.Schema != "page.home_page.v3" {
		t.Fatalf("meta %+v", p.Meta)
	}
}

func TestHomeCompaniesRowsFromStore(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	rd := siteReaders{underlying: companiesStore(), account: holdsAAA{}, aliases: []string{"main"}}
	body, err := buildHome(t.Context(), a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"AAA"`) || strings.Contains(string(body), `"BBB"`) {
		t.Fatalf("only the held name belongs in the card: %s", body)
	}
	if !strings.Contains(string(body), "Held companies: 1") {
		t.Fatalf("say is not the held count: %s", body)
	}
}

func TestHomeCompaniesNoAccountNoRows(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	rd := siteReaders{underlying: companiesStore()}
	body, err := buildHome(t.Context(), a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"AAA"`) || !strings.Contains(string(body), "Held companies: 0") {
		t.Fatalf("no account reader must give no rows: %s", body)
	}
}

type fakeEarningsDates struct{ rows []contracts.EarningsDate }

func (f fakeEarningsDates) EarningsDates(context.Context, string, contracts.Day, contracts.Day) ([]contracts.EarningsDate, error) {
	return f.rows, nil
}

func homeEarningsCell(t *testing.T, rd siteReaders) (kind, reason, text string) {
	t.Helper()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	rd.underlying, rd.account, rd.aliases = companiesStore(), holdsAAA{}, []string{"main"}
	body, err := buildHome(t.Context(), a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Companies struct {
			Card struct {
				Rows []struct {
					Earnings struct {
						Kind   string `json:"kind"`
						Reason string `json:"reason"`
						Text   string `json:"text"`
					} `json:"earnings"`
				} `json:"rows"`
			} `json:"card"`
		} `json:"companies"`
	}
	if err := json.Unmarshal(body, &p); err != nil || len(p.Companies.Card.Rows) != 1 {
		t.Fatalf("rows %d, err %v: %s", len(p.Companies.Card.Rows), err, body)
	}
	e := p.Companies.Card.Rows[0].Earnings
	return e.Kind, e.Reason, e.Text
}

func TestHomeCompaniesEarningsFromStore(t *testing.T) {
	t.Parallel()
	report := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	rd := siteReaders{earnings: fakeEarningsDates{rows: []contracts.EarningsDate{{
		Status: gen.EarningsDateStatus("listed"), ReportDate: &types.SerializableDate{Time: report},
	}}}}
	kind, reason, text := homeEarningsCell(t, rd)
	if kind != "value" || text != "11-20" || reason == "no_reader" {
		t.Fatalf("earnings cell must carry the stored date and no no_reader gap: %q %q %q", kind, reason, text)
	}
}

func TestHomeCompaniesNoEarningsReaderIsGap(t *testing.T) {
	t.Parallel()
	kind, reason, _ := homeEarningsCell(t, siteReaders{})
	if kind != "gap" || reason != "no_reader" {
		t.Fatalf("a site without an earnings reader keeps the no_reader gap: %q %q", kind, reason)
	}
}
