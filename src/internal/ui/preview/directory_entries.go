package preview

import (
	"container/heap"
	"errors"
	"io"
	"io/fs"
	"os"
	"slices"
)

const directoryPreviewBatchSize = 256

// directoryPreviewEntries selects only the visible prefix of directories-first,
// name order. Batched unsorted reads avoid retaining and sorting an entire large
// directory just to display a few rows. At most one batch plus limit entries are
// retained, independent of the directory's size.
func directoryPreviewEntries(path string, limit int) ([]fs.DirEntry, error) {
	directory, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	// Retain one entry even for a zero-height viewport so a nonempty directory
	// remains distinguishable from an empty one, as in the full-directory read.
	limit = max(1, limit)
	entries := previewEntryHeap{}
	for {
		batch, err := directory.ReadDir(directoryPreviewBatchSize)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		for _, entry := range batch {
			if len(entries) < limit {
				heap.Push(&entries, entry)
			} else if comparePreviewEntries(entry, entries[0]) < 0 {
				entries[0] = entry
				heap.Fix(&entries, 0)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	slices.SortFunc(entries, comparePreviewEntries)
	return entries, nil
}

func comparePreviewEntries(a, b fs.DirEntry) int {
	if a.IsDir() != b.IsDir() {
		if a.IsDir() {
			return -1
		}
		return 1
	}
	if a.Name() < b.Name() {
		return -1
	}
	if a.Name() > b.Name() {
		return 1
	}
	return 0
}

// The worst retained entry is at the root, so smaller candidates replace it.
type previewEntryHeap []fs.DirEntry

func (h *previewEntryHeap) Len() int           { return len(*h) }
func (h *previewEntryHeap) Less(i, j int) bool { return comparePreviewEntries((*h)[i], (*h)[j]) > 0 }
func (h *previewEntryHeap) Swap(i, j int)      { (*h)[i], (*h)[j] = (*h)[j], (*h)[i] }
func (h *previewEntryHeap) Push(value any) {
	//nolint:errcheck // container/heap's untyped interface receives only fs.DirEntry values from directoryPreviewEntries.
	*h = append(*h, value.(fs.DirEntry))
}
func (h *previewEntryHeap) Pop() any {
	last := len(*h) - 1
	entry := (*h)[last]
	(*h)[last] = nil
	*h = (*h)[:last]
	return entry
}
