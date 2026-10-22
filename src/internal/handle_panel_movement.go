package internal

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/pkg/utils"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/internal/archivebrowser"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
)

// Back to parent directory
func (m *model) parentDirectory() {
	if err := m.updateCurrentFilePanelDir(".."); err != nil {
		m.showUserError("Could not open parent folder", err)
	}
}

// Enter directory or open file with default application
// TODO: Unit test this
func (m *model) enterPanel() {
	panel := m.getFocusedFilePanel()

	if panel.Empty() {
		return
	}
	selectedItem := panel.GetFocusedItem()
	if variable.ChooserFile != "" && (!selectedItem.Directory || panel.SelectedCount() > 0) {
		if err := m.chooserFileWriteAndQuit(m.chooserPaths()...); err == nil {
			return
		} else {
			slog.Error("Error while writing to chooser file, continuing with file open", "error", err)
		}
	}
	if selectedItem.Directory {
		m.enterDirectoryItem(selectedItem)
		return
	}

	if archivebrowser.Supported(selectedItem.Location) {
		if err := m.openArchive(selectedItem.Location); err != nil {
			m.showUserError("Could not browse archive", err)
		}
		return
	}
	m.executeOpenCommand()
}

func (m *model) enterDirectoryItem(selectedItem filepanel.Element) {
	targetPath := selectedItem.Location
	if selectedItem.Info.Mode()&os.ModeSymlink != 0 {
		var symlinkErr error
		targetPath, symlinkErr = filepath.EvalSymlinks(targetPath)
		if symlinkErr != nil {
			return
		}
		// The resolved target is no longer a link; retain the directory check.
		if targetInfo, lstatErr := os.Lstat(targetPath); lstatErr != nil || !targetInfo.IsDir() {
			return
		}
	}
	if err := m.updateCurrentFilePanelDir(targetPath); err != nil {
		slog.Error("Error while changing to directory", "error", err, "target", targetPath)
	}
}

func (m *model) executeOpenCommand() {
	panel := m.getFocusedFilePanel()
	filePath := panel.GetFocusedItem().Location
	if err := m.executeOpenPath(filePath); err != nil {
		m.showUserError("Could not open item", err)
	}
}

func (m *model) executeOpenPath(filePath string) error {
	openCommand := "xdg-open"
	switch runtime.GOOS {
	case utils.OsDarwin:
		openCommand = "open"
	case utils.OsWindows:
		dllpath := filepath.Join(os.Getenv("SYSTEMROOT"), "System32", "rundll32.exe")
		dllfile := "url.dll,FileProtocolHandler"

		//nolint:gosec // Uses Windows system handler to open the selected file.
		cmd := exec.Command(dllpath, dllfile, filePath)
		return cmd.Start()
	}

	// For now open_with works only for mac and linux
	// TODO: Make it in parity with windows.
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))
	if extEditor, ok := common.Config.OpenWith[ext]; ok {
		openCommand = extEditor
	}

	cmd := exec.Command(openCommand, filePath)
	utils.DetachFromTerminal(cmd)
	return cmd.Start()
}

// Switch to the directory where the sidebar cursor is located
func (m *model) sidebarSelectDirectory() {
	// We can't do this when we have only divider directories
	// m.sidebarModel.directories[m.sidebarModel.cursor].location would point to a divider dir.
	if m.sidebarModel.NoActualDir() {
		return
	}
	// TODO(Refactor): Move this to a function m.ResetFocus()
	m.focusPanel = nonePanelFocus
	panel := m.getFocusedFilePanel()

	err := m.updateCurrentFilePanelDir(m.sidebarModel.GetCurrentDirectoryLocation())
	if err != nil {
		slog.Error("Error switching to sidebar directory", "error", err)
	}
	panel.IsFocused = true
}

// Toggle dotfile display or not
func (m *model) toggleDotFileController() {
	m.fileModel.ToggleDotFile()
	err := utils.WriteBoolFile(variable.ToggleDotFile, m.fileModel.DisplayDotFiles)
	if err != nil {
		slog.Error("Error while updating toggleDotFile data", "error", err)
	}
}

// Toggle dotfile display or not
func (m *model) toggleFooterController() tea.Cmd {
	m.toggleFooter = !m.toggleFooter
	err := utils.WriteBoolFile(variable.ToggleFooter, m.toggleFooter)
	if err != nil {
		slog.Error("Error while updating toggleFooter data", "error", err)
	}
	m.setHeightValues()
	return m.updateComponentDimensions()
}

// Focus on search bar
func (m *model) searchBarFocus() {
	panel := m.getFocusedFilePanel()
	if panel.SearchBar.Focused() {
		panel.SearchBar.Blur()
	} else {
		panel.SearchBar.Focus()
		m.firstTextInput = true
	}

	// config search bar width
	panel.SearchBar.SetWidth(m.fileModel.SinglePanelWidth - common.InnerPadding)
}

func (m *model) sidebarSearchBarFocus() {
	if m.sidebarModel.SearchBarFocused() {
		// Ideally Code should never reach here. Once sidebar is focused, we should
		// not cause sidebarSearchBarFocus() event by pressing search key
		slog.Error("sidebarSearchBarFocus() called on Focused sidebar")
		m.sidebarModel.SearchBarBlur()
		return
	}
	m.sidebarModel.SearchBarFocus()
	m.firstTextInput = true
}
