package main

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/local-e2e.md#3-local-check-net

const (
	netLookbackDays = 14 // the range both sides are read over, as the fred_series manifest's lookback
	netSeriesData   = "obs.series_point"
)

// netSeriesIDs are the FRED series of the fred_series manifest (internal/jobs/fredseries/job.toml, whose list is
// not exported); the macro page reads one of them. Own list, because the manifest gives no accessor.
var netSeriesIDs = []string{"DGS10", "DGS3MO", "SP500", "VIXCLS", "VXNCLS", "RVXCLS", "VXVCLS", "WALCL", "WDTGAL", "RRPONTSYD", "DFF", "BAMLH0A0HYM2"}

// checkFredNet is the FRED line of `ot check scheduler` (Decision 4g): with --net, the latest stored point of each
// FRED series against the latest public observation, FAIL when the stored day is older, EXPLAINED window_open
// while the window of the dataset is open (exps is what the scheduler check read). Without --net it prints one
// offline line and makes no network call. It only reads.
func checkFredNet(ctx context.Context, e *checkEnv, now time.Time, exps []store.Expectation) {
	zone := e.a.profile.Zone
	if zone == nil {
		zone = time.UTC
	}
	today := now.In(zone)
	if !e.opts.Net {
		e.c.emit(Result{Status: statusExplained, Check: "fred.public", Day: today.Format(schedDayLayout), Reason: "offline: pass --net"})
		return
	}
	fail := func(check, reason string) {
		e.c.emit(Result{Status: statusFail, Check: check, Reason: reason})
	}
	key := fredKey(e.a)
	if key == "" {
		fail("fred.public", "FRED_API_KEY is not set")
		return
	}
	dsn := e.a.getenv(pagesDSNName)
	if dsn == "" {
		fail("fred.stored", pagesDSNName+" is not set")
		return
	}
	ser, err := e.a.deps.openSeries(ctx, dsn)
	if err != nil {
		fail("fred.stored", "open series store with "+pagesDSNName+" failed")
		return
	}
	defer func() { _ = ser.Close() }()
	src, err := e.a.deps.newFRED(sources.FREDConfig{BaseURL: fredBaseURL, APIKey: key, Clock: e.a.deps.newClock()})
	if err != nil {
		fail("fred.public", "build FRED source failed")
		return
	}
	from := contracts.Day(today.AddDate(0, 0, -netLookbackDays).Format(schedDayLayout))
	to := contracts.Day(today.Format(schedDayLayout))
	pts, err := src.Fetch(ctx, sources.FetchPlan{SeriesIDs: netSeriesIDs, From: from, To: to})
	if err != nil {
		// The text of a source error carries no key; only the class is shown the same way as elsewhere.
		fail("fred.public", "fetch failed: "+err.Error())
		return
	}
	netCompare(ctx, e, ser, latestDays(pts), netWindowOpen(now, exps), from, to)
}

// netCompare is one line per series: the stored latest day against the public one.
func netCompare(ctx context.Context, e *checkEnv, ser store.SeriesReader, public map[string]string, open bool, from, to contracts.Day) {
	for _, id := range netSeriesIDs {
		check := "fred." + id
		pub, ok := public[id]
		if !ok {
			e.c.emit(Result{Status: statusFail, Check: check, Reason: "no public observation in " + string(from) + ".." + string(to)})
			continue
		}
		stored, err := ser.SeriesPoints(ctx, id, from, to)
		if err != nil {
			e.c.emit(Result{Status: statusFail, Check: check, Day: pub, Reason: "read stored points failed"})
			continue
		}
		have := latestDays(stored)[id]
		switch {
		case have >= pub:
			e.c.emit(Result{Status: statusPass, Check: check, Day: pub, Reason: "stored " + have + " matches public"})
		case open:
			e.c.emit(Result{Status: statusExplained, Check: check, Day: pub, Reason: "window_open"})
		default:
			e.c.emit(Result{Status: statusFail, Check: check, Day: pub, Reason: "stored " + dayOrNone(contracts.Day(have)) + " is older than public " + pub})
		}
	}
}

// latestDays is the latest observation day with a value, per series id.
func latestDays(pts []contracts.SeriesPoint) map[string]string {
	out := map[string]string{}
	for _, p := range pts {
		if p.Value == nil {
			continue
		}
		if d := p.ObservationDate.Format(schedDayLayout); d > out[p.SeriesID] {
			out[p.SeriesID] = d
		}
	}
	return out
}

// netWindowOpen holds when a window of the series dataset is not complete and has not closed (DueBy after now).
func netWindowOpen(now time.Time, exps []store.Expectation) bool {
	for _, x := range exps {
		if x.DatasetID == netSeriesData && !x.Closed && x.PresentDay != x.ExpectedDay && x.DueBy.After(now) {
			return true
		}
	}
	return false
}
