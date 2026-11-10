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
		Account: fakeAccount{}, Candidates: fakeCandidates{},
		Eod: fakeEod{}, Catalog: fakeCatalog{}, Names: fakeNames{}, Terms: fakeTerms{},
		Sessions: fakeSessions{}, Tracker: fakeTracker{}, Replay: fakeReplay{},
		Params:     trackapi.TrackParams{MarkMaxAgeSessions: 3, RecentDays: 30},
		CalendarID: "XNYS",
	}
	at := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	b1, err1 := option_tracking_page.Build(context.Background(), c, noSeries{}, "2026-11-10", at)
	b2, err2 := option_tracking_page.NewPage(c).Build(context.Background(), noSeries{}, "2026-11-10", at)
	if err1 != nil || err2 != nil || len(b1) == 0 || string(b1) != string(b2) {
		t.Fatalf("errors = %v, %v; want the same page from both", err1, err2)
	}
}

func TestBuildNeedsZoneTrackerReplay(t *testing.T) {
	t.Parallel()
	ok := option_tracking_page.Config{RunID: "r", BuildID: "b", Scope: "s", Zone: time.UTC,
		Tracker: fakeTracker{}, Replay: fakeReplay{}}
	at := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	if _, err := option_tracking_page.Build(context.Background(), ok, noSeries{}, "2026-11-10", at); err != nil {
		t.Fatalf("complete config: %v", err)
	}
	for name, mut := range map[string]func(*option_tracking_page.Config){
		"zone":    func(c *option_tracking_page.Config) { c.Zone = nil },
		"tracker": func(c *option_tracking_page.Config) { c.Tracker = nil },
		"replay":  func(c *option_tracking_page.Config) { c.Replay = nil },
	} {
		c := ok
		mut(&c)
		if _, err := option_tracking_page.Build(context.Background(), c, noSeries{}, "2026-11-10", at); err == nil {
			t.Errorf("nil %s gives no error", name)
		}
	}
}
