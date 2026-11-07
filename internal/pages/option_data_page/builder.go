// Package option_data_page builds the page.option_data_page.v1 view model from the stored option
// end-of-day rows and quote snapshots. It only selects and formats stored values; it computes
// nothing, reads no clock and waits for no collection.
package option_data_page

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	dayLayout = "2006-01-02"

	lookbackDays = 7
	maxRows      = 500
	maxSeries    = 3
	maxPoints    = 200
	maxGaps      = 20
	maxNotes     = 20

	defaultCurrency = "USD"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Config carries the identity of the run, the underlying and the two readers.
// Zone is the owner's zone, given by the caller; data times are written in it.
type Config struct {
	RunID        string
	BuildID      string
	Scope        string
	Zone         *time.Location
	UnderlyingID int64
	Label        string
	Eod          store.OptionEodReader
	Quotes       store.QuoteSnapshotReader
	// Notes are the texts of the page's notes, shown as given; at most maxNotes are kept.
	Notes []string
	// Symbol is the underlying's symbol; empty: symbol is null.
	Symbol string
	// Terms reads the option terms of the kept rows; nil: expiry and right are null and strike is a no_data gap.
	Terms store.OptionTermsReader
	// Choices are the underlyings the page links to; empty: no underlyings field.
	Choices []Choice
}

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type status struct {
	Code int    `json:"code"`
	Text string `json:"text"`
}

type chainRow struct {
	InstrumentID int64   `json:"instrument_id"`
	Expiry       *string `json:"expiry"`
	Right        *string `json:"right"`
	Strike       any     `json:"strike"`
	Status       status  `json:"status"`
	Bid          any     `json:"bid"`
	Ask          any     `json:"ask"`
	Last         any     `json:"last"`
	Volume       any     `json:"volume"`
	OpenInterest any     `json:"open_interest"`
	IV           any     `json:"iv"`
	Delta        any     `json:"delta"`
	Gamma        any     `json:"gamma"`
	Theta        any     `json:"theta"`
	Vega         any     `json:"vega"`
}

type underlying struct {
	ID         int64   `json:"id"`
	Symbol     *string `json:"symbol"`
	Label      string  `json:"label"`
	SessionDay *string `json:"session_day"`
	Price      any     `json:"price"`
	Currency   string  `json:"currency"`
}

type chain struct {
	SessionDay *string    `json:"session_day"`
	Rows       []chainRow `json:"rows"`
}

type quotePoint struct {
	SessionDay string `json:"session_day"`
	Slot       string `json:"slot"`
	Bid        any    `json:"bid"`
	Ask        any    `json:"ask"`
	Last       any    `json:"last"`
}

type quoteSeries struct {
	InstrumentID int64        `json:"instrument_id"`
	Caption      string       `json:"caption"`
	Points       []quotePoint `json:"points"`
}

type viewModel struct {
	Schema        string        `json:"schema"`
	SchemaVersion int           `json:"schema_version"`
	RunID         string        `json:"run_id"`
	BuildID       string        `json:"build_id"`
	Scope         string        `json:"scope"`
	GeneratedAt   string        `json:"generated_at"`
	DataTimes     []sourceTime  `json:"data_times"`
	RunStatus     string        `json:"run_status"`
	Stale         *struct{}     `json:"stale"`
	Title         string        `json:"title"`
	Sentence      string        `json:"sentence"`
	ChainTitle    string        `json:"chain_title"`
	SeriesTitle   string        `json:"series_title"`
	NotesTitle    string        `json:"notes_title"`
	Notes         []string      `json:"notes"`
	Gaps          []gap         `json:"gaps"`
	Underlying    underlying    `json:"underlying"`
	Chain         chain         `json:"chain"`
	QuoteSeries   []quoteSeries `json:"quote_series"`
	Underlyings   []choiceView  `json:"underlyings,omitempty"`
}

func validate(c Config) error {
	if c.Zone == nil {
		return fmt.Errorf("option_data_page: nil zone")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "scope": c.Scope} {
		if !idPattern.MatchString(v) {
			return fmt.Errorf("option_data_page: bad %s %q", name, v)
		}
	}
	if c.Eod == nil || c.Quotes == nil {
		return fmt.Errorf("option_data_page: nil reader")
	}
	return checkChoices(c.Choices)
}

// symbolPtr is nil for an empty symbol, so the page shows null.
func symbolPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func fmtTime(t time.Time, zone *time.Location) string { return t.In(zone).Format(time.RFC3339) }

// Build returns the page.option_data_page.v1 JSON for day. generatedAt is written in UTC; the
// page is "ok" when the eod read holds rows and "partial" with a gap when it is empty.
// A reader error is returned and no page with it.
func Build(ctx context.Context, c Config, day contracts.Day, generatedAt time.Time) ([]byte, error) {
	if err := validate(c); err != nil {
		return nil, err
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("option_data_page: bad day %q: %w", day, err)
	}
	from := contracts.Day(d.AddDate(0, 0, -lookbackDays).Format(dayLayout))

	eod, err := c.Eod.OptionEod(ctx, c.UnderlyingID, from, day)
	if err != nil {
		return nil, fmt.Errorf("option_data_page: read option eod: %w", err)
	}

	vm := viewModel{
		Schema: "option_data_page", SchemaVersion: 1,
		RunID: c.RunID, BuildID: c.BuildID, Scope: c.Scope,
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		DataTimes:   []sourceTime{}, RunStatus: "ok",
		Title: "Option data", ChainTitle: "Option chain, last stored day",
		SeriesTitle: "Quotes by slot", NotesTitle: "Notes",
		Notes: append([]string{}, limit(c.Notes, maxNotes)...), Gaps: []gap{}, QuoteSeries: []quoteSeries{},
		Underlying: underlying{ID: c.UnderlyingID, Symbol: symbolPtr(c.Symbol), Label: c.Label, Price: noData(), Currency: defaultCurrency},
		Chain:      chain{Rows: []chainRow{}}, Underlyings: choiceViews(c.Choices, c.UnderlyingID),
	}

	rows, latest, err := selectRows(eod)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		vm.RunStatus = "partial"
		vm.Gaps = limit([]gap{newGap("no option data stored")}, maxGaps)
		vm.Sentence = fmt.Sprintf("0 contracts on %s.", day)
		return json.Marshal(vm)
	}

	sessionDay := rows[0].SessionDay.Format(dayLayout)
	vm.Underlying.SessionDay = &sessionDay
	vm.Underlying.Price = priceCell(latest.UnderlyingPrice)
	if latest.UnderlyingCcy != "" {
		vm.Underlying.Currency = latest.UnderlyingCcy
	}
	vm.Chain.SessionDay = &sessionDay
	var eodTime time.Time
	for _, r := range rows {
		vm.Chain.Rows = append(vm.Chain.Rows, toRow(r))
		if r.ReceivedAt.After(eodTime) {
			eodTime = r.ReceivedAt
		}
	}
	if err := applyTerms(ctx, c.Terms, rows, vm.Chain.Rows); err != nil {
		return nil, err
	}
	vm.DataTimes = append(vm.DataTimes, sourceTime{Source: "option_eod", DataTime: fmtTime(eodTime, c.Zone)})
	vm.Sentence = fmt.Sprintf("%d contracts on %s.", len(rows), sessionDay)

	var quoteTime time.Time
	for _, r := range limit(rows, maxSeries) {
		id := int64(r.InstrumentID)
		qs, err := c.Quotes.QuoteSnapshots(ctx, id, from, day)
		if err != nil {
			return nil, fmt.Errorf("option_data_page: read quotes of %d: %w", id, err)
		}
		s := quoteSeries{InstrumentID: id, Caption: "Contract " + strconv.FormatInt(id, 10), Points: []quotePoint{}}
		for _, q := range limit(qs, maxPoints) {
			s.Points = append(s.Points, quotePoint{
				SessionDay: q.SessionDay.Format(dayLayout), Slot: q.Slot,
				Bid: priceCell(q.Bid), Ask: priceCell(q.Ask), Last: priceCell(q.Last),
			})
			if q.ReceivedAt.After(quoteTime) {
				quoteTime = q.ReceivedAt
			}
		}
		vm.QuoteSeries = append(vm.QuoteSeries, s)
	}
	if !quoteTime.IsZero() {
		vm.DataTimes = append(vm.DataTimes, sourceTime{Source: "quote_snapshot", DataTime: fmtTime(quoteTime, c.Zone)})
	}
	return json.Marshal(vm)
}

// selectRows keeps the rows of the latest session day, one per instrument (the greatest
// received_at, a tie keeps the later in reader order), ordered by open interest descending
// (null last) then instrument id, at most maxRows. latest is the row of that day with the
// greatest received_at (a tie keeps the later).
func selectRows(eod []contracts.OptionEod) ([]contracts.OptionEod, contracts.OptionEod, error) {
	var latest contracts.OptionEod
	if len(eod) == 0 {
		return nil, latest, nil
	}
	lastDay := ""
	for _, r := range eod {
		if s := r.SessionDay.Format(dayLayout); s > lastDay {
			lastDay = s
		}
	}
	byInstrument := map[int]contracts.OptionEod{}
	haveLatest := false
	for _, r := range eod {
		if r.SessionDay.Format(dayLayout) != lastDay {
			continue
		}
		if _, ok := statusText[r.Status]; !ok {
			return nil, latest, fmt.Errorf("option_data_page: instrument %d has status %d", r.InstrumentID, r.Status)
		}
		if !haveLatest || !r.ReceivedAt.Before(latest.ReceivedAt) {
			latest, haveLatest = r, true
		}
		if prev, ok := byInstrument[r.InstrumentID]; !ok || !r.ReceivedAt.Before(prev.ReceivedAt) {
			byInstrument[r.InstrumentID] = r
		}
	}
	rows := make([]contracts.OptionEod, 0, len(byInstrument))
	for _, r := range byInstrument {
		rows = append(rows, r)
	}
	slices.SortFunc(rows, func(a, b contracts.OptionEod) int {
		switch {
		case a.OpenInterest == nil && b.OpenInterest != nil:
			return 1
		case a.OpenInterest != nil && b.OpenInterest == nil:
			return -1
		case a.OpenInterest != nil && *a.OpenInterest != *b.OpenInterest:
			return cmp.Compare(*b.OpenInterest, *a.OpenInterest)
		}
		return cmp.Compare(a.InstrumentID, b.InstrumentID)
	})
	return limit(rows, maxRows), latest, nil
}

func toRow(r contracts.OptionEod) chainRow {
	return chainRow{
		InstrumentID: int64(r.InstrumentID),
		Strike:       newGap("no strike"),
		Status:       status{Code: r.Status, Text: statusText[r.Status]},
		Bid:          priceCell(r.Bid), Ask: priceCell(r.Ask), Last: priceCell(r.Last),
		Volume: countCell(r.Volume), OpenInterest: countCell(r.OpenInterest),
		IV:    ivCell(r.IvDecimal),
		Delta: greekCell(r.Delta, "delta"), Gamma: greekCell(r.Gamma, "gamma"),
		Theta: greekCell(r.ThetaPerCalendarDay, "theta"), Vega: greekCell(r.VegaPerVolPoint, "vega"),
	}
}
