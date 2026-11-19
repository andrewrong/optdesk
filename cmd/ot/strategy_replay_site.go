package main

import (
	"context"
	"maps"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/strategy_replay_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/strategy-replay-page.md#3 (Decisions 1, 7, 11).

// The route of the page and of its owner post under the site root, and the name of the page family.
const (
	strategyReplayRoute     = "strategy-replay"
	strategyReplayPostRoute = "strategy-replay-post"
	strategyReplayName      = "strategy_replay_page"
	// strategyReplayOrder sits after base_data_page (135) and before the valuation pages (140), so the tail of the
	// known pages stays as it is.
	strategyReplayOrder = 136
)

func init() {
	registerPage(pageFamily{name: strategyReplayName, route: strategyReplayRoute, order: strategyReplayOrder,
		build: buildStrategyReplay, named: strategyReplayNamed})
}

// buildStrategyReplay is the form view with the defaults. With rd.edit nil (no owner token or settings line) the
// page carries the notice form_off and no form.
func buildStrategyReplay(ctx context.Context, _ *app, clk clock.Clock, _ string, rd siteReaders) ([]byte, error) {
	in := strategy_replay_page.Input{GeneratedAt: generatedAtOf(clk), View: strategy_replay_page.ViewForm}
	if rd.edit == nil {
		in.Notices = []string{strategy_replay_page.NoticeFormOff}
	} else {
		f := strategy_replay_page.DefaultForm()
		in.Form = &f
	}
	return strategy_replay_page.Build(ctx, in)
}

// generatedAtOf is the time of the page as the contracts write it.
func generatedAtOf(clk clock.Clock) string { return clk.Now().UTC().Format(time.RFC3339) }

// strategyReplayNamed is the named builder of ot page --name strategy_replay_page.
func strategyReplayNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	rd, closeAll, err := openSiteReaders(ctx, a)
	if err != nil {
		return pages.Page{}, nil, err
	}
	b := func(ctx context.Context, _ store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
		return buildStrategyReplay(ctx, a, nowClock(now), string(day), rd)
	}
	return pages.Page{Meta: pages.Meta{Name: strategyReplayName, Schema: "page.strategy_replay_page.v1"}, Build: b}, closeAll, nil
}

// replayReportInput copies the result into the plain input of the page, field by field: no arithmetic, no
// rounding. The excluded rows carry the key of the report as their reason and their text.
func replayReportInput(res replayResult) *strategy_replay_page.ReportInput {
	r, h := res.Report, res.Header
	out := &strategy_replay_page.ReportInput{
		AsOf: string(h.AsOf), Lots: h.Lots, FirstQuoteDay: string(r.FirstQuoteDay), LastQuoteDay: string(r.LastQuoteDay),
		LotCount: r.Lots, LotsWithQuote: r.LotsWithQuote, StillOpen: r.StillOpen, LotDays: r.LotDays, QuoteDays: r.QuoteDays,
		MinLots: h.MinLots,
	}
	for _, v := range h.Variants {
		out.Variants = append(out.Variants, strategy_replay_page.Variant{Name: v.Name, ParamsHash: v.Hash})
	}
	for _, k := range slices.Sorted(maps.Keys(r.Excluded)) {
		out.Excluded = append(out.Excluded, strategy_replay_page.Excluded{Reason: k, Text: k, Lots: r.Excluded[k]})
	}
	for _, l := range r.Lines {
		out.Lines = append(out.Lines, strategy_replay_page.Line{
			Set: l.Set, Variant: l.Variant, Currency: l.Currency, Lots: l.Lots, Cycles: l.Cycles, ClosedEarly: l.ClosedEarly,
			Held: l.Held, Assigned: l.Assigned, Rolled: l.Rolled, InMoney: l.InTheMoney,
			KeptShare: l.KeptShare, MeanGross: l.MeanGross, WorstGross: l.WorstGross,
			HoldKeptShare: l.HoldKeptShare, ActualKeptShare: l.ActualKeptShare, UnderMinimum: l.UnderMinimum,
		})
	}
	return out
}
