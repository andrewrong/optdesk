package positions_page

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/shared"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/spot-from-bars.md#Decisions (Decisions 2, 3, 5, 6, 7).

const (
	barBasis        = "trades"
	barLookbackDays = 14  // calendar days of bars read up to the capture day
	stockIDBatch    = 500 // names of one StockIDs call
	noteSymbols     = 8   // symbols named by the fallback note
	noteMaxRunes    = 200 // schema limit of one note
)

// StockIDReader reads the instruments of type stock whose name is in names; a name returned once is one
// instrument, a name returned twice is shared and gives no price.
type StockIDReader interface {
	StockIDs(ctx context.Context, names []string) ([]store.InstrumentName, error)
}

// fallback is a stock priced from the close of a stored bar.
type fallback struct{ Symbol, Day string }

// barFigures is the price and previous close per con id after the bars were read: Price is the marks plus
// the bar closes of the unmarked stocks, Prev the previous close of the stocks that have one.
type barFigures struct {
	Price, Prev map[int64]string
	Fallbacks   []fallback
	Rows        []contracts.PriceBar // every bar row read, for shared.BarSourceOf
}

// barPrices completes the marks of the capture on capDay from the daily bars: a stock with no mark takes the
// close of its newest bar, a stock with a mark keeps it, and the previous close is the bar before the day of
// the price (before capDay for a mark). A bar of another currency, one bar only, a symbol shared by two
// instruments or an unknown symbol leaves the gap. Without Bars or StockIDs the marks come back as they are.
// One StockIDs call for all stock symbols, one PriceBars call per resolved row.
func (c Config) barPrices(ctx context.Context, ps []store.Position, marks map[int64]string, capDay string) (barFigures, error) {
	out := barFigures{Price: marks, Prev: map[int64]string{}}
	if c.Bars == nil || c.StockIDs == nil {
		return out, nil
	}
	capD, err := time.Parse(dayLayout, capDay)
	if err != nil {
		return out, fmt.Errorf("positions_page: bad capture day %q: %w", capDay, err)
	}
	names := stockNames(ps)
	if len(names) == 0 {
		return out, nil
	}
	found, err := c.StockIDs.StockIDs(ctx, names)
	if err != nil {
		return out, fmt.Errorf("positions_page: read stock ids: %w", err)
	}
	ids := map[string][]int64{}
	for _, f := range found {
		ids[f.Name] = append(ids[f.Name], f.InstrumentID)
	}
	price := make(map[int64]string, len(marks))
	for k, v := range marks {
		price[k] = v
	}
	out.Price = price
	from := contracts.Day(capD.AddDate(0, 0, -barLookbackDays).Format(dayLayout))
	for _, p := range ps {
		if p.SecType != typeStock || len(ids[p.Symbol]) != 1 {
			continue
		}
		bars, err := c.Bars.PriceBars(ctx, ids[p.Symbol][0], barBasis, from, contracts.Day(capDay))
		if err != nil {
			return out, fmt.Errorf("positions_page: read price bars of %s: %w", p.Symbol, err)
		}
		out.Rows = append(out.Rows, bars...)
		cs := dayCloses(bars, capDay)
		if _, marked := price[p.ConID]; marked {
			if v, ok := prevOfMark(cs, capDay, p.Currency); ok {
				out.Prev[p.ConID] = v
			}
			continue
		}
		last, prev, ok := barPrice(cs, capDay, p.Currency)
		if !ok {
			continue
		}
		price[p.ConID] = last.close
		out.Fallbacks = append(out.Fallbacks, fallback{Symbol: p.Symbol, Day: last.day})
		if prev != "" {
			out.Prev[p.ConID] = prev
		}
	}
	return out, nil
}

// stockNames are the distinct stock symbols of ps, at most stockIDBatch.
func stockNames(ps []store.Position) []string {
	var names []string
	for _, p := range ps {
		if p.SecType == typeStock && p.Symbol != "" && !slices.Contains(names, p.Symbol) && len(names) < stockIDBatch {
			names = append(names, p.Symbol)
		}
	}
	return names
}

type dayClose struct{ day, close, currency string }

// dayCloses are the usable bars (a close, a day up to capDay), ascending.
func dayCloses(bars []contracts.PriceBar, capDay string) []dayClose {
	var cs []dayClose
	for _, b := range bars {
		if d := b.SessionDay.Format(dayLayout); b.Close != "" && d <= capDay {
			cs = append(cs, dayClose{d, b.Close, b.Currency})
		}
	}
	return cs
}

// prevOfMark is the close of the newest bar before capDay, when it is in currency.
func prevOfMark(cs []dayClose, capDay, currency string) (string, bool) {
	if i := slices.IndexFunc(cs, func(c dayClose) bool { return c.day >= capDay }); i >= 0 {
		cs = cs[:i]
	}
	if n := len(cs); n > 0 && cs[n-1].currency == currency {
		return cs[n-1].close, true
	}
	return "", false
}

// barPrice is the bar shared.Pick takes from cs (no prints, the look-back of the bars read) as the price,
// when it is in currency, and the close of the bar before it ("" when there is none or it is in another
// currency).
func barPrice(cs []dayClose, capDay, currency string) (last dayClose, prev string, ok bool) {
	var src shared.Source
	for _, c := range cs {
		src.Bars = append(src.Bars, shared.Bar{Day: contracts.Day(c.day), Close: c.close})
	}
	picked, found := shared.Pick(src, contracts.Day(capDay))
	if !found {
		return dayClose{}, "", false
	}
	i := slices.IndexFunc(cs, func(c dayClose) bool { return c.day == string(picked.Day) && c.close == picked.Price })
	if i < 0 || cs[i].currency != currency {
		return dayClose{}, "", false
	}
	if i > 0 && cs[i-1].currency == currency {
		prev = cs[i-1].close
	}
	return cs[i], prev, true
}

// fallbackNote is the one line of the account that names the bar-priced symbols, at most noteSymbols and a
// count of the rest, with their bar days; "" when there is none. It fits noteMaxRunes by naming fewer symbols.
func fallbackNote(fb []fallback) string {
	if len(fb) == 0 {
		return ""
	}
	n := min(noteSymbols, len(fb))
	for {
		var syms, days []string
		for _, f := range fb[:n] {
			syms = append(syms, f.Symbol)
			if !slices.Contains(days, f.Day) {
				days = append(days, f.Day)
			}
		}
		slices.Sort(days)
		s := textBarStart + strings.Join(syms, ", ")
		if rest := len(fb) - n; rest > 0 {
			s += " and " + strconv.Itoa(rest) + " more"
		}
		s += textBarMid + strings.Join(days, ", ") + textBarEnd
		if len([]rune(s)) <= noteMaxRunes || n == 1 {
			return s
		}
		n--
	}
}
