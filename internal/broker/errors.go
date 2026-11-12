package broker

func (e notServedError) Error() string { return string(e) }
