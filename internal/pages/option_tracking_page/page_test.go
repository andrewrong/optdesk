package option_tracking_page_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/pages/option_tracking_page"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestNewPageMeta(t *testing.T) {
	t.Parallel()
	p := option_tracking_page.NewPage(option_tracking_page.Config{})
	if p.Meta.Name != "option_tracking_page" || p.Meta.Schema != "page.option_tracking_page.v1" {
		t.Fatalf("Meta = %+v", p.Meta)
	}
	if p.Build == nil {
		t.Fatal("Build is nil")
	}
}

type fakeTracker struct{}

func (fakeTracker) Track(trackapi.TrackInput, trackapi.TrackParams) trackapi.TrackResult {
	return trackapi.TrackResult{}
}

type fakeReplay struct{}

func (fakeReplay) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	return compute.LedgerResult{}
}

type (
	fakeLedger     struct{ store.OptionLedgerReader }
	fakeAccount    struct{ store.AccountReader }
	fakeCandidates struct{ store.CandidateReader }
	fakeEod        struct{ store.OptionEodReader }
	fakeCatalog    struct{ store.OptionEodCatalog }
	fakeNames      struct{ store.InstrumentNameReader }
	fakeTerms      struct{ store.OptionTermsReader }
	fakeSessions   struct{ store.SessionDayReader }
	noSeries       struct{ store.SeriesReader }
)

func TestConfigContract(t *testing.T) {
	t.Parallel()
	c := option_tracking_page.Config{
		RunID: "r", BuildID: "b", Scope: "s", Zone: time.UTC,
		Ledger: fakeLedger{}, Account: fakeAccount{}, Candidates: fakeCandidates{},
		Eod: fakeEod{}, Catalog: fakeCatalog{}, Names: fakeNames{}, Terms: fakeTerms{},
		Sessions: fakeSessions{}, Tracker: fakeTracker{}, Replay: fakeReplay{},
		Params:     trackapi.TrackParams{MarkMaxAgeSessions: 3, RecentDays: 30},
		CalendarID: "XNYS",
	}
	at := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	_, err1 := option_tracking_page.Build(context.Background(), c, noSeries{}, "2026-11-10", at)
	_, err2 := option_tracking_page.NewPage(c).Build(context.Background(), noSeries{}, "2026-11-10", at)
	if err1 == nil || err2 == nil {
		t.Fatalf("errors = %v, %v; want both non-nil", err1, err2)
	}
	if err1.Error() != err2.Error() || err1.Error() != "option_tracking_page: not built" {
		t.Fatalf("errors = %q, %q", err1, err2)
	}
}
