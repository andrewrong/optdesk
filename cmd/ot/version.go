package main

import "fmt"

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func init() {
	register("version", func(a *app, _ []string) error {
		_, err := fmt.Fprintf(a.stdout, "ot %s (profile %s)\n", version, a.profile.Name)
		return err
	})
}
