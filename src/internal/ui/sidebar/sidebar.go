package sidebar

import (
	"errors"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/internal/common"
)

// PinnedItemRename initiates the rename process for the currently selected pinned directory.
func (s *Model) PinnedItemRename() {
	pinnedBegin, pinnedEnd := s.pinnedIndexRange()
	// We have not selected a pinned directory, rename is not allowed
	if s.cursor < pinnedBegin || s.cursor > pinnedEnd {
		return
	}

	nameLen := len(s.directories[s.cursor].Name)
	cursorPos := nameLen

	s.renaming = true
	s.rename = common.GeneratePinnedRenameTextInput(cursorPos, s.directories[s.cursor].Name)
}

// CancelSidebarRename aborts the rename process for a pinned directory.
func (s *Model) CancelSidebarRename() {
	s.rename.Blur()
	s.renaming = false
}

// ConfirmSidebarRename finalizes the rename process and saves changes to the pinned directories file.
func (s *Model) ConfirmSidebarRename() error {
	s.refreshDirty = true
	itemLocation := s.directories[s.cursor].Location
	newItemName := s.rename.Value()

	// recover the state of rename
	s.CancelSidebarRename()

	if err := s.pinnedMgr.Rename(itemLocation, newItemName); err != nil {
		return err
	}
	s.directories[s.cursor].Name = newItemName
	return nil
}

// UpdateState handles the sidebar's state updates in response to Bubble Tea messages.
func (s *Model) UpdateState(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	if s.renaming {
		s.rename, cmd = s.rename.Update(msg)
	} else if s.searchBar.Focused() {
		s.searchBar, cmd = s.searchBar.Update(msg)
	}

	if s.cursor < 0 {
		s.cursor = 0
	}
	return cmd
}

// HandleSearchBarKey processes key events specifically for the sidebar's search bar.
func (s *Model) HandleSearchBarKey(msg string) {
	switch {
	case slices.Contains(common.Hotkeys.CancelTyping, msg):
		s.SearchBarBlur()
		s.searchBar.SetValue("")
	case slices.Contains(common.Hotkeys.ConfirmTyping, msg):
		s.SearchBarBlur()
		s.resetCursor()
	}
}

// UpdateDirectoriesIfNeeded reuses the current snapshot for unrelated input events.
// Search changes and in-app mutations are immediate; external changes are polled.
func (s *Model) UpdateDirectoriesIfNeeded() {
	s.updateDirectoriesIfNeeded(time.Now())
}

const directoryRefreshInterval = time.Second

func (s *Model) updateDirectoriesIfNeeded(now time.Time) {
	if s.Disabled() {
		return
	}
	if s.refreshDirty || s.lastRefresh.IsZero() || s.lastSearch != s.searchBar.Value() ||
		now.Sub(s.lastRefresh) >= directoryRefreshInterval {
		s.refreshDirectories(now)
	}
}

// UpdateDirectories explicitly refreshes, bypassing the routine input throttle.
func (s *Model) UpdateDirectories() {
	s.refreshDirectories(time.Now())
}

func (s *Model) refreshDirectories(now time.Time) {
	if s.Disabled() {
		return
	}
	selectedLocation := s.GetCurrentDirectoryLocation()
	previous := s.directories
	if s.searchBar.Value() != "" {
		s.directories = getFilteredDirectories(s.searchBar.Value(), s.pinnedMgr, s.recentMgr, s.sections)
	} else {
		s.directories = getDirectories(s.pinnedMgr, s.recentMgr, s.sections)
	}
	s.lastRefresh = now
	s.lastSearch = s.searchBar.Value()
	s.refreshDirty = false
	if slices.Equal(previous, s.directories) {
		return // input/timer refreshes must not undo independent wheel scrolling
	}
	if selectedLocation != "" {
		for i := range s.directories {
			if sameRecentPath(s.directories[i].Location, selectedLocation) {
				s.cursor = i
				s.updateRenderIndex()
				return
			}
		}
	}
	// This is needed, as due to filtering, the cursor might be invalid
	if s.isCursorInvalid() {
		s.resetCursor()
	}
}

// TogglePinnedDirectory adds or removes a directory from the pinned list.
func (s *Model) TogglePinnedDirectory(dir string) error {
	if s.pinnedMgr == nil {
		return errors.New("favourites storage is unavailable")
	}
	s.refreshDirty = true
	return s.pinnedMgr.Toggle(dir)
}

// AddRecent records a directory as the most recently visited folder.
func (s *Model) AddRecent(path string) error {
	if s.recentMgr == nil {
		return errors.New("recent directory manager is unavailable")
	}
	s.refreshDirty = true
	return s.recentMgr.Add(path)
}

// New initializes and returns a new Model for the sidebar correctly set up with configuration.
func New() Model {
	recentMgr := NewRecentFileManager(variable.RecentFile)
	if common.Config.SidebarWidth == 0 {
		return Model{
			disabled:  true,
			recentMgr: &recentMgr,
		}
	}
	// pinnedMgr is created here, can be done higher up in the call chain
	pinnedMgr := NewPinnedFileManager(variable.PinnedFile)
	s := Model{
		renderIndex: 0,
		searchBar:   common.GenerateSearchBar(),
		pinnedMgr:   &pinnedMgr,
		recentMgr:   &recentMgr,
		width:       common.Config.SidebarWidth + common.BorderPadding,
		height:      minHeight,
		disabled:    false,
		sections:    common.Config.SidebarSections,
	}

	s.UpdateDirectories()
	s.searchBar.SetWidth(s.width - common.BorderPadding - searchBarPadding)
	s.searchBar.Placeholder = "Search"
	return s
}
