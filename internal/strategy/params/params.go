// Package params hashes the named parameters of a strategy.
package params

import (
	"crypto/sha1" // #nosec G505 -- an identity of a parameter set, not a secret
	"encoding/hex"
	"sort"
	"strings"
)

// Hash is the SHA-1 hex of the sorted key=value lines joined by newlines; the same map gives the same hash.
func Hash(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	for i, k := range keys {
		lines[i] = k + "=" + values[k]
	}
	sum := sha1.Sum([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}
