package migrate_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestMigrationVersionsAreUnique fails when two files under migrations/ carry
// the same version number, so a clash fails the pull request and not main.
func TestMigrationVersionsAreUnique(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("../../../migrations/*.sql")
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no migration files found under migrations/")
	}

	seen := make(map[string]string, len(files))
	for _, f := range files {
		name := filepath.Base(f)
		digits, _, ok := strings.Cut(name, "_")
		if !ok || digits == "" || strings.Trim(digits, "0123456789") != "" {
			t.Errorf("migration file %q does not start with digits and _", name)
			continue
		}
		// the version is a number: 0035 and 35 are the same version
		version := strings.TrimLeft(digits, "0")
		if prev, dup := seen[version]; dup {
			t.Errorf("duplicate migration version %s: %s and %s", digits, prev, name)
			continue
		}
		seen[version] = name
	}
}
