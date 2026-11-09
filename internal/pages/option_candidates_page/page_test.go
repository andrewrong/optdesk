package option_candidates_page_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

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
