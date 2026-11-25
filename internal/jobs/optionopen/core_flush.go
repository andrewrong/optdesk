package optionopen

import (
	"context"
	"time"
)

// flushLimit bounds the final write of a cut run, inside the grace the runner leaves after a cancel.
const flushLimit = 3 * time.Second

// flush passes the rows added since the last call to Flush. After a cancel only the final call writes, once, through
// a context that is not cancelled and ends after flushLimit. A Flush error is kept in st.err and ends the run.
func (c *Core) flush(ctx context.Context, st *runState, final bool) {
	if c.Flush == nil || st.err != nil || st.flushed == len(st.rows) {
		return
	}
	if ctx.Err() != nil {
		if !final {
			return
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), flushLimit)
		defer cancel()
	}
	if err := c.Flush(ctx, st.rows[st.flushed:]); err != nil {
		st.err = err
		return
	}
	st.flushed = len(st.rows)
}

// cutAll marks the answers of the calls that the cancel of the context cut: they get no row.
func cutAll(as []answer) {
	for i := range as {
		as[i] = answer{cut: true}
	}
}

// groupEnd is the state of the spot at the end of a group.
type groupEnd struct {
	skew  bool
	price float64
	ts    *time.Time
}

// addGroup makes the rows of the answered targets of a group. The row carries the stock's own price (the spot read
// for the group) whenever it is positive, cut or not; a cut group has skew false. Only when no stock price was read
// does the row keep the option quote's UndPrice, the IBKR option-model value. A target whose call was cut gets no
// row.
func (c *Core) addGroup(ctx context.Context, st *runState, g []Target, as []answer, requested time.Time, e groupEnd) {
	for i, t := range g {
		if as[i].cut {
			continue
		}
		r := c.row(t, as[i], requested)
		if e.price > 0 {
			price := e.price
			r.UnderlyingPrice, r.UnderlyingEventTs = &price, e.ts
			r.Skew = e.skew && ctx.Err() == nil
		} else {
			// No spot was read: the row keeps no underlying price, not even the option-model value.
			r.UnderlyingPrice, r.UnderlyingEventTs = nil, nil
		}
		c.logNotOk(t, r)
		c.add(st, t, r)
	}
}
