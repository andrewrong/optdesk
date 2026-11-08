// Package report_page builds the report_page v2 page: the envelope, the risks block as the
// bytes a RiskBlockFunc gives, and four blocks that have no producer yet (state gap, reason
// not_built). It computes no figure and reads no clock.
package report_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const timeLayout = "2006-01-02T15:04:05-07:00"

// RiskBlockFunc returns the JSON of the RisksBlock of report_page v2 for a day.
type RiskBlockFunc func(ctx context.Context, day contracts.Day, generatedAt time.Time) ([]byte, error)

// Config is one report: a market (US or JP), a slot (open or close), the owner's zone
// (generated_at is written in it) and the source of the risks block.
type Config struct {
	Market, Slot string
	Zone         *time.Location
	Risks        RiskBlockFunc
}

type banner struct {
	Left  string `json:"left"`
	Right string `json:"right"`
}

type blocks struct {
	Actions      actionsBlock      `json:"actions"`
	AccountMoves accountMovesBlock `json:"account_moves"`
	Risks        json.RawMessage   `json:"risks"`
	MarketWatch  marketWatchBlock  `json:"market_watch"`
	GlobalAssets globalAssetsBlock `json:"global_assets"`
}

type page struct {
	Schema        string        `json:"schema"`
	SchemaVersion int           `json:"schema_version"`
	Market        string        `json:"market"`
	Slot          string        `json:"slot"`
	SessionDate   contracts.Day `json:"session_date"`
	GeneratedAt   string        `json:"generated_at"`
	Title         string        `json:"title"`
	Say           string        `json:"say"`
	Banner        banner        `json:"banner"`
	StatusLines   []struct{}    `json:"status_lines"`
	Stale         *struct{}     `json:"stale"`
	Blocks        blocks        `json:"blocks"`
}

// Build returns the JSON of report_page v2 for day. A market outside US, JP, a slot outside
// open, close, a nil zone or Risks, or an error of Risks is an error of the call.
func Build(ctx context.Context, c Config, day contracts.Day, generatedAt time.Time) ([]byte, error) {
	if c.Market != "US" && c.Market != "JP" {
		return nil, fmt.Errorf("report_page: market %q is not US or JP", c.Market)
	}
	if c.Slot != "open" && c.Slot != "close" {
		return nil, fmt.Errorf("report_page: slot %q is not open or close", c.Slot)
	}
	if c.Zone == nil || c.Risks == nil {
		return nil, errors.New("report_page: nil zone or risks function")
	}
	if day == "" || generatedAt.IsZero() {
		return nil, errors.New("report_page: empty day or generated_at")
	}
	risks, err := c.Risks(ctx, day, generatedAt)
	if err != nil {
		return nil, fmt.Errorf("report_page: risks block: %w", err)
	}
	p := page{
		Schema: "report_page", SchemaVersion: 2, Market: c.Market, Slot: c.Slot,
		SessionDate: day, GeneratedAt: generatedAt.In(c.Zone).Format(timeLayout),
		Title: pageTitle, Say: pageSay, Banner: banner{bannerLeft, bannerRight},
		StatusLines: []struct{}{},
		Blocks: blocks{
			Actions:      actionsBlock{newGapBlock(textActions), notBuilt(textActions)},
			AccountMoves: accountMovesBlock{newGapBlock(textMoves), notBuilt(textMoves), notBuilt(textMoves)},
			Risks:        risks,
			MarketWatch:  marketWatchBlock{newGapBlock(textWatch), notBuilt(textWatch), notBuilt(textWatch)},
			GlobalAssets: globalAssetsBlock{newGapBlock(textGlobal), notBuilt(textGlobal), notBuilt(textGlobal), chartAria},
		},
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("report_page: marshal: %w", err)
	}
	return out, nil
}
