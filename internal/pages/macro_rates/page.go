package macro_rates

import "github.com/andrewrong/optdesk/internal/pages"

// Page is the macro_rates page: its Meta and Build.
func Page() pages.Page {
	return pages.Page{
		Meta:  pages.Meta{Name: "macro_rates", Schema: "page.macro_rates.v3"},
		Build: Build,
	}
}
