package main

// sampleHome is the route `/` redirects to: the first item of the top bar.
const sampleHome = "home"

// sampleSchemas maps each sample route to the schema its page is shown with. A sample route reads no
// store: the vm names the page and the web bundle holds the payload.
var sampleSchemas = map[string]string{
	"home":      "home_page",
	"household": "household_page",
	"positions": "positions_page",
	"risk":      "household_risk",
	"options":   "option_page",
	"advice":    "advice_page",
}

// sampleVM returns the view model of a sample route, the page.sample_page.v1 constant of its schema;
// ok is false when path is not a sample route.
func sampleVM(path string) ([]byte, bool) {
	schema, ok := sampleSchemas[path]
	if !ok {
		return nil, false
	}
	return []byte(`{"schema":"sample_page","schema_version":1,"page":"` + schema + `"}`), true
}
