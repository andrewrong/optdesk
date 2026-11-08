package calendar

import (
	"sort"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type differ struct{}

// NewDiffer is the real clock.NewSessionDayDifferFunc.
func NewDiffer() clock.SessionDayDiffer { return differ{} }

func dayOf(r contracts.SessionDay) contracts.Day {
	return contracts.Day(r.SessionDay.Time.Format(dayLayout))
}

func sameRow(a, b contracts.SessionDay) bool {
	return a.IsSession == b.IsSession && a.EarlyClose == b.EarlyClose &&
		a.OpenAt.Equal(b.OpenAt) && a.CloseAt.Equal(b.CloseAt)
}

// Diff reports every day on which the slices differ, sorted by day. A row differs when IsSession,
// EarlyClose, OpenAt or CloseAt differ; the input order does not matter.
func (differ) Diff(old, fresh []contracts.SessionDay) []clock.SessionDayChange {
	oldByDay := make(map[contracts.Day]*contracts.SessionDay, len(old))
	for i := range old {
		oldByDay[dayOf(old[i])] = &old[i]
	}
	freshByDay := make(map[contracts.Day]*contracts.SessionDay, len(fresh))
	for i := range fresh {
		freshByDay[dayOf(fresh[i])] = &fresh[i]
	}
	var out []clock.SessionDayChange
	for d, o := range oldByDay {
		f := freshByDay[d]
		if f == nil || !sameRow(*o, *f) {
			out = append(out, clock.SessionDayChange{Day: d, Before: o, After: f})
		}
	}
	for d, f := range freshByDay {
		if oldByDay[d] == nil {
			out = append(out, clock.SessionDayChange{Day: d, After: f})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out
}
