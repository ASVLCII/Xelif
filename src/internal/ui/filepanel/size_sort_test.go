package filepanel

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

func TestSizeSortMatchesUncachedOrdering(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		for seed := range 4 {
			t.Run(fmt.Sprintf("reversed-%t/seed-%d", reversed, seed), func(t *testing.T) {
				elements := make([]Element, 48)
				counts := make(map[string]int)
				for index := range elements {
					value := (index*17 + seed) % len(elements)
					name := fmt.Sprintf("entry-%02d", value)
					elements[index] = Element{
						Name:      name,
						Location:  name,
						Directory: value < 32,
						Info:      sizeSortFileInfo(value % 5),
					}
					counts[name] = value % 7
				}
				readDir := func(path string) ([]os.DirEntry, error) {
					var err error
					if path == "entry-00" || path == "entry-01" {
						err = os.ErrPermission
					}
					// Retain the old len(entries) behavior, including partial reads
					// accompanied by an error and ordinary nil-entry failures.
					return make([]os.DirEntry, counts[path]), err
				}
				want := slices.Clone(elements)
				sort.Slice(want, uncachedSizeSortOrdering(want, reversed, readDir))
				sort.Slice(elements, getSizeOrderingFuncWithReadDir(elements, reversed, readDir))
				if !slices.Equal(sizeSortNames(elements), sizeSortNames(want)) {
					t.Fatalf("ordering changed: got %v, want %v", sizeSortNames(elements), sizeSortNames(want))
				}
			})
		}
	}
}

func TestSizeSortReadsEachDirectoryOnce(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		t.Run(fmt.Sprintf("reversed-%t", reversed), func(t *testing.T) {
			elements := make([]Element, 32)
			for index := range elements {
				name := fmt.Sprintf("directory-%02d", index)
				elements[index] = Element{Name: name, Location: name, Directory: true}
			}
			reads := make(map[string]int)
			readDir := func(path string) ([]os.DirEntry, error) {
				reads[path]++
				if path == "directory-00" {
					return nil, os.ErrPermission
				}
				return make([]os.DirEntry, len(path)%5), nil
			}
			order := getSizeOrderingFuncWithReadDir(elements, reversed, readDir)
			// Explicitly repeat a failed read too, regardless of sort's choices.
			_ = order(0, 1)
			_ = order(0, 1)
			sort.Slice(elements, order)
			for _, element := range elements {
				if reads[element.Location] != 1 {
					t.Errorf("read %s %d times, want once", element.Location, reads[element.Location])
				}
			}
		})
	}
}

func TestSizeSortMixedPairsDoNotReadDirectories(t *testing.T) {
	elements := []Element{
		{Name: "directory", Directory: true},
		{Name: "file", Info: sizeSortFileInfo(1)},
	}
	for _, reversed := range []bool{false, true} {
		order := getSizeOrderingFuncWithReadDir(elements, reversed, func(string) ([]os.DirEntry, error) {
			t.Fatal("mixed directory/file comparison read a directory")
			return nil, nil
		})
		if !order(0, 1) || order(1, 0) {
			t.Fatalf("directory must remain first when reversed=%t", reversed)
		}
	}
}

func TestSizeSortPreservesEqualSizeComparisons(t *testing.T) {
	elements := []Element{
		{Name: "file-a", Info: sizeSortFileInfo(1)},
		{Name: "file-b", Info: sizeSortFileInfo(1)},
		{Name: "directory-a", Directory: true},
		{Name: "directory-b", Directory: true},
	}
	for _, reversed := range []bool{false, true} {
		order := getSizeOrderingFuncWithReadDir(elements, reversed, func(string) ([]os.DirEntry, error) {
			return nil, nil
		})
		if order(0, 1) || order(1, 0) || order(2, 3) || order(3, 2) {
			t.Fatalf("equal sizes must compare equal when reversed=%t", reversed)
		}
	}
}

func TestSizeSortRefreshesDirectoryCountsOnNextSort(t *testing.T) {
	root := t.TempDir()
	empty := filepath.Join(root, "empty")
	one := filepath.Join(root, "one")
	for _, path := range []string{empty, one} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(one, "child"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	got := sizeSortNames(sortFileElement(sortmodel.SortBySize, false, entries, root))
	if !slices.Equal(got, []string{"empty", "one"}) {
		t.Fatalf("first sort: got %v, want [empty one]", got)
	}
	for _, name := range []string{"child-a", "child-b"} {
		if err := os.WriteFile(filepath.Join(empty, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	got = sizeSortNames(sortFileElement(sortmodel.SortBySize, false, entries, root))
	if !slices.Equal(got, []string{"one", "empty"}) {
		t.Fatalf("next sort used stale counts: got %v, want [one empty]", got)
	}
}

func uncachedSizeSortOrdering(
	elements []Element,
	reversed bool,
	readDir func(string) ([]os.DirEntry, error),
) sliceOrderFunc {
	return func(i, j int) bool {
		if elements[i].Directory != elements[j].Directory {
			return elements[i].Directory
		}
		if reversed {
			i, j = j, i
		}
		if elements[i].Directory && elements[j].Directory {
			filesI, _ := readDir(elements[i].Location)
			filesJ, _ := readDir(elements[j].Location)
			return len(filesI) < len(filesJ)
		}
		return elements[i].Info.Size() < elements[j].Info.Size()
	}
}

func sizeSortNames(elements []Element) []string {
	names := make([]string, len(elements))
	for index := range elements {
		names[index] = elements[index].Name
	}
	return names
}

type sizeSortFileInfo int64

func (info sizeSortFileInfo) Name() string       { return "" }
func (info sizeSortFileInfo) Size() int64        { return int64(info) }
func (info sizeSortFileInfo) Mode() os.FileMode  { return 0 }
func (info sizeSortFileInfo) ModTime() time.Time { return time.Time{} }
func (info sizeSortFileInfo) IsDir() bool        { return false }
func (info sizeSortFileInfo) Sys() any           { return nil }
