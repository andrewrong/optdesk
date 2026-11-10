package main

import (
	"context"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// demoReader is the series reader of the demo profile: three typed points of each of the seven
// series of the macro page, with the unit the loader stores (percent for the two rates, index_points
// for the five indices).
type demoReader struct{}

// demoUnits maps each demo series to its stored unit.
var demoUnits = map[string]string{
	"DGS10": "percent", "DGS3MO": "percent",
	"SP500": "index_points", "VIXCLS": "index_points", "VXNCLS": "index_points",
	"RVXCLS": "index_points", "VXVCLS": "index_points",
}

func demoPoint(id, unit, day string, v *float64) contracts.SeriesPoint {
	d, _ := time.Parse(dayLayout, day)
	return contracts.SeriesPoint{
		SeriesID:        id,
		ObservationDate: types.SerializableDate{Time: d},
		Unit:            contracts.SeriesUnit(unit),
		Value:           v,
	}
}

// SeriesPoints returns the demo points of one of the seven series between from and to; other series are empty.
func (demoReader) SeriesPoints(_ context.Context, seriesID string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	unit, ok := demoUnits[seriesID]
	if !ok {
		return nil, nil
	}
	a, b := 4.1, 4.2
	if unit == "index_points" {
		a, b = 5000.25, 5010.5
	}
	all := []struct {
		day string
		v   *float64
	}{{"2026-11-02", &a}, {"2026-11-03", nil}, {"2026-11-04", &b}}
	var out []contracts.SeriesPoint
	for _, p := range all {
		if d := contracts.Day(p.day); d >= from && d <= to {
			out = append(out, demoPoint(seriesID, unit, p.day, p.v))
		}
	}
	return out, nil
}

// demoMarketFigures are the five figures of market_page with the subject each is stored under.
var demoMarketFigures = map[string]string{
	"market.vix_pctile_3y": "VIXCLS", "market.vxn_pctile_3y": "VXNCLS", "market.rvx_pctile_3y": "RVXCLS",
	"market.sp500_vs_200d": "SP500", "market.vix_term_ratio": "VIXCLS",
}

// demoMarketReadings is the ReadingFunc of market_page in the demo profile: one fixed ok reading
// of day to, carrying the parameter data_day, for each of the five figures; other figures have no row.
func demoMarketReadings(ctx context.Context, name, subjectKind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
	if _, ok := demoMarketFigures[name]; !ok || to < from {
		return nil, nil
	}
	rows, err := demoView{}.WinnerReadings(ctx, name, subjectKind, subject, from, to)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	v := "0.5"
	switch name {
	case "market.sp500_vs_200d":
		v = "0.031200"
	case "market.vix_term_ratio":
		v = "0.950000"
	}
	rows[0].Value = &v
	return rows, nil
}

// The demo profile of the notices page: two aliases, the first with a capture and the fixed result below, the
// second with no capture (the block no_capture).
const (
	demoNoticesMain   = "main"
	demoNoticesSecond = "second"
)

// demoNoticeAccount is the account reader of the demo profile: one position for the main alias on every day,
// nothing for any other alias.
type demoNoticeAccount struct{}

// Positions returns the one demo position of the main alias; any other alias has none.
func (demoNoticeAccount) Positions(_ context.Context, alias, day string) ([]store.Position, error) {
	if alias != demoNoticesMain {
		return []store.Position{}, nil
	}
	return []store.Position{{Alias: alias, SessionDay: day, Symbol: "DEMO", SecType: "OPT", Currency: "USD",
		Expiry: "2026-11-20", Strike: "95", Multiplier: "100", Right: "put", Quantity: "-1"}}, nil
}

// Values returns no value rows.
func (demoNoticeAccount) Values(context.Context, string, string) ([]store.AccountValue, error) {
	return []store.AccountValue{}, nil
}

// Executions returns no execution rows.
func (demoNoticeAccount) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return []store.Execution{}, nil
}

// demoNoticeRule is the rule of the demo profile: whatever the input, one notice of each severity.
type demoNoticeRule struct{}

// Evaluate returns the fixed demo result.
func (demoNoticeRule) Evaluate(compute.NoticeInput, compute.NoticeParams) compute.NoticeResult {
	return compute.NoticeResult{RuleVersion: 1, Notices: []compute.Notice{
		{Code: "cushion_low", Severity: "act", Symbol: "DEMO", Contract: "DEMO 2026-11-20 95 put", Expiry: "2026-11-20"},
		{Code: "expiry_near", Severity: "warn", Symbol: "DEMO", Contract: "DEMO 2026-11-20 95 put", Expiry: "2026-11-20"},
		{Code: "capture_stale", Severity: "info"},
	}, CashByDate: []compute.CashDay{}, Cushion: "0.12"}
}
