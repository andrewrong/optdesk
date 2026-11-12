package optionledger

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/flex-xml-import.md#4 import-log-store.

var _ ImportLog = (*Store)(nil)

// recentFetchesSQL reads the view only. $1 is the login, $2 the most records to return. The filter on the query
// form keeps the daily records of a flex.toml login that is named like the import login out of the list.
const recentFetchesSQL = `SELECT ` + fetchColumns + ` FROM rd.flex_fetch_r1 ` +
	`WHERE login = $1 AND query_id LIKE 'file:%' ORDER BY finished_at DESC, fetch_id DESC LIMIT $2`

// RecentFetches returns the import records of login, newest first, at most limit of them (see ImportLog). A
// limit under 1 or a login with no record gives an empty slice.
func (s *Store) RecentFetches(ctx context.Context, login string, limit int) ([]store.FlexFetchRecord, error) {
	out := []store.FlexFetchRecord{}
	if limit < 1 {
		return out, nil
	}
	err := s.db.Select(ctx, recentFetchesSQL, []any{login, limit}, func(sc scanner) error {
		var rec store.FlexFetchRecord
		var first, last *time.Time
		var id string
		if err := sc.Scan(&rec.Login, &rec.QueryID, &first, &last, &rec.StartedAt, &rec.FinishedAt, &rec.Status,
			&rec.ErrorKind, &rec.ErrorCode, &rec.RawSHA256, &rec.ParserVersion, &rec.Statements,
			&rec.UnmappedStatement, &rec.RowsParsed, &rec.RowsNew, &rec.RowsRevised, &rec.RowsUnchanged, &id); err != nil {
			return err
		}
		rec.FirstDay, rec.LastDay = dayText(first), dayText(last)
		out = append(out, rec)
		return nil
	})
	if err != nil {
		return nil, wrap("read recent fetches", err)
	}
	return out, nil
}
