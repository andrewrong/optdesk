package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDemoProfile(t *testing.T) {
	t.Parallel()
	var read []string
	getenv := func(k string) string {
		read = append(read, k)
		return ""
	}
	var out, errb bytes.Buffer
	a, err := newApp(getenv, &out, &errb)
	if err != nil {
		t.Fatal(err)
	}
	if a.profile.Name != "demo" {
		t.Fatalf("profile %q, want demo", a.profile.Name)
	}
	for _, k := range read {
		if k != "OT_HOME" && k != "OT_PROFILE" {
			t.Errorf("read environment variable %s", k)
		}
	}
	if len(read) == 0 {
		t.Fatal("the profile was not selected from the environment")
	}
}

func TestVersionCommand(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	code := run([]string{"version"}, func(string) string { return "" }, &out, &errb)
	if code != 0 || !strings.Contains(out.String(), "ot dev (profile demo)") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errb.String())
	}
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	code := run([]string{"nope"}, func(string) string { return "" }, &out, &errb)
	if code != 2 || !strings.Contains(errb.String(), "version") {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
}

func TestBadProfileStopsStart(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	code := run([]string{"version"}, func(k string) string {
		if k == "OT_PROFILE" {
			return "real"
		}
		return ""
	}, &out, &errb)
	if code != 2 || out.Len() != 0 {
		t.Fatalf("exit %d, stdout %q", code, out.String())
	}
}
