package instrument

import (
	"errors"
	"log/slog"
)

// logger returns the store's logger, or slog.Default() read now when none is set, so a later slog.SetDefault reaches it.
func (s *instrumentStore) logger() *slog.Logger {
	if s.log != nil {
		return s.log
	}
	return slog.Default()
}

// sqlClass returns the SQLSTATE of the first error in the chain of err that has one, else "other". It is never the
// error text, which may carry values.
func sqlClass(err error) string {
	var se interface{ SQLState() string }
	if errors.As(err, &se) {
		return se.SQLState()
	}
	return "other"
}

// logTermsRolledBack writes the one line of a terms insert that was rolled back to its savepoint: the instrument id and
// the error class, never a value.
func (s *instrumentStore) logTermsRolledBack(id int64, class string) {
	s.logger().Warn("option terms insert rolled back", "component", "store", "status", "rolled_back", "instrument_id", id, "err", class)
}
