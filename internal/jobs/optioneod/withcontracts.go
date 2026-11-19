package optioneod

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// ContractReader is the read side of openquote.OptionContractStore: the contracts the evening resolve stored.
type ContractReader interface {
	// ListOptions returns the stored contracts of the underlying; an underlying with none gives an empty slice.
	ListOptions(ctx context.Context, underlyingID int64) ([]openquote.OptionRef, error)
}

// WithContracts sets the stored-contract reader and the instrument reader of a job built by this package and returns
// the same job: a pair that c holds is not resolved at run time. A nil c returns the job unchanged and any other job is
// an error. Call it once per job, before the first run.
func WithContracts(jb contracts.Job, c ContractReader, in store.InstrumentReader) (contracts.Job, error) {
	j, ok := jb.(*job)
	if !ok {
		return nil, errors.New("optioneod: WithContracts takes a job built by this package")
	}
	if c != nil {
		j.stored, j.insts = c, in
	}
	return jb, nil
}
