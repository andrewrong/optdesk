package cutover

import (
	"errors"
	"fmt"
	"io"

	"github.com/pelletier/go-toml/v2"
)

// explainedEntry is one [[explained]] entry of the owner: the figure id, why the old and the new value
// differ, and the date the owner wrote it.
type explainedEntry struct {
	ID     string `toml:"id"`
	Reason string `toml:"reason"`
	Date   any    `toml:"date"`
}

// readExplained reads the [[explained]] entries into the set of explained ids. An entry with an empty id,
// reason or date is refused.
func readExplained(r io.Reader) (map[string]bool, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, errors.New("explained: read failed")
	}
	var doc struct {
		Explained []explainedEntry `toml:"explained"`
	}
	if err := toml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("explained: %w", err)
	}
	ids := make(map[string]bool, len(doc.Explained))
	for i, e := range doc.Explained {
		if e.ID == "" {
			return nil, fmt.Errorf("explained: entry %d has no id", i+1)
		}
		if e.Reason == "" {
			return nil, fmt.Errorf("explained: entry %d (%s) has an empty reason", i+1, e.ID)
		}
		if d, ok := e.Date.(fmt.Stringer); e.Date == nil || (ok && d.String() == "") || e.Date == "" {
			return nil, fmt.Errorf("explained: entry %d (%s) has no date", i+1, e.ID)
		}
		ids[e.ID] = true
	}
	return ids, nil
}
