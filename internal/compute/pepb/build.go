// Package pepb forms the PE and the PB of a company for every stored day from the facts filed
// on or before that day (docs/design/company-pepb.md, Decisions 1 to 6, 10 and 11). It reads
// nothing and keeps no clock: the facts, the bars and the page day come in.
package pepb

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/pepbapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// New returns the BuildFunc over the given Stats.
func New(stats compute.Stats) pepbapi.BuildFunc {
	return func(in pepbapi.Input) pepbapi.Result { return build(stats, in) }
}

// dayRes is the ratio of one kind on one day, or the reason there is none.
type dayRes struct {
	reason     string
	ratio, num decimal.Decimal
	basisEnd   time.Time
	basisFiled time.Time
	shares     decimal.Decimal
	sharesDay  time.Time
}

type snap struct {
	ttm ttmRes
	eq  eqRes
	sh  shRes
}

type bar struct {
	d     time.Time
	close decimal.Decimal
}

// goodBars keeps the bars with a positive close on or before today, oldest first.
func goodBars(bars []contracts.PriceBar, today time.Time) []bar {
	var out []bar
	for _, b := range bars {
		c, err := decimal.NewFromString(b.Close)
		if err != nil || !c.IsPositive() || b.SessionDay.After(today) {
			continue
		}
		out = append(out, bar{d: b.SessionDay.Time, close: c})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].d.Before(out[j].d) })
	return out
}

func build(stats compute.Stats, in pepbapi.Input) pepbapi.Result {
	pe := pepbapi.Reading{Kind: "pe"}
	pb := pepbapi.Reading{Kind: "pb"}
	todayT, err := time.Parse("2006-01-02", string(in.Today))
	bars := goodBars(in.Bars, todayT)
	if err != nil || len(bars) == 0 {
		return pepbapi.Result{PE: missing(pe, "no_price"), PB: missing(pb, "no_price")}
	}
	last := bars[len(bars)-1]
	pe.Day, pb.Day = contracts.Day(fmtDay(last.d)), contracts.Day(fmtDay(last.d))
	if daysBetween(last.d, todayT) > pepbapi.StalePriceDays {
		return pepbapi.Result{PE: missing(pe, "price_stale"), PB: missing(pb, "price_stale")}
	}

	fs := parseFacts(in.Facts, todayT)
	cut, jumped := jumpCut(fs)
	cache := map[int]*snap{}
	snapAt := func(d time.Time) *snap {
		n := sort.Search(len(fs), func(i int) bool { return fs[i].filed.After(d) })
		if s, ok := cache[n]; ok {
			return s
		}
		s := &snap{ttm: ttm(fs[:n]), eq: equity(fs[:n]), sh: shares(fs[:n])}
		cache[n] = s
		return s
	}

	var peDays, pbDays []day
	var peToday, pbToday dayRes
	for i, b := range bars {
		s := snapAt(b.d)
		common := dayRes{}
		switch {
		case jumped && b.d.Before(cut):
			common.reason = "share_count_jump"
		case !s.sh.ok:
			common.reason = "no_shares"
		case s.sh.mismatch:
			common.reason = "share_count_mismatch"
		}
		common.shares, common.sharesDay = s.sh.count, s.sh.filed
		pr := common
		if pr.reason == "" {
			pr = ratioOf(common, b.close, s.ttm.reason, s.ttm.val, s.ttm.end, s.ttm.filed)
		}
		br := common
		if br.reason == "" {
			br = ratioOf(common, b.close, s.eq.reason, s.eq.val, s.eq.end, s.eq.filed)
		}
		if pr.reason == "" {
			peDays = append(peDays, day{b.d, pr.ratio})
		}
		if br.reason == "" {
			pbDays = append(pbDays, day{b.d, br.ratio})
		}
		if i == len(bars)-1 {
			peToday, pbToday = pr, br
		}
	}
	return pepbapi.Result{
		PE: finish(stats, pe, peToday, peDays),
		PB: finish(stats, pb, pbToday, pbDays),
	}
}

// ratioOf is close times shares over num, or the reason the input has none.
func ratioOf(c dayRes, close decimal.Decimal, reason string, num decimal.Decimal, end, filed time.Time) dayRes {
	if reason != "" {
		c.reason = reason
		return c
	}
	c.num, c.basisEnd, c.basisFiled = num, end, filed
	c.ratio = close.Mul(c.shares).Div(num)
	return c
}

func missing(r pepbapi.Reading, reason string) pepbapi.Reading {
	r.State, r.Reason = stateOf(reason), reason
	r.Text = reasonWords(upper(r.Kind), reason)
	return r
}

func finish(stats compute.Stats, r pepbapi.Reading, today dayRes, days []day) pepbapi.Reading {
	r.Shares, r.SharesFiled = today.shares.Round(0).String(), contracts.Day(fmtDay(today.sharesDay))
	if today.sharesDay.IsZero() {
		r.Shares, r.SharesFiled = "", ""
	}
	if today.reason != "" {
		return missing(r, today.reason)
	}
	r.State = pepbapi.StateOK
	r.Ratio = today.ratio.StringFixed(1)
	r.BasisEnd, r.BasisFiled = contracts.Day(fmtDay(today.basisEnd)), contracts.Day(fmtDay(today.basisFiled))
	fill(&r, stats, days, today.num, today.shares)
	r.Text = sentence(r)
	return r
}

func sentence(r pepbapi.Reading) string {
	kind := upper(r.Kind)
	basis := "TTM to"
	if r.Kind == "pb" {
		basis = "equity at"
	}
	pct := fmt.Sprintf("no percentile: %d days since %s", r.WindowDays, r.WindowFrom)
	if r.PercentileReason == "" {
		n, _ := strconv.Atoi(r.Percentile)
		pct = fmt.Sprintf("%s percentile of %d stored days since %s", ordinal(n), r.WindowDays, r.WindowFrom)
	}
	return fmt.Sprintf("%s %s on %s (%s %s, filed %s), %s; on %s shares filed %s",
		kind, r.Ratio, r.Day, basis, r.BasisEnd, r.BasisFiled, pct, shortCount(r.Shares), r.SharesFiled)
}

// shortCount prints a whole share count with thousands separators, in millions from 1,000,000.
func shortCount(s string) string {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return s
	}
	suffix := ""
	if d.GreaterThanOrEqual(decimal.NewFromInt(1_000_000)) {
		d, suffix = d.Div(decimal.NewFromInt(1_000_000)).Round(0), "M"
	}
	t := d.String()
	var parts []string
	for len(t) > 3 {
		parts = append([]string{t[len(t)-3:]}, parts...)
		t = t[:len(t)-3]
	}
	parts = append([]string{t}, parts...)
	return strings.Join(parts, ",") + suffix
}
