package option_boards

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// maxTableRows is the schema limit of the candidate table (Decision 6).
const maxTableRows = 200

// candidatesAnchor is the id of the candidate table's section, the target of all_link.
const candidatesAnchor = "candidates-US"

const staleNoRowsText = gapNoRows

var errNoZone = errors.New("option_boards: nil zone")

// BuildOption builds page.option_page.v1 (Decisions 3 to 7 and 9). r is unused; at is given in UTC and written
// in c.Zone as generated_at. A nil Candidates gives the gap no_reader; a store error fails the build.
func BuildOption(ctx context.Context, c OptionConfig, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, errNoZone
	}
	gaps := []gap{}
	notes := []string{jpNote}
	l := loaded{Rows: []store.Candidate{}}
	if c.Candidates == nil {
		gaps = append(gaps, mkGap("no_reader", gapNoReader))
	} else {
		var err error
		if l, err = loadShortPuts(ctx, c.Candidates, day); err != nil {
			return nil, err
		}
		if !l.Found {
			gaps = append(gaps, mkGap("no_data", gapNoRows))
		}
	}
	stale, err := optionStale(l, day, c.Zone)
	if err != nil {
		return nil, err
	}
	dataTimes := []sourceTime{}
	if l.Found {
		dataTimes = append(dataTimes, sourceTime{Source: candidatesSource, DataTime: zoneTime(l.ComputedAt, c.Zone)})
		if l.Stale {
			gaps = append(gaps, mkGap("stale", gapStale))
		}
	}

	cards := cardRows(l.Rows)
	tableRows := l.Rows
	if len(tableRows) > maxTableRows {
		notes = append(notes, fmt.Sprintf("候选表只显示前 %d 个,共 %d 个", maxTableRows, len(l.Rows)))
		tableRows = tableRows[:maxTableRows]
	}
	shown := append(append([]store.Candidate{}, tableRows...), cards...)
	eod, err := loadEod(ctx, c.Eod, shown, l.Day)
	if err != nil {
		return nil, err
	}

	pane := marketPane{
		Market: "US", Label: "美股", Currency: "USD",
		Headline:        fmt.Sprintf("%d 个值得看。", len(cards)),
		CardsTitle:      "今日机会",
		Cards:           buildOptionCards(cards, eod),
		CandidatesTitle: "候选表",
		Candidates:      buildTable(tableRows, eod),
		AllLink:         link{Text: fmt.Sprintf("全部 %d 个机会", len(l.Rows)), Target: candidatesAnchor},
		Chain:           buildChain(l.Rows, cards),
	}
	p := optionPayload{
		Schema: "option_page", SchemaVersion: 1,
		RunID: c.RunID, BuildID: c.BuildID, Scope: c.Scope,
		GeneratedAt: zoneTime(at, c.Zone),
		DataTimes:   dataTimes,
		RunStatus:   runStatus(gaps),
		Title:       optionTitle,
		Stale:       stale,
		Markets:     []marketPane{pane},
		Notes:       notes,
		Gaps:        gaps,
	}
	return json.Marshal(p)
}

// optionStale is the label of Decision 3 in its three cases.
func optionStale(l loaded, day contracts.Day, zone *time.Location) (staleLabel, error) {
	if !l.Found {
		start, err := dayStart(day, zone)
		if err != nil {
			return staleLabel{}, err
		}
		return staleLabel{DataTime: start, StaleAfter: start, Text: staleNoRowsText}, nil
	}
	text := staleDayPrefix + string(l.Day)
	endDay := day
	if l.Stale {
		text += staleDaySuffix
		endDay = l.Day
	}
	end, err := dayEnd(endDay, zone)
	if err != nil {
		return staleLabel{}, err
	}
	return staleLabel{DataTime: zoneTime(l.ComputedAt, zone), StaleAfter: end, Text: text}, nil
}
