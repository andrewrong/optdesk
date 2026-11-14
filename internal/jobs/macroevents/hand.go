package macroevents

import (
	_ "embed"
	"fmt"
	"slices"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/contracts"
)

//go:embed hand_events.toml
var handBytes []byte

// handSource is one [[source]] table: the dates of one kind and year, with the page they were read from.
type handSource struct {
	Kind    string   `toml:"kind"`
	Year    int      `toml:"year"`
	URL     string   `toml:"url"`
	ReadDay string   `toml:"read_day"`
	Dates   []string `toml:"dates"`
}

// The kinds of the hand-kept file; fomc_minutes is derived from fomc.
const (
	kindFOMC       = "fomc"
	kindMinutes    = "fomc_minutes"
	kindBOJ        = "boj"
	kindQuarterEnd = "quarter_end"
	kindUSTax      = "us_tax"
)

var handKinds = []string{kindFOMC, kindBOJ, kindQuarterEnd, kindUSTax}

const minutesLag = 21

func parseHand(data []byte) ([]handSource, error) {
	var doc struct {
		Source []handSource `toml:"source"`
	}
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("macroevents: hand file: %w", err)
	}
	for i, s := range doc.Source {
		if !slices.Contains(handKinds, s.Kind) {
			return nil, fmt.Errorf("macroevents: hand file: source %d has an unknown kind", i)
		}
		if s.URL == "" {
			return nil, fmt.Errorf("macroevents: hand file: source %d (%s %d) has no url", i, s.Kind, s.Year)
		}
		if _, err := time.Parse("2006-01-02", s.ReadDay); err != nil {
			return nil, fmt.Errorf("macroevents: hand file: source %d (%s %d) has no read_day", i, s.Kind, s.Year)
		}
		for _, d := range s.Dates {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				return nil, fmt.Errorf("macroevents: hand file: source %d (%s %d) has a date that is not YYYY-MM-DD", i, s.Kind, s.Year)
			}
		}
	}
	return doc.Source, nil
}

// minutesDay is the day the minutes of a meeting ending on end are published: end + 21 days.
func minutesDay(end contracts.Day) contracts.Day {
	t, err := time.Parse("2006-01-02", string(end))
	if err != nil {
		return end
	}
	return contracts.Day(t.AddDate(0, 0, minutesLag).Format("2006-01-02"))
}
