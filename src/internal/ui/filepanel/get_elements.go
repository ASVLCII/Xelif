package filepanel

import (
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"
)

// TODO : Take common.Config.CaseSensitiveSort as a function parameter
// and also consider testing this caseSensitive with both true and false in
// our unit_test TestReturnDirElement
// getDirectoryElements returns the directory elements for the panel's current location
func (m *Model) getDirectoryElements(displayDotFile bool) []Element {
	dirEntries, err := os.ReadDir(m.Location)
	if err != nil {
		slog.Error("Error while returning folder elements", "error", err)
		return nil
	}

	dirEntries = slices.DeleteFunc(dirEntries, func(e os.DirEntry) bool {
		// Entries not needed to be considered
		_, err := e.Info()
		return err != nil || (strings.HasPrefix(e.Name(), ".") && !displayDotFile)
	})

	// No files/directories to process
	if len(dirEntries) == 0 {
		return nil
	}
	return sortFileElement(m.SortKind, m.SortReversed, dirEntries, m.Location)
}

// getDirectoryElementsBySearch returns filtered directory elements based on search string
func (m *Model) getDirectoryElementsBySearch(displayDotFile bool) []Element {
	searchString := m.SearchBar.Value()
	items, err := os.ReadDir(m.Location)
	if err != nil {
		slog.Error("Error while return folder element function", "error", err)
		return nil
	}

	if len(items) == 0 {
		return nil
	}

	folderElementMap := map[string]os.DirEntry{}
	fileAndDirectories := []string{}

	for _, item := range items {
		fileInfo, err := item.Info()
		if err != nil {
			continue
		}
		if !displayDotFile && strings.HasPrefix(fileInfo.Name(), ".") {
			continue
		}

		fileAndDirectories = append(fileAndDirectories, item.Name())
		folderElementMap[item.Name()] = item
	}
	if len(fileAndDirectories) == 0 {
		return nil
	}
	// https://github.com/reinhrst/fzf-lib/blob/main/core.go#L43
	// fzf returns matches ordered by score; we subsequently sort by the chosen sort option.
	fzfResults := utils.FzfSearch(searchString, fileAndDirectories)
	dirElements := make([]os.DirEntry, 0, len(fzfResults))
	for _, item := range fzfResults {
		resultItem := folderElementMap[item.Key]
		dirElements = append(dirElements, resultItem)
	}

	return sortFileElement(m.SortKind, m.SortReversed, dirElements, m.Location)
}

// Helper to decide whether to skip updating a panel this tick.
func (m *Model) shouldSkipPanelUpdate(nowTime time.Time) bool {
	if m.LastTimeGetElement.IsZero() || m.TargetFile != "" {
		return false
	}
	if !m.IsFocused {
		return nowTime.Sub(m.LastTimeGetElement) < nonFocussedPanelReRenderTime
	}

	// A loaded empty directory (or a search with no matches) is stable too.
	// Changes to location, query and sort inputs are checked separately below.
	if !m.Empty() && m.NeedsReRender() {
		return false
	}
	reRenderTime := max(1, min(m.ElemCount()/ReRenderChunkDivisor, ReRenderMaxDelay))
	return nowTime.Sub(m.LastTimeGetElement) < time.Duration(reRenderTime)*time.Second
}

func (m *Model) UpdateElementsIfNeeded(force bool, displayDotFile bool) {
	nowTime := time.Now()
	inputs := panelElementInputs{
		location:          m.Location,
		search:            m.SearchBar.Value(),
		sortKind:          m.SortKind,
		sortReversed:      m.SortReversed,
		displayDotFile:    displayDotFile,
		caseSensitiveSort: common.Config.CaseSensitiveSort,
	}
	if force || inputs != m.lastElementInputs || !m.shouldSkipPanelUpdate(nowTime) {
		// Load elements for this panel (with/without search filter)
		m.element = m.getElements(displayDotFile)
		m.clearMissingSelectionAnchor()
		// Update file panel list
		m.LastTimeGetElement = nowTime
		m.lastElementInputs = inputs

		// For hover to file on first time loading
		if m.TargetFile != "" {
			m.applyTargetFileCursor()
		}

		// If cursor becomes invalid due to element update, reset
		if m.ValidateCursorAndRenderIndex() != nil {
			m.scrollToCursor(0)
		}
	}
}

// Retrieves elements for a panel based on search bar value and sort options.
func (m *Model) getElements(displayDotFile bool) []Element {
	if m.SearchBar.Value() != "" {
		return m.getDirectoryElementsBySearch(displayDotFile)
	}
	return m.getDirectoryElements(displayDotFile)
}
