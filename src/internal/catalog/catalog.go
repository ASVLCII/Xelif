// Package catalog stores user-defined tags and named path collections.
// It treats paths as opaque metadata and never inspects or modifies the paths.
package catalog

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

var (
	ErrInvalidName        = errors.New("invalid catalog name")
	ErrInvalidPath        = errors.New("catalog path must be absolute")
	ErrCollectionExists   = errors.New("collection already exists")
	ErrCollectionNotFound = errors.New("collection not found")
)

type pathEntry struct {
	path string
	tags map[string]struct{}
}

// Catalog is a concurrency-safe, in-memory catalog backed by one JSON file.
// Mutations are persisted only when Save is called.
type Catalog struct {
	mu          sync.RWMutex
	ioMu        sync.Mutex
	filePath    string
	paths       map[string]pathEntry
	collections map[string]map[string]string
	replaceFile func(string, string) error
}

// New creates an empty catalog associated with filePath.
func New(filePath string) (*Catalog, error) {
	if filePath == "" {
		return nil, fmt.Errorf("catalog file: %w", ErrInvalidPath)
	}
	clean, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("catalog file: %w", ErrInvalidPath)
	}
	clean = filepath.Clean(clean)
	return &Catalog{
		filePath:    clean,
		paths:       make(map[string]pathEntry),
		collections: make(map[string]map[string]string),
		replaceFile: atomicReplace,
	}, nil
}

// Open creates a catalog and loads it. A missing file is treated as an empty
// catalog so first-run callers do not need a separate existence check.
func Open(filePath string) (*Catalog, error) {
	c, err := New(filePath)
	if err != nil {
		return nil, err
	}
	if err := c.Load(); err != nil {
		return nil, err
	}
	return c, nil
}

// AddTag associates tag with an absolute path. Repeated additions are
// idempotent.
func (c *Catalog) AddTag(path, tag string) error {
	clean, key, err := canonicalPath(path)
	if err != nil {
		return err
	}
	if err := validateName(tag); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.paths[key]
	if !ok {
		entry = pathEntry{path: clean, tags: make(map[string]struct{})}
	}
	entry.tags[tag] = struct{}{}
	c.paths[key] = entry
	return nil
}

// RemoveTag removes an association. Removing an absent association is
// idempotent.
func (c *Catalog) RemoveTag(path, tag string) error {
	_, key, err := canonicalPath(path)
	if err != nil {
		return err
	}
	if err := validateName(tag); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.paths[key]
	if !ok {
		return nil
	}
	delete(entry.tags, tag)
	if len(entry.tags) == 0 {
		delete(c.paths, key)
	}
	return nil
}

// Tags returns the tags associated with path in lexical order.
func (c *Catalog) Tags(path string) ([]string, error) {
	_, key, err := canonicalPath(path)
	if err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.paths[key]
	if !ok {
		return nil, nil
	}
	return sortedSet(entry.tags), nil
}

// ListTags returns every tag currently used by at least one path.
func (c *Catalog) ListTags() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	tags := make(map[string]struct{})
	for _, entry := range c.paths {
		for tag := range entry.tags {
			tags[tag] = struct{}{}
		}
	}
	return sortedSet(tags)
}

// PathsByAllTags returns paths having every supplied tag. With no tags, it
// returns all tagged paths.
func (c *Catalog) PathsByAllTags(tags ...string) ([]string, error) {
	if err := validateNames(tags); err != nil {
		return nil, err
	}
	wanted := uniqueStrings(tags)
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, 0)
	for _, entry := range c.paths {
		matched := true
		for _, tag := range wanted {
			if _, ok := entry.tags[tag]; !ok {
				matched = false
				break
			}
		}
		if matched {
			result = append(result, entry.path)
		}
	}
	sortPathStrings(result)
	return result, nil
}

// PathsByAnyTags returns paths having at least one supplied tag. With no
// tags, it returns an empty result.
func (c *Catalog) PathsByAnyTags(tags ...string) ([]string, error) {
	if err := validateNames(tags); err != nil {
		return nil, err
	}
	wanted := uniqueStrings(tags)
	if len(wanted) == 0 {
		return nil, nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, 0)
	for _, entry := range c.paths {
		for _, tag := range wanted {
			if _, ok := entry.tags[tag]; ok {
				result = append(result, entry.path)
				break
			}
		}
	}
	sortPathStrings(result)
	return result, nil
}

// CreateCollection creates a named set of absolute paths.
func (c *Catalog) CreateCollection(name string, paths []string) error {
	if err := validateName(name); err != nil {
		return err
	}
	items := make(map[string]string, len(paths))
	for _, path := range paths {
		clean, key, err := canonicalPath(path)
		if err != nil {
			return err
		}
		if _, exists := items[key]; !exists {
			items[key] = clean
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.collections[name]; exists {
		return ErrCollectionExists
	}
	c.collections[name] = items
	return nil
}

// DeleteCollection deletes a collection.
func (c *Catalog) DeleteCollection(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.collections[name]; !exists {
		return ErrCollectionNotFound
	}
	delete(c.collections, name)
	return nil
}

// ListCollections returns collection names in lexical order.
func (c *Catalog) ListCollections() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.collections) == 0 {
		return nil
	}
	names := make([]string, 0, len(c.collections))
	for name := range c.collections {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetCollection returns a copy of a collection's paths in deterministic order.
func (c *Catalog) GetCollection(name string) ([]string, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	items, exists := c.collections[name]
	if !exists {
		return nil, ErrCollectionNotFound
	}
	result := make([]string, 0, len(items))
	for _, path := range items {
		result = append(result, path)
	}
	sortPathStrings(result)
	return result, nil
}

// UpdatePath rewrites oldPath and all of its recorded descendants to newPath.
// Collisions merge tags and deduplicate collection entries.
func (c *Catalog) UpdatePath(oldPath, newPath string) error {
	oldClean, _, err := canonicalPath(oldPath)
	if err != nil {
		return err
	}
	newClean, _, err := canonicalPath(newPath)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	updatedPaths := make(map[string]pathEntry, len(c.paths))
	for _, entry := range c.paths {
		path := entry.path
		if rewritten, ok := rewriteDescendant(path, oldClean, newClean); ok {
			path = rewritten
		}
		_, key, _ := canonicalPath(path)
		target, exists := updatedPaths[key]
		if !exists {
			target = pathEntry{path: path, tags: make(map[string]struct{})}
		} else if path < target.path {
			target.path = path
		}
		for tag := range entry.tags {
			target.tags[tag] = struct{}{}
		}
		updatedPaths[key] = target
	}
	c.paths = updatedPaths

	for name, collection := range c.collections {
		updated := make(map[string]string, len(collection))
		for _, path := range collection {
			if rewritten, ok := rewriteDescendant(path, oldClean, newClean); ok {
				path = rewritten
			}
			_, key, _ := canonicalPath(path)
			if existing, exists := updated[key]; !exists || path < existing {
				updated[key] = path
			}
		}
		c.collections[name] = updated
	}
	return nil
}

func validateNames(names []string) error {
	for _, name := range names {
		if err := validateName(name); err != nil {
			return err
		}
	}
	return nil
}

func validateName(name string) error {
	if !utf8.ValidString(name) || strings.TrimSpace(name) == "" || name == "." || name == ".." {
		return ErrInvalidName
	}
	for _, r := range name {
		if r == '/' || r == '\\' || r < ' ' || r == 0x7f {
			return ErrInvalidName
		}
	}
	return nil
}

func canonicalPath(path string) (string, string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", "", ErrInvalidPath
	}
	clean := filepath.Clean(path)
	key := clean
	if runtime.GOOS == "windows" {
		key = strings.ToLower(clean)
	}
	return clean, key, nil
}

func sortedSet(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	result := make([]string, 0, len(set))
	for item := range set {
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func sortPathStrings(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		_, left, _ := canonicalPath(paths[i])
		_, right, _ := canonicalPath(paths[j])
		if left == right {
			return paths[i] < paths[j]
		}
		return left < right
	})
}

func rewriteDescendant(path, oldRoot, newRoot string) (string, bool) {
	pathVolume, pathParts := splitPath(path)
	oldVolume, oldParts := splitPath(oldRoot)
	if !pathEqual(pathVolume, oldVolume) || len(pathParts) < len(oldParts) {
		return path, false
	}
	for i := range oldParts {
		if !pathEqual(pathParts[i], oldParts[i]) {
			return path, false
		}
	}
	result := newRoot
	for _, part := range pathParts[len(oldParts):] {
		result = filepath.Join(result, part)
	}
	return result, true
}

func splitPath(path string) (string, []string) {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	rest := strings.TrimPrefix(clean, volume)
	rest = strings.Trim(rest, string(filepath.Separator))
	if rest == "" {
		return volume, nil
	}
	return volume, strings.Split(rest, string(filepath.Separator))
}

func pathEqual(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
