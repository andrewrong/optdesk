package computefigures

import (
	"errors"
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// isRv20Key is true for the key of one of the manifest's rv20 index names.
func (j *job) isRv20Key(name, kind, subj string) bool {
	if j.m.Rv20 == "" || name != j.m.Rv20 || kind != kindUnderlying {
		return false
	}
	for _, n := range j.m.Rv20Indices {
		if n == subj {
			return true
		}
	}
	return false
}

// runRv20 stores the 20-day realised volatility of each named index. The name is resolved to the id of the
// instrument of type index through IndexIDs; the closes are read and stored by runHv30 under the rule of
// market.rv20. A missing reader is a permanent failure of the key, an unknown name a retryable one (bars_refresh
// may create the instrument later in the day), an ambiguous name a permanent one. Nothing is stored for a failed key.
func (r *runState) runRv20(names []string) {
	if len(names) == 0 {
		return
	}
	rule := r.j.rule[r.j.m.Rv20]
	switch {
	case r.j.stores.PriceBars == nil:
		r.failEach(names, contracts.Permanent, errors.New("no price bar reader"))
		return
	case r.j.stores.IndexIDs == nil:
		r.failEach(names, contracts.Permanent, errors.New("no index reader"))
		return
	}
	rows, err := r.j.stores.IndexIDs.IndexIDs(r.ctx, names)
	if err != nil {
		r.failStore(fmt.Errorf("computefigures: index instruments: %w", err))
		return
	}
	byName := map[string][]int64{}
	for _, row := range rows {
		byName[row.Name] = append(byName[row.Name], row.InstrumentID)
	}
	var ts []target
	for _, n := range names {
		switch ids := byName[n]; len(ids) {
		case 0:
			r.fail(contracts.Retryable, fmt.Errorf("computefigures: no index instrument named %s", n))
		case 1:
			ts = append(ts, target{rule: rule, subj: subject{kind: kindUnderlying, name: n, id: ids[0]}})
		default:
			r.fail(contracts.Permanent, fmt.Errorf("computefigures: index instrument %s is ambiguous: %d rows", n, len(ids)))
		}
	}
	r.runHv30(ts)
}

// failEach fails the key of every name with the same cause.
func (r *runState) failEach(names []string, kind contracts.ErrorKind, cause error) {
	for _, n := range names {
		r.fail(kind, fmt.Errorf("computefigures: %s of %s: %w", r.j.m.Rv20, n, cause))
	}
}
