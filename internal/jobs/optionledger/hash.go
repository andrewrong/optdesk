package optionledger

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// AttributesSHA256 implements jobs.FlexAttributesSHA256Func: it removes the keys accountId and acctAlias,
// encodes the rest as one JSON object (keys ascending by byte order, no whitespace, strings in UTF-8 with
// only ", \ and control characters below 0x20 escaped, no HTML escaping) and returns the lower-case hex
// SHA-256 of that text. Control characters are written \b \t \n \f \r or \u00xx (lower-case hex). The map
// is not modified.
func AttributesSHA256(attrs map[string]string) (string, error) {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		if k == "accountId" || k == "acctAlias" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys) // byte order
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		writeString(&b, k)
		b.WriteByte(':')
		writeString(&b, attrs[k])
	}
	b.WriteByte('}')
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:]), nil
}

func writeString(b *strings.Builder, s string) {
	const hexDigits = "0123456789abcdef"
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			b.WriteString(`\"`)
		case c == '\\':
			b.WriteString(`\\`)
		case c == '\b':
			b.WriteString(`\b`)
		case c == '\t':
			b.WriteString(`\t`)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\f':
			b.WriteString(`\f`)
		case c == '\r':
			b.WriteString(`\r`)
		case c < 0x20:
			b.WriteString(`\u00`)
			b.WriteByte(hexDigits[c>>4])
			b.WriteByte(hexDigits[c&15])
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
}
