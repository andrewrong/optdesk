package calendar

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var _ clock.NewSessionDayDifferFunc = NewDiffer

const (
	open  = "2026-11-02T14:30:00Z"
	shut  = "2026-11-02T21:00:00Z"
	early = "2026-11-02T18:00:00Z"
)

func TestDiffAdded(t *testing.T) {
	t.Parallel()
	old := []contracts.SessionDay{row("2026-11-02", true, open, shut, false)}
	fresh := []contracts.SessionDay{old[0], row("2026-11-03", true, open, shut, false)}
	got := NewDiffer().Diff(old, fresh)
	if len(got) != 1 || got[0].Day != "2026-11-03" || got[0].Before != nil || got[0].After == nil {
		t.Fatalf("got %+v", got)
	}
}

func TestDiffEarlyCloseChanged(t *testing.T) {
	t.Parallel()
	old := []contracts.SessionDay{row("2026-11-02", true, open, shut, false)}
	fresh := []contracts.SessionDay{row("2026-11-02", true, open, early, true)}
	got := NewDiffer().Diff(old, fresh)
	if len(got) != 1 || got[0].Before == nil || got[0].After == nil || got[0].Before.EarlyClose || !got[0].After.EarlyClose {
		t.Fatalf("got %+v", got)
	}
}

func TestDiffHolidayMoved(t *testing.T) {
	t.Parallel()
	old := []contracts.SessionDay{
		row("2026-11-02", true, open, shut, false),
		row("2026-11-03", false, "", "", false),
	}
	fresh := []contracts.SessionDay{
		row("2026-11-02", false, "", "", false),
		row("2026-11-03", true, open, shut, false),
	}
	got := NewDiffer().Diff(old, fresh)
	if len(got) != 2 || got[0].Day != "2026-11-02" || got[1].Day != "2026-11-03" {
		t.Fatalf("got %+v", got)
	}
	if !got[0].Before.IsSession || got[0].After.IsSession || got[1].Before.IsSession || !got[1].After.IsSession {
		t.Fatalf("got %+v", got)
	}
}

func TestDiffOnlyInOld(t *testing.T) {
	t.Parallel()
	old := []contracts.SessionDay{row("2026-11-02", true, open, shut, false)}
	got := NewDiffer().Diff(old, nil)
	if len(got) != 1 || got[0].Day != "2026-11-02" || got[0].After != nil || got[0].Before == nil {
		t.Fatalf("got %+v", got)
	}
}

func TestDiffEqualIsEmpty(t *testing.T) {
	t.Parallel()
	rows := []contracts.SessionDay{row("2026-11-02", true, open, shut, false), row("2026-11-03", false, "", "", false)}
	if got := NewDiffer().Diff(rows, append([]contracts.SessionDay(nil), rows...)); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestDiffUnsortedInput(t *testing.T) {
	t.Parallel()
	a := row("2026-11-02", true, open, shut, false)
	b := row("2026-11-03", false, "", "", false)
	c := row("2026-11-04", false, "", "", false)
	old := []contracts.SessionDay{c, a}
	fresh := []contracts.SessionDay{b, a}
	got := NewDiffer().Diff(old, fresh)
	if len(got) != 2 || got[0].Day != "2026-11-03" || got[1].Day != "2026-11-04" {
		t.Fatalf("got %+v", got)
	}
}
