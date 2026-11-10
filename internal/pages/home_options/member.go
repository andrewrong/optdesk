// Package home_options builds the options member of the home page: a page.options_summary.v4
// document read from the replayed option ledger (store.OptionLedgerReader replayed by a
// compute.OptionLedger, as option_tracking_page does). It reads no clock and makes no request.
// A month with no ledger row is a hole (a no_data gap), never zero, and the closed series text
// is built only from ledger rows.
package home_options

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	dayLayout   = "2006-01-02"
	monthLayout = "2006-01"
	timeLayout  = "2006-01-02T15:04:05-07:00"
	months      = 6
	staleDays   = 4 // the freshness rule of the home members: data day + 4 days at 09:30
	minus       = "−"
	usd         = "USD"
	actionOpen  = "open"
	statusOpen  = "open"

	title         = "期权概览"
	subtitle      = "来自成交账本"
	sayNone       = "期权账本暂无记录。"
	sayMonth      = "本月卖出 %d 组，收取权利金 %s。"
	textNoRows    = "账本暂无记录"
	textNoReader  = "期权账本暂未接入"
	textHole      = "该月无账本记录"
	textNoClosed  = "该月没有已平仓记录"
	textMaxLoss   = "最大亏损需要组合结构，账本不给出"
	textOtherCcy  = "账本含非美元持仓，未计入本页"
	textNoDay     = "没有账本日"
	seriesText    = "近六个月权利金"
	seriesCaption = "单位美元，按开仓日计入月份；无账本记录的月份留空"
	noteCash      = "权利金按开仓成交现金计，不含手续费；卖出为正，买入为负。"
	noteCount     = "组数按开仓行计，每行一组。"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Config carries the identity of the run, the owner's zone, the ledger reader and the replay.
// A nil Ledger gives every figure as a no_data gap. A nil Replay with stored rows, like a nil
// Zone, is an error of Build.
type Config struct {
	RunID   string
	BuildID string
	Scope   string
	Zone    *time.Location
	Ledger  store.OptionLedgerReader
	Replay  compute.OptionLedger
}

type gap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type figure struct {
	Kind  string      `json:"kind"`
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
	Tone  string      `json:"tone,omitempty"`
}

type tile struct {
	Label   string `json:"label"`
	Figure  any    `json:"figure"`
	Caption string `json:"caption,omitempty"`
}

type tiles struct {
	Sold        tile `json:"sold_count_month"`
	PremiumMon  tile `json:"premium_month_usd"`
	OpenPremium tile `json:"open_premium_usd"`
	MaxLoss     tile `json:"max_loss_usd"`
}

type point struct {
	Month   string `json:"month"`
	Label   string `json:"label"`
	Premium any    `json:"premium"`
	Closed  any    `json:"closed"`
}

type stale struct {
	DataTime string `json:"data_time"`
	Text     string `json:"text"`
	IsStale  bool   `json:"is_stale"`
}

type card struct {
	Title         string   `json:"title"`
	Say           string   `json:"say"`
	Subtitle      string   `json:"subtitle"`
	Stale         stale    `json:"stale"`
	Tiles         tiles    `json:"tiles"`
	Series        []point  `json:"premium_series_6m"`
	SeriesText    string   `json:"premium_series_text"`
	ClosedText    string   `json:"closed_series_text"`
	SeriesCaption string   `json:"premium_series_caption"`
	Notes         []string `json:"notes"`
	Gaps          []gap    `json:"gaps"`
}

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type summary struct {
	Schema        string       `json:"schema"`
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	BuildID       string       `json:"build_id"`
	Scope         string       `json:"scope"`
	Currency      string       `json:"currency"`
	GeneratedAt   string       `json:"generated_at"`
	StaleAfter    string       `json:"stale_after"`
	DataTimes     []sourceTime `json:"data_times"`
	RunStatus     string       `json:"run_status"`
	Card          card         `json:"card"`
}

// month is the sums of one calendar month; seen is true when a ledger row falls in it.
type month struct {
	seen      bool
	premium   *big.Rat // cash of the opening rows, sold positive
	sold      int      // opening rows that sold
	closed    *big.Rat // realized result of the positions that closed in it
	hasClosed bool
}

// Build returns the page.options_summary.v4 JSON for day; at is generated_at, written in
// c.Zone. The series are the six calendar months ending with the month of day. Rows dated
// after day are ignored. An error of c.Ledger is an error of Build.
func Build(ctx context.Context, c Config, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, errors.New("home_options: nil zone")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "scope": c.Scope} {
		if !idPattern.MatchString(v) {
			return nil, fmt.Errorf("home_options: bad %s %q", name, v)
		}
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("home_options: bad day %q: %w", day, err)
	}
	var positions []compute.LedgerPosition
	hasReader := c.Ledger != nil
	if hasReader {
		if positions, err = replay(ctx, c, day); err != nil {
			return nil, err
		}
	}
	agg, open, newest, otherCcy := aggregate(positions, d)
	s := assemble(c, d, at, agg, open, newest, hasReader, otherCcy)
	return json.Marshal(s)
}

// replay replays every stored alias as of day and returns the positions of all of them.
func replay(ctx context.Context, c Config, day contracts.Day) ([]compute.LedgerPosition, error) {
	aliases, err := c.Ledger.StoredAliases(ctx)
	if err != nil {
		return nil, fmt.Errorf("home_options: read aliases: %w", err)
	}
	var out []compute.LedgerPosition
	for _, alias := range aliases {
		rows, err := c.Ledger.FlexTradeRows(ctx, alias)
		if err != nil {
			return nil, fmt.Errorf("home_options: read trades of %s: %w", alias, err)
		}
		if len(rows) == 0 {
			continue
		}
		if c.Replay == nil {
			return nil, errors.New("home_options: nil replay")
		}
		trades := make([]compute.LedgerTrade, 0, len(rows))
		for _, r := range rows {
			var m map[string]string
			if err := json.Unmarshal([]byte(r.Attributes), &m); err != nil {
				return nil, fmt.Errorf("home_options: attributes of %s: %w", r.TransactionID, err)
			}
			trades = append(trades, compute.LedgerTrade{TransactionID: r.TransactionID, Revision: r.Revision, Attributes: m})
		}
		out = append(out, c.Replay.Replay(compute.LedgerInput{Alias: alias, Trades: trades, AsOf: day}, compute.LedgerParams{}).Positions...)
	}
	return out, nil
}

func rat(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "/") {
		return nil, false
	}
	return new(big.Rat).SetString(s)
}

// aggregate sums the USD positions by calendar month of day d back six months; newest is the
// newest ledger day not after d; otherCcy is true when a position of another currency was left out.
func aggregate(positions []compute.LedgerPosition, d time.Time) (agg map[string]*month, open *big.Rat, newest contracts.Day, other bool) {
	agg = map[string]*month{}
	open = new(big.Rat)
	for i := 0; i < months; i++ {
		agg[monthKey(d, -i)] = &month{premium: new(big.Rat), closed: new(big.Rat)}
	}
	last := contracts.Day(d.Format(dayLayout))
	touch := func(day contracts.Day) *month {
		if day == "" || day > last {
			return nil
		}
		if day > newest {
			newest = day
		}
		return agg[string(day)[:7]]
	}
	for _, p := range positions {
		if p.Currency != usd {
			other = true
			continue
		}
		mult, ok := rat(p.Multiplier)
		if !ok {
			mult = big.NewRat(100, 1)
		}
		addFills(p, mult, last, touch, open)
		if p.Status != statusOpen {
			if m := touch(p.ClosedOn); m != nil {
				m.seen = true
				if r, ok := rat(p.RealizedPnL); ok {
					m.closed.Add(m.closed, r)
					m.hasClosed = true
				}
			}
		}
	}
	return agg, open, newest, other
}

// addFills adds the opening rows of p that are not after last to the month of each row and, for a
// sold row of a still open position, to open.
func addFills(p compute.LedgerPosition, mult *big.Rat, last contracts.Day, touch func(contracts.Day) *month, open *big.Rat) {
	for _, f := range p.Fills {
		if f.Day == "" || f.Day > last {
			continue
		}
		m := touch(f.Day)
		if m != nil {
			m.seen = true
		}
		if f.Action != actionOpen {
			continue
		}
		q, ok1 := rat(f.Quantity)
		px, ok2 := rat(f.Price)
		if !ok1 || !ok2 {
			continue
		}
		cash := new(big.Rat).Mul(q, px)
		cash.Mul(cash, mult).Neg(cash) // sold (negative quantity) is cash in
		if q.Sign() < 0 && p.Status == statusOpen {
			open.Add(open, cash)
		}
		if m == nil {
			continue
		}
		m.premium.Add(m.premium, cash)
		if q.Sign() < 0 {
			m.sold++
		}
	}
}

func monthKey(d time.Time, back int) string {
	first := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
	return first.AddDate(0, back, 0).Format(monthLayout)
}

func assemble(c Config, d time.Time, at time.Time, agg map[string]*month, open *big.Rat, newest contracts.Day, hasReader bool, otherCcy bool) summary {
	cur := agg[monthKey(d, 0)]
	noRows := newest == ""
	noText := textNoRows
	if !hasReader {
		noText = textNoReader
	}
	base := d
	baseText := textNoDay
	if newest != "" {
		base, _ = time.Parse(dayLayout, string(newest))
		baseText = "账本截至 " + base.Format("01-02")
	}
	staleAfter := time.Date(base.Year(), base.Month(), base.Day()+staleDays, 9, 30, 0, 0, c.Zone)
	s := summary{
		Schema: "options_summary", SchemaVersion: 4,
		RunID: c.RunID, BuildID: c.BuildID, Scope: c.Scope, Currency: usd,
		GeneratedAt: at.In(c.Zone).Format(timeLayout),
		StaleAfter:  staleAfter.Format(timeLayout),
		DataTimes:   []sourceTime{},
		RunStatus:   "partial",
	}
	if newest != "" {
		s.DataTimes = append(s.DataTimes, sourceTime{Source: "option_ledger", DataTime: midnight(base, c.Zone)})
	}
	k := &s.Card
	k.Title, k.Subtitle, k.SeriesText, k.SeriesCaption = title, subtitle, seriesText, seriesCaption
	k.Stale = stale{DataTime: midnight(base, c.Zone), Text: baseText, IsStale: at.After(staleAfter)}
	k.Notes = []string{noteCash, noteCount}
	k.Gaps = []gap{{Kind: "gap", Reason: "no_data", Text: textMaxLoss}}
	if noRows {
		k.Gaps = append([]gap{noData(noText)}, k.Gaps...)
	}
	if otherCcy {
		k.Gaps = append(k.Gaps, gap{Kind: "gap", Reason: "unsupported_currency", Text: textOtherCcy})
	}

	fillSeries(k, agg, d)

	// the tiles of the month of day
	k.Tiles = tiles{
		Sold:        tile{Label: "本月卖出", Figure: noData(noText)},
		PremiumMon:  tile{Label: "本月权利金", Figure: noData(noText)},
		OpenPremium: tile{Label: "未平权利金", Figure: noData(noText)},
		MaxLoss:     tile{Label: "最大亏损", Figure: noData(textMaxLoss)},
	}
	k.Say = sayNone
	if !noRows {
		k.Tiles.OpenPremium.Figure = plain(open)
	}
	if cur.seen {
		k.Tiles.Sold = tile{Label: "本月卖出", Caption: "按开仓行计", Figure: map[string]any{
			"kind": "value", "value": cur.sold, "text": fmt.Sprintf("%d 组", cur.sold), "unit": "structures"}}
		k.Tiles.PremiumMon.Figure = signed(cur.premium)
		k.Say = fmt.Sprintf(sayMonth, cur.sold, signedText(cur.premium))
	}
	return s
}

// fillSeries sets the six points and the two series texts of k, oldest month first.
func fillSeries(k *card, agg map[string]*month, d time.Time) {
	var premiumParts, closedParts []string
	holes := 0
	for i := months - 1; i >= 0; i-- {
		key := monthKey(d, -i)
		m := agg[key]
		label := key[5:]
		label = strings.TrimPrefix(label, "0") + "月"
		pt := point{Month: key, Label: label}
		if !m.seen {
			holes++
			pt.Premium, pt.Closed = noData(textHole), noData(textHole)
		} else {
			pt.Premium = signed(m.premium)
			premiumParts = append(premiumParts, label+" "+signedText(m.premium))
			if m.hasClosed {
				pt.Closed = signed(m.closed)
				closedParts = append(closedParts, label+" "+signedText(m.closed))
			} else {
				pt.Closed = noData(textNoClosed)
			}
		}
		k.Series = append(k.Series, pt)
	}
	if len(premiumParts) > 0 {
		k.SeriesText = seriesText + "：" + strings.Join(premiumParts, "，")
	}
	if holes > 0 {
		k.SeriesText += fmt.Sprintf("；%d 个月无账本记录", holes)
	}
	if len(closedParts) > 0 {
		k.ClosedText = "近六个月已平仓盈亏：" + strings.Join(closedParts, "，")
	} else {
		k.ClosedText = "近六个月账本没有已平仓记录"
	}

}

func noData(text string) gap { return gap{Kind: "gap", Reason: "no_data", Text: text} }

func midnight(t time.Time, zone *time.Location) string {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, zone).Format(timeLayout)
}

// roundHalfEven rounds r to places decimals as an integer count of 10^-places.
func roundHalfEven(r *big.Rat, places int) *big.Int {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	x := new(big.Rat).Mul(r, new(big.Rat).SetInt(scale))
	neg := x.Sign() < 0
	x.Abs(x)
	q, rem := new(big.Int).QuoRem(x.Num(), x.Denom(), new(big.Int))
	switch new(big.Int).Lsh(rem, 1).Cmp(x.Denom()) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	if neg {
		q.Neg(q)
	}
	return q
}

// twoPlaces prints r rounded to cents as decimal text.
func twoPlaces(r *big.Rat) json.Number {
	return json.Number(new(big.Rat).SetFrac(roundHalfEven(r, 2), big.NewInt(100)).FloatString(2))
}

// dollars prints the whole-dollar amount of |r| with thousands separators.
func dollars(r *big.Rat) string {
	n := new(big.Int).Abs(roundHalfEven(r, 0)).String()
	var parts []string
	for len(n) > 3 {
		parts = append([]string{n[len(n)-3:]}, parts...)
		n = n[:len(n)-3]
	}
	return strings.Join(append([]string{n}, parts...), ",")
}

func signedText(r *big.Rat) string {
	if roundHalfEven(r, 0).Sign() < 0 {
		return minus + "$" + dollars(r)
	}
	return "+$" + dollars(r)
}

func signed(r *big.Rat) figure {
	tone := "flat"
	switch roundHalfEven(r, 0).Sign() {
	case 1:
		tone = "up"
	case -1:
		tone = "down"
	}
	return figure{Kind: "value", Value: twoPlaces(r), Text: signedText(r), Unit: "usd_signed", Tone: tone}
}

func plain(r *big.Rat) figure {
	return figure{Kind: "value", Value: twoPlaces(r), Text: "$" + dollars(r), Unit: "usd"}
}
