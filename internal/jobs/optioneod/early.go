package optioneod

import (
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// reasonBeforeSlot is the word of the Retryable error of a run that started before the slot of its day.
const reasonBeforeSlot = "before_slot"

// beforeSlot tells whether now is before the slot instant of day: the day at the manifest slot HHMM (seconds 0) in the
// manifest zone. A slot that does not parse is never early.
func (j *job) beforeSlot(day contracts.Day, now time.Time) bool {
	at, err := time.ParseInLocation("2006-01-02 1504", string(day)+" "+j.m.Slot, j.loc)
	if err != nil {
		return false
	}
	return now.Before(at)
}
