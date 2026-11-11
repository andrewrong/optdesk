package underlying

// Error makes InvalidError an error, so Apply can return it. It is the one method of the package and sits outside
// api.go, which holds declarations only.
func (e *InvalidError) Error() string { return "underlying: " + e.Field + ": " + e.Why }
