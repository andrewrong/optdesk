package instrument

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenInstrumentNameFunc = OpenInstrumentNames

// OpenInstrumentNames is a stub: the reader of rd.instrument_r1 is not implemented yet.
func OpenInstrumentNames(context.Context, string) (store.InstrumentNameStore, error) {
	return nil, permanent(errors.New("instrument: instrument names: not implemented"))
}
