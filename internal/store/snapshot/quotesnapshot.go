package snapshot

import (
	"context"
	"fmt"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenQuoteSnapshotFunc = OpenQuoteSnapshot

// insertQuoteSnapshotSQL appends one row; a retry of the same capture, slot and instrument is a conflict on the
// primary key and adds no row. Arguments, in order: session_day, capture_id, slot, instrument_id, received_at,
// generation, market_data_type, event_ts, status, error_code, currency, bid, ask, bid_size, ask_size, last,
// last_size, volume.
const insertQuoteSnapshotSQL = `INSERT INTO obs.quote_snapshot (session_day, capture_id, slot, instrument_id, ` +
	`received_at, generation, market_data_type, event_ts, status, error_code, currency, bid, ask, bid_size, ` +
	`ask_size, last, last_size, volume) ` +
	`VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18) ` +
	`ON CONFLICT DO NOTHING`

// selectQuoteSnapshotSQL reads the view of the table. Arguments: instrument_id, first day, last day.
const selectQuoteSnapshotSQL = `SELECT session_day, capture_id::text, slot, instrument_id, received_at, generation, ` +
	`market_data_type, event_ts, status, error_code, currency, bid, ask, bid_size, ask_size, last, last_size, volume ` +
	`FROM rd.quote_snapshot_r1 ` +
	`WHERE instrument_id = $1 AND session_day >= $2 AND session_day <= $3 ` +
	`ORDER BY session_day, slot, capture_id`

type quoteSnapshotStore struct{ db database }

// OpenQuoteSnapshot connects to the database named by dsn and returns the quote snapshot store.
// It applies no migration: the schema is made by goose with the role ot_owner.
func OpenQuoteSnapshot(ctx context.Context, dsn string) (store.QuoteSnapshotStore, error) {
	db, err := openDB(ctx, "quote_snapshot", dsn)
	if err != nil {
		return nil, err
	}
	return &quoteSnapshotStore{db: db}, nil
}

// InsertQuoteSnapshots appends the rows in one transaction; a row whose key (session_day, capture_id, slot,
// instrument_id) exists is left as it is. A -1, NaN or infinity in any numeric column is stored as NULL; the status
// of the row and the other columns are kept. An empty input makes no database call.
func (s *quoteSnapshotStore) InsertQuoteSnapshots(ctx context.Context, rows []contracts.QuoteSnapshot) error {
	if len(rows) == 0 {
		return nil
	}
	err := s.db.InTx(ctx, func(tx execer) error {
		for _, r := range rows {
			if err := tx.Exec(ctx, insertQuoteSnapshotSQL, quoteSnapshotArgs(r)...); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return retryable(fmt.Errorf("quote_snapshot: insert: %w", err))
	}
	return nil
}

// quoteSnapshotArgs lists the arguments of insertQuoteSnapshotSQL for r, every numeric column screened.
func quoteSnapshotArgs(r contracts.QuoteSnapshot) []any {
	return []any{
		r.SessionDay.Time, r.CaptureID, r.Slot, r.InstrumentID, r.ReceivedAt, r.Generation, r.MarketDataType,
		r.EventTs, r.Status, r.ErrorCode, r.Currency,
		screenSentinel(r.Bid), screenSentinel(r.Ask), screenSentinel(r.BidSize), screenSentinel(r.AskSize),
		screenSentinel(r.Last), screenSentinel(r.LastSize), screenSentinel(r.Volume),
	}
}

// QuoteSnapshots returns the rows of instrumentID with session_day between from and to, both inclusive, ordered by
// session_day, slot and capture_id. No row in the range gives an empty slice and no error. Instants are returned in UTC.
func (s *quoteSnapshotStore) QuoteSnapshots(ctx context.Context, instrumentID int64, from, to contracts.Day) ([]contracts.QuoteSnapshot, error) {
	f, err := parseDay(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDay(to)
	if err != nil {
		return nil, err
	}
	out := []contracts.QuoteSnapshot{}
	err = s.db.Select(ctx, selectQuoteSnapshotSQL, []any{instrumentID, f, t}, func(sc scanner) error {
		var r contracts.QuoteSnapshot
		var d time.Time
		if err := sc.Scan(&d, &r.CaptureID, &r.Slot, &r.InstrumentID, &r.ReceivedAt, &r.Generation,
			&r.MarketDataType, &r.EventTs, &r.Status, &r.ErrorCode, &r.Currency, &r.Bid, &r.Ask, &r.BidSize,
			&r.AskSize, &r.Last, &r.LastSize, &r.Volume); err != nil {
			return err
		}
		r.SessionDay = types.SerializableDate{Time: d}
		r.ReceivedAt = r.ReceivedAt.UTC()
		r.EventTs = utc(r.EventTs)
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("quote_snapshot: select: %w", err))
	}
	return out, nil
}

// Close releases the connections.
func (s *quoteSnapshotStore) Close() error { return s.db.Close() }
