package globalsearch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

type countedInfoEntry struct {
	name      string
	infoCalls int
}

func (e *countedInfoEntry) Name() string      { return e.name }
func (e *countedInfoEntry) IsDir() bool       { return false }
func (e *countedInfoEntry) Type() fs.FileMode { return 0 }
func (e *countedInfoEntry) Info() (fs.FileInfo, error) {
	e.infoCalls++
	return nil, fs.ErrPermission
}

func TestLocalSearchRejectsNamesBeforeMetadataAndHonorsVisitLimit(t *testing.T) {
	var response Response
	walker := localSearchWalker{
		ctx:      context.Background(),
		root:     t.TempDir(),
		query:    Query{Terms: []string{"needle"}},
		options:  normalizeLocalSearchOptions(LocalSearchOptions{VisitLimit: 1}),
		response: &response,
	}
	entry := &countedInfoEntry{name: "haystack"}
	err := walker.visit(filepath.Join(walker.root, entry.name), entry, nil)
	if !errors.Is(err, errSearchLimitReached) || walker.visited != 1 {
		t.Fatalf("visit limit was bypassed: err=%v visited=%d", err, walker.visited)
	}
	if entry.infoCalls != 0 || walker.skippedErrors != 0 {
		t.Fatalf("nonmatching name required metadata: calls=%d errors=%d", entry.infoCalls, walker.skippedErrors)
	}

	matching := &countedInfoEntry{name: "needle"}
	err = walker.visit(filepath.Join(walker.root, matching.name), matching, nil)
	if !errors.Is(err, errSearchLimitReached) || matching.infoCalls != 1 || walker.skippedErrors != 1 {
		t.Fatalf(
			"matching metadata errors were lost: err=%v calls=%d errors=%d",
			err, matching.infoCalls, walker.skippedErrors,
		)
	}
}

func TestLocalSearchDescendsIntoNonmatchingDirectory(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "unrelated")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "needle.txt")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	response := SearchLocalQuery(context.Background(), root, Query{Terms: []string{"needle"}}, LocalSearchOptions{})
	if response.Err != nil || response.Partial || len(response.Results) != 1 || response.Results[0].Path != target {
		t.Fatalf("nonmatching parent hid matching descendant: %#v", response)
	}
}

func BenchmarkLocalSearchSelectiveName(b *testing.B) {
	root := b.TempDir()
	for index := range 10000 {
		name := fmt.Sprintf("haystack-%05d.txt", index)
		if index%1000 == 0 {
			name = fmt.Sprintf("needle-%05d.txt", index)
		}
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o600); err != nil {
			b.Fatal(err)
		}
	}
	query := Query{Terms: []string{"needle"}}
	options := LocalSearchOptions{ResultLimit: 100, VisitLimit: 20000}
	b.ReportAllocs()
	for b.Loop() {
		result := SearchLocalQuery(context.Background(), root, query, options)
		if result.Err != nil || result.Partial || len(result.Results) != 10 {
			b.Fatalf("unexpected result: %#v", result)
		}
	}
}
