// Package sharpmove says whether a day change and a 5-day change, in percent, are sharp.
// The limits are constants; the package does no I/O, reads no clock and embeds no file.
package sharpmove

import "math"

const (
	// DayLimit is the limit, in percent, for the day change.
	DayLimit = 3.0
	// FiveDayLimit is the limit, in percent, for the 5-day change.
	FiveDayLimit = 8.0
)

// Mark reports whether the day change and the 5-day change reach their limits in absolute
// value, the limit included. A nil change is a gap and is never sharp. The two results are
// independent.
func Mark(day, five *float64) (daySharp, fiveSharp bool) {
	return reaches(day, DayLimit), reaches(five, FiveDayLimit)
}

// Limits returns the day limit and the 5-day limit, in percent.
func Limits() (day, fiveDay float64) {
	return DayLimit, FiveDayLimit
}

func reaches(v *float64, limit float64) bool {
	return v != nil && math.Abs(*v) >= limit
}
