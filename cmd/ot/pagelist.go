package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// zoneEnvName is the environment name the error of a nil zone names.
const zoneEnvName = "OT_ZONE"

// macroPageName is the second name of the list of pages: it is built in the zone of the profile.
const macroPageName = "macro_page"

// pageNamed returns the page --name selects: an entry of a.deps.pageList, or macro_page built
// in the zone of the profile, and a close function the caller defers. An unknown name is an
// error that lists the known names.
func pageNamed(a *app, name string) (pages.Page, func(), error) {
	if p, err := findPage(a.deps.pageList, name); err == nil {
		return p, func() {}, nil
	}
	if name == macroPageName {
		if a.profile.Zone == nil {
			return pages.Page{}, nil, errors.New("macro_page needs the owner's zone: " + zoneEnvName + " is not set")
		}
		read, closeRead, err := macroRead(context.Background(), a)
		if err != nil {
			return pages.Page{}, nil, err
		}
		return a.deps.macroPage(a.profile.Zone, buildIDOf(version), read), closeRead, nil
	}
	if name == candidatesName {
		return candidatesPageNamed(context.Background(), a)
	}
	if name == noticesName {
		return noticesPageNamed(context.Background(), a)
	}
	known := make([]string, 0, len(a.deps.pageList)+1)
	for _, p := range a.deps.pageList {
		known = append(known, p.Meta.Name)
	}
	known = append(known, macroPageName, reportName, candidatesName, noticesName)
	return pages.Page{}, nil, fmt.Errorf("unknown page %q; known pages: %s", name, strings.Join(known, ", "))
}

// macroRead returns the reading function of macro_page and a close function: the winner readings of
// the reading view opened with the .env line DSNEnvPages. With the demo profile, or when the line is
// missing or empty, it is nil and no store is opened, so the page keeps its no_data gap.
func macroRead(ctx context.Context, a *app) (compute.ReadingFunc, func(), error) {
	if a.profile.Name == "demo" {
		return nil, func() {}, nil
	}
	env, err := a.envFile()
	if err != nil {
		return nil, nil, err
	}
	dsn := env[store.DSNEnvPages]
	if dsn == "" {
		return nil, func() {}, nil
	}
	v, err := a.deps.openReadingView(ctx, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open reading view with %s failed", store.DSNEnvPages)
	}
	return v.WinnerReadings, func() { _ = v.Close() }, nil
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
