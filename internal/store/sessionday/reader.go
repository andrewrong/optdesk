package sessionday

import (
	"context"
	"fmt"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const selectSQL = `SELECT calendar_id, session_day, is_session, open_at, close_at, early_close ` +
	`FROM rd.session_day_r1 ` +
	`WHERE calendar_id = $1 AND session_day >= $2 AND session_day <= $3 ` +
	`ORDER BY session_day`

// SessionDays returns the rows of calendarID with session_day between from and to, both inclusive, ordered by session_day.
// A calendar with no rows in the range gives an empty slice and no error. Instants are returned in UTC.
func (s *sessionDayStore) SessionDays(ctx context.Context, calendarID string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	f, err := parseDay(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDay(to)
	if err != nil {
		return nil, err
	}
	out := []contracts.SessionDay{}
	err = s.db.Select(ctx, selectSQL, []any{calendarID, f, t}, func(sc scanner) error {
		var r contracts.SessionDay
		var day types.SerializableDate
		if err := sc.Scan(&r.CalendarID, &day.Time, &r.IsSession, &r.OpenAt, &r.CloseAt, &r.EarlyClose); err != nil {
			return err
		}
		r.SessionDay = day
		r.OpenAt, r.CloseAt = r.OpenAt.UTC(), r.CloseAt.UTC()
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("sessionday: select: %w", err))
	}
	return out, nil
}
