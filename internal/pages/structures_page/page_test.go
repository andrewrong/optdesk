package structures_page_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/structures_page"
	"github.com/andrewrong/optdesk/internal/store"
)

type (
	fakeLedger   struct{ store.OptionLedgerReader }
	fakeAccount  struct{ store.AccountReader }
	fakeEod      struct{ store.OptionEodReader }
	fakeSessions struct{ store.SessionDayReader }
	fakeTerms    struct{ store.OptionTermsReader }
	noSeries     struct{ store.SeriesReader }
)

func TestNewListPageMeta(t *testing.T) {
	t.Parallel()
	p := structures_page.NewListPage(structures_page.Config{}, "main")
	if p.Meta.Name != "structures_list" || p.Meta.Schema != "page.structures_list.v1" {
		t.Fatalf("Meta = %+v", p.Meta)
	}
	if p.Build == nil {
		t.Fatal("Build is nil")
	}
}

func TestNewDetailPageMeta(t *testing.T) {
	t.Parallel()
	p := structures_page.NewDetailPage(structures_page.Config{}, "main", "main:T1:1")
	if p.Meta.Name != "structure_detail" || p.Meta.Schema != "page.structure_detail.v3" {
		t.Fatalf("Meta = %+v", p.Meta)
	}
	if p.Build == nil {
		t.Fatal("Build is nil")
	}
}

func TestConfigContract(t *testing.T) {
	t.Parallel()
	c := structures_page.Config{
		RunID: "r", BuildID: "b", Zone: time.UTC, Aliases: []string{"main"},
		Ledger: fakeLedger{}, Account: fakeAccount{}, Eod: fakeEod{}, Sessions: fakeSessions{}, Terms: fakeTerms{},
		Ledgers:    func() compute.OptionLedger { return nil },
		Structures: func() compute.OptionStructures { return nil },
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	day := contracts.Day("2026-01-02")
	if _, err := structures_page.NewDetailPage(structures_page.Config{}, "main", "main:T1:1").Build(context.Background(), noSeries{}, day, at); err == nil {
		t.Fatal("detail: a nil zone gave no error")
	}
	if _, err := structures_page.NewDetailPage(c, "nobody", "main:T1:1").Build(context.Background(), noSeries{}, day, at); err == nil {
		t.Fatal("detail: an unknown alias gave no error")
	}
	if _, err := structures_page.NewListPage(structures_page.Config{}, "main").Build(context.Background(), noSeries{}, day, at); err == nil {
		t.Fatal("list: a nil zone gave no error")
	}
}
