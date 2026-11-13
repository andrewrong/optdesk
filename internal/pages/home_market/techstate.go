package home_market

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	figTech     = "market.tech_state"
	techKind    = "market"
	techSubject = "NDX_SPX"

	techNoData  = "Tech state: no data, not stored yet"
	techStale   = "Tech state: data is stale"
	techMissing = "Tech state: no data, inputs incomplete"
)

// techNotes reads the newest market.tech_state reading within readingDays of d. With a state it puts the sentence
// and the numbers line before rest; otherwise it returns rest as it is and the one gap of the view's reason. A
// reader error is an error, never a gap.
func techNotes(ctx context.Context, c Config, d time.Time, rest []string) ([]string, []gap, error) {
	r, err := newestOf(ctx, c, figTech, techKind, techSubject, d)
	if err != nil {
		return nil, nil, err
	}
	v := FromReading(r, contracts.Day(d.Format(dayLayout)))
	if v.State != "" {
		return append([]string{v.Say, v.Numbers}, rest...), nil, nil
	}
	text := techMissing
	switch v.Gap {
	case "no_data":
		text = techNoData
	case "stale_input":
		text = techStale
	}
	return rest, []gap{mkGap(v.Gap, text)}, nil
}
