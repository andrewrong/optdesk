package snapshot

import (
	"context"
	"errors"
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

var _ optionday.OpenDayReaderFunc = OpenConidDayReader

// conidSource maps ibkr_conids to instrument ids.
type conidSource interface {
	// InstrumentIDs returns instrument id by conid for the conids master knows; an unknown conid is absent.
	InstrumentIDs(ctx context.Context, conids []int64) (map[int64]int64, error)
}

// selectOptionConidSQL reads the view rd.option_conid_r1. Argument: conids.
const selectOptionConidSQL = `SELECT instrument_id, conid FROM rd.option_conid_r1 WHERE conid = ANY($1)`

// conidDB is the conidSource over rd.option_conid_r1.
type conidDB struct{ db database }

func (c *conidDB) InstrumentIDs(ctx context.Context, conids []int64) (map[int64]int64, error) {
	idOf := map[int64]int64{}
	err := c.db.Select(ctx, selectOptionConidSQL, []any{conids}, func(sc scanner) error {
		var id, conid int64
		if err := sc.Scan(&id, &conid); err != nil {
			return err
		}
		if prev, ok := idOf[conid]; ok && prev != id {
			return permanent(fmt.Errorf("option_conid: conid %d is the current conid of instruments %d and %d", conid, prev, id))
		}
		idOf[conid] = id
		return nil
	})
	if err != nil {
		if _, ok := errors.AsType[contracts.TypedError](err); ok {
			return nil, err
		}
		return nil, retryable(fmt.Errorf("option_conid: select: %w", err))
	}
	return idOf, nil
}

// conidDays is an optionday.DayReader whose Days speaks conids.
type conidDays struct {
	src   conidSource
	inner optionday.DayReader
}

func newConidDays(src conidSource, inner optionday.DayReader) optionday.DayReader {
	return &conidDays{src: src, inner: inner}
}

// OpenConidDayReader connects to the database named by dsn (role ot_ro_pages or ot_runner) and returns a reader
// whose Days takes ibkr_conids, not instrument ids, and returns them in Day.InstrumentID. It reads the instrument
// ids from the view rd.option_conid_r1 (migration 0055; without it every Days call fails with a Retryable error),
// calls the Days of OpenDayReader with them and rewrites each row's InstrumentID to the conid asked for.
// ChainDay and Close are those of OpenDayReader, so a row of ChainDay carries the instrument id: this reader
// gives Day.InstrumentID two meanings, one per method.
//
// Two instruments sharing one current conid make the whole Days call fail with a Permanent error naming the
// conid (the unique index of 0044 is on (instrument_id, scheme), not on the value). A conid absent from master
// returns no row and no error, which a page words as "no stored quote". Only the current identifier is read.
func OpenConidDayReader(ctx context.Context, dsn string) (optionday.DayReader, error) {
	db, err := openDB(ctx, "option_conid", dsn)
	if err != nil {
		return nil, err
	}
	return newConidDays(&conidDB{db: db}, &dayReader{db: db}), nil
}

// Days returns the rows of the given conids with session_day between from and to, both inclusive.
func (c *conidDays) Days(ctx context.Context, conids []int64, from, to contracts.Day) ([]optionday.Day, error) {
	if len(conids) == 0 {
		return []optionday.Day{}, nil
	}
	idOf, err := c.src.InstrumentIDs(ctx, conids)
	if err != nil {
		return nil, err
	}
	conidOf := make(map[int64][]int64, len(idOf))
	ids := make([]int64, 0, len(idOf))
	for _, conid := range conids {
		id, ok := idOf[conid]
		if !ok {
			continue
		}
		if _, seen := conidOf[id]; !seen {
			ids = append(ids, id)
		}
		conidOf[id] = append(conidOf[id], conid)
	}
	if len(ids) == 0 {
		return []optionday.Day{}, nil
	}
	rows, err := c.inner.Days(ctx, ids, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]optionday.Day, 0, len(rows))
	for _, r := range rows {
		for _, conid := range dedup(conidOf[r.InstrumentID]) {
			r.InstrumentID = conid
			out = append(out, r)
		}
	}
	return out, nil
}

func dedup(xs []int64) []int64 {
	seen := make(map[int64]bool, len(xs))
	out := xs[:0:0]
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// ChainDay is the ChainDay of the inner reader.
func (c *conidDays) ChainDay(ctx context.Context, underlyingID int64, day contracts.Day) ([]optionday.Day, error) {
	return c.inner.ChainDay(ctx, underlyingID, day)
}

// Close releases the connection.
func (c *conidDays) Close() error { return c.inner.Close() }
