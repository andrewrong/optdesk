// Package report_blocks writes the actions, account_moves, market_watch and global_assets blocks of the report page v2
// as block JSON. It copies the decimal text of the figures of blockapi.PageFigures and computes
// nothing, sorts no figure, reads no clock and sends no notification: the meters are on the web
// page only.
package report_blocks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

const (
	calendarID         = "XNYS"
	dayLayout          = "2006-01-02"
	sessionLookback    = 15 // calendar days of session days to read
	staleSessions      = 3  // an input older than this many session days is a stale_input gap
	markMaxAgeSessions = 5  // LedgerParams default; the replay needs it for marks this block never reads
	maxGaps            = 20 // maxItems of gaps in the schema
)

// BlockFunc returns the JSON of one block of the report page v2 for a day.
type BlockFunc func(ctx context.Context, day contracts.Day, generatedAt time.Time) ([]byte, error)

// Config is what the blocks need. Zone is the owner's zone (nil is an error of the call); Aliases are
// the configured account aliases, the first is the main account. A nil reader or constructor gives
// the block gap no_reader; a reader error is an error of the call, never a gap.
type Config struct {
	Zone     *time.Location
	Aliases  []string
	Account  store.AccountReader
	Ledger   store.OptionLedgerReader
	Ledgers  compute.NewOptionLedgerFunc
	Series   store.SeriesReader
	Sessions store.SessionDayReader
	Figures  blockapi.NewPageFiguresFunc

	// The global assets block: the stored bars, the names of the stock instruments, the listed assets and the figures.
	Bars     bar.PriceBarReader
	StockIDs StockIDReader
	Assets   []Asset
	Global   blockapi.NewGlobalFiguresFunc
}

type gapJSON struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type cellJSON struct {
	Text  string      `json:"text"`
	Tone  string      `json:"tone"`
	Value json.Number `json:"value,omitempty"`
	Unit  string      `json:"unit,omitempty"`
}

// head is the common part of a block.
type head struct {
	Key       string    `json:"key"`
	Title     string    `json:"title"`
	Say       string    `json:"say"`
	Sub       string    `json:"sub"`
	DataTimes string    `json:"data_times"`
	State     string    `json:"state"`
	Gaps      []gapJSON `json:"gaps"`
}

func newHead(key, title, sub, say, dataTimes string) head {
	return head{Key: key, Title: title, Say: say, Sub: sub, DataTimes: dataTimes, State: "ok", Gaps: []gapJSON{}}
}

// addGap names a gap on the block once and makes the block partial; a block with no usable part is set to gap by its builder.
func (h *head) addGap(g gapJSON) {
	h.State = "partial"
	if len(h.Gaps) < maxGaps && !slices.Contains(h.Gaps, g) {
		h.Gaps = append(h.Gaps, g)
	}
}

func gapOf(reason string) gapJSON { return gapJSON{Reason: reason, Text: gapText(reason)} }

// gapBlock is the whole-block gap: state gap, one Gap named in gaps. extra is marshalled into the block.
func gapHead(key, title, sub, say, reason string) head {
	h := newHead(key, title, sub, say, noDataTimes)
	h.State, h.Gaps = "gap", []gapJSON{gapOf(reason)}
	return h
}

// number is the decimal text as a JSON number, or empty when it is not a plain decimal.
func number(s string) json.Number {
	if s == "" {
		return ""
	}
	for i, r := range s {
		if (r == '-' && i == 0) || r == '.' || (r >= '0' && r <= '9') {
			continue
		}
		return ""
	}
	return json.Number(s)
}

// tone is up, down or flat by the sign of the decimal text; it does no arithmetic.
func tone(s string) string {
	switch {
	case strings.HasPrefix(s, "-"):
		return "down"
	case strings.Trim(s, "0.") == "":
		return "flat"
	default:
		return "up"
	}
}

func signed(s string) string {
	if strings.HasPrefix(s, "-") || strings.Trim(s, "0.") == "" {
		return s
	}
	return "+" + s
}

// figureCell is the cell of a figure, or its Gap; unit is the schema unit of a value in unit's currency.
func figureCell(f compute.FigureValue, currency, unit string, withSign bool) any {
	if f.Gap != "" {
		return gapOf(f.Gap)
	}
	text := f.Value
	if withSign {
		text = signed(text)
	}
	switch unit {
	case "percent", "share_percent":
		return cellJSON{Text: text + "%", Tone: tone(f.Value), Value: number(f.Value), Unit: unit}
	}
	c := cellJSON{Text: text + " " + currency, Tone: "none"}
	if withSign {
		c.Tone = tone(f.Value)
	}
	if currency == "USD" {
		c.Value, c.Unit = number(f.Value), unit
	}
	return c
}

func textCell(s string) cellJSON { return cellJSON{Text: s, Tone: "none"} }

func (c Config) ready() error {
	if c.Zone == nil {
		return errors.New("report_blocks: no zone")
	}
	return nil
}

func parseDay(day contracts.Day, generatedAt time.Time) (time.Time, error) {
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return d, fmt.Errorf("report_blocks: bad day %q: %w", day, err)
	}
	if generatedAt.IsZero() {
		return d, errors.New("report_blocks: empty generated_at")
	}
	return d, nil
}

// sessionDays returns the session days up to and including d, ascending.
func (c Config) sessionDays(ctx context.Context, d time.Time) ([]contracts.Day, error) {
	from := contracts.Day(d.AddDate(0, 0, -sessionLookback).Format(dayLayout))
	rows, err := c.Sessions.SessionDays(ctx, calendarID, from, contracts.Day(d.Format(dayLayout)))
	if err != nil {
		return nil, fmt.Errorf("report_blocks: read session days: %w", err)
	}
	out := make([]contracts.Day, 0, len(rows))
	for _, r := range rows {
		if r.IsSession {
			out = append(out, contracts.Day(r.SessionDay.Format(dayLayout)))
		}
	}
	slices.Sort(out)
	return out, nil
}

func valuesIn(vs []store.AccountValue) []blockapi.ValueIn {
	out := make([]blockapi.ValueIn, 0, len(vs))
	for _, v := range vs {
		out = append(out, blockapi.ValueIn{Tag: v.Tag, Value: v.Value, Currency: v.Currency})
	}
	return out
}

func currencyOf(vs []store.AccountValue, tag string) string {
	for _, v := range vs {
		if v.Tag == tag {
			return v.Currency
		}
	}
	return ""
}

// obligations replays the ledger of alias and returns its open short puts.
func (c Config) obligations(ctx context.Context, alias string, sessions []contracts.Day, day contracts.Day) ([]blockapi.ObligationIn, error) {
	rows, err := c.Ledger.FlexTradeRows(ctx, alias)
	if err != nil {
		return nil, fmt.Errorf("report_blocks: read ledger: %w", err)
	}
	trades := make([]compute.LedgerTrade, 0, len(rows))
	for _, r := range rows {
		attrs := map[string]string{}
		if err := json.Unmarshal([]byte(r.Attributes), &attrs); err != nil {
			return nil, fmt.Errorf("report_blocks: ledger row %s: %w", r.TransactionID, err)
		}
		trades = append(trades, compute.LedgerTrade{TransactionID: r.TransactionID, Revision: r.Revision, Attributes: attrs})
	}
	res := c.Ledgers().Replay(compute.LedgerInput{Alias: alias, Trades: trades, SessionDays: sessions, AsOf: day},
		compute.LedgerParams{MarkMaxAgeSessions: markMaxAgeSessions})
	var out []blockapi.ObligationIn
	for _, p := range res.Positions {
		if p.Status != "open" || p.Right != "P" || !strings.HasPrefix(p.Quantity, "-") {
			continue
		}
		out = append(out, blockapi.ObligationIn{
			Underlying: p.Underlying, Currency: p.Currency, Strike: p.Strike, Multiplier: p.Multiplier,
			Contracts: strings.TrimPrefix(p.Quantity, "-"), PremiumIncome: p.PremiumIncome,
		})
	}
	return out, nil
}

func marshal(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("report_blocks: marshal: %w", err)
	}
	return b, nil
}
