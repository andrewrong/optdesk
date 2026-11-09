package cutover

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Classes of a figure (Decision 8 of docs/design/cut-over.md).
const (
	classReplayable = "replayable"
	classLiveOnly   = "live_only"
	classNew        = "new"
)

type classEntry struct {
	ID      string `toml:"id"`
	Prefix  string `toml:"prefix"`
	Version *int   `toml:"version"`
	Class   string `toml:"class"`
}

// classTable is the [[class]] entries of the classes file.
type classTable []classEntry

// readClasses reads the classes file and refuses an entry that does not name exactly one of id and prefix,
// a version without an id, or a class other than the three.
func readClasses(r io.Reader) (classTable, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, errors.New("classes: read failed")
	}
	var doc struct {
		Class []classEntry `toml:"class"`
	}
	if err := toml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("classes: %w", err)
	}
	for i, e := range doc.Class {
		switch {
		case (e.ID == "") == (e.Prefix == ""):
			return nil, fmt.Errorf("classes: entry %d needs exactly one of id and prefix", i+1)
		case e.Version != nil && e.ID == "":
			return nil, fmt.Errorf("classes: entry %d has a version without an id", i+1)
		case e.Class != classReplayable && e.Class != classLiveOnly && e.Class != classNew:
			return nil, fmt.Errorf("classes: entry %d has an unknown class", i+1)
		}
	}
	return classTable(doc.Class), nil
}

// classify applies the matching rule: an entry with id and version wins; else an entry with id; else the
// longest prefix that equals the id or is followed by a dot in it. ok is false when nothing matches.
func (t classTable) classify(id string, version int) (class string, ok bool) {
	byID, longest := "", -1
	for _, e := range t {
		switch {
		case e.ID == id && e.Version != nil && *e.Version == version:
			return e.Class, true
		case e.ID == id && e.Version == nil:
			byID = e.Class
		case e.Prefix != "" && (id == e.Prefix || strings.HasPrefix(id, e.Prefix+".")) && len(e.Prefix) > longest:
			longest, class = len(e.Prefix), e.Class
		}
	}
	if byID != "" {
		return byID, true
	}
	return class, longest >= 0
}
