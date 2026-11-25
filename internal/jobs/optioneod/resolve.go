package optioneod

import (
	"context"
	"sync"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// resolveInFlight is the number of ResolveContract asks of a key in flight at once; it equals the gate of the broker
// client (ResolveInFlight of the broker config), a larger number would only queue there.
const resolveInFlight = 5

// resolvePairs resolves every pair with at most resolveInFlight asks in flight and returns the listed ones in pair order,
// whatever order the asks end in. checkBudget is read before each ask. Pairs are taken in order; after a failure no new ask
// starts, and the error returned is that of the first failing pair in pair order. A pair that is not listed is dropped and
// an empty multiplier is filled from the pair.
func resolvePairs(ctx context.Context, resolve func(context.Context, broker.Contract) (broker.Resolved, error),
	pairs []broker.Contract, checkBudget func() contracts.TypedError) ([]broker.Contract, contracts.TypedError) {
	var (
		mu      sync.Mutex
		next    int
		stopped bool
		errs    = make([]contracts.TypedError, len(pairs))
		got     = make([]broker.Resolved, len(pairs))
		wg      sync.WaitGroup
	)
	claim := func() (int, bool) {
		mu.Lock()
		defer mu.Unlock()
		if stopped || next >= len(pairs) {
			return 0, false
		}
		i := next
		next++
		if terr := checkBudget(); terr != nil {
			errs[i], stopped = terr, true
			return 0, false
		}
		return i, true
	}
	for w := 0; w < min(resolveInFlight, len(pairs)); w++ {
		wg.Go(func() {
			for {
				i, ok := claim()
				if !ok {
					return
				}
				r, err := resolve(ctx, pairs[i])
				mu.Lock()
				if err != nil {
					errs[i], stopped = asTyped(err, contracts.Permanent), true
				} else {
					got[i] = r
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return nil, e
		}
	}
	var out []broker.Contract
	for i, r := range got {
		if !r.Listed {
			continue
		}
		c := r.Contract
		if c.Multiplier == "" {
			c.Multiplier = pairs[i].Multiplier
		}
		out = append(out, c)
	}
	return out, nil
}
