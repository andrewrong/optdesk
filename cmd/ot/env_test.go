package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestReadEnvFileComments(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), ".env")
	if err := writeFile(p, "# c\n\nA=1\n  # d\nB=two words\n"); err != nil {
		t.Fatal(err)
	}
	m, err := readEnvFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 || m["A"] != "1" || m["B"] != "two words" {
		t.Fatalf("map %v", m)
	}
}

func TestReadEnvFileQuotes(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), ".env")
	if err := writeFile(p, "A=\"x y\"\nB='z'\nC=\"\"q\"\"\nD=\"open\n"); err != nil {
		t.Fatal(err)
	}
	m, err := readEnvFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if m["A"] != "x y" || m["B"] != "z" || m["C"] != "\"q\"" || m["D"] != "\"open" {
		t.Fatalf("map %v", m)
	}
}

func TestReadEnvFileMissing(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), ".env")
	_, err := readEnvFile(p)
	if err == nil || !strings.Contains(err.Error(), p) {
		t.Fatalf("error %v does not name %s", err, p)
	}
}
