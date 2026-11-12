package optioneod

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const jpDay = contracts.Day("2026-11-09")

var jpKey = contracts.ExpectedKey{DatasetID: "obs.option_eod_jp", SubjectKey: "N225"}

func newJPRig(t *testing.T, at string) *rig {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	now, err := time.ParseInLocation("2006-01-02 15:04", at, loc)
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{b: newBroker(), res: &fakeResolver{}, w: &fakeWriter{}, clk: clockwork.NewFakeClockAt(now)}
	r.b.price = num(40000)
	r.b.clk = r.clk
	r.b.chains = []broker.Chain{
		{Exchange: "OSE.JPN", TradingClass: "225W", Multiplier: "1000", Expirations: []string{"20261113"}, Strikes: []string{"40000"}},
		{Exchange: "OSE.JPN", TradingClass: "N225M", Multiplier: "100", Expirations: []string{"20261113"}, Strikes: []string{"40000"}},
		{Exchange: "OSE.JPN", TradingClass: "225", Multiplier: "1000", Expirations: []string{"20261113"}, Strikes: []string{"39750", "40000"}},
	}
	r.job, err = build(manifestJPBytes, r.b, r.res, r.w, r.clk)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestJPManifestSlotAndWindow(t *testing.T) {
	t.Parallel()
	r := newJPRig(t, "2026-11-09 16:00")
	j, err := NewJP(r.b, r.res, r.w, r.clk)
	if err != nil {
		t.Fatal(err)
	}
	jj := j.(*job)
	if jj.m.Zone != "Asia/Tokyo" || j.ID() != "option_eod_jp" {
		t.Fatalf("zone %q id %q", jj.m.Zone, j.ID())
	}
	cal, anchor, zone, off := jj.Schedule()
	if cal != "XTKS" || anchor != "time" || zone != "Asia/Tokyo" || off != 15*time.Hour+50*time.Minute {
		t.Fatalf("schedule %s %s %s %s", cal, anchor, zone, off)
	}
	wc, open, closeOff := jj.Window()
	if wc != "XTKS" || open != 0 || closeOff != 2*time.Hour {
		t.Fatalf("window %s %s %s", wc, open, closeOff)
	}
}

func TestJPPlanKeysUseLedgerDataset(t *testing.T) {
	t.Parallel()
	keys, _, err := planFrom(manifestJPBytes, jpDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != jpKey {
		t.Fatalf("keys %+v", keys)
	}
	us, _, err := planFrom(manifestBytes, jpDay)
	if err != nil || us[0].DatasetID != "obs.option_eod" {
		t.Fatalf("us keys %+v %v", us, err)
	}
}

func TestPairsFiltersTradingClass(t *testing.T) {
	t.Parallel()
	r := newJPRig(t, "2026-11-09 16:00")
	res := r.run(jpDay, jpKey)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	rows := r.w.all()
	if len(rows) != 4 || len(r.b.snaps) != 2 {
		t.Fatalf("rows %d snaps %d", len(rows), len(r.b.snaps))
	}
	for _, c := range r.b.snaps[1].cs {
		if c.TradingClass != "225" || c.Multiplier != "1000" {
			t.Fatalf("contract %+v", c)
		}
	}
}

func TestChainOfClassRule(t *testing.T) {
	t.Parallel()
	chains := []broker.Chain{
		{Exchange: "ARCA", TradingClass: "B"},
		{Exchange: "SMART", TradingClass: "2B"},
		{Exchange: "SMART", TradingClass: "B", Multiplier: "100"},
	}
	m := manifest{}
	c, class, ok := m.chainOf(chains, underlying{Symbol: "B", Exchange: "SMART"})
	if !ok || c.Exchange != "SMART" || c.TradingClass != "B" || class != "B" {
		t.Fatalf("got %+v %q %v", c, class, ok)
	}
	if _, class, ok = m.chainOf(chains, underlying{Symbol: "B", Exchange: "XXX"}); ok || class != "B" {
		t.Fatal("no chain on the exchange must give none")
	}
	if _, _, ok = m.chainOf(chains, underlying{Symbol: "Q", Exchange: "SMART"}); ok {
		t.Fatal("no chain of the symbol class must give none, not the first chain")
	}
	m.TradingClass = "2B"
	if c, class, ok = m.chainOf(chains, underlying{Symbol: "B", Exchange: "SMART"}); !ok || c.TradingClass != "2B" || class != "2B" {
		t.Fatalf("manifest class %+v %v", c, ok)
	}
	m.TradingClass = "Z"
	if _, class, ok = m.chainOf(chains, underlying{Symbol: "B", Exchange: "SMART"}); ok || class != "Z" {
		t.Fatal("class without chain must give none")
	}
}

func TestRunAfterStopAfterStores(t *testing.T) {
	t.Parallel()
	for _, at := range []string{"2026-11-09 17:00", "2026-11-09 18:27", "2026-11-09 23:30"} {
		late := newJPRig(t, at)
		if res := late.run(jpDay, jpKey); res.Err != nil || late.b.calls() == 0 || res.Usable != 0 {
			t.Fatalf("%s: %v calls %d usable %d", at, res.Err, late.b.calls(), res.Usable)
		}
		rows := late.w.all()
		if len(rows) == 0 {
			t.Fatalf("%s: no rows", at)
		}
		for _, row := range rows {
			if row.Status != statusNoQuote || row.ErrorCode == nil || *row.ErrorCode != "AFTER_SESSION" ||
				row.Bid != nil || row.Ask != nil || row.Last != nil || row.UnderlyingPrice != nil {
				t.Fatalf("%s: row %+v", at, row)
			}
		}
	}
}

func TestRunBeforeStopAfterKeepsClose(t *testing.T) {
	t.Parallel()
	r := newJPRig(t, "2026-11-09 16:50")
	res := r.run(jpDay, jpKey)
	rows := r.w.all()
	if res.Err != nil || len(rows) == 0 || res.Usable != len(rows) {
		t.Fatalf("result %+v rows %d", res, len(rows))
	}
	for _, row := range rows {
		if row.Status != statusOK || row.Bid == nil || row.ErrorCode != nil {
			t.Fatalf("row %+v", row)
		}
	}
}

func TestJPRowsCurrencyJPY(t *testing.T) {
	t.Parallel()
	r := newJPRig(t, "2026-11-09 16:00")
	if res := r.run(jpDay, jpKey); res.Err != nil {
		t.Fatal(res.Err)
	}
	rows := r.w.all()
	if len(rows) == 0 {
		t.Fatal("no rows")
	}
	for _, row := range rows {
		if row.Currency != "JPY" {
			t.Fatalf("currency %q", row.Currency)
		}
	}
}
