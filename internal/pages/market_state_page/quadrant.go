package market_state_page

// quadrant is one corner of the rotation map: an id and its fixed text. The four are the same for
// every input; no label is chosen per fund.
type quadrant struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func quadrants() []quadrant {
	return []quadrant{
		{"leading", "Leading: 63-session gap above 0, pace above 0"},
		{"weakening", "Weakening: 63-session gap above 0, pace below 0"},
		{"lagging", "Lagging: 63-session gap below 0, pace below 0"},
		{"improving", "Improving: 63-session gap below 0, pace above 0"},
	}
}
