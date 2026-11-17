package data_health_page

import (
	"strings"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

const (
	readingDataset = "readings.reading"
	lateText       = "the source publishes a day later (FRED, the Federal Reserve data service, posts a day's point on the next day)"
)

// LateText sets the due text of a missing cell of obs.series_point or readings.reading whose every missing key is a
// FRED series with its newest point on the window day just before the cell's day: FRED posts a day's point on the
// next day, so such a gap is no fault. A cell with a due text, a key that is no known series, or an older newest
// point is left unchanged. The repair verdict is not touched.
func LateText(dataset string, cells []Cell, days []contracts.Day, newest []coverage.Newest) {
	if dataset != fredDataset && dataset != readingDataset {
		return
	}
	newestDay := make(map[string]contracts.Day, len(newest))
	for _, n := range newest {
		newestDay[n.SeriesID] = n.Day
	}
	prev := make(map[contracts.Day]contracts.Day, len(days))
	for i := 1; i < len(days); i++ {
		prev[days[i]] = days[i-1]
	}
	for i := range cells {
		c := &cells[i]
		p, ok := prev[c.Day]
		if c.State != string(coverage.StateMissing) || c.DueText != "" || !ok || len(c.MissingKeys) == 0 {
			continue
		}
		late := true
		for _, k := range c.MissingKeys {
			if dataset == readingDataset {
				k = k[strings.LastIndex(k, "|")+1:]
			}
			if d, found := newestDay[k]; !found || d != p {
				late = false
				break
			}
		}
		if late {
			c.DueText = lateText
		}
	}
}
