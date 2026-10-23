package duplicates

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsIdentityJunctionCycleAndDirectoryAliases(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "directory")
	writeFile(t, filepath.Join(directory, "one"), "same")
	writeFile(t, filepath.Join(directory, "two"), "same")
	alias := filepath.Join(root, "alias")
	cycle := filepath.Join(directory, "cycle")
	for _, junction := range []struct{ path, target string }{{alias, directory}, {cycle, root}} {
		output, err := exec.Command("cmd", "/c", "mklink", "/J", junction.path, junction.target).CombinedOutput()
		if err != nil {
			t.Fatalf("create fixture junction: %v: %s", err, output)
		}
		t.Cleanup(func() {
			if err := os.Remove(junction.path); err != nil && !os.IsNotExist(err) {
				t.Errorf("remove fixture junction: %v", err)
			}
		})
	}

	t.Run("promoted-index", func(t *testing.T) {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatal(err)
		}
		linked, err := os.Stat(alias)
		if err != nil {
			t.Fatal(err)
		}
		set := identitySet{promoted: true}
		if set.containsOrAdd(directory, info) || !set.containsOrAdd(alias, linked) {
			t.Fatal("indexed identity did not recognize the junction's directory target")
		}
		if len(set.indexed) != 1 || len(set.fallback) != 0 {
			t.Fatalf("junction query unexpectedly used fallback: indexed=%d fallback=%d", len(set.indexed), len(set.fallback))
		}
	})

	t.Run("scan-cycle", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// IncludeHardLinks makes a missed directory alias visible in file
		// counts instead of concealing it behind file identity deduplication.
		result, err := Scan(ctx, []string{root, alias, directory}, Options{FollowSymlinks: true, IncludeHardLinks: true})
		if err != nil || result.FilesScanned != 2 || result.FilesHashed != 2 || len(result.Groups) != 1 || len(result.Warnings) != 0 {
			t.Fatalf("junction cycle/alias scan = %#v, error=%v", result, err)
		}
	})
}

func TestWindowsIdentityUsesHandleAndRejectsDifferentFile(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	writeFile(t, first, "identical metadata")
	writeFile(t, second, "identical metadata")
	info, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := identityOf(first, info); !ok {
		t.Fatal("expected a native Windows identity")
	}
	if _, ok := identityOf(second, info); ok {
		t.Fatal("accepted an identity belonging to a different file")
	}
	if _, ok := identityOf(filepath.Join(root, "missing"), info); ok {
		t.Fatal("expected missing path to use fallback")
	}
}

func TestWindowsIdentityFallbackCrossLookup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file")
	link := filepath.Join(root, "hardlink")
	writeFile(t, path, "content")
	if err := os.Link(path, link); err != nil {
		t.Fatal(err)
	}
	for _, fallbackFirst := range []bool{false, true} {
		t.Run(fmt.Sprint("fallbackFirst=", fallbackFirst), func(t *testing.T) {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			linked, err := os.Stat(link)
			if err != nil {
				t.Fatal(err)
			}
			firstPath, secondPath := path, ""
			if fallbackFirst {
				firstPath, secondPath = "", link
			}
			set := identitySet{promoted: true}
			if set.containsOrAdd(firstPath, info) {
				t.Fatal("first observation reported as duplicate")
			}
			if !set.containsOrAdd(secondPath, linked) {
				t.Fatal("missed hard link across indexed/fallback boundary")
			}
		})
	}
}

func TestWindowsIdentityDirectoryAndHardLink(t *testing.T) {
	root := t.TempDir()
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	var set identitySet
	if set.containsOrAdd(root, info) || !set.containsOrAdd(root, info) {
		t.Fatal("directory identity was not stable")
	}
}

func TestWindowsIdentityRejectsReplacedSnapshot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file")
	writeFile(t, path, "old")
	handle, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := handle.Stat()
	handle.Close()
	if err != nil {
		t.Fatal(err)
	}
	// Rename keeps the original file alive, preventing file ID reuse.
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "new")
	if _, ok := identityOf(path, info); ok {
		t.Fatal("indexed replacement using the original snapshot")
	}
}

func TestWindowsIdentityPromotionRetainsUnavailableEntries(t *testing.T) {
	root := t.TempDir()
	var set identitySet
	var unavailable os.FileInfo
	for index := range identityIndexThreshold + 1 {
		path := filepath.Join(root, fmt.Sprint(index))
		writeFile(t, path, "content")
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		lookupPath := path
		if index == 0 {
			// Simulate an identity-query failure without disabling SameFile.
			lookupPath = ""
			unavailable = info
		}
		if set.containsOrAdd(lookupPath, info) {
			t.Fatalf("unique file %d reported as duplicate", index)
		}
		if index < identityIndexThreshold && (set.promoted || len(set.indexed) != 0) {
			t.Fatal("small scan opened an identity index")
		}
	}
	if !set.promoted || len(set.indexed) != identityIndexThreshold || len(set.fallback) != 1 {
		t.Fatalf("promotion lost entries: indexed=%d fallback=%d promoted=%v", len(set.indexed), len(set.fallback), set.promoted)
	}
	if !set.containsOrAdd(filepath.Join(root, "0"), unavailable) {
		t.Fatal("promotion lost the entry whose identity query failed")
	}
}

// BenchmarkWindowsIdentitySet compares the production index with the exact
// aff5844 fallback loop, using identical files and warmed FileInfo snapshots.
// Keeping the reference here makes the quadratic fallback reproducible without
// requiring a second checkout or private Go runtime fields.
func BenchmarkWindowsIdentitySet(b *testing.B) {
	for _, count := range []int{256, 2048, 8192} {
		root := b.TempDir()
		paths := make([]string, count)
		infos := make([]os.FileInfo, count)
		for index := range paths {
			paths[index] = filepath.Join(root, fmt.Sprint(index))
			if err := os.WriteFile(paths[index], nil, 0600); err != nil {
				b.Fatal(err)
			}
			info, err := os.Stat(paths[index])
			if err != nil {
				b.Fatal(err)
			}
			if !os.SameFile(info, info) {
				b.Fatal("could not load native identity")
			}
			infos[index] = info
		}
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			b.Run("indexed", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					var set identitySet
					for index, info := range infos {
						if set.containsOrAdd(paths[index], info) {
							b.Fatal("unexpected duplicate")
						}
					}
				}
			})
			b.Run("aff5844-fallback", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					var fallback []os.FileInfo
					for _, info := range infos {
						for _, previous := range fallback {
							if os.SameFile(previous, info) {
								b.Fatal("unexpected duplicate")
							}
						}
						fallback = append(fallback, info)
					}
				}
			})
		})
	}
}
