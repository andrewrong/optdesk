package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/andrewrong/optdesk/internal/pages"
)

// zoneEnvName is the environment name the error of a nil zone names.
const zoneEnvName = "OT_ZONE"

// macroPageName is the second name of the list of pages: it is built in the zone of the profile.
const macroPageName = "macro_page"

// pageNamed returns the page --name selects: an entry of a.deps.pageList, or macro_page built
// in the zone of the profile. An unknown name is an error that lists the known names.
func pageNamed(a *app, name string) (pages.Page, error) {
	if p, err := findPage(a.deps.pageList, name); err == nil {
		return p, nil
	}
	if name == macroPageName {
		if a.profile.Zone == nil {
			return pages.Page{}, errors.New("macro_page needs the owner's zone: " + zoneEnvName + " is not set")
		}
		return a.deps.macroPage(a.profile.Zone, buildIDOf(version)), nil
	}
	known := make([]string, 0, len(a.deps.pageList)+1)
	for _, p := range a.deps.pageList {
		known = append(known, p.Meta.Name)
	}
	known = append(known, macroPageName)
	return pages.Page{}, fmt.Errorf("unknown page %q; known pages: %s", name, strings.Join(known, ", "))
}

var notIDChar = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// buildIDOf turns a version into a build id: every character outside [A-Za-z0-9_-] becomes
// "-", the result is cut to 64 characters, and an empty version gives "dev".
func buildIDOf(v string) string {
	if v == "" {
		return "dev"
	}
	id := notIDChar.ReplaceAllString(v, "-")
	if len(id) > 64 {
		id = id[:64]
	}
	return id
}
