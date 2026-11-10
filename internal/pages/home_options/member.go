// Package home_options builds the options member of the home page: a page.options_summary.v4
// document read from the replayed option ledger (store.OptionLedgerReader replayed by a
// compute.OptionLedger, as option_tracking_page does). The builder computes no figure: the sums
// come from a Figures, as decimal text or a gap, and the builder copies that text, adding only
// the sign, the dollar sign and the thousands separators. It reads no clock and makes no request.
// A month with no ledger row is a hole (a no_data gap), never zero, and the closed series text
// is built only from ledger rows.
package home_options

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
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

	title         = "期权概览"
	subtitle      = "来自成交账本"
	sayNone       = "期权账本暂无记录。"
	sayMonth      = "本月卖出 %d 组，收取权利金 %s。"
	textNoRows    = "账本暂无记录"
	textNoReader  = "期权账本暂未接入"
	textHole      = "该月无账本记录"
	textBadRow    = "账本行缺少数量、价格或乘数"
	sayNoPremium  = "本月卖出 %d 组，权利金暂无数据。"
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

// Figures computes the figures of the member from the replayed positions. Every number is exact
// decimal text and a gap carries its reason; the implementation lives in compute, the builder only
// places what it returns (Decisions 1, 3 and 4 of pages-real-data.md).
//
// The implementation sums the USD positions only (OtherCurrency says one was left out), counts a
// month as Seen when a ledger row of it (an opening or a closing fill) is dated in the month and
// not after AsOf, and gives a figure that needs a missing quantity, price or multiplier as a gap,
// never as zero. Premium is the cash of the opening rows of the month, sold positive, rounded
// half-even to two places; Closed is the realized result of the positions closed in the month,
// two places, a gap when none closed; Sold is the count of selling opening rows as integer text;
// OpenPremium is the cash of the still open sold rows of every month, two places.
type Figures interface {
	Options(FigureIn) FigureOut
}

// FigureIn is the input of Figures.Options; Months are YYYY-MM keys, oldest first.
type FigureIn struct {
	Positions []compute.LedgerPosition
	AsOf      contracts.Day
	Months    []string
}

// FigureOut is the result of Figures.Options: one MonthFigure per month asked, in order.
// Newest is the newest ledger day not after AsOf, empty with no ledger row.
type FigureOut struct {
	Months        []MonthFigure
	OpenPremium   compute.FigureValue
	Newest        contracts.Day
	OtherCurrency bool
}

// MonthFigure is the figures of one calendar month.
type MonthFigure struct {
	Month                 string
	Seen                  bool
	Sold, Premium, Closed compute.FigureValue
}

// Config carries the identity of the run, the owner's zone, the ledger reader, the replay and the
// figures. A nil Ledger gives every figure as a no_data gap. A nil Replay with stored rows, a nil
// Figures with replayed positions and a nil Zone are errors of Build.
type Config struct {
	RunID   string
	BuildID string
	Scope   string
	Zone    *time.Location
	Ledger  store.OptionLedgerReader
	Replay  compute.OptionLedger
	Figures Figures
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

// Build returns the page.options_summary.v4 JSON for day; at is generated_at, written in
// c.Zone. The series are the six calendar months ending with the month of day. An error of
// c.Ledger is an error of Build.
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
	keys := monthKeys(d)
	var out FigureOut
	if len(positions) > 0 {
		if c.Figures == nil {
			return nil, errors.New("home_options: nil figures")
		}
		out = c.Figures.Options(FigureIn{Positions: positions, AsOf: day, Months: keys})
	}
	return json.Marshal(assemble(c, at, out, keys, hasReader))
}

// monthKeys are the six calendar months ending with the month of d, oldest first.
func monthKeys(d time.Time) []string {
	first := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
	keys := make([]string, 0, months)
	for i := months - 1; i >= 0; i-- {
		keys = append(keys, first.AddDate(0, -i, 0).Format(monthLayout))
	}
	return keys
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

// assemble builds the document from the figures out, asked for the months keys.
func assemble(c Config, at time.Time, out FigureOut, keys []string, hasReader bool) summary {
	noRows := out.Newest == ""
	noText := textNoRows
	if !hasReader {
		noText = textNoReader
	}
	base, baseText := at, textNoDay
	if !noRows {
		if t, err := time.Parse(dayLayout, string(out.Newest)); err == nil {
			base, baseText = t, "账本截至 "+t.Format("01-02")
		}
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
	if !noRows {
		s.DataTimes = append(s.DataTimes, sourceTime{Source: "option_ledger", DataTime: midnight(base, c.Zone)})
	}
	k := &s.Card
	k.Title, k.Subtitle, k.SeriesText, k.SeriesCaption = title, subtitle, seriesText, seriesCaption
	k.Stale = stale{DataTime: midnight(base, c.Zone), Text: baseText, IsStale: at.After(staleAfter)}
	k.Notes = []string{noteCash, noteCount}
	k.Gaps = []gap{noData(textMaxLoss)}
	if noRows {
		k.Gaps = append([]gap{noData(noText)}, k.Gaps...)
	}
	if out.OtherCurrency {
		k.Gaps = append(k.Gaps, gap{Kind: "gap", Reason: "unsupported_currency", Text: textOtherCcy})
	}
	byMonth := map[string]MonthFigure{}
	for _, m := range out.Months {
		byMonth[m.Month] = m
	}
	fillSeries(k, keys, byMonth)
	fillTiles(k, byMonth[keys[len(keys)-1]], out.OpenPremium, noRows, noText)
	return s
}

// fillSeries sets the six points and the two series texts of k, oldest month first. A month that
// is not Seen is a hole; the texts list only the months that have a figure.
func fillSeries(k *card, keys []string, byMonth map[string]MonthFigure) {
	var premiumParts, closedParts []string
	holes := 0
	for _, key := range keys {
		m := byMonth[key]
		label := key + "月"
		if t, err := time.Parse(monthLayout, key); err == nil {
			label = strconv.Itoa(int(t.Month())) + "月"
		}
		pt := point{Month: key, Label: label, Premium: noData(textHole), Closed: noData(textHole)}
		if !m.Seen {
			holes++
			k.Series = append(k.Series, pt)
			continue
		}
		pt.Premium = money(m.Premium, true, textBadRow)
		pt.Closed = money(m.Closed, true, textNoClosed)
		if f, ok := pt.Premium.(figure); ok {
			premiumParts = append(premiumParts, label+" "+f.Text)
		}
		if f, ok := pt.Closed.(figure); ok {
			closedParts = append(closedParts, label+" "+f.Text)
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

// fillTiles sets the four tiles and the say of k from the figures of the current month cur.
func fillTiles(k *card, cur MonthFigure, open compute.FigureValue, noRows bool, noText string) {
	k.Tiles = tiles{
		Sold:        tile{Label: "本月卖出", Figure: noData(noText)},
		PremiumMon:  tile{Label: "本月权利金", Figure: noData(noText)},
		OpenPremium: tile{Label: "未平权利金", Figure: noData(noText)},
		MaxLoss:     tile{Label: "最大亏损", Figure: noData(textMaxLoss)},
	}
	k.Say = sayNone
	if !noRows {
		k.Tiles.OpenPremium.Figure = money(open, false, textBadRow)
	}
	if !cur.Seen {
		return
	}
	k.Tiles.Sold.Figure = noData(textBadRow)
	sold, soldOK := count(cur.Sold)
	if soldOK {
		k.Tiles.Sold = tile{Label: "本月卖出", Caption: "按开仓行计", Figure: map[string]any{
			"kind": "value", "value": sold, "text": strconv.Itoa(sold) + " 组", "unit": "structures"}}
	}
	k.Tiles.PremiumMon.Figure = money(cur.Premium, true, textBadRow)
	f, premiumOK := k.Tiles.PremiumMon.Figure.(figure)
	switch {
	case soldOK && premiumOK:
		k.Say = fmt.Sprintf(sayMonth, sold, f.Text)
	case soldOK:
		k.Say = fmt.Sprintf(sayNoPremium, sold)
	}
}

func noData(text string) gap { return gap{Kind: "gap", Reason: "no_data", Text: text} }

func midnight(t time.Time, zone *time.Location) string {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, zone).Format(timeLayout)
}

var (
	cents    = regexp.MustCompile(`^(-?)([0-9]+)\.([0-9]{2})$`)
	whole    = regexp.MustCompile(`^[0-9]{1,6}$`)
	gapNames = map[string]bool{"missing_quote": true, "no_data": true, "unsupported_currency": true}
)

// count reads an integer count from its decimal text.
func count(f compute.FigureValue) (int, bool) {
	if f.Gap != "" || !whole.MatchString(f.Value) {
		return 0, false
	}
	n, err := strconv.Atoi(f.Value)
	return n, err == nil
}

// money places the dollar amount f, which has two decimals, as a figure: only text is rearranged
// (sign, dollar sign, thousands separators), no number is computed. signed gives a usd_signed
// figure with its tone; an unsigned figure must not be negative. A gap keeps its reason when the
// schema has it and is no_data otherwise; text that is no two-decimal amount is a no_data gap
// with the text bad.
func money(f compute.FigureValue, signed bool, bad string) any {
	if f.Gap != "" {
		reason := f.Gap
		if !gapNames[reason] {
			reason = "no_data"
		}
		return gap{Kind: "gap", Reason: reason, Text: bad}
	}
	m := cents.FindStringSubmatch(f.Value)
	if m == nil {
		return noData(bad)
	}
	neg, digits, frac := m[1] == "-", strings.TrimLeft(m[2], "0"), m[3]
	if digits == "" {
		digits = "0"
	}
	zero := digits == "0" && frac == "00"
	if zero {
		neg = false
	}
	if neg && !signed {
		return noData(bad)
	}
	num := digits + "." + frac
	var groups []string
	for len(digits) > 3 {
		groups = append([]string{digits[len(digits)-3:]}, groups...)
		digits = digits[:len(digits)-3]
	}
	amount := "$" + strings.Join(append([]string{digits}, groups...), ",") + "." + frac
	if neg {
		num = "-" + num
	}
	if !signed {
		return figure{Kind: "value", Value: json.Number(num), Text: amount, Unit: "usd"}
	}
	tone, sign := "flat", "+"
	switch {
	case neg:
		tone, sign = "down", minus
	case !zero:
		tone = "up"
	}
	return figure{Kind: "value", Value: json.Number(num), Text: sign + amount, Unit: "usd_signed", Tone: tone}
}
