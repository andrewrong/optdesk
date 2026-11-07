package option_data_page

// Choice is one underlying the page links to. Href is a path of this server; the builder checks it against the schema pattern.
type Choice struct {
	ID    int64
	Label string
	Href  string
}
