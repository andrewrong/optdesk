package option_result_page_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/option_result_page"
	"github.com/andrewrong/optdesk/internal/store"
)

type (
	fakeLedger   struct{ store.OptionLedgerReader }
	fakeSessions struct{ store.SessionDayReader }
	noSeries     struct{ store.SeriesReader }
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
	c := option_result_page.Config{
		RunID: "r", BuildID: "b", Zone: time.UTC, Aliases: []string{"main"},
		Ledger: fakeLedger{}, Sessions: fakeSessions{},
		Ledgers: func() compute.OptionLedger { return nil },
		Results: func() compute.OptionResults { return nil },
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	_, err := option_result_page.NewPage(c, "main").Build(context.Background(), noSeries{}, contracts.Day("2026-01-02"), at)
	if err == nil || err.Error() != "option_result_page: not built" {
		t.Fatalf("err = %v", err)
	}
}
