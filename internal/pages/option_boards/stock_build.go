package option_boards

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	noticeText   = "Positions: see /options/tracking"
	noMatrixText = "No candidate rows for this underlying"
	noRowsText   = "No contracts for this underlying"
)

// stockRow is one stored row of the company with its parsed gates and end-of-day text.
type stockRow struct {
	C     store.Candidate
	Gates []gateResult
	Eod   eodText
	Quote quote
	Live  *livePrices // the prices of the open or refresh row, nil for a close archive quote
}

func (r stockRow) ok() bool { return r.C.Status == "ok" }

// BuildStock builds page.stock_option_page.v3. r is unused; at is given in UTC and written in c.Zone as generated_at.
// The company is c.Key, or for a bare key the display name of the first ok row of the data day, else of the first row.
func BuildStock(ctx context.Context, c StockConfig, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, errors.New("option_boards: nil zone")
	}
	p := stockPayload{
		Schema: "stock_option_page", SchemaVersion: 3, RunID: c.RunID, BuildID: c.BuildID, Scope: c.Scope,
		GeneratedAt: zoneTime(at, c.Zone), DataTimes: []sourceTime{}, Title: stockTitle,
		Maps: []priceMap{}, Cards: []chosenCard{}, Held: []heldItem{}, Footer: footerText, Refresh: refreshOf(c.Refresh),
		Matrix:    matrixBlock{Expiries: []matrixExpiry{}, Strikes: []string{}, Cells: []matrixCell{}, Legend: matrixLegend},
		Contracts: contractsBlock{Columns: contractColumns(), Rows: []contractLine{}, Basis: contractsBasis, Notes: []string{}},
	}
	var gaps []gap
	notice := noticeText
	var ld loaded
	if c.Candidates == nil {
		gaps = append(gaps, mkGap("no_reader", gapNoReader))
		p.SharedGap = strPtr(gapNoReader)
	} else {
		var err error
		if ld, err = loadShortPuts(ctx, c.Candidates, day); err != nil {
			return nil, err
		}
		if !ld.Found {
			gaps = append(gaps, mkGap("no_data", gapNoRows))
			p.SharedGap = strPtr(gapNoRows)
		}
	}
	key := ""
	names := displayNames{}
	var mine []store.Candidate
	if ld.Found {
		var err error
		if names, err = loadNames(ctx, c.Names, ld.Rows); err != nil {
			return nil, err
		}
		key = c.Key
		if key == "" {
			key = bareKey(ld.Rows, names)
		}
		upper := strings.ToUpper(key)
		for _, r := range ld.Rows {
			if strings.ToUpper(names.of(r)) == upper || r.Issuer == key {
				mine = append(mine, r)
			}
		}
		p.DataTimes = []sourceTime{{Source: candidatesSource, DataTime: zoneTime(ld.ComputedAt, c.Zone)}}
		if len(mine) == 0 {
			gaps = append(gaps, mkGap("no_data", noMatrixText))
			p.SharedGap = strPtr(noMatrixText)
		}
	}
	var err error
	if p.VolView, err = buildVolView(ctx, c.Readings, volSubject(key, c.Key, mine, names), day); err != nil {
		return nil, err
	}
	if len(mine) > 0 {
		key = names.of(mine[0])
	}
	p.Top = topBar{Symbol: topSymbol(key), Trend: trendText, Policy: policyText}
	if len(mine) == 0 {
		p.NoCard = strPtr(noMatrixText)
		p.Matrix.Empty = strPtr(noMatrixText)
		p.Contracts.Empty = strPtr(noRowsText)
		p.Notice = &notice
		p.RunStatus = runStatus(gaps)
		return json.Marshal(p)
	}
	more, notes, err := fillCompany(ctx, c, day, &p, mine, ld, names)
	if err != nil {
		return nil, err
	}
	gaps = append(gaps, more...)
	notice += notes
	p.Notice = &notice
	p.RunStatus = runStatus(gaps)
	return json.Marshal(p)
}

// fillCompany fills the top bar, matrix, cards, maps and contracts from the rows of the company; it returns the
// gaps it found and the notes to append to the notice.
func fillCompany(ctx context.Context, c StockConfig, day contracts.Day, p *stockPayload,
	mine []store.Candidate, ld loaded, names displayNames) ([]gap, string, error) {
	var gaps []gap
	rows, dayNote, err := prepareRows(ctx, c, mine, day, ld.Day)
	if err != nil {
		return nil, "", err
	}
	p.Top.Name = names.ticker(mine[0].UnderlyingID)
	if err := fillTop(&p.Top, rows); err != nil {
		return nil, "", err
	}
	if ld.Stale {
		stale, err := staleOf(ld, c.Zone)
		if err != nil {
			return nil, "", err
		}
		p.Stale = &stale
		gaps = append(gaps, mkGap("stale", gapStale))
		p.SharedGap = strPtr(gapStale)
	}
	chosen := chosenIndex(rows)
	matrix, cutNote, err := buildMatrix(rows, chosen)
	if err != nil {
		return nil, "", err
	}
	p.Matrix = matrix
	if p.Cards, err = buildCards(rows, chosen, matrix.Cells); err != nil {
		return nil, "", err
	}
	if p.Maps, err = buildMaps(rows, matrix.Expiries); err != nil {
		return nil, "", err
	}
	if noYieldAnywhere(matrix.Cells) {
		p.Matrix.Empty = strPtr(matrixEmptyText(ld.Day))
		p.Matrix.Cells, p.Matrix.Strikes, p.Matrix.Expiries = []matrixCell{}, []string{}, []matrixExpiry{}
	}
	var contractsCut string
	if p.Contracts.Rows, contractsCut, err = buildContractRows(rows); err != nil {
		return nil, "", err
	}
	return gaps, cutNote + contractsCut + dayNote, nil
}

func strPtr(s string) *string { return &s }

// topSymbol is the symbol of the top bar; the schema wants a text, so an empty key is shown as a dash.
func topSymbol(key string) string {
	if key == "" {
		return "—"
	}
	return key
}

// bareKey is the display name of the first ok row, else of the first row.
func bareKey(rows []store.Candidate, names displayNames) string {
	for _, r := range rows {
		if r.Status == "ok" {
			return names.of(r)
		}
	}
	return names.of(rows[0])
}

// prepareRows pairs each stored row with its gates, close text and quote. day is the build day, which the open rows
// are of; dataDay is the day of the stored rows.
func prepareRows(ctx context.Context, c StockConfig, mine []store.Candidate, day, dataDay contracts.Day) ([]stockRow, string, error) {
	eod, err := loadEod(ctx, c.Eod, mine, dataDay)
	if err != nil {
		return nil, "", err
	}
	opens, err := loadOpens(ctx, c.Quotes, day)
	if err != nil {
		return nil, "", err
	}
	qs := quoteSource{opens: opens, eod: eod, day: dataDay, zone: c.Zone}
	rows := make([]stockRow, len(mine))
	for i, r := range mine {
		gates, err := parseGates(r.Gates)
		if err != nil {
			return nil, "", fmt.Errorf("option_boards: gates of instrument %d: %w", r.InstrumentID, err)
		}
		rows[i] = stockRow{C: r, Gates: gates, Eod: eodTextFor(eod, r.InstrumentID), Quote: qs.of(r.InstrumentID), Live: qs.live(r.InstrumentID)}
	}
	dayNote := ""
	if n := chainDayNote(eod, dataDay); n != "" {
		dayNote = ". " + n
	}
	return rows, dayNote, nil
}

// staleOf is the label of a data day before the build day.
func staleOf(ld loaded, zone *time.Location) (staleLabel, error) {
	end, err := dayEnd(ld.Day, zone)
	if err != nil {
		return staleLabel{}, err
	}
	return staleLabel{DataTime: zoneTime(ld.ComputedAt, zone), StaleAfter: end, Text: staleDayPrefix + string(ld.Day) + staleDaySuffix}, nil
}

// chosenIndex is the index of the ok row with the lowest stored Rank, or -1.
func chosenIndex(rows []stockRow) int {
	best := -1
	for i, r := range rows {
		if !r.ok() || r.C.Rank == nil {
			continue
		}
		if best < 0 || *r.C.Rank < *rows[best].C.Rank {
			best = i
		}
	}
	return best
}
