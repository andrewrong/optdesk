// Package toml loads the owner's account book from $OT_HOME/accounts.toml.
//
// The file holds lines `<alias> = "<U-number>"` and one line `main = "<alias>"`. Design:
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

const mainKey = "main"

type book struct {
	byID    map[string]string
	main    string
	aliases []string
}

var _ accounts.Book = (*book)(nil)

func (b *book) Alias(id string) (string, bool) { a, ok := b.byID[id]; return a, ok }
func (b *book) Main() string                   { return b.main }
func (b *book) Aliases() []string              { return append([]string(nil), b.aliases...) }

// Load reads the file at path and returns the book it holds. It refuses a missing file, a missing
// or dangling `main`, a bad or reserved alias, a repeated alias or id and a bad id shape; every
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
		if !aliasShape.MatchString(key) {
			return nil, fmt.Errorf("accounts: line %d: the alias is not %s", line, aliasShape)
		}
		if aliasSeen[key] {
			return nil, fmt.Errorf("accounts: line %d: the alias is repeated", line)
		}
		if !idShape.MatchString(val) {
			return nil, fmt.Errorf("accounts: line %d: the id is not a U and 5 to 9 digits", line)
		}
		if _, dup := b.byID[val]; dup {
			return nil, fmt.Errorf("accounts: line %d: the id is already given to another alias", line)
		}
		aliasSeen[key] = true
		b.byID[val] = key
		b.aliases = append(b.aliases, key)
	}
	if err := p.Error(); err != nil {
		var pe *unstable.ParserError
		if errors.As(err, &pe) {
			return nil, fmt.Errorf("accounts: line %d: not valid TOML", lineAt(data, offsetOf(data, pe.Highlight)))
		}
		return nil, fmt.Errorf("accounts: not valid TOML: %w", err)
	}
	if mainLine == 0 {
		return nil, errors.New(`accounts: line 1: the line main = "<alias>" is missing`)
	}
	if !aliasSeen[mainName] {
		return nil, fmt.Errorf("accounts: line %d: main names no alias of the file", mainLine)
	}
	b.main = mainName
	return b, nil
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
