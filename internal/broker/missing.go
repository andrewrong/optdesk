package broker

// String gives the word of a reason: not_received_release, not_received_grace, invalid, error,
// received_zero or market_data_type; any other value gives "unknown".
func (r MissingReason) String() string {
	switch r {
	case MissingNotReceivedRelease:
		return "not_received_release"
	case MissingNotReceivedGrace:
		return "not_received_grace"
	case MissingInvalid:
		return "invalid"
	case MissingError:
		return "error"
	case MissingReceivedZero:
		return "received_zero"
	case MissingMarketDataType:
		return "market_data_type"
	}
	return "unknown"
}
