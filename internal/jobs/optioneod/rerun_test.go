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
type fakeCalendar struct{ open map[string]time.Time }

func (f fakeCalendar) Session(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	return clock.Session{CalendarID: id, Day: day, IsSession: true, OpenAt: f.open[id]}, nil
}

func (f fakeCalendar) NextSession(ctx context.Context, id string, day contracts.Day) (clock.Session, error) {
	return f.Session(ctx, id, day)
}

func TestSlotMatchesSchedule(t *testing.T) {
	t.Parallel()
	// Monday 2026-11-09: XNYS opens 09:30 EST (14:30Z), XTKS opens 09:00 JST (00:00Z).
	cal := fakeCalendar{open: map[string]time.Time{
		"XNYS": time.Date(2026, 11, 9, 14, 30, 0, 0, time.UTC),
		"XTKS": time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
	}}
	for name, tc := range map[string]struct {
		data []byte
		want string
	}{
		"us":    {manifestBytes, "1100"},
		"index": {manifestIndexBytes, "1100"},
		"jp":    {manifestJPBytes, "1030"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			j, err := build(tc.data, nil, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			calID, anchor, _, offset := j.Schedule()
			if anchor != anchorOpen {
				t.Fatalf("anchor %q", anchor)
			}
			s, err := cal.Session(context.Background(), calID, "2026-11-09")
			if err != nil {
				t.Fatal(err)
			}
			at := s.OpenAt.Add(offset).In(j.loc)
			if got := at.Format("1504"); got != j.m.Slot || got != tc.want {
				t.Fatalf("scheduled %s, slot %q, want %q", got, j.m.Slot, tc.want)
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
	parse := func(slot string) error {
		_, err := parseManifest([]byte("id = \"x\"\nslot = \"" + slot + "\"\n[[underlying]]\nsymbol = \"A\"\ncon_id = 1\ncurrency = \"USD\"\n"))
		return err
	}
	if err := parse("1100"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"11", "11000", "11a0", "abcd"} {
		if err := parse(bad); err == nil {
			t.Fatalf("slot %q accepted", bad)
		}
	}
}
