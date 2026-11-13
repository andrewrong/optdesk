package optioneod

import (
	"errors"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

// WithReasons stores w in a job built by this package and returns the same job; a nil w returns the job unchanged
// and any other job is an error. Call it once per job, before the first run. No run reads the writer yet.
func WithReasons(jb contracts.Job, w runview.ReasonWriter) (contracts.Job, error) {
	j, ok := jb.(*job)
	if !ok {
		return nil, errors.New("optioneod: WithReasons takes a job built by this package")
	}
	if w != nil {
		j.reasons = w
	}
	return jb, nil
}
