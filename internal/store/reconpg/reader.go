package reconpg

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	reconstore "github.com/andrewrong/optdesk/internal/store/recon"
)

// OpenReader connects to the database named by dsn, the line of the page role ot_ro_pages. It reads the view
// rd.recon_latest_r1 and nothing else.
func OpenReader(ctx context.Context, dsn string) (reconstore.Reader, error) {
	s, err := open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return s, nil
}

var _ reconstore.OpenReaderFunc = OpenReader

// latestTimeout bounds the query.
const latestTimeout = 10 * time.Second

// latestSQL reads the view in run order; the items of a run come in item_rank order, and a run with no item has one
// row with null item columns. The cap on items is the first parameter.
const latestSQL = `SELECT run_id, pair_id, session_day::text, started_at, finished_at, status, reason, n, matched, diff,
 missing_ours, missing_other, skipped, pairs_asked, pairs_compared, tolerance_text, source_id,
 item_key, item_company, item_ours, item_other, item_diff, item_kind, item_reason
FROM rd.recon_latest_r1
WHERE item_rank IS NULL OR item_rank <= $1
ORDER BY pair_id, session_day, run_id, item_rank`

// Latest returns the newest finished run per pair and day, each with at most reconstore.LatestItemsMax items in the
// severity order diff, missing_ours, missing_other, skipped, then company and key. Runs come ordered by pair and
// day. The slices are non-nil.
func (s *store) Latest(ctx context.Context) ([]reconstore.Run, error) {
	ctx, cancel := context.WithTimeout(ctx, latestTimeout)
	defer cancel()
	out := []reconstore.Run{}
	var lastID int64 = -1
	err := s.db.Select(ctx, latestSQL, []any{reconstore.LatestItemsMax}, func(sc scanner) error {
		var (
			id                                         int64
			r                                          reconstore.Run
			day                                        string
			key, company, ours, other, diff, kind, why *string
		)
		if err := sc.Scan(&id, &r.PairID, &day, &r.StartedAt, &r.FinishedAt, &r.Status, &r.Reason, &r.N, &r.Matched,
			&r.Diff, &r.MissingOurs, &r.MissingOther, &r.Skipped, &r.PairsAsked, &r.PairsCompared, &r.ToleranceText,
			&r.SourceID, &key, &company, &ours, &other, &diff, &kind, &why); err != nil {
			return err
		}
		if id != lastID {
			r.SessionDay = contracts.Day(day)
			r.Items = []reconstore.Item{}
			out = append(out, r)
			lastID = id
		}
		if key != nil && kind != nil {
			cur := &out[len(out)-1]
			cur.Items = append(cur.Items, reconstore.Item{Key: *key, Company: deref(company), Ours: deref(ours),
				Other: deref(other), Diff: deref(diff), Kind: *kind, Reason: deref(why)})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reconpg: latest: %w", err)
	}
	return out, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
