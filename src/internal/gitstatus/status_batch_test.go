package gitstatus

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestStatesForMatchesIndividualLookups(t *testing.T) {
	root := t.TempDir()
	snapshot := Snapshot{Root: root, Entries: []Entry{
		{Path: filepath.Join("nested", "ignored", "file"), State: Ignored},
		{Path: filepath.Join("nested", "untracked", "file"), State: Untracked},
		{Path: filepath.Join("nested", "modified", "file"), State: Modified},
		{Path: filepath.Join("nested", "added", "file"), State: Added},
		{Path: filepath.Join("nested", "renamed", "file"), State: Renamed},
		{Path: filepath.Join("nested", "deleted", "file"), State: Deleted},
		{Path: filepath.Join("nested", "conflict", "file"), State: Conflict},
		{Path: "exact", State: Ignored},
		{Path: filepath.Join("exact", "file"), State: Conflict},
		{Path: "duplicate", State: Added},
		{Path: "duplicate", State: Deleted},
	}}
	paths := []string{".", root, "clean", "nested", "exact", "exact/file", "duplicate", "", "../outside"}
	for _, entry := range snapshot.Entries {
		paths = append(paths, entry.Path, filepath.Join(root, entry.Path), filepath.Dir(entry.Path))
	}
	for pass := range 2 {
		states, pathErrors := snapshot.StatesFor(paths)
		invalid := 0
		for _, path := range paths {
			want, err := snapshot.StateFor(path)
			if err != nil {
				invalid++
				if _, found := states[path]; found {
					t.Fatalf("invalid path %q has a state", path)
				}
				continue
			}
			got := states[path]
			if got == "" {
				got = Clean
			}
			if got != want {
				t.Fatalf("pass %d: StateFor(%q) = %q, batch = %q", pass, path, want, got)
			}
		}
		if len(pathErrors) != invalid {
			t.Fatalf("got %d errors for %d invalid paths", len(pathErrors), invalid)
		}
		// A batch index must not retain stale entries across calls.
		snapshot.Entries = []Entry{{Path: "other", State: Modified}}
	}
}

func BenchmarkSnapshotPanelStates(b *testing.B) {
	for _, count := range []int{100, 10000} {
		b.Run(fmt.Sprintf("entries_%d", count), func(b *testing.B) {
			root := b.TempDir()
			snapshot := Snapshot{Root: root, Entries: make([]Entry, count)}
			paths := make([]string, count)
			for index := range count {
				path := fmt.Sprintf("dir-%04d/file-%05d", index/10, index)
				snapshot.Entries[index] = Entry{Path: filepath.FromSlash(path), State: Modified}
				paths[index] = filepath.Join(root, filepath.FromSlash(path))
			}
			b.ReportAllocs()
			for b.Loop() {
				states, errs := snapshot.StatesFor(paths)
				if len(errs) != 0 || len(states) != count {
					b.Fatal("incorrect batch result")
				}
			}
		})
	}
}
