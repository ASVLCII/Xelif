package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const catalogFilePerm = 0o600

// CurrentVersion is the JSON persistence format written by this package.
const CurrentVersion = 1

type diskCatalog struct {
	Version     int              `json:"version"`
	Paths       []diskPath       `json:"paths"`
	Collections []diskCollection `json:"collections"`
}

type diskPath struct {
	Path string   `json:"path"`
	Tags []string `json:"tags"`
}

type diskCollection struct {
	Name  string   `json:"name"`
	Paths []string `json:"paths"`
}

// Load transactionally replaces the in-memory state with the file contents.
// Missing files load as empty catalogs. On any other error, existing state is
// unchanged.
func (c *Catalog) Load() error {
	c.ioMu.Lock()
	defer c.ioMu.Unlock()

	data, err := os.ReadFile(c.filePath)
	if errors.Is(err, os.ErrNotExist) {
		c.mu.Lock()
		c.paths = make(map[string]pathEntry)
		c.collections = make(map[string]map[string]string)
		c.mu.Unlock()
		return nil
	}
	if err != nil {
		return fmt.Errorf("read catalog: %w", err)
	}
	disk, err := decodeCatalog(data)
	if err != nil {
		return &CorruptionError{Path: c.filePath, Err: err}
	}
	if disk.Version != CurrentVersion {
		return &VersionError{Path: c.filePath, Found: disk.Version, Supported: CurrentVersion}
	}
	paths, collections, err := stateFromDisk(disk)
	if err != nil {
		return &CorruptionError{Path: c.filePath, Err: err}
	}

	c.mu.Lock()
	c.paths = paths
	c.collections = collections
	c.mu.Unlock()
	return nil
}

// Save writes a deterministic snapshot using a same-directory temporary file
// and atomic replacement. The previous file remains intact if replacement
// fails.
func (c *Catalog) Save() error {
	c.ioMu.Lock()
	defer c.ioMu.Unlock()

	disk := c.snapshot()
	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return fmt.Errorf("encode catalog: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(c.filePath)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(c.filePath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create catalog temporary file: %w", err)
	}
	tempName := temp.Name()
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = os.Remove(tempName)
		}
	}()
	if err := temp.Chmod(catalogFilePerm); err != nil {
		_ = temp.Close()
		return fmt.Errorf("secure catalog temporary file: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write catalog temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync catalog temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close catalog temporary file: %w", err)
	}
	if err := c.replaceFile(tempName, c.filePath); err != nil {
		return fmt.Errorf("replace catalog: %w", err)
	}
	keepTemp = false
	if err := os.Chmod(c.filePath, catalogFilePerm); err != nil {
		return fmt.Errorf("secure catalog file: %w", err)
	}
	return nil
}

func decodeCatalog(data []byte) (diskCatalog, error) {
	var disk diskCatalog
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&disk); err != nil {
		return disk, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return disk, errors.New("multiple JSON values")
		}
		return disk, err
	}
	return disk, nil
}

func stateFromDisk(disk diskCatalog) (map[string]pathEntry, map[string]map[string]string, error) {
	paths, err := pathsFromDisk(disk.Paths)
	if err != nil {
		return nil, nil, err
	}
	collections, err := collectionsFromDisk(disk.Collections)
	if err != nil {
		return nil, nil, err
	}
	return paths, collections, nil
}

func pathsFromDisk(items []diskPath) (map[string]pathEntry, error) {
	paths := make(map[string]pathEntry, len(items))
	for _, item := range items {
		clean, key, err := canonicalPath(item.Path)
		if err != nil {
			return nil, fmt.Errorf("path %q: %w", item.Path, err)
		}
		entry, exists := paths[key]
		if !exists {
			entry = pathEntry{path: clean, tags: make(map[string]struct{})}
		}
		if err := addDiskTags(entry.tags, item.Tags); err != nil {
			return nil, err
		}
		if len(entry.tags) > 0 {
			paths[key] = entry
		}
	}
	return paths, nil
}

func addDiskTags(tags map[string]struct{}, diskTags []string) error {
	for _, tag := range diskTags {
		if err := validateName(tag); err != nil {
			return fmt.Errorf("tag %q: %w", tag, err)
		}
		tags[tag] = struct{}{}
	}
	return nil
}

func collectionsFromDisk(diskCollections []diskCollection) (map[string]map[string]string, error) {
	collections := make(map[string]map[string]string, len(diskCollections))
	for _, collection := range diskCollections {
		if err := validateName(collection.Name); err != nil {
			return nil, fmt.Errorf("collection %q: %w", collection.Name, err)
		}
		if _, exists := collections[collection.Name]; exists {
			return nil, fmt.Errorf("duplicate collection %q", collection.Name)
		}
		items, err := collectionPathsFromDisk(collection)
		if err != nil {
			return nil, err
		}
		collections[collection.Name] = items
	}
	return collections, nil
}

func collectionPathsFromDisk(collection diskCollection) (map[string]string, error) {
	items := make(map[string]string, len(collection.Paths))
	for _, path := range collection.Paths {
		clean, key, err := canonicalPath(path)
		if err != nil {
			return nil, fmt.Errorf("collection %q path %q: %w", collection.Name, path, err)
		}
		if _, exists := items[key]; !exists {
			items[key] = clean
		}
	}
	return items, nil
}

func (c *Catalog) snapshot() diskCatalog {
	c.mu.RLock()
	defer c.mu.RUnlock()
	disk := diskCatalog{
		Version:     CurrentVersion,
		Paths:       make([]diskPath, 0, len(c.paths)),
		Collections: make([]diskCollection, 0, len(c.collections)),
	}
	for _, entry := range c.paths {
		disk.Paths = append(disk.Paths, diskPath{Path: entry.path, Tags: sortedSet(entry.tags)})
	}
	sort.Slice(disk.Paths, func(i, j int) bool {
		_, left, _ := canonicalPath(disk.Paths[i].Path)
		_, right, _ := canonicalPath(disk.Paths[j].Path)
		if left == right {
			return disk.Paths[i].Path < disk.Paths[j].Path
		}
		return left < right
	})
	for name, items := range c.collections {
		paths := make([]string, 0, len(items))
		for _, path := range items {
			paths = append(paths, path)
		}
		sortPathStrings(paths)
		disk.Collections = append(disk.Collections, diskCollection{Name: name, Paths: paths})
	}
	sort.Slice(disk.Collections, func(i, j int) bool { return disk.Collections[i].Name < disk.Collections[j].Name })
	return disk
}

func atomicReplace(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
