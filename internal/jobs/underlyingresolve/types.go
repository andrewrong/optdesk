package underlyingresolve

import (
	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// NewFunc is the constructor of the underlying_resolve job (underlyingresolve.New; docs/design/underlyings.md,
// Decision 7). Each run reads the pending names and the unresolved names older than retry_days through the Settler,
// asks the broker once per name (ResolveContract, one at a time) and settles each: listed gives the con_id,
// not listed or ambiguous (listed with no con_id) gives unresolved with a reason, a broker or store error settles
// nothing and the name is asked again the next run. At most max_names names a run. The job holds the Settler, not
// the Writer.
type NewFunc func(broker.Broker, underlying.Settler, clock.Clock) (contracts.Job, error)

// DatasetID is the dataset name of the keys of the job: the subject key is the name_key of a name.
const DatasetID = "settings.underlying"

var _ NewFunc = New
