package fredseries

import (
	"errors"
	"time"
)

// windowOf reads the [window] table: rule must be after_close, lateness_hours (at least 0) is the close
// offset in hours, calendar is the optional calendar id. The open offset is always 0. Errors name the key only.
func windowOf(m manifest) (calendarID string, openOffset, closeOffset time.Duration, err error) {
	if m.Window.Rule != "after_close" {
		return "", 0, 0, errors.New("fredseries: manifest: window.rule must be after_close")
	}
	if m.Window.LatenessHours < 0 {
		return "", 0, 0, errors.New("fredseries: manifest: window.lateness_hours must be at least 0")
	}
	return m.Window.Calendar, 0, time.Duration(m.Window.LatenessHours) * time.Hour, nil
}

// Window is the window rule of the manifest as plain values; the rule is carried, not applied.
func (j *job) Window() (calendarID string, openOffset, closeOffset time.Duration) {
	return j.cal, j.open, j.close
}
