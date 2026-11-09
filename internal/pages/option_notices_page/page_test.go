package option_notices_page_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/option_notices_page"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeAccount struct{}

func (fakeAccount) Positions(context.Context, string, string) ([]store.Position, error) {
	return []store.Position{}, nil
}

func (fakeAccount) Values(context.Context, string, string) ([]store.AccountValue, error) {
	return []store.AccountValue{}, nil
}

func (fakeAccount) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return []store.Execution{}, nil
}

type fakeCandidates struct{}

func (fakeCandidates) Candidates(context.Context, contracts.Day, string) ([]store.Candidate, error) {
	return []store.Candidate{}, nil
}

type fakeEarnings struct{}

func (fakeEarnings) EarningsDates(context.Context, string, contracts.Day, contracts.Day) ([]contracts.EarningsDate, error) {
	return []contracts.EarningsDate{}, nil
}

type fakeRule struct{}

func (fakeRule) Evaluate(compute.NoticeInput, compute.NoticeParams) compute.NoticeResult {
	return compute.NoticeResult{}
}

type noSeries struct{ store.SeriesReader }

func TestNewPageMeta(t *testing.T) {
	t.Parallel()
	p := option_notices_page.NewPage(option_notices_page.Config{})
	if p.Meta.Name != "option_notices_page" || p.Meta.Schema != "page.option_notices_page.v1" {
		t.Fatalf("Meta = %+v", p.Meta)
	}
	if p.Build == nil {
		t.Fatal("Build is nil")
	}
}

func TestConfigContract(t *testing.T) {
	t.Parallel()
	c := option_notices_page.Config{
		RunID: "r", BuildID: "b", Scope: "s", Zone: time.UTC,
		Main: "main", Aliases: []string{"main", "other"},
		Account: fakeAccount{}, Candidates: fakeCandidates{}, Earnings: fakeEarnings{},
		Rule: fakeRule{}, Params: compute.NoticeParams{ExpiryDays: 3, CushionWarn: "0.2", CushionAct: "0.1"},
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	day := contracts.Day("2026-01-02")
	for name, build := range map[string]func() ([]byte, error){
		"Build": func() ([]byte, error) { return option_notices_page.Build(context.Background(), c, noSeries{}, day, at) },
		"NewPage": func() ([]byte, error) {
			return option_notices_page.NewPage(c).Build(context.Background(), noSeries{}, day, at)
		},
	} {
		b, err := build()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var out struct {
			Schema string `json:"schema"`
			Day    string `json:"day"`
		}
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if out.Schema != "option_notices_page" || out.Day != "2026-01-02" {
			t.Errorf("%s: output %+v", name, out)
		}
	}
}
