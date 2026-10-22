// Package batchrename plans and applies explicit, collision-safe rename batches.
package batchrename

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

var (
	ErrNoTransformation         = errors.New("batch rename has no transformation")
	ErrNoChanges                = errors.New("batch rename would not change any names")
	ErrDuplicateSource          = errors.New("batch rename contains a duplicate source")
	ErrDestinationExists        = errors.New("batch rename destination already exists")
	ErrDestinationConflict      = errors.New("batch rename destinations conflict")
	ErrSourceDestinationOverlap = errors.New("batch rename destination overlaps a source")
)

// Rule applies a literal replacement followed by optional prefix and suffix.
// An empty Find skips replacement; at least one field must transform the name.
type Rule struct {
	Find    string
	Replace string
	Prefix  string
	Suffix  string
}

// Entry is one immutable source/destination pair.
type Entry struct {
	Source      string
	Destination string
}

// Plan is a validated batch. Entries are sorted by source for deterministic previews.
type Plan struct{ Entries []Entry }

// NewPlan validates sources and calculates destinations without changing disk state.
func NewPlan(paths []string, rule Rule) (Plan, error) {
	if rule.Find == "" && rule.Prefix == "" && rule.Suffix == "" {
		return Plan{}, ErrNoTransformation
	}

	sourceKeys, entries, err := buildEntries(paths, rule)
	if err != nil {
		return Plan{}, err
	}
	if len(entries) == 0 {
		return Plan{}, ErrNoChanges
	}
	if err := validateDestinations(entries, sourceKeys); err != nil {
		return Plan{}, err
	}
	slices.SortFunc(entries, func(a, b Entry) int { return strings.Compare(a.Source, b.Source) })
	return Plan{Entries: entries}, nil
}

func buildEntries(paths []string, rule Rule) (map[string]string, []Entry, error) {
	sourceKeys := make(map[string]string, len(paths))
	entries := make([]Entry, 0, len(paths))
	for _, raw := range paths {
		source, info, err := inspectSource(raw)
		if err != nil {
			return nil, nil, err
		}
		key := pathKey(source)
		if previous, exists := sourceKeys[key]; exists {
			return nil, nil, fmt.Errorf("%w: %q and %q", ErrDuplicateSource, previous, source)
		}
		sourceKeys[key] = source

		name := transformedName(info.Name(), rule)
		if err := validateName(name); err != nil {
			return nil, nil, fmt.Errorf("rename %q: %w", source, err)
		}
		destination := filepath.Join(filepath.Dir(source), name)
		if pathKey(destination) != key {
			entries = append(entries, Entry{Source: source, Destination: destination})
		}
	}
	return sourceKeys, entries, nil
}

func inspectSource(raw string) (string, os.FileInfo, error) {
	source, err := filepath.Abs(filepath.Clean(raw))
	if err != nil {
		return "", nil, fmt.Errorf("normalize source %q: %w", raw, err)
	}
	info, err := os.Lstat(source)
	if err != nil {
		return "", nil, fmt.Errorf("inspect source %q: %w", source, err)
	}
	if info.Name() == "." || info.Name() == ".." {
		return "", nil, fmt.Errorf("invalid source name %q", info.Name())
	}
	return source, info, nil
}

func transformedName(name string, rule Rule) string {
	if rule.Find != "" {
		name = strings.ReplaceAll(name, rule.Find, rule.Replace)
	}
	return rule.Prefix + name + rule.Suffix
}

func validateDestinations(entries []Entry, sourceKeys map[string]string) error {
	destinationKeys := make(map[string]string, len(entries))
	for _, entry := range entries {
		key := pathKey(entry.Destination)
		if previous, exists := destinationKeys[key]; exists {
			return fmt.Errorf("%w: %q and %q target %q",
				ErrDestinationConflict, previous, entry.Source, entry.Destination)
		}
		destinationKeys[key] = entry.Source
		if source, overlaps := sourceKeys[key]; overlaps {
			return fmt.Errorf("%w: %q targets source %q",
				ErrSourceDestinationOverlap, entry.Source, source)
		}
		if _, err := os.Lstat(entry.Destination); err == nil {
			return fmt.Errorf("%w: %q", ErrDestinationExists, entry.Destination)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect destination %q: %w", entry.Destination, err)
		}
	}
	return nil
}

// Renamer must move source to destination without overwriting destination.
type Renamer func(source, destination string) error

// Apply executes a validated plan and rolls back completed entries on failure.
func Apply(ctx context.Context, plan Plan, rename Renamer) error {
	if ctx == nil {
		return errors.New("batch rename: nil context")
	}
	if rename == nil {
		return errors.New("batch rename: nil renamer")
	}
	completed := make([]Entry, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		if err := ctx.Err(); err != nil {
			return rollback(completed, rename, fmt.Errorf("batch rename cancelled: %w", err))
		}
		if err := rename(entry.Source, entry.Destination); err != nil {
			return rollback(completed, rename,
				fmt.Errorf("rename %q to %q: %w", entry.Source, entry.Destination, err))
		}
		completed = append(completed, entry)
	}
	return nil
}

func rollback(completed []Entry, rename Renamer, cause error) error {
	errs := []error{cause}
	for index := len(completed) - 1; index >= 0; index-- {
		entry := completed[index]
		if err := rename(entry.Destination, entry.Source); err != nil {
			errs = append(errs, fmt.Errorf("rollback %q to %q: %w", entry.Destination, entry.Source, err))
		}
	}
	return errors.Join(errs...)
}

func validateName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return fmt.Errorf("invalid destination name %q", name)
	}
	if strings.ContainsRune(name, 0) {
		return errors.New("destination name contains NUL")
	}
	return nil
}

func pathKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}
