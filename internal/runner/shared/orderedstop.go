package shared

import "errors"

// ErrOrderedStop is the cause of a context cancelled because the process was told to stop (SIGTERM, an
// interrupt, a replaced executable), not because the work failed. A run cut with this cause is recorded
// with the reason ordered_stop, which counts toward neither the two-run cap nor the crash brake.
var ErrOrderedStop = errors.New("ordered stop")
