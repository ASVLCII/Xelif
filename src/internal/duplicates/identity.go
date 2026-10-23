package duplicates

import "os"

type fileIdentity struct {
	device uint64
	inode  uint64
}

// Index only operating-system identities, never names, sizes, or timestamps.
// Windows can fail to obtain an identity for an otherwise valid FileInfo, so
// retain indexed snapshots there to compare with the fallback in both directions.
type identitySet struct {
	indexed      map[fileIdentity]struct{}
	fallback     []os.FileInfo
	indexedInfos []os.FileInfo
	pendingPaths []string
	promoted     bool
}

func (s *identitySet) containsOrAdd(path string, info os.FileInfo) bool {
	if identityIndexThreshold > 0 && !s.promoted {
		// Small Windows scans are faster using SameFile's lazily cached IDs
		// than opening every path again. Promote only when the quadratic
		// comparison cost starts to dominate; Unix always indexes immediately.
		if len(s.fallback) < identityIndexThreshold {
			if sameFileIn(s.fallback, info) {
				return true
			}
			s.fallback = append(s.fallback, info)
			s.pendingPaths = append(s.pendingPaths, path)
			return false
		}
		s.promoted = true
		infos, paths := s.fallback, s.pendingPaths
		s.fallback, s.pendingPaths = nil, nil
		for index, previous := range infos {
			s.containsOrAdd(paths[index], previous)
		}
	}
	if identity, ok := identityOf(path, info); ok {
		if _, exists := s.indexed[identity]; exists {
			return true
		}
		if sameFileIn(s.fallback, info) {
			return true
		}
		if s.indexed == nil {
			s.indexed = make(map[fileIdentity]struct{})
		}
		s.indexed[identity] = struct{}{}
		if retainIndexedInfos {
			s.indexedInfos = append(s.indexedInfos, info)
		}
		return false
	}
	if sameFileIn(s.fallback, info) || sameFileIn(s.indexedInfos, info) {
		return true
	}
	s.fallback = append(s.fallback, info)
	return false
}

func sameFileIn(infos []os.FileInfo, info os.FileInfo) bool {
	for _, previous := range infos {
		if os.SameFile(previous, info) {
			return true
		}
	}
	return false
}
