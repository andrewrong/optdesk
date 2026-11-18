package option_candidates_page_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/option_candidates_page"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestNewPageMeta(t *testing.T) {
	t.Parallel()
	p := option_candidates_page.NewPage(option_candidates_page.Config{})
	if p.Meta.Name != "option_candidates_page" || p.Meta.Schema != "page.option_candidates_page.v1" {
		t.Fatalf("Meta = %+v", p.Meta)
	}
	if p.Build == nil {
		t.Fatal("Build is nil")
	}
}

type fakeCandidates struct{}

func (fakeCandidates) Candidates(context.Context, contracts.Day, string) ([]store.Candidate, error) {
	return []store.Candidate{}, nil
}

type noSeries struct{ store.SeriesReader }

func TestConfigContract(t *testing.T) {
	t.Parallel()
	c := option_candidates_page.Config{
		RunID: "r", BuildID: "b", Scope: "s",
		Zone: time.UTC, Candidates: fakeCandidates{},
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	day := contracts.Day("2026-01-02")
	for name, build := range map[string]func() ([]byte, error){
		"Build": func() ([]byte, error) {
			return option_candidates_page.Build(context.Background(), c, noSeries{}, day, at)
		},
		"NewPage": func() ([]byte, error) {
			return option_candidates_page.NewPage(c).Build(context.Background(), noSeries{}, day, at)
		},
	} {
		out, err := build()
		if err != nil || !json.Valid(out) || !strings.Contains(string(out), `"schema":"option_candidates_page"`) {
			t.Fatalf("%s = %q, %v", name, out, err)
		}
	}
}

type fakeNames struct{}

func (fakeNames) InstrumentNames(context.Context, []int64) ([]store.InstrumentName, error) {
	return nil, nil
}

func TestConfigNamesField(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	day := contracts.Day("2026-01-02")
	for _, names := range []store.InstrumentNameReader{fakeNames{}, nil} {
		c := option_candidates_page.Config{
			RunID: "r", BuildID: "b", Scope: "s",
			Zone: time.UTC, Candidates: fakeCandidates{}, Names: names,
		}
		for name, build := range map[string]func() ([]byte, error){
			"Build": func() ([]byte, error) {
				return option_candidates_page.Build(context.Background(), c, noSeries{}, day, at)
			},
			"NewPage": func() ([]byte, error) {
				return option_candidates_page.NewPage(c).Build(context.Background(), noSeries{}, day, at)
			},
		} {
			out, err := build()
			if err != nil || !json.Valid(out) {
				t.Fatalf("%s (names %T) = %q, %v", name, names, out, err)
			}
		}
	}
}

type fakeEvaluator struct{}

func (fakeEvaluator) Evaluate(optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
	return optionevidenceapi.Evidence{}, nil
}

type fakeReadings struct{ store.ReadingViewReader }

func TestConfigEvidenceFields(t *testing.T) {
	t.Parallel()
	c := option_candidates_page.Config{
		RunID: "r", BuildID: "b", Scope: "s",
		Zone: time.UTC, Candidates: fakeCandidates{},
		Evidence: fakeEvaluator{},
		ATMFor: func(context.Context, int, contracts.Day, contracts.Day, string) (string, string, error) {
			return "0.294", "95.00", nil
		},
		Readings: fakeReadings{},
	}
	p := option_candidates_page.NewPage(c)
	if p.Meta.Name != "option_candidates_page" || p.Meta.Schema != "page.option_candidates_page.v1" {
		t.Fatalf("Meta = %+v", p.Meta)
	}
}

func TestConfigTierFieldsNilBuild(t *testing.T) {
	t.Parallel()
	c := option_candidates_page.Config{
		RunID: "r", BuildID: "b", Scope: "s",
		Zone: time.UTC, Candidates: fakeCandidates{},
		Tiers: nil, Account: nil, Aliases: nil, Owner: nil, RuleParams: nil,
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	out, err := option_candidates_page.Build(context.Background(), c, noSeries{}, contracts.Day("2026-01-02"), at)
	if err != nil || !json.Valid(out) || !strings.Contains(string(out), `"schema":"option_candidates_page"`) {
		t.Fatalf("Build = %q, %v", out, err)
	}
	s := string(out)
	if i := strings.Index(s, "short_put_tiers"); i >= 0 && !strings.HasPrefix(s[i:], `short_put_tiers":null`) {
		t.Fatalf("short_put_tiers is not null: %s", s[i:])
	}
}

func TestConfigValuationFieldsNilBuild(t *testing.T) {
	t.Parallel()
	c := option_candidates_page.Config{
		RunID: "r", BuildID: "b", Scope: "s",
		Zone: time.UTC, Candidates: fakeCandidates{},
		Valuation: nil, ValueLines: nil,
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	out, err := option_candidates_page.Build(context.Background(), c, noSeries{}, contracts.Day("2026-01-02"), at)
	if err != nil || !json.Valid(out) || !strings.Contains(string(out), `"schema":"option_candidates_page"`) {
		t.Fatalf("Build = %q, %v", out, err)
	}
	var page struct {
		Tiers *struct {
			Companies []map[string]any `json:"companies"`
		} `json:"short_put_tiers"`
	}
	if err := json.Unmarshal(out, &page); err != nil {
		t.Fatal(err)
	}
	if page.Tiers != nil {
		for _, co := range page.Tiers.Companies {
			if v, ok := co["valuation"]; ok && v != nil {
				t.Fatalf("valuation = %v, want null", v)
			}
		}
	}
}
