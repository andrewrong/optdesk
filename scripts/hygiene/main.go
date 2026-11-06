// Command hygiene scans the files that git lists for values that must not be committed:
// account ids, local paths, personal e-mail addresses and fixtures that the manifest of
// their directory does not list.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const manifestName = "MANIFEST.synthetic-v1.json"

var (
	accountID = regexp.MustCompile(`\b[DU]U?\d{6,8}\b`)
	email     = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}\b`)
	// lineRules are the patterns that apply to every line of a scanned file.
	lineRules = []struct {
		name string
		re   *regexp.Regexp
	}{
		{"account id", accountID},
		{"/Users/ path", regexp.MustCompile(`/Users/`)},
		{"/Volumes/ path", regexp.MustCompile(`/Volumes/`)},
	}
)

// exempt reports whether a path is not scanned.
func exempt(p string) bool {
	switch p {
	case "pnpm-lock.yaml", "go.sum", "uv.lock":
		return true // the root files only
	}
	if path.Base(p) == manifestName {
		return true
	}
	return strings.HasPrefix(p, "docs/design/") || strings.HasPrefix(p, "scripts/hygiene/")
}

// noreply reports whether an address is a no-reply address.
func noreply(addr string) bool {
	local, domain, _ := strings.Cut(strings.ToLower(addr), "@")
	return strings.HasPrefix(local, "noreply") || strings.HasPrefix(local, "no-reply") ||
		strings.HasPrefix(domain, "noreply.") || strings.Contains(domain, ".noreply.")
}

// scanText returns the findings of one file's content.
func scanText(p string, content []byte) []string {
	var out []string
	for i, line := range strings.Split(string(content), "\n") {
		for _, r := range lineRules {
			if r.re.MatchString(line) {
				out = append(out, fmt.Sprintf("%s:%d: %s", p, i+1, r.name))
			}
		}
		for _, m := range email.FindAllString(line, -1) {
			if !noreply(m) {
				out = append(out, fmt.Sprintf("%s:%d: e-mail address that is not a noreply address", p, i+1))
			}
		}
	}
	return out
}

// fixtureDir returns the fixtures directory a path lies in and the path relative to it.
func fixtureDir(p string) (dir, rel string, ok bool) {
	parts := strings.Split(p, "/")
	for i := len(parts) - 2; i >= 0; i-- {
		if parts[i] == "fixtures" {
			return path.Join(parts[:i+1]...), path.Join(parts[i+1:]...), true
		}
	}
	return "", "", false
}

// listed reads the files a manifest lists; an entry is a path or an object with a path.
func listed(root, dir string) (map[string]bool, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(dir), manifestName))
	if err != nil {
		return nil, err
	}
	var m struct {
		Marker string            `json:"marker"`
		Files  []json.RawMessage `json:"files"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s/%s: %w", dir, manifestName, err)
	}
	if m.Marker != "" && m.Marker != "synthetic-v1" {
		return nil, fmt.Errorf("%s/%s: marker %q, want synthetic-v1", dir, manifestName, m.Marker)
	}
	set := map[string]bool{}
	for _, raw := range m.Files {
		var name string
		var obj struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(raw, &name) != nil {
			if err := json.Unmarshal(raw, &obj); err != nil {
				return nil, fmt.Errorf("%s/%s: unreadable entry %s", dir, manifestName, raw)
			}
			name = obj.Path
		}
		set[name] = true
	}
	return set, nil
}

func trackedFiles(root string) ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", root, "ls-files", "-z")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// scan returns the findings for every listed file under root.
func scan(root string) ([]string, error) {
	files, err := trackedFiles(root)
	if err != nil {
		return nil, err
	}
	var out []string
	manifests := map[string]map[string]bool{}
	for _, p := range files {
		if path.Base(p) != manifestName {
			if dir, rel, ok := fixtureDir(p); ok {
				set, seen := manifests[dir]
				if !seen {
					set, err = listed(root, dir)
					if err != nil && !errors.Is(err, os.ErrNotExist) {
						return nil, err
					}
					manifests[dir] = set
				}
				if !set[rel] {
					out = append(out, fmt.Sprintf("%s: fixture not listed in %s/%s", p, dir, manifestName))
				}
			}
		}
		if exempt(p) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if errors.Is(err, os.ErrNotExist) || bytes.IndexByte(b, 0) >= 0 {
			continue // deleted in the working tree, or binary
		}
		if err != nil {
			return nil, err
		}
		out = append(out, scanText(p, b)...)
	}
	return out, nil
}

func run(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("hygiene", flag.ContinueOnError)
	fl.SetOutput(stderr)
	root := fl.String("root", ".", "the repository")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	found, err := scan(*root)
	if err != nil {
		fmt.Fprintln(stderr, "hygiene:", err)
		return 2
	}
	for _, f := range found {
		fmt.Fprintln(stderr, "hygiene:", f)
	}
	if len(found) > 0 {
		return 1
	}
	fmt.Fprintln(stdout, "hygiene: ok")
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
