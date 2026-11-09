package optionresult

import (
	"slices"

	"github.com/andrewrong/optdesk/internal/compute"
)

// reliability counts the rows by agreement of BrokerRealized with the replayed figure
// (equal as numbers; a figure that is not a number differs) and lists the ids the replay
// left out. The ids never enter a figure.
func reliability(rows []row, unresolved, review []string) compute.ResultReliability {
	var out compute.ResultReliability
	for _, r := range rows {
		if r.pos.BrokerRealized == "" {
			out.NoBroker++
			continue
		}
		if b, ok := parse(r.pos.BrokerRealized); ok && b.Cmp(r.realized) == 0 {
			out.Equal++
		} else {
			out.Differ++
		}
	}
	out.Unresolved = slices.Clone(unresolved)
	out.Review = slices.Clone(review)
	return out
}
