package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

// checkZhTexts returns one finding per problem of a zh_texts.go file: every declaration that is not a const
// declaration, and every const value that is not a string literal.
func checkZhTexts(path string, src []byte) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return []string{path + ": " + err.Error()}
	}
	var out []string
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			out = append(out, fset.Position(d.Pos()).String()+": only const declarations are allowed")
			continue
		}
		for _, s := range gd.Specs {
			for _, v := range s.(*ast.ValueSpec).Values {
				if lit, ok := v.(*ast.BasicLit); !ok || lit.Kind != token.STRING {
					out = append(out, fset.Position(v.Pos()).String()+": const value is not a string literal")
				}
			}
		}
	}
	return out
}

func TestZhTextsDoor(t *testing.T) {
	t.Parallel()
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join("testdata", "zhdoor", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if got := checkZhTexts("good.go", read("good.go.txt")); len(got) != 0 {
		t.Errorf("good.go.txt: unexpected findings %v", got)
	}
	for _, name := range []string{"with_func", "with_var", "with_import", "with_type"} {
		if got := checkZhTexts(name+".go", read(name+".go.txt")); len(got) == 0 {
			t.Errorf("%s.go.txt: no finding", name)
		}
	}
	if got := checkZhTexts("c.go", []byte("package demo\n\nconst A = 1\n")); len(got) != 1 {
		t.Errorf("non-string const: findings %v, want 1", got)
	}
	for path, want := range map[string]bool{
		"internal/pages/option_candidates_page/zh_texts.go":     true,
		"internal/pages/option_candidates_page/sub/zh_texts.go": false,
		"internal/compute/pagefig/zh_texts.go":                  false,
		"cmd/ot/zh_texts.go":                                    false,
		"internal/pages/p/zh_text.go":                           false,
		"internal/pages/p/zh_texts_extra.go":                    false,
	} {
		if got := isZhTextsDoor(path); got != want {
			t.Errorf("isZhTextsDoor(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestZhTextsDoorOtherFileStillFails(t *testing.T) {
	t.Parallel()
	text := "package p\n\nconst A = \"说明\"\n"
	if got := cjkFindings("internal/pages/option_candidates_page/page.go", text); len(got) == 0 {
		t.Error("page.go with a Chinese literal reported no finding")
	}
	if got := cjkFindings("internal/pages/option_candidates_page/zh_texts.go", text); len(got) != 0 {
		t.Errorf("zh_texts.go reported %v", got)
	}
}
