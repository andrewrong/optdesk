package main

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
)

func TestBuildIDOf(t *testing.T) {
	t.Parallel()
	ok := regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	for _, c := range []struct{ in, want string }{
		{"dev", "dev"},
		{"", "dev"},
		{"v1.2.3+abc", "v1-2-3-abc"},
		{strings.Repeat("a", 100), strings.Repeat("a", 64)},
	} {
		got := buildIDOf(c.in)
		if got != c.want || !ok.MatchString(got) {
			t.Errorf("buildIDOf(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := buildIDOf(strings.Repeat("é", 100)); len(got) != 64 || !ok.MatchString(got) {
		t.Errorf("multibyte input gave %q", got)
	}
}

func TestPageNamedUnknown(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.deps.pageList = []pages.Page{fakePage("alpha", "a"), fakePage("beta", "b")}
	_, err := pageNamed(a, "nosuch")
	want := `unknown page "nosuch"; known pages: alpha, beta, macro_page, report_page`
	if err == nil || err.Error() != want {
		t.Fatalf("err %v, want %s", err, want)
	}
	p, err := pageNamed(a, "beta")
	if err != nil || p.Meta.Name != "beta" {
		t.Fatalf("beta: %q %v", p.Meta.Name, err)
	}
}

func TestPageNamedNilZone(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Zone = nil
	a.deps.macroPage = func(*time.Location, string) pages.Page {
		t.Error("macroPage called with a nil zone")
		return pages.Page{}
	}
	_, err := pageNamed(a, "macro_page")
	if err == nil || !strings.Contains(err.Error(), "OT_ZONE") {
		t.Fatalf("err %v", err)
	}
}

type onePoint struct{}

func (onePoint) SeriesPoints(_ context.Context, id string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	if id != "DGS10" {
		return nil, nil
	}
	d, _ := time.Parse(dayLayout, "2026-11-04")
	v := 4.2
	return []contracts.SeriesPoint{{
		SeriesID:        "DGS10",
		ObservationDate: types.SerializableDate{Time: d},
		Unit:            contracts.SeriesUnit("percent"),
		Value:           &v,
	}}, nil
}

func TestMacroPageRealBuild(t *testing.T) {
	t.Parallel()
	p := newDeps().macroPage(time.UTC, "dev")
	b, err := p.Build(t.Context(), onePoint{}, "2026-11-05", time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if m["schema"] != "macro_page" || m["schema_version"] != float64(1) || p.Meta.Schema != "page.macro_page.v1" ||
		m["run_id"] != "ot-page" || m["scope"] != "macro" || m["build_id"] != "dev" {
		t.Fatalf("meta %+v page %s", p.Meta, b)
	}
}

func TestUnknownPageListsReports(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.deps.pageList = []pages.Page{fakePage("alpha", "a")}
	_, err := pageNamed(a, "nosuch")
	want := `unknown page "nosuch"; known pages: alpha, macro_page, report_page`
	if err == nil || err.Error() != want {
		t.Fatalf("err %v, want %s", err, want)
	}
}
