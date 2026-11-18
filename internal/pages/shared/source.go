package shared

import (
	"context"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

const barBasis = "trades"

// SourceOf is the candidates of one underlying for Pick on day: a print for every option_eod row
// of rows that has an underlying price (the session day as Day, the event time as At) and, when r
// is not nil, the trades bars of the instrument id of the LookbackDays days up to day. A nil r
// gives no bars. A reader error is returned as it came.
func SourceOf(ctx context.Context, r bar.PriceBarReader, id int64, rows []contracts.OptionEod, day contracts.Day) (Source, error) {
	var s Source
	for _, row := range rows {
		if row.UnderlyingPrice == nil {
			continue
		}
		s.Prints = append(s.Prints, Print{
			Day:   contracts.Day(row.SessionDay.Format(time.DateOnly)),
			Price: strconv.FormatFloat(*row.UnderlyingPrice, 'f', -1, 64),
			At:    row.UnderlyingEventTs,
		})
	}
	if r == nil {
		return s, nil
	}
	end, err := time.Parse(time.DateOnly, string(day))
	if err != nil {
		return s, err
	}
	from := contracts.Day(end.AddDate(0, 0, -LookbackDays).Format(time.DateOnly))
	got, err := r.PriceBars(ctx, id, barBasis, from, day)
	if err != nil {
		return s, err
	}
	for _, b := range got {
		s.Bars = append(s.Bars, Bar{Day: contracts.Day(b.SessionDay.Format(time.DateOnly)), Close: b.Close})
	}
	return s, nil
}
