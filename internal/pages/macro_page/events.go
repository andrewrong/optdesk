package macro_page

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

// Design: docs/design/macro-events.md, Decisions 2, 3, 8 and 9.

const (
	// eventWindowDays is the window of the page: the page day through the page day plus this many days.
	eventWindowDays = 60
	// eventPastDays is how many days before the page day the table still lists events (macro-event-values Decision 10).
	eventPastDays = 7
	// maxEvents is the maxItems of the events array in the schema.
	maxEvents = 20
	// staleFetchDays: a FRED kind whose newest run day is more than this many days old is stale.
	staleFetchDays = 10
	// maxGapText is the maxLength of a gap text in the schema.
	maxGapText = 200
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
	{kind: "cpi", source: "fred", name: "CPI", when: whenET830, high: true},
	{kind: "employment_situation", source: "fred", name: "Employment situation", when: whenET830, high: true},
	{kind: "pce", source: "fred", name: "PCE", when: whenET830, high: true},
	{kind: "gdp", source: "fred", name: "GDP", when: whenET830, high: true},
	{kind: "ppi", source: "fred", name: "PPI", when: whenET830},
	{kind: "retail_sales", source: "fred", name: "Retail sales", when: whenET830},
	{kind: "jolts", source: "fred", name: "JOLTS", when: whenET1000},
	{kind: "h41", source: "fred", name: "H.4.1", when: whenET1630},
	{kind: "fomc", source: "file", name: "FOMC decision", when: whenET1400, high: true, entered: "FOMC"},
	{kind: "fomc_minutes", source: "file", name: "FOMC minutes", when: whenET1400},
	{kind: "boj", source: "file", name: "BoJ decision", when: whenJSTMidday, high: true, entered: "BoJ"},
	{kind: "quarter_end", source: "file", name: "Quarter-end", when: whenDateOnly, entered: "Quarter-end"},
	{kind: "us_tax", source: "file", name: "US corporate tax date", when: whenDateOnly, entered: "US tax"},
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
	// Previous is the text of the previous figure, "-" for a kind with no figure, or a gap.
	Previous any `json:"previous"`
	// Written only with a Liquidity function (page.macro_page.v2): the figures of the previous and the actual.
	PreviousFigure any `json:"previous_figure,omitempty"`
	Actual         any `json:"actual,omitempty"`
	// Written only with a Window function.
	DaysTo   *int  `json:"days_to,omitempty"`
	InWindow *bool `json:"in_window,omitempty"`
}

func newEventRow(e macroevent.Event, cells eventCells, figures bool) eventRow {
	c, ok := conventionOf[e.Kind]
	if !ok {
		c = eventConvention{name: e.Kind, when: whenDateOnly}
	}
	when := c.when
	if c.high {
		when += whenHighSuffix
	}
	if e.Source == "file" && e.RunDay != "" {
		when += whenReadPrefix + string(e.RunDay)
	}
	row := eventRow{Date: string(e.Date), WhenText: when, NameText: c.name, Previous: cells.Previous}
	if figures {
		row.PreviousFigure, row.Actual = cells.PreviousFigure, cells.Actual
	}
	return row
}

// eventBlock reads the stored events of the window and the gaps that say what is missing from them. An error of
// the reader is an error, never a gap.
func eventBlock(ctx context.Context, r store.SeriesReader, rd macroevent.Reader, d time.Time, figures bool) ([]eventRow, []macroevent.Event, []gap, error) {
	day := contracts.Day(d.Format(dayLayout))
	end := d.AddDate(0, 0, eventWindowDays)
	from := contracts.Day(d.AddDate(0, 0, -eventPastDays).Format(dayLayout))
	events, err := rd.Between(ctx, from, contracts.Day(end.Format(dayLayout)))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("macro_page: read events: %w", err)
	}
	kinds, err := rd.Kinds(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("macro_page: read event kinds: %w", err)
	}
	entered, err := enteredYears(ctx, rd, d, end)
	if err != nil {
		return nil, nil, nil, err
	}

	shown := events
	if len(shown) > maxEvents {
		shown = shown[:maxEvents]
	}
	cells, err := eventValues(ctx, r, rd, shown, d)
	if err != nil {
		return nil, nil, nil, err
	}
	rows := make([]eventRow, 0, maxEvents)
	for i, e := range shown {
		rows = append(rows, newEventRow(e, cells[i], figures))
	}
	var gaps []gap
	add := func(reason, prefix string, items []string) {
		if len(items) > 0 {
			gaps = append(gaps, gap{Reason: reason, Text: mergedLine(prefix, items)})
		}
	}
	stale, none, held := kindGaps(kinds, day)
	add("stale", gapPrefixStale, stale)
	add("no_data", gapPrefixNone, none)
	add("stale_input", gapPrefixHeld, held)
	add("no_data", gapPrefixNotEntered, notEntered(entered, d.Year(), end.Year()))
	if len(events) > maxEvents {
		gaps = append(gaps, gap{Reason: "missing_input", Text: fmt.Sprintf(gapEventsNotShown,
			rows[len(rows)-1].Date, maxEvents, len(events))})
	}
	return rows, events, gaps, nil
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

// mergedLine joins items after prefix with commas; a list too long for the schema is cut with the "N more" ending of zh_texts.go, counted in characters.
func mergedLine(prefix string, items []string) string {
	for n := len(items); n > 0; n-- {
		line := prefix + strings.Join(items[:n], ", ")
		if n < len(items) {
			line += fmt.Sprintf(gapMoreAfterList, len(items)-n)
		}
		if utf8.RuneCountInString(line) <= maxGapText {
			return line
		}
	}
	return prefix + fmt.Sprintf(gapMoreAfterPrefix, len(items))
}

// withEvents turns the page to its events form when c.Events is set: the title, the sentence, the rows, and
// the old "Event calendar: no data yet" gap (the last of vm.Gaps) dropped. It returns the event gaps, which the
// caller appends after the ease gap is settled. With no reader it changes nothing.
func withEvents(ctx context.Context, r store.SeriesReader, c Config, d time.Time, vm *viewModel) ([]gap, error) {
	if c.Events == nil {
		return nil, nil
	}
	rows, events, gaps, err := eventBlock(ctx, r, c.Events, d, c.Liquidity != nil)
	if err != nil {
		return nil, err
	}
	vm.events = events
	vm.EventsTitle = eventsTitleWithReader
	vm.Sentence = sentenceEv
	vm.Gaps = vm.Gaps[:1]
	vm.Events = rows
	return gaps, nil
}

// markWindow writes days_to and in_window on each row: in_window is true when the days to the event are zero or more
// and at most days.
func markWindow(rows []eventRow, d time.Time, days int) {
	for i := range rows {
		ed, err := time.Parse(dayLayout, rows[i].Date)
		if err != nil {
			continue
		}
		n := daysBetween(d, ed)
		in := n >= 0 && n <= days
		rows[i].DaysTo, rows[i].InWindow = &n, &in
	}
}
