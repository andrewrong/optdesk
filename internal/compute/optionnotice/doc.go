// Package optionnotice is the rule behind compute.OptionNotices: the expiry, call guard,
// earnings, assignment, cushion and stale-capture notices of one alias and the assignment
// cash by date, on exact rationals (math/big) read from decimal text. It performs no I/O,
// reads no clock and imports no time package: every day comes in as contracts.Day text and
// is counted as calendar days (no session calendar; holidays do not move an expiry).
package optionnotice
