package structures_page

// The reasons a multi-leg unit of kind other was made, one line each, written in the note of the page.
const (
	// reasonOppositeSidesExpiries: legs of one right on opposite sides at different expiries.
	reasonOppositeSidesExpiries = "同一方向的期权腿多空相反且到期日不同"
	// reasonPutCallOneExpiry: a put and a call at one expiry.
	reasonPutCallOneExpiry = "同一到期日同时持有认沽和认购"
)
