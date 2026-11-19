package optioneod

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// storedContracts answers the con_id of a pair from the contracts the evening resolve stored for one stock.
type storedContracts struct {
	conIDs map[string]int64
}

// loadStored lists the stored options of the stock undID once and reads the ibkr_conid of their instruments.
func loadStored(ctx context.Context, c ContractReader, in store.InstrumentReader, undID int64) (*storedContracts, error) {
	if in == nil {
		return nil, errors.New("no instrument reader")
	}
	refs, err := c.ListOptions(ctx, undID)
	if err != nil {
		return nil, err
	}
	s := &storedContracts{conIDs: map[string]int64{}}
	if len(refs) == 0 {
		return s, nil
	}
	ids := make([]int64, len(refs))
	for i, r := range refs {
		ids[i] = r.InstrumentID
	}
	insts, err := in.Instruments(ctx, ids)
	if err != nil {
		return nil, err
	}
	conID := map[int64]int64{}
	for _, inst := range insts {
		for _, idf := range inst.Identifiers {
			if n, err := strconv.ParseInt(idf.Value, 10, 64); err == nil && idf.Scheme == "ibkr_conid" && n > 0 {
				conID[inst.ID] = n
			}
		}
	}
	for _, r := range refs {
		if n, ok := conID[r.InstrumentID]; ok {
			s.conIDs[storedKey(r.TradingClass, strings.ReplaceAll(string(r.Expiry), "-", ""), r.Strike, r.Right, r.Multiplier)] = n
		}
	}
	return s, nil
}

func storedKey(class, expiry, strike, right, multiplier string) string {
	return class + "|" + pairKey(expiry, strike, right) + "|" + strings.TrimSpace(multiplier)
}

// conID is the con_id of the stored reference with the pair's trading class, expiry day, strike value, right and
// multiplier.
func (s *storedContracts) conID(p broker.Contract) (int64, bool) {
	n, ok := s.conIDs[storedKey(p.TradingClass, p.Expiry, p.Strike, p.Right, p.Multiplier)]
	return n, ok
}

// listed resolves the pairs and drops those not listed. With a stored-contract reader a pair the evening resolve
// stored is not asked: it becomes a contract with the stored con_id and the multiplier of the pair, and is not
// checked for listing. A read error falls back to resolving every pair. The order of the pairs is kept.
func (r *runState) listed(ctx context.Context, u underlying, undID int64, pairs []broker.Contract) ([]broker.Contract, contracts.TypedError) {
	if r.j.stored == nil {
		return resolvePairs(ctx, r.j.b.ResolveContract, pairs, r.checkBudget)
	}
	st, err := loadStored(ctx, r.j.stored, r.j.insts, undID)
	if err != nil {
		r.j.log().Warn("optioneod: stored contracts not read", "job", r.j.m.ID, "symbol", u.Symbol, "err", err.Error())
		st = &storedContracts{}
	}
	start := r.j.clk.Now()
	var missing []broker.Contract
	have := make([]int64, len(pairs))
	for i, p := range pairs {
		if n, ok := st.conID(p); ok {
			have[i] = n
		} else {
			missing = append(missing, p)
		}
	}
	resolved, terr := resolvePairs(ctx, r.j.b.ResolveContract, missing, r.checkBudget)
	if terr != nil {
		return nil, terr
	}
	// resolved is the listed subsequence of missing, in order, so the next one belongs to the next listed missing pair
	// whose own key it carries; interleave by walking the pairs.
	out := make([]broker.Contract, 0, len(pairs))
	next := 0
	mi := 0
	for i, p := range pairs {
		if have[i] != 0 {
			p.ConID = have[i]
			out = append(out, p)
			continue
		}
		m := missing[mi]
		mi++
		if next < len(resolved) && sameContract(resolved[next], m) {
			out = append(out, resolved[next])
			next++
		}
	}
	stored := len(pairs) - len(missing)
	r.j.log().Info("optioneod: contracts", "stored", stored, "resolved", len(missing), "not_listed", len(missing)-len(resolved),
		"seconds", r.j.clk.Now().Sub(start).Seconds())
	return out, nil
}

// sameContract is true when a resolved contract is the answer for pair p: same expiry day, strike value and right.
func sameContract(c, p broker.Contract) bool {
	return pairKey(c.Expiry, c.Strike, c.Right) == pairKey(p.Expiry, p.Strike, p.Right)
}

func (j *job) log() *slog.Logger {
	if j.logger != nil {
		return j.logger
	}
	return slog.Default()
}
