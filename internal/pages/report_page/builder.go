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

// BlockFunc returns the JSON of one block of report_page v2 for a day (same shape as RiskBlockFunc).
type BlockFunc func(ctx context.Context, day contracts.Day, generatedAt time.Time) ([]byte, error)

// Config is one report: a market (US or JP), a slot (open or close), the owner's zone
// (generated_at is written in it), the source of the risks block and the sources of the actions,
// account_moves and market_watch blocks, and the source of the global_assets block. A nil one of the three keeps
// the not_built block; for market JP they are never called and the block is a jp_slice_missing gap. GlobalAssets is
// called only for market US and slot close; a nil one, market JP and slot open keep the not_built block.
type Config struct {
	Market, Slot string
	Zone         *time.Location
	Risks        RiskBlockFunc
	Actions      BlockFunc
	AccountMoves BlockFunc
	MarketWatch  BlockFunc
	GlobalAssets BlockFunc
}

type banner struct {
	Left  string `json:"left"`
	Right string `json:"right"`
}

type blocks struct {
	Actions      json.RawMessage `json:"actions"`
	AccountMoves json.RawMessage `json:"account_moves"`
	Risks        json.RawMessage `json:"risks"`
	MarketWatch  json.RawMessage `json:"market_watch"`
	GlobalAssets json.RawMessage `json:"global_assets"`
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
	actions, err := c.block(ctx, c.Actions, day, generatedAt, "actions", textActions, func(g gap) any {
		return actionsBlock{newGapBlock(textActions, g), g}
	})
	if err != nil {
		return nil, err
	}
	moves, err := c.block(ctx, c.AccountMoves, day, generatedAt, "account_moves", textMoves, func(g gap) any {
		return accountMovesBlock{newGapBlock(textMoves, g), g, g}
	})
	if err != nil {
		return nil, err
	}
	watch, err := c.block(ctx, c.MarketWatch, day, generatedAt, "market_watch", textWatch, func(g gap) any {
		return marketWatchBlock{newGapBlock(textWatch, g), g, g}
	})
	if err != nil {
		return nil, err
	}
	global, err := c.globalAssets(ctx, day, generatedAt)
	if err != nil {
		return nil, err
	}
	p := page{
		Schema: "report_page", SchemaVersion: 2, Market: c.Market, Slot: c.Slot,
		SessionDate: day, GeneratedAt: generatedAt.In(c.Zone).Format(timeLayout),
		Title: pageTitle, Say: pageSay, Banner: banner{bannerLeft, bannerRight},
		StatusLines: []struct{}{},
		Blocks: blocks{
			Actions:      actions,
			AccountMoves: moves,
			Risks:        risks,
			MarketWatch:  watch,
			GlobalAssets: global,
		},
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("report_page: marshal: %w", err)
	}
	return out, nil
}

// block is the JSON of one of the three blocks: market JP gives the jp_slice_missing gap without
// calling f, a nil f the not_built gap, otherwise the bytes of f; an error of f is an error of the call.
func (c Config) block(ctx context.Context, f BlockFunc, day contracts.Day, at time.Time,
	name string, t blockText, gapped func(gap) any) (json.RawMessage, error) {
	switch {
	case c.Market == "JP":
		return marshalBlock(gapped(jpMissing(t)))
	case f == nil:
		return marshalBlock(gapped(notBuilt(t)))
	}
	b, err := f(ctx, day, at)
	if err != nil {
		return nil, fmt.Errorf("report_page: %s block: %w", name, err)
	}
	return b, nil
}

// globalAssets is the bytes of GlobalAssets for market US and slot close, otherwise the not_built block.
func (c Config) globalAssets(ctx context.Context, day contracts.Day, at time.Time) (json.RawMessage, error) {
	if c.GlobalAssets == nil || c.Market != "US" || c.Slot != "close" {
		g := notBuilt(textGlobal)
		return marshalBlock(globalAssetsBlock{newGapBlock(textGlobal, g), g, g, chartAria})
	}
	b, err := c.GlobalAssets(ctx, day, at)
	if err != nil {
		return nil, fmt.Errorf("report_page: global_assets block: %w", err)
	}
	return b, nil
}

func marshalBlock(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("report_page: marshal: %w", err)
	}
	return b, nil
}
