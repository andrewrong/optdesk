package cutover

import "errors"

// errAliasNotMapped is the error of a line whose alias the alias map does not name. It holds no alias.
var errAliasNotMapped = errors.New("alias not named by the alias map")

// mapAlias returns the alias of the store for the alias of the file. A nil or empty map keeps every alias
// (the default is the same label); a map with at least one entry names the aliases it accepts, and any other
// alias is refused.
func mapAlias(aliasMap map[string]string, alias string) (string, error) {
	if len(aliasMap) == 0 {
		return alias, nil
	}
	to, ok := aliasMap[alias]
	if !ok || to == "" {
		return "", errAliasNotMapped
	}
	return to, nil
}
