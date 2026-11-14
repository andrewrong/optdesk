//go:build pg

package snapshot

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// storedDay is what obs.option_day holds for one contract on 2026-11-06.
type storedDay struct {
	bid        *string
	status     int
	receivedAt time.Time
	source     string
}

func readDay(t *testing.T, name string, instrument int) storedDay {
	t.Helper()
	var d storedDay
	err := connectAs(t, name, "").QueryRow(context.Background(), `SELECT bid::text, status, received_at, source `+
		`FROM obs.option_day WHERE instrument_id = $1 AND session_day = '2026-11-06'`, instrument).
		Scan(&d.bid, &d.status, &d.receivedAt, &d.source)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestOptionDayKeepsFieldsByCoalesce(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	first := sessionRow(t, capA, 0, fp(1.25))
	insertEod(t, name, first)
	insertEod(t, name, sessionRow(t, capB, 5, nil))
	d := readDay(t, name, 9)
	if d.bid == nil || *d.bid != "1.25" {
		t.Fatalf("bid %v, want the stored 1.25", d.bid)
	}
	// The later row leaves every compared column as stored, so the day row is not rewritten and keeps its received_at.
	if want := first.ReceivedAt; !d.receivedAt.Equal(want) {
		t.Fatalf("received_at %v, want %v", d.receivedAt, want)
	}
	// An earlier received_at changes nothing.
	insertEod(t, name, sessionRow(t, "00000000-0000-4000-8000-0000000000a3", -30, fp(9)))
	after := readDay(t, name, 9)
	if after.bid == nil || *after.bid != "1.25" || !after.receivedAt.Equal(d.receivedAt) {
		t.Fatalf("an earlier row changed the day row: %+v", after)
	}
}

func TestOptionDayNewestUsable(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	older := sessionRow(t, capA, 0, fp(1.25))
	newer := sessionRow(t, capB, 10, fp(1.5))
	off := sessionRow(t, "00000000-0000-4000-8000-0000000000a3", 20, fp(3))
	off.SnapshotKind = "off_session"
	insertEod(t, name, newer, older, off)
	d := readDay(t, name, 9)
	if d.bid == nil || *d.bid != "1.5" || d.status != 1 || d.source != "option_eod" {
		t.Fatalf("day row %+v, want the newest usable row (bid 1.5)", d)
	}
}

func TestOptionDayReaderChain(t *testing.T) {
	t.Parallel()
	name := newDatabase(t)
	a, b := sessionRow(t, capA, 0, fp(1.25)), sessionRow(t, capA, 0, fp(2.5))
	b.InstrumentID = 10
	other := sessionRow(t, capA, 0, fp(4))
	other.InstrumentID, other.UnderlyingID = 11, 7
	insertEod(t, name, a, b, other)

	p := connectAs(t, name, "ot_ro_pages")
	if _, err := p.Exec(context.Background(), `SELECT count(*) FROM obs.option_day`); err == nil {
		t.Fatal("ot_ro_pages reads the table")
	}
	r, err := OpenDayReader(context.Background(), connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	got, err := r.ChainDay(context.Background(), 8, contracts.Day("2026-11-06"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].InstrumentID != 9 || got[1].InstrumentID != 10 {
		t.Fatalf("chain %+v", got)
	}
	if g := got[0]; g.Bid == nil || *g.Bid != "1.25" || g.SessionDay != "2026-11-06" || g.Source != "option_eod" ||
		g.OiAsofDate == nil || *g.OiAsofDate != "2026-11-05" || !g.ReceivedAt.Equal(a.ReceivedAt) {
		t.Fatalf("first row %+v", g)
	}
	days, err := r.Days(context.Background(), []int64{9, 11}, "2026-11-01", "2026-11-30")
	if err != nil || len(days) != 2 || days[1].InstrumentID != 11 {
		t.Fatalf("days %+v, err %v", days, err)
	}
}
