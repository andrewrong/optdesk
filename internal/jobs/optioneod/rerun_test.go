package optioneod

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// fakeCalendar answers one ordinary session day per calendar with the open instants of an ordinary day.
type fakeCalendar struct{ open, close map[string]time.Time }

func (f fakeCalendar) Session(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	return clock.Session{CalendarID: id, Day: day, IsSession: true, OpenAt: f.open[id], CloseAt: f.close[id]}, nil
}

func (f fakeCalendar) NextSession(ctx context.Context, id string, day contracts.Day) (clock.Session, error) {
	return f.Session(ctx, id, day)
}

func TestSlotMatchesSchedule(t *testing.T) {
	t.Parallel()
	// Monday 2026-11-09: XNYS 09:30-16:00 EST, XTKS opens 09:00 JST (00:00Z). Friday 2026-11-27 is a half day: close 13:00 EST.
	cal := fakeCalendar{
		open: map[string]time.Time{
			"XNYS": time.Date(2026, 11, 9, 14, 30, 0, 0, time.UTC),
			"XTKS": time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
		},
		close: map[string]time.Time{
			"XNYS": time.Date(2026, 11, 9, 21, 0, 0, 0, time.UTC),
			"XTKS": time.Date(2026, 11, 9, 6, 30, 0, 0, time.UTC), // 15:30 JST
		},
	}
	half := fakeCalendar{
		open: map[string]time.Time{"XNYS": time.Date(2026, 11, 27, 14, 30, 0, 0, time.UTC)},
		close: map[string]time.Time{
			"XNYS": time.Date(2026, 11, 27, 18, 0, 0, 0, time.UTC),
			"XTKS": time.Date(2026, 11, 27, 6, 30, 0, 0, time.UTC), // Tokyo has no half day
		},
	}
	for name, tc := range map[string]struct {
		data       []byte
		anchor     string
		want       string
		wantOrd    string
		wantHalfAt string
	}{
		"us":    {manifestBytes, anchorClose, "close-120m", "1400", "1100"},
		"index": {manifestIndexBytes, anchorClose, "close-15m", "1545", "1245"},
		"jp":    {manifestJPBytes, anchorClose, "close-15m", "1515", "1515"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			j, err := build(tc.data, nil, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			calID, anchor, _, offset := j.Schedule()
			if anchor != tc.anchor {
				t.Fatalf("anchor %q", anchor)
			}
			ctx := context.Background()
			day := contracts.Day("2026-11-09")
			s, err := cal.Session(ctx, calID, day)
			if err != nil {
				t.Fatal(err)
			}
			if anchor == anchorOpen {
				at := s.OpenAt.Add(offset).In(j.loc)
				if got := at.Format("1504"); got != j.m.Slot || got != tc.want {
					t.Fatalf("scheduled %s, slot %q, want %q", got, j.m.Slot, tc.want)
				}
				return
			}
			if j.m.Slot != tc.want {
				t.Fatalf("slot %q, want %q", j.m.Slot, tc.want)
			}
			if got := s.CloseAt.Add(offset).In(j.loc).Format("1504"); got != tc.wantOrd {
				t.Fatalf("ordinary day slot instant %s, want %s", got, tc.wantOrd)
			}
			hs, err := half.Session(ctx, calID, "2026-11-27")
			if err != nil {
				t.Fatal(err)
			}
			if got := hs.CloseAt.Add(offset).In(j.loc).Format("1504"); got != tc.wantHalfAt {
				t.Fatalf("half day slot instant %s, want %s", got, tc.wantHalfAt)
			}
		})
	}
}

func idsOf(r *rig) []string {
	var out []string
	for _, row := range r.w.all() {
		out = append(out, row.CaptureID)
	}
	return out
}

func TestCaptureIDSameForRerun(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 11:00")
	if res := r.run(testDay, testKey); res.Err != nil {
		t.Fatal(res.Err)
	}
	first := idsOf(r)
	r.clk.Advance(7 * time.Minute)
	if res := r.run(testDay, testKey); res.Err != nil {
		t.Fatal(res.Err)
	}
	all := idsOf(r)
	if len(all) != 2*len(first) || len(first) == 0 {
		t.Fatalf("ids %d then %d", len(first), len(all))
	}
	for i, id := range all {
		if id != first[i%len(first)] {
			t.Fatalf("row %d: id %s, first run %s", i, id, first[i%len(first)])
		}
	}
}

type rowKey struct {
	day        string
	capture    string
	instrument int
}

func keysOf(r *rig) []rowKey {
	var out []rowKey
	for _, row := range r.w.all() {
		out = append(out, rowKey{row.SessionDay.Format(time.DateOnly), row.CaptureID, row.InstrumentID})
	}
	return out
}

func TestRerunSameKeys(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 11:00")
	if res := r.run(testDay, testKey); res.Err != nil {
		t.Fatal(res.Err)
	}
	first := keysOf(r)
	r.clk.Advance(30 * time.Minute)
	if res := r.run(testDay, testKey); res.Err != nil {
		t.Fatal(res.Err)
	}
	second := keysOf(r)[len(first):]
	if len(first) == 0 || len(second) != len(first) {
		t.Fatalf("first %d keys, second %d", len(first), len(second))
	}
	seen := map[rowKey]bool{}
	for _, k := range first {
		seen[k] = true
	}
	for _, k := range second {
		if !seen[k] {
			t.Fatalf("key %+v of the re-run was not in the first run", k)
		}
	}
}

func TestEmptySlotRefused(t *testing.T) {
	t.Parallel()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	clk := clockwork.NewFakeClockAt(time.Date(2026, 11, 6, 11, 0, 0, 0, loc))
	r := newRig(t, "2026-11-06 11:00")
	j, err := build([]byte(`
id = "option_eod"
version = 1
[[underlying]]
symbol = "TEST"
sec_type = "STK"
exchange = "SMART"
currency = "USD"
con_id = 1000001
price_increment = "0.01"
lot_size = "1"
`), r.b, r.res, r.w, clk)
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), testDay, []contracts.ExpectedKey{testKey})
	if kindOf(res) != contracts.Permanent {
		t.Fatalf("result %+v, want a Permanent error", res)
	}
	if len(r.w.calls) != 0 {
		t.Fatal("a refused run wrote rows")
	}
}

func TestSlotMustBeFourDigits(t *testing.T) {
	t.Parallel()
	parse := func(slot, offset string) error {
		sched := "\n[schedule]\ncalendar = \"XNYS\"\nanchor = \"session_close\"\noffset = \"" + offset + "\"\n"
		_, err := parseManifest([]byte("id = \"x\"\nslot = \"" + slot + "\"\n[[underlying]]\nsymbol = \"A\"\ncon_id = 1\ncurrency = \"USD\"\n" + keys + sched))
		return err
	}
	for _, ok := range []string{"1100", "close-15m"} {
		if err := parse(ok, "-15m"); err != nil {
			t.Fatalf("slot %q: %v", ok, err)
		}
	}
	for _, bad := range []string{"11", "11000", "11a0", "abcd", "close-", "close-15", "close-30m"} {
		if err := parse(bad, "-15m"); err == nil {
			t.Fatalf("slot %q accepted", bad)
		}
	}
}
