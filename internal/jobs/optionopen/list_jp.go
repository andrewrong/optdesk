package optionopen

import (
	"context"
	_ "embed"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// The list job over its two other manifests (docs/design/option-open-gap-jp.md, Decisions 7 and 13): the JP list
// pass and the US zone slot. Both call newListPreopen; the job code is in list.go.

//go:embed job_list_preopen_jp.toml
var listManifestJP []byte

//go:embed job_list_zone.toml
var listManifestZone []byte

// listRules are the JP list rules: the JP names that have options, the stock code as class (225 for the index),
// the exchange of the chain definition and its multiplier. Nothing is guessed.
func (r jpRules) listRules() listRules {
	return listRules{
		Names: func(ctx context.Context, s Stored, day time.Time) ([]stock, []store.Position, error) {
			return r.stored(s).stocks(ctx, day)
		},
		Class:      func(st stock) string { return r.tradingClass(st.Symbol) },
		Exchange:   r.Chain.Exchange,
		Multiplier: r.Chain.Multiplier,
	}
}

// jpLister sends the index names as the index they are: the list job sends every name as a stock.
type jpLister struct {
	broker.ContractLister
	chains chainLister
	r      jpRules
}

func (l jpLister) index(u broker.Contract) broker.Contract {
	if l.r.isIndex(u.Symbol) {
		u.SecType, u.Exchange = "IND", l.r.Chain.Exchange
	}
	return u
}

func (l jpLister) ListChains(ctx context.Context, u broker.Contract) ([]broker.Chain, error) {
	return l.chains.ListChains(ctx, l.index(u))
}

func (l jpLister) ListContracts(ctx context.Context, u broker.Contract, expiry contracts.Day, right, class string) ([]broker.Contract, error) {
	return l.ContractLister.ListContracts(ctx, l.index(u), expiry, right, class)
}

// NewListPreopenJP builds option_list_preopen_jp from the embedded job_list_preopen_jp.toml: the list job over
// the JP names, DTE 14 to 90, from the session open, stored in master.option_list.jp.
func NewListPreopenJP(d ListDeps) (contracts.Job, error) {
	r, err := parseJPRules(planManifestJP)
	if err != nil {
		return nil, err
	}
	cl, ok := d.Lister.(chainLister)
	if !ok {
		return nil, errors.New("optionopen: the JP list needs a lister that also lists chains")
	}
	d.Lister = jpLister{d.Lister, cl, r}
	return newListPreopen(listManifestJP, r.listRules(), d)
}

// NewListZone builds option_list_zone from the embedded job_list_zone.toml: the US zone slot before the open.
func NewListZone(d ListDeps) (contracts.Job, error) {
	return newListPreopen(listManifestZone, usRules(), d)
}
