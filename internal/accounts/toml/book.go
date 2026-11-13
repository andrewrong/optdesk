// Package toml loads the owner's account book from $OT_HOME/accounts.toml.
//
// The file holds lines `<alias> = "<U-number>"` and one line `main = "<alias>"`, and the optional line `statement = [...]`. Design:
// docs/design/account-identity.md#2 accounts-file; format: docs/contracts/accounts.md.
package toml

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/pelletier/go-toml/v2/unstable"

	"github.com/andrewrong/optdesk/internal/accounts"
)

var (
	aliasShape = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	idShape    = regexp.MustCompile(`^U[0-9]{5,9}$`)
)

const (
	mainKey      = "main"
	statementKey = "statement"
)

type book struct {
	byID    map[string]string
	main    string
	aliases []string
	ids     []string
	stmt    []string
}

var (
	_ accounts.Book    = (*book)(nil)
	_ accounts.Sourced = (*book)(nil)
)

func (b *book) Alias(id string) (string, bool) { a, ok := b.byID[id]; return a, ok }
func (b *book) Main() string                   { return b.main }
func (b *book) Aliases() []string              { return append([]string(nil), b.aliases...) }
func (b *book) IDs() []string                  { return append([]string(nil), b.ids...) }
func (b *book) Statement() []string            { return append([]string(nil), b.stmt...) }

// Load reads the file at path and returns the book it holds. It refuses a missing file, a missing
// or dangling `main`, a bad `statement` line, a bad or reserved alias, a repeated alias or id and a bad id shape; every
// refusal after the read names the line number and never an account id.
func Load(path string) (accounts.Book, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("accounts: line 1: cannot read %s: %w", path, err)
	}
	var (
		p         unstable.Parser
		b         = &book{byID: map[string]string{}}
		aliasSeen = map[string]bool{}
		mainName  string
		mainLine  int
		stmtLine  int
		stmtItems []string
	)
	p.Reset(data)
	lineOf := func(raw unstable.Range) int { return lineAt(data, int(raw.Offset)) }
	for p.NextExpression() {
		n := p.Expression()
		if n.Kind != unstable.KeyValue {
			return nil, fmt.Errorf("accounts: line %d: only `alias = \"U-number\"` lines are allowed",
				headerLine(&p, n, data))
		}
		it := n.Key()
		it.Next()
		kn := it.Node()
		line := lineOf(p.Range(kn.Data))
		if it.Next() {
			return nil, fmt.Errorf("accounts: line %d: a dotted key is not allowed", line)
		}
		key := string(kn.Data)
		v := n.Value()
		if key == statementKey {
			if stmtLine != 0 {
				return nil, fmt.Errorf("accounts: line %d: statement is given twice", line)
			}
			stmtLine = line
			if stmtItems, err = statementItems(v, line); err != nil {
				return nil, err
			}
			continue
		}
		if v.Kind != unstable.String {
			return nil, fmt.Errorf("accounts: line %d: the value must be a string", line)
		}
		val := string(v.Data)
		if key == mainKey {
			if mainLine != 0 {
				return nil, fmt.Errorf("accounts: line %d: main is given twice", line)
			}
			mainName, mainLine = val, line
			continue
		}
		if err := checkAlias(key, val, line, aliasSeen, b.byID); err != nil {
			return nil, err
		}
		aliasSeen[key] = true
		b.byID[val] = key
		b.aliases = append(b.aliases, key)
		b.ids = append(b.ids, val)
	}
	if err := p.Error(); err != nil {
		var pe *unstable.ParserError
		if errors.As(err, &pe) {
			return nil, fmt.Errorf("accounts: line %d: not valid TOML", lineAt(data, offsetOf(data, pe.Highlight)))
		}
		return nil, fmt.Errorf("accounts: not valid TOML: %w", err)
	}
	if err := checkMain(mainName, mainLine, aliasSeen); err != nil {
		return nil, err
	}
	if err := checkStatement(stmtItems, aliasSeen, mainName, stmtLine); err != nil {
		return nil, err
	}
	b.main, b.stmt = mainName, stmtItems
	return b, nil
}

// checkMain refuses a missing `main` line and a `main` naming no alias.
func checkMain(name string, line int, aliases map[string]bool) error {
	if line == 0 {
		return errors.New(`accounts: line 1: the line main = "<alias>" is missing`)
	}
	if !aliases[name] {
		return fmt.Errorf("accounts: line %d: main names no alias of the file", line)
	}
	return nil
}

// checkAlias refuses an alias line with a bad or repeated alias or a bad or repeated id.
func checkAlias(key, val string, line int, aliasSeen map[string]bool, byID map[string]string) error {
	if !aliasShape.MatchString(key) {
		return fmt.Errorf("accounts: line %d: the alias is not %s", line, aliasShape)
	}
	if aliasSeen[key] {
		return fmt.Errorf("accounts: line %d: the alias is repeated", line)
	}
	if !idShape.MatchString(val) {
		return fmt.Errorf("accounts: line %d: the id is not a U and 5 to 9 digits", line)
	}
	if _, dup := byID[val]; dup {
		return fmt.Errorf("accounts: line %d: the id is already given to another alias", line)
	}
	return nil
}

// statementItems reads the array of the statement line as strings.
func statementItems(v *unstable.Node, line int) ([]string, error) {
	if v.Kind != unstable.Array {
		return nil, fmt.Errorf("accounts: line %d: statement must be an array of aliases", line)
	}
	var items []string
	for c := v.Children(); c.Next(); {
		item := c.Node()
		if item.Kind != unstable.String {
			return nil, fmt.Errorf("accounts: line %d: each statement item must be a string", line)
		}
		items = append(items, string(item.Data))
	}
	return items, nil
}

// checkStatement refuses an item that is no alias of the file, the main alias or a repeat.
func checkStatement(items []string, aliases map[string]bool, mainName string, line int) error {
	seen := map[string]bool{}
	for _, a := range items {
		switch {
		case !aliases[a]:
			return fmt.Errorf("accounts: line %d: statement names no alias of the file", line)
		case a == mainName:
			return fmt.Errorf("accounts: line %d: statement must not name the main alias", line)
		case seen[a]:
			return fmt.Errorf("accounts: line %d: statement repeats an alias", line)
		}
		seen[a] = true
	}
	return nil
}

// lineAt returns the 1-based line of the byte offset.
func lineAt(data []byte, off int) int {
	line := 1
	for i := 0; i < off && i < len(data); i++ {
		if data[i] == '\n' {
			line++
		}
	}
	return line
}

// offsetOf returns where sub, a sub-slice of data, starts; 0 when it cannot be told.
func offsetOf(data, sub []byte) int {
	off := cap(data) - cap(sub)
	if off < 0 || off > len(data) {
		return 0
	}
	return off
}

// headerLine returns the line of a table header; the node has no Data of its own, so its first key
// tells where it is.
func headerLine(p *unstable.Parser, n *unstable.Node, data []byte) int {
	it := n.Key()
	it.Next()
	return lineAt(data, int(p.Range(it.Node().Data).Offset))
}
