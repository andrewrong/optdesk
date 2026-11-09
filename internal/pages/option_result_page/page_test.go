package option_result_page_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/option_result_page"
)

func TestNewPageMeta(t *testing.T) {
	t.Parallel()
	p := option_result_page.NewPage(option_result_page.Config{}, "main")
	if p.Meta.Name != "option_result_page" || p.Meta.Schema != "page.option_result_page.v1" {
		t.Fatalf("Meta = %+v", p.Meta)
	}
	if p.Build == nil {
		t.Fatal("Build is nil")
	}
}

func TestConfigContract(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_empty.json")
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, c.Zone)
	b, err := option_result_page.NewPage(c, "main").Build(context.Background(), noSeries{}, contracts.Day("2026-01-02"), at)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
}
