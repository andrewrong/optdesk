package macro_page

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

// Design: docs/design/macro-events.md, Decisions 2, 3, 8 and 9.

const (
	// eventWindowDays is the window of the page: the page day through the page day plus this many days.
	eventWindowDays = 60
	// maxEvents is the maxItems of the events array in the schema.
	maxEvents = 20
	// staleFetchDays: a FRED kind whose newest run day is more than this many days old is stale.
	staleFetchDays = 10
	// maxGapText is the maxLength of a gap text in the schema.
	maxGapText = 200

	eventsTitleWithReader = "Next 60 days, dates in the publisher's own zone (ET, JST)"
)

// eventConvention is the one row of the table of conventions of a kind.
type eventConvention struct {
	kind    string
	source  string // "fred" or "file"
	name    string
	when    string // the conventional time and zone
	high    bool   // importance: shown as "high" in when_text, no tag in the name
	entered string // label of the kind in the "Not entered" gap; empty for a kind that is not entered by hand
}

// conventions is the table of Decision 2, in the order the gap lines list the kinds.
var conventions = []eventConvention{
	{kind: "cpi", source: "fred", name: "CPI", when: "usually 8:30 ET", high: true},
	{kind: "employment_situation", source: "fred", name: "Employment situation", when: "usually 8:30 ET", high: true},
	{kind: "pce", source: "fred", name: "PCE", when: "usually 8:30 ET", high: true},
	{kind: "gdp", source: "fred", name: "GDP", when: "usually 8:30 ET", high: true},
	{kind: "ppi", source: "fred", name: "PPI", when: "usually 8:30 ET"},
	{kind: "retail_sales", source: "fred", name: "Retail sales", when: "usually 8:30 ET"},
	{kind: "jolts", source: "fred", name: "JOLTS", when: "usually 10:00 ET"},
	{kind: "h41", source: "fred", name: "H.4.1", when: "usually 4:30 pm ET"},
	{kind: "fomc", source: "file", name: "FOMC decision", when: "usually 2:00 pm ET", high: true, entered: "FOMC"},
	{kind: "fomc_minutes", source: "file", name: "FOMC minutes", when: "usually 2:00 pm ET"},
	{kind: "boj", source: "file", name: "BoJ decision", when: "usually around midday JST, date in JST", high: true, entered: "BoJ"},
	{kind: "quarter_end", source: "file", name: "Quarter-end", when: "date only", entered: "Quarter-end"},
	{kind: "us_tax", source: "file", name: "US corporate tax date", when: "date only", entered: "US tax"},
}

var conventionOf = func() map[string]eventConvention {
	m := make(map[string]eventConvention, len(conventions))
	for _, c := range conventions {
		m[c.kind] = c
	}
	return m
}()

// eventRow is one entry of the events array: date as stored, the publisher's own calendar day.
type eventRow struct {
	Date     string `json:"date"`
	WhenText string `json:"when_text"`
	NameText string `json:"name_text"`
	Previous gap    `json:"previous"`
}

func newEventRow(e macroevent.Event) eventRow {
	c, ok := conventionOf[e.Kind]
	if !ok {
		c = eventConvention{name: e.Kind, when: "date only"}
	}
	when := c.when
	if c.high {
		when += ", high"
	}
	if e.Source == "file" && e.RunDay != "" {
		when += ", read " + string(e.RunDay)
	}
	return eventRow{
		Date:     string(e.Date),
		WhenText: when,
		NameText: c.name,
		Previous: gap{Reason: "not_built", Text: "previous value: not built"},
	}
}

// eventBlock reads the stored events of the window and the gaps that say what is missing from them. An error of
// the reader is an error, never a gap.
func eventBlock(ctx context.Context, rd macroevent.Reader, d time.Time) ([]eventRow, []gap, error) {
	day := contracts.Day(d.Format(dayLayout))
	end := d.AddDate(0, 0, eventWindowDays)
	events, err := rd.Between(ctx, day, contracts.Day(end.Format(dayLayout)))
	if err != nil {
		return nil, nil, fmt.Errorf("macro_page: read events: %w", err)
	}
	kinds, err := rd.Kinds(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("macro_page: read event kinds: %w", err)
	}
	entered, err := enteredYears(ctx, rd, d, end)
	if err != nil {
		return nil, nil, err
	}

	rows := make([]eventRow, 0, maxEvents)
	for i, e := range events {
		if i == maxEvents {
			break
		}
		rows = append(rows, newEventRow(e))
	}
	var gaps []gap
	add := func(reason, prefix string, items []string) {
		if len(items) > 0 {
			gaps = append(gaps, gap{Reason: reason, Text: mergedLine(prefix, items)})
		}
	}
	stale, none, held := kindGaps(kinds, day)
	add("stale", "Dates older than 10 days: ", stale)
	add("no_data", "No dates stored: ", none)
	add("stale_input", "Stored dates kept, the source's answer was shorter: ", held)
	add("no_data", "Not entered: ", notEntered(entered, d.Year(), end.Year()))
	if len(events) > maxEvents {
		gaps = append(gaps, gap{Reason: "missing_input", Text: fmt.Sprintf("Events after %s not shown: %d of %d shown",
			rows[len(rows)-1].Date, maxEvents, len(events))})
	}
	return rows, gaps, nil
}

// kindGaps sorts the kinds into the three lists of Decision 8 that come from Reader.Kinds, in table order.
func kindGaps(kinds []macroevent.Kind, day contracts.Day) (stale, none, held []string) {
	byKind := map[string]macroevent.Kind{}
	for _, k := range kinds {
		byKind[k.Kind+"/"+k.Source] = k
	}
	limit := ""
	if d, err := time.Parse(dayLayout, string(day)); err == nil {
		limit = d.AddDate(0, 0, -staleFetchDays).Format(dayLayout)
	}
	for _, c := range conventions {
		k, ok := byKind[c.kind+"/"+c.source]
		if c.source == "fred" && (!ok || k.LastDate <= day) {
			none = append(none, c.name)
		}
		if !ok {
			continue
		}
		if c.source == "fred" && string(k.LastRunDay) < limit {
			stale = append(stale, fmt.Sprintf("%s (fetched %s)", c.name, k.LastRunDay))
		}
		if k.HeldDay != "" && k.LastDate > day {
			held = append(held, fmt.Sprintf("%s (%s)", c.name, k.HeldDay))
		}
	}
	return stale, none, held
}

// enteredYears returns the file kinds that have a file row in each year of the page day and, when the window
// reaches it, the next year: year -> kind -> true.
func enteredYears(ctx context.Context, rd macroevent.Reader, d, end time.Time) (map[int]map[string]bool, error) {
	out := map[int]map[string]bool{}
	for y := d.Year(); y <= end.Year(); y++ {
		rows, err := rd.Between(ctx, contracts.Day(fmt.Sprintf("%04d-01-01", y)), contracts.Day(fmt.Sprintf("%04d-12-31", y)))
		if err != nil {
			return nil, fmt.Errorf("macro_page: read events of %d: %w", y, err)
		}
		out[y] = map[string]bool{}
		for _, e := range rows {
			if e.Source == "file" {
				out[y][e.Kind] = true
			}
		}
	}
	return out, nil
}

func notEntered(entered map[int]map[string]bool, first, last int) []string {
	var out []string
	for _, c := range conventions {
		if c.entered == "" {
			continue
		}
		for y := first; y <= last; y++ {
			if !entered[y][c.kind] {
				out = append(out, fmt.Sprintf("%s %d", c.entered, y))
			}
		}
	}
	return out
}

// mergedLine joins items after prefix with commas; a list too long for the schema is cut with "and N more".
func mergedLine(prefix string, items []string) string {
	for n := len(items); n > 0; n-- {
		line := prefix + strings.Join(items[:n], ", ")
		if n < len(items) {
			line += fmt.Sprintf(", and %d more", len(items)-n)
		}
		if len(line) <= maxGapText {
			return line
		}
	}
	return fmt.Sprintf("%sand %d more", prefix, len(items))
}

// withEvents turns the page to its events form when c.Events is set: the title, the sentence, the rows, and
// the old "Event calendar: no data yet" gap (the last of vm.Gaps) dropped. It returns the event gaps, which the
// caller appends after the ease gap is settled. With no reader it changes nothing.
func withEvents(ctx context.Context, c Config, d time.Time, vm *viewModel) ([]gap, error) {
	if c.Events == nil {
		return nil, nil
	}
	rows, gaps, err := eventBlock(ctx, c.Events, d)
	if err != nil {
		return nil, err
	}
	vm.EventsTitle = eventsTitleWithReader
	vm.Sentence = sentenceEv
	vm.Gaps = vm.Gaps[:1]
	vm.Events = rows
	return gaps, nil
}
