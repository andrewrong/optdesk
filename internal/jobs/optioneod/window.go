package optioneod

import (
	"errors"
	"time"
)

// windowOf reads the [window] table: rule must be in_session, open_after_minutes (at least 0) is the open offset
// in minutes, calendar is the optional calendar id. The close offset is always 0 (the session close).
func windowOf(m manifest) (calendarID string, openOffset, closeOffset time.Duration, err error) {
	if m.Window.Rule != "in_session" {
		return "", 0, 0, errors.New("optioneod: manifest: window.rule must be in_session")
	}
	if *m.Window.OpenAfterMinutes < 0 {
		return "", 0, 0, errors.New("optioneod: manifest: window.open_after_minutes must be at least 0")
	}
	return m.Window.Calendar, time.Duration(*m.Window.OpenAfterMinutes) * time.Minute, 0, nil
}

// Window is the window rule of the manifest as plain values; the rule is carried, not applied.
func (j *job) Window() (calendarID string, openOffset, closeOffset time.Duration) {
	return j.cal, j.open, j.close
}
