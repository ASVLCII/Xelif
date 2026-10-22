// Package savedsearch persists small, named Search Everywhere queries.
package savedsearch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const (
	currentVersion  = 1
	maxQueryBytes   = 4096
	privateFileMode = 0o600
)

var (
	ErrNotFound     = errors.New("saved search not found")
	ErrInvalidName  = errors.New("invalid saved search name")
	ErrInvalidQuery = errors.New("invalid saved search query")
	ErrCorrupt      = errors.New("saved search store is corrupt")
	ErrVersion      = errors.New("unsupported saved search version")
)

type Search struct {
	Name  string `json:"name"`
	Query string `json:"query"`
}

type document struct {
	Version  int      `json:"version"`
	Searches []Search `json:"searches"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Store { return &Store{path: path} }

func (s *Store) List() ([]Search, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Store) Get(name string) (Search, error) {
	if err := validateName(name); err != nil {
		return Search{}, err
	}
	searches, err := s.List()
	if err != nil {
		return Search{}, err
	}
	for _, search := range searches {
		if strings.EqualFold(search.Name, name) {
			return search, nil
		}
	}
	return Search{}, fmt.Errorf("%w: %q", ErrNotFound, name)
}

func (s *Store) Save(name, query string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if err := validateQuery(query); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	searches, err := s.load()
	if err != nil {
		return err
	}
	replaced := false
	for index := range searches {
		if strings.EqualFold(searches[index].Name, name) {
			searches[index] = Search{Name: name, Query: query}
			replaced = true
			break
		}
	}
	if !replaced {
		searches = append(searches, Search{Name: name, Query: query})
	}
	return s.write(searches)
}

func (s *Store) Delete(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	searches, err := s.load()
	if err != nil {
		return err
	}
	index := slices.IndexFunc(searches, func(search Search) bool { return strings.EqualFold(search.Name, name) })
	if index < 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	searches = append(searches[:index], searches[index+1:]...)
	return s.write(searches)
}

func (s *Store) load() ([]Search, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []Search{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read saved searches: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var doc document
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing JSON", ErrCorrupt)
	}
	if doc.Version != currentVersion {
		return nil, fmt.Errorf("%w: %d", ErrVersion, doc.Version)
	}
	seen := make(map[string]struct{}, len(doc.Searches))
	for _, search := range doc.Searches {
		if err := validateName(search.Name); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrCorrupt, err)
		}
		if err := validateQuery(search.Query); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrCorrupt, err)
		}
		key := strings.ToLower(search.Name)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("%w: duplicate name %q", ErrCorrupt, search.Name)
		}
		seen[key] = struct{}{}
	}
	slices.SortFunc(doc.Searches, func(a, b Search) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return slices.Clone(doc.Searches), nil
}

func (s *Store) write(searches []Search) error {
	slices.SortFunc(searches, func(a, b Search) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	data, err := json.Marshal(document{Version: currentVersion, Searches: searches})
	if err != nil {
		return fmt.Errorf("encode saved searches: %w", err)
	}
	data = append(data, '\n')
	directory := filepath.Dir(s.path)
	if makeErr := os.MkdirAll(directory, 0o700); makeErr != nil {
		return fmt.Errorf("create saved search directory: %w", makeErr)
	}
	temporary, err := os.CreateTemp(directory, ".saved-searches.tmp-*")
	if err != nil {
		return fmt.Errorf("create saved search temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(privateFileMode); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("replace saved searches: %w", err)
	}
	return nil
}

func validateName(name string) error {
	if name == "" || name != strings.TrimSpace(name) || !utf8.ValidString(name) || len(name) > 128 {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	for _, character := range name {
		if unicode.IsControl(character) || strings.ContainsRune(`/\\:*?"<>|`, character) {
			return fmt.Errorf("%w: %q", ErrInvalidName, name)
		}
	}
	return nil
}

func validateQuery(query string) error {
	if strings.TrimSpace(query) == "" || !utf8.ValidString(query) || len(query) > maxQueryBytes {
		return ErrInvalidQuery
	}
	return nil
}
