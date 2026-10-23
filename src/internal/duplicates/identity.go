package duplicates

import "os"

type fileIdentity struct {
	device uint64
	inode  uint64
}

// Real Unix file identities can be indexed directly, avoiding a comparison
// against every previously seen file. Other platforms retain os.SameFile's
// semantics rather than guessing identity from names, sizes, or timestamps.
type identitySet struct {
	indexed  map[fileIdentity]struct{}
	fallback []os.FileInfo
}

func (s *identitySet) containsOrAdd(info os.FileInfo) bool {
	if identity, ok := identityOf(info); ok {
		if _, exists := s.indexed[identity]; exists {
			return true
		}
		if s.indexed == nil {
			s.indexed = make(map[fileIdentity]struct{})
		}
		s.indexed[identity] = struct{}{}
		return false
	}
	for _, previous := range s.fallback {
		if os.SameFile(previous, info) {
			return true
		}
	}
	s.fallback = append(s.fallback, info)
	return false
}
