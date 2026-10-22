package gitstatus

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
)

// State is a concise file or aggregate directory status.
type State string

const (
	Clean     State = "clean"
	Modified  State = "modified"
	Added     State = "added"
	Deleted   State = "deleted"
	Renamed   State = "renamed"
	Untracked State = "untracked"
	Ignored   State = "ignored"
	Conflict  State = "conflict"
)

const (
	priorityClean = iota
	priorityIgnored
	priorityUntracked
	priorityModified
	priorityAdded
	priorityRenamed
	priorityDeleted
	priorityConflict
)

const (
	ordinaryFieldCount = 9
	renameFieldCount   = 10
	conflictFieldCount = 11
	statusPathOffset   = 2
)

// Entry describes one path reported by Git. OriginalPath is set for renames.
// Paths are relative to Snapshot.Root and use the platform path separator.
type Entry struct {
	Path         string
	OriginalPath string
	State        State
}

// Snapshot is an immutable-by-convention status result. Entries contains only
// non-clean paths; clean paths are resolved by StateFor.
type Snapshot struct {
	Root    string
	Entries []Entry
}

// StateFor returns the exact path state, or the highest-priority descendant
// state for a directory. Paths may be absolute or relative to Snapshot.Root.
func (s Snapshot) StateFor(path string) (State, error) {
	paths, err := normalizePaths(s.Root, []string{path}, true)
	if err != nil {
		return Clean, err
	}
	target := filepath.Clean(filepath.FromSlash(paths[0]))
	best := Clean
	for _, entry := range s.Entries {
		entryPath := filepath.Clean(entry.Path)
		if entryPath == target {
			return entry.State, nil
		}
		if target == "." || strings.HasPrefix(entryPath, target+string(filepath.Separator)) {
			if statePriority(entry.State) > statePriority(best) {
				best = entry.State
			}
		}
	}
	return best, nil
}

func statePriority(state State) int {
	switch state {
	case Conflict:
		return priorityConflict
	case Deleted:
		return priorityDeleted
	case Renamed:
		return priorityRenamed
	case Added:
		return priorityAdded
	case Modified:
		return priorityModified
	case Untracked:
		return priorityUntracked
	case Ignored:
		return priorityIgnored
	case Clean:
		return priorityClean
	default:
		return priorityClean
	}
}

func parsePorcelainV2(data []byte) ([]Entry, error) {
	records := bytes.Split(data, []byte{0})
	entries := make([]Entry, 0, len(records))
	for i := 0; i < len(records); i++ {
		record := records[i]
		if len(record) == 0 {
			continue
		}

		entry, include, consumeNext, err := parsePorcelainRecord(record, records[i+1:])
		if err != nil {
			return nil, err
		}
		if include {
			entries = append(entries, entry)
		}
		if consumeNext {
			i++
		}
	}
	return entries, nil
}

func parsePorcelainRecord(record []byte, following [][]byte) (Entry, bool, bool, error) {
	switch record[0] {
	case '1':
		entry, err := parseOrdinaryRecord(record)
		return entry, true, false, err
	case '2':
		entry, err := parseRenameRecord(record, following)
		return entry, true, true, err
	case 'u':
		entry, err := parseConflictRecord(record)
		return entry, true, false, err
	case '?':
		entry, err := parsePathRecord(record, Untracked, "untracked")
		return entry, true, false, err
	case '!':
		entry, err := parsePathRecord(record, Ignored, "ignored")
		return entry, true, false, err
	case '#':
		// Headers are accepted for forward compatibility, though Status does
		// not currently request them.
		return Entry{}, false, false, nil
	default:
		return Entry{}, false, false, errors.New("unknown porcelain v2 record type")
	}
}

func parseOrdinaryRecord(record []byte) (Entry, error) {
	fields := bytes.SplitN(record, []byte{' '}, ordinaryFieldCount)
	if len(fields) != ordinaryFieldCount || len(fields[1]) != 2 {
		return Entry{}, errors.New("malformed ordinary status record")
	}
	return Entry{Path: cleanGitPath(fields[ordinaryFieldCount-1]), State: stateFromXY(fields[1])}, nil
}

func parseRenameRecord(record []byte, following [][]byte) (Entry, error) {
	fields := bytes.SplitN(record, []byte{' '}, renameFieldCount)
	if len(fields) != renameFieldCount || len(fields[1]) != 2 || len(following) == 0 {
		return Entry{}, errors.New("malformed rename status record")
	}
	return Entry{
		Path:         cleanGitPath(fields[renameFieldCount-1]),
		OriginalPath: cleanGitPath(following[0]),
		State:        Renamed,
	}, nil
}

func parseConflictRecord(record []byte) (Entry, error) {
	fields := bytes.SplitN(record, []byte{' '}, conflictFieldCount)
	if len(fields) != conflictFieldCount {
		return Entry{}, errors.New("malformed conflict status record")
	}
	return Entry{Path: cleanGitPath(fields[conflictFieldCount-1]), State: Conflict}, nil
}

func parsePathRecord(record []byte, state State, kind string) (Entry, error) {
	if len(record) <= statusPathOffset || record[1] != ' ' {
		return Entry{}, errors.New("malformed " + kind + " status record")
	}
	return Entry{Path: cleanGitPath(record[statusPathOffset:]), State: state}, nil
}

func cleanGitPath(path []byte) string {
	return filepath.Clean(filepath.FromSlash(string(path)))
}

func stateFromXY(xy []byte) State {
	if bytes.ContainsAny(xy, "U") || string(xy) == "AA" || string(xy) == "DD" {
		return Conflict
	}
	if bytes.ContainsAny(xy, "D") {
		return Deleted
	}
	if xy[0] == 'A' {
		return Added
	}
	return Modified
}
