package catalog

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestRoundTripQueriesAndCollections(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "catalog.json")
	alpha := filepath.Join(dir, "Alpha file.txt")
	beta := filepath.Join(dir, "資料", "Beta.txt")

	c, err := New(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ path, tag string }{
		{beta, "work"},
		{alpha, "favorite"},
		{alpha, "work"},
		{alpha, "work"}, // duplicate additions are idempotent
	} {
		if addErr := c.AddTag(item.path, item.tag); addErr != nil {
			t.Fatal(addErr)
		}
	}
	if collectionErr := c.CreateCollection("Current projects", []string{beta, alpha, alpha}); collectionErr != nil {
		t.Fatal(collectionErr)
	}

	assertStrings(t, c.ListTags(), []string{"favorite", "work"})
	all, err := c.PathsByAllTags("favorite", "work")
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, all, []string{alpha})
	matchingAny, err := c.PathsByAnyTags("favorite", "work")
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, matchingAny, sortedPaths(alpha, beta))
	assertStrings(t, c.ListCollections(), []string{"Current projects"})
	paths, err := c.GetCollection("Current projects")
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, paths, sortedPaths(alpha, beta))

	if saveErr := c.Save(); saveErr != nil {
		t.Fatal(saveErr)
	}
	loaded, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, loaded.ListTags(), []string{"favorite", "work"})
	paths, err = loaded.GetCollection("Current projects")
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, paths, sortedPaths(alpha, beta))

	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("catalog permissions are not private: %o", info.Mode().Perm())
	}
}

func TestRemoveTagAndDeleteCollection(t *testing.T) {
	t.Parallel()

	c := newTestCatalog(t)
	path := filepath.Join(t.TempDir(), "item")
	if err := c.AddTag(path, "one"); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveTag(path, "one"); err != nil {
		t.Fatal(err)
	}
	assertStrings(t, c.ListTags(), nil)

	if err := c.CreateCollection("set", []string{path}); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteCollection("set"); err != nil {
		t.Fatal(err)
	}
	assertStrings(t, c.ListCollections(), nil)
	if _, err := c.GetCollection("set"); !errors.Is(err, ErrCollectionNotFound) {
		t.Fatalf("GetCollection error = %v, want ErrCollectionNotFound", err)
	}
}

func TestValidationAndTraversal(t *testing.T) {
	t.Parallel()

	c := newTestCatalog(t)
	abs := filepath.Join(t.TempDir(), "item")

	for _, path := range []string{"", "relative", filepath.Join("..", "escape")} {
		if err := c.AddTag(path, "valid"); !errors.Is(err, ErrInvalidPath) {
			t.Errorf("AddTag(%q) error = %v, want ErrInvalidPath", path, err)
		}
	}
	for _, name := range []string{"", "   ", ".", "..", "a/b", `a\b`, "line\nbreak"} {
		if err := c.AddTag(abs, name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("AddTag tag %q error = %v, want ErrInvalidName", name, err)
		}
		if err := c.CreateCollection(name, []string{abs}); !errors.Is(err, ErrInvalidName) {
			t.Errorf("CreateCollection name %q error = %v, want ErrInvalidName", name, err)
		}
	}
	if err := c.AddTag(abs, "café photos"); err != nil {
		t.Fatalf("Unicode/spaces rejected: %v", err)
	}
	if err := c.CreateCollection("旅行 plans", []string{abs}); err != nil {
		t.Fatalf("Unicode/spaces rejected: %v", err)
	}
	if err := c.AddTag(abs, "  spaced tag  "); err != nil {
		t.Fatalf("surrounding spaces rejected: %v", err)
	}
}

func TestNewAcceptsCallerSuppliedRelativeFilePath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	c, err := New("catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	if saveErr := c.Save(); saveErr != nil {
		t.Fatal(saveErr)
	}
	if _, err := os.Stat(filepath.Join(dir, "catalog.json")); err != nil {
		t.Fatal(err)
	}
}

func TestUpdatePathUpdatesDescendantsAndMerges(t *testing.T) {
	t.Parallel()

	c := newTestCatalog(t)
	dir := t.TempDir()
	oldRoot := filepath.Join(dir, "old")
	newRoot := filepath.Join(dir, "new")
	child := filepath.Join(oldRoot, "sub", "item.txt")
	existing := filepath.Join(newRoot, "sub", "item.txt")
	unrelated := filepath.Join(dir, "older", "item.txt")

	for _, item := range []struct{ path, tag string }{
		{oldRoot, "root"}, {child, "child"}, {existing, "existing"}, {unrelated, "other"},
	} {
		if err := c.AddTag(item.path, item.tag); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.CreateCollection("move", []string{oldRoot, child, existing, unrelated}); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdatePath(oldRoot, newRoot); err != nil {
		t.Fatal(err)
	}

	paths, err := c.PathsByAnyTags("root")
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, paths, []string{newRoot})
	paths, err = c.PathsByAllTags("child", "existing")
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, paths, []string{existing})
	paths, err = c.GetCollection("move")
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, paths, sortedPaths(newRoot, existing, unrelated))
}

func TestMissingFileOpensEmpty(t *testing.T) {
	t.Parallel()

	c, err := Open(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, c.ListTags(), nil)
	assertStrings(t, c.ListCollections(), nil)
}

func TestCorruptionAndVersionErrorsAreTypedAndLoadIsTransactional(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "catalog.json")
	c, err := New(file)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "item")
	if err := c.AddTag(path, "keep"); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(file, []byte(`{"version":1,`), 0o600); err != nil {
		t.Fatal(err)
	}
	var corrupt *CorruptionError
	if err := c.Load(); !errors.As(err, &corrupt) {
		t.Fatalf("Load error = %v, want *CorruptionError", err)
	}
	assertStrings(t, c.ListTags(), []string{"keep"})

	if err := os.WriteFile(file, []byte(`{"version":99,"paths":[],"collections":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var version *VersionError
	if err := c.Load(); !errors.As(err, &version) {
		t.Fatalf("Load error = %v, want *VersionError", err)
	}
	if version.Found != 99 || version.Supported != CurrentVersion {
		t.Fatalf("version error = %#v", version)
	}
	assertStrings(t, c.ListTags(), []string{"keep"})
}

func TestDeterministicEncodingAndOrdering(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "catalog.json")
	c, err := New(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ path, tag string }{
		{filepath.Join(dir, "z"), "zeta"},
		{filepath.Join(dir, "a"), "alpha"},
		{filepath.Join(dir, "m"), "middle"},
	} {
		if addErr := c.AddTag(item.path, item.tag); addErr != nil {
			t.Fatal(addErr)
		}
	}
	if collectionErr := c.CreateCollection("z collection", []string{filepath.Join(dir, "z")}); collectionErr != nil {
		t.Fatal(collectionErr)
	}
	if collectionErr := c.CreateCollection("a collection", []string{filepath.Join(dir, "a")}); collectionErr != nil {
		t.Fatal(collectionErr)
	}
	if saveErr := c.Save(); saveErr != nil {
		t.Fatal(saveErr)
	}
	first, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if saveErr := c.Save(); saveErr != nil {
		t.Fatal(saveErr)
	}
	second, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("encoding changed between saves:\nfirst: %s\nsecond: %s", first, second)
	}
	assertStrings(t, c.ListTags(), []string{"alpha", "middle", "zeta"})
	assertStrings(t, c.ListCollections(), []string{"a collection", "z collection"})
}

func TestSaveReplacementFailurePreservesPreviousFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "catalog.json")
	path := filepath.Join(dir, "item")
	c, err := New(file)
	if err != nil {
		t.Fatal(err)
	}
	if addErr := c.AddTag(path, "before"); addErr != nil {
		t.Fatal(addErr)
	}
	if saveErr := c.Save(); saveErr != nil {
		t.Fatal(saveErr)
	}
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if addErr := c.AddTag(path, "after"); addErr != nil {
		t.Fatal(addErr)
	}
	injected := errors.New("injected replacement failure")
	c.replaceFile = func(string, string) error { return injected }
	if saveErr := c.Save(); !errors.Is(saveErr, injected) {
		t.Fatalf("Save error = %v, want injected failure", saveErr)
	}
	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("failed save changed previous file:\nbefore: %s\nafter: %s", before, after)
	}
	temps, err := filepath.Glob(filepath.Join(dir, ".catalog.json.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temps) != 0 {
		t.Fatalf("failed save left temporary files: %v", temps)
	}
}

func TestWindowsPathsAreCaseInsensitive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path semantics")
	}
	t.Parallel()

	dir := t.TempDir()
	c := newTestCatalog(t)
	upper := filepath.Join(dir, "Folder", "Item.txt")
	lower := strings.ToLower(upper)
	if err := c.AddTag(upper, "one"); err != nil {
		t.Fatal(err)
	}
	if err := c.AddTag(lower, "two"); err != nil {
		t.Fatal(err)
	}
	paths, err := c.PathsByAllTags("one", "two")
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, paths, []string{upper})
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	c := newTestCatalog(t)
	dir := t.TempDir()
	var wg sync.WaitGroup
	for range 24 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path := filepath.Join(dir, "same")
			for range 40 {
				if err := c.AddTag(path, "shared"); err != nil {
					t.Errorf("AddTag: %v", err)
					return
				}
				_, _ = c.PathsByAnyTags("shared")
				_ = c.ListTags()
			}
		}()
	}
	wg.Wait()
	paths, err := c.PathsByAnyTags("shared")
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, paths, []string{filepath.Join(dir, "same")})
}

func newTestCatalog(t *testing.T) *Catalog {
	t.Helper()
	c, err := New(filepath.Join(t.TempDir(), "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func sortedPaths(paths ...string) []string {
	result := append([]string(nil), paths...)
	sortPathStrings(result)
	return result
}
